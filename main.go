package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/tierney/morserunner-go/pkg/audio"
	"github.com/tierney/morserunner-go/pkg/engine"
	"github.com/tierney/morserunner-go/pkg/web"
)

func main() {
	headlessFlag := flag.Bool("headless", false, "Run without interactive REPL")
	webFlag := flag.Bool("web", false, "Enable web dashboard")
	jsonLogs := flag.Bool("json-logs", false, "Output logs in JSON format")
	logFile := flag.String("log-file", "morserunner.log", "Path to log file")

	// Initialize structured logging

	rate := 16000
	blockSize := 512

	wpmFlag := flag.Int("wpm", 30, "CW speed (15-50)")
	pitchFlag := flag.Float64("pitch", 600.0, "Sidetone pitch (Hz)")
	bwFlag := flag.Float64("bw", 500.0, "Filter bandwidth (Hz)")
	noiseFlag := flag.Float64("noise", 0.05, "Background noise level")
	qrmFlag := flag.Float64("qrm", 0.0, "QRM interference level")
	qsbFlag := flag.Bool("qsb", false, "Enable QSB (fading)")
	flutterFlag := flag.Bool("flutter", false, "Enable Flutter (Aurora)")
	lidsFlag := flag.Bool("lids", false, "Enable LIDs mode")
	contestFlag := flag.String("contest", "WPX", "Contest type (WPX, ARRLDX, POTA)")
	parkFlag := flag.String("park", "K-1234", "Park ID for POTA")
	socketFlag := flag.String("socket", "/tmp/morserunner.sock", "IPC socket path")

	flag.Parse()

	f, err := os.OpenFile(*logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer f.Close()

	var logHandler slog.Handler
	if *jsonLogs {
		logHandler = slog.NewJSONHandler(io.MultiWriter(os.Stderr, f), nil)
	} else {
		logHandler = slog.NewTextHandler(io.MultiWriter(os.Stderr, f), nil)
	}
	slog.SetDefault(slog.New(logHandler))

	slog.Info("MorseRunner-Go Engine Initializing", "target", "macOS (M4 Pro) / Linux")

	driver, err := audio.NewDriver(rate)
	if err != nil {
		log.Fatal(err)
	}
	defer driver.Close()

	sidecar := audio.NewSidecarServer(*socketFlag)
	if err := sidecar.Start(); err != nil {
		log.Printf("Failed to start sidecar: %v", err)
	}
	defer sidecar.Close()

	var webServer *web.Server
	if *webFlag {
		webServer = web.NewServer()
	}

	c := engine.NewContest(rate)
	registry := engine.DefaultRegistry()

	// Apply flags to engine
	c.MyWpm = *wpmFlag
	c.Keyer.SetWpm(*wpmFlag, *wpmFlag)
	c.Mixer.Pitch = *pitchFlag
	c.Mixer.UpdateFilter(*bwFlag)
	c.NoiseLevel = *noiseFlag
	c.QRMLevel = *qrmFlag
	c.QSBEnabled = *qsbFlag
	c.FlutterEnabled = *flutterFlag
	c.LIDs = *lidsFlag

	switch strings.ToUpper(*contestFlag) {
	case "POTA":
		c.Rules = &engine.POTARules{ParkID: *parkFlag}
	case "ARRLDX":
		c.Rules = &engine.ARRLDXRules{}
	default:
		c.Rules = &engine.WPXRules{}
	}

	if webServer != nil {
		webServer.CmdHandler = func(cmd string, params map[string]interface{}) {
			args := []string{}
			if v, ok := params["value"]; ok {
				args = append(args, fmt.Sprintf("%v", v))
			}

			if err := registry.ExecuteCommand(c, cmd, args); err != nil {
				slog.Error("Web: Command execution failed", "cmd", cmd, "error", err)
			} else {
				slog.Info("Web: Command executed", "cmd", cmd, "args", args)
			}
		}
		webServer.Start(":8080")
		defer func() {
			sdCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			webServer.Shutdown(sdCtx)
		}()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		slog.Info("Shutdown signal received", "signal", sig)
		cancel()
	}()

	pr, pw := io.Pipe()

	// Engine loop
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				samples := c.NextBlock(blockSize)
				pcm := audio.Float32ToS16(samples)
				pw.Write(pcm)
				sidecar.Broadcast(pcm)
				if webServer != nil {
					webServer.BroadcastAudio(pcm)
				}
			}
		}
	}()

	// State broadcast loop
	if webServer != nil {
		go func() {
			ticker := time.NewTicker(200 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					state := map[string]interface{}{
						"type":  "state",
						"wpm":   c.MyWpm,
						"pitch": c.Mixer.Pitch,
						"noise": c.NoiseLevel,
						"bw":    c.Bandwidth,
						"score": c.Log.Score(),
						"qsos":  len(c.Log.Qsos),
						"log":   c.Log.Qsos,
					}

					stations := []map[string]interface{}{}
					for _, op := range c.Stations {
						stations = append(stations, map[string]interface{}{
							"call":  op.Station.Call,
							"bfo":   op.Station.Bfo,
							"state": op.Station.State,
						})
					}
					state["stations"] = stations
					webServer.BroadcastJSON(state)
				}
			}
		}()
	}

	// Audio output loop
	go func() {
		err = driver.Play(ctx, pr)
		if err != nil && err != context.Canceled {
			log.Fatal(err)
		}
	}()

	// Sidecar command loop
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case cmdLine, ok := <-sidecar.CommandChan:
				if !ok {
					return
				}
				if err := registry.Execute(c, cmdLine); err != nil {
					slog.Error("Sidecar: Command execution failed", "cmd", cmdLine, "error", err)
				}
			}
		}
	}()

	slog.Info("Engine running")

	if *headlessFlag {
		slog.Info("Headless mode active. Waiting for signals...")
		<-ctx.Done()
		return
	}

	fmt.Println("Commands: wpm <n>, pileup <n>, stop, exit")

	// CLI Loop (context-aware)
	inputChan := make(chan string)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			inputChan <- scanner.Text()
		}
	}()

	for {
		fmt.Print("> ")
		select {
		case <-ctx.Done():
			slog.Info("CLI: Context cancelled, exiting loop")
			return
		case input := <-inputChan:
			if input == "exit" || input == "quit" {
				cancel()
				return
			}
			if err := registry.Execute(c, input); err != nil {
				slog.Error("CLI: Command failed", "error", err)
			}
		}
	}
}
