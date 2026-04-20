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
	"strconv"
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

	flag.Parse()

	// Initialize structured logging
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
			switch cmd {
			case "wpm":
				if v, ok := params["value"].(float64); ok {
					wpm := int(v)
					c.MyWpm = wpm
					c.Keyer.SetWpm(wpm, wpm)
					slog.Info("Web: WPM set", "value", wpm)
				}
			case "pitch":
				if v, ok := params["value"].(float64); ok {
					c.Mixer.Pitch = v
					slog.Info("Web: Pitch set", "value", v)
				}
			case "noise":
				if v, ok := params["value"].(float64); ok {
					c.NoiseLevel = v
					slog.Info("Web: Noise set", "value", v)
				}
			case "bw":
				if v, ok := params["value"].(float64); ok {
					c.Bandwidth = v
					c.Mixer.UpdateFilter(v)
					slog.Info("Web: Bandwidth set", "value", v)
				}
			case "pileup":
				count := 5
				if v, ok := params["value"].(float64); ok {
					count = int(v)
				}
				c.StartPileup(count)
				slog.Info("Web: Started pile-up", "count", count)
			case "tx":
				if v, ok := params["value"].(string); ok {
					c.ProcessUserTX(v)
					slog.Info("Web: TX sent", "msg", v)
				}
			case "stop":
				c.Stations = nil
				c.TestTone = false
				slog.Info("Web: Stopped all stations")
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
			case cmdLine := <-sidecar.CommandChan:
				handleCommand(cmdLine, c, cancel)
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
			handleCommand(input, c, cancel)
		}
	}
}

// handleCommand parses and executes a single CLI or IPC command string.
func handleCommand(input string, c *engine.Contest, cancel context.CancelFunc) {
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}
	parts := strings.Split(input, " ")
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "exit", "quit":
		cancel()
	case "wpm":
		if len(parts) > 1 {
			wpm, _ := strconv.Atoi(parts[1])
			if wpm >= 15 && wpm <= 50 {
				c.MyWpm = wpm
				c.Keyer.SetWpm(wpm, wpm)
				slog.Info("WPM set", "value", wpm)
			}
		}
	case "pileup":
		count := 5
		if len(parts) > 1 {
			count, _ = strconv.Atoi(parts[1])
		}
		c.StartPileup(count)
		slog.Info("Started pile-up", "count", count)
	case "noise":
		if len(parts) > 1 {
			level, _ := strconv.ParseFloat(parts[1], 64)
			c.NoiseLevel = level
			slog.Info("Noise level set", "value", level)
		}
	case "qrm":
		if len(parts) > 1 {
			level, _ := strconv.ParseFloat(parts[1], 64)
			c.QRMLevel = level
			slog.Info("QRM level set", "value", level)
		}
	case "test":
		if len(parts) > 1 {
			c.TestTone = (parts[1] == "on")
			slog.Info("Test tone updated", "enabled", c.TestTone)
		}
	case "pitch":
		if len(parts) > 1 {
			pitch, _ := strconv.ParseFloat(parts[1], 64)
			c.Mixer.Pitch = pitch
			slog.Info("Pitch set", "value", pitch)
		}
	case "bw":
		if len(parts) > 1 {
			bw, _ := strconv.ParseFloat(parts[1], 64)
			c.Bandwidth = bw
			c.Mixer.UpdateFilter(bw)
			slog.Info("Bandwidth set", "value", bw)
		}
	case "rit":
		if len(parts) > 1 {
			rit, _ := strconv.ParseFloat(parts[1], 64)
			c.RIT = rit
			slog.Info("RIT set", "value", rit)
		}
	case "tx":
		if len(parts) > 1 {
			msg := strings.Join(parts[1:], " ")
			c.ProcessUserTX(msg)
			slog.Info("TX sent", "msg", msg)
		}
	case "score":
		slog.Info("Current Stats",
			"qsos", len(c.Log.Qsos),
			"mults", c.Log.TotalMults(),
			"points", c.Log.TotalPoints(),
			"score", c.Log.Score())
	case "lids":
		if len(parts) > 1 {
			c.LIDs = (parts[1] == "on")
			slog.Info("LIDs mode updated", "enabled", c.LIDs)
		}
	case "pota":
		park := "K-1234"
		if len(parts) > 1 {
			park = parts[1]
		}
		c.Rules = &engine.POTARules{ParkID: park}
		slog.Info("Switched to POTA mode", "park", park)
	case "qsb":
		if len(parts) > 1 {
			c.QSBEnabled = (parts[1] == "on")
			slog.Info("QSB (Fading) updated", "enabled", c.QSBEnabled)
		}
	case "flutter":
		if len(parts) > 1 {
			c.FlutterEnabled = (parts[1] == "on")
			slog.Info("Flutter (Aurora) updated", "enabled", c.FlutterEnabled)
		}
	case "stop":
		c.Stations = nil
		c.TestTone = false
		slog.Info("Stopped all stations and test tone")
	default:
		slog.Warn("Unknown command", "input", input)
	}
}
