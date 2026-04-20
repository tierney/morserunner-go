package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/tierney/morserunner-go/pkg/audio"
	"github.com/tierney/morserunner-go/pkg/engine"
)

func main() {
	fmt.Println("MorseRunner-Go Engine Initializing...")
	fmt.Println("Target: macOS (M4 Pro) / Linux")

	rate := 16000
	blockSize := 512

	// AI-friendly CLI flags
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
	headlessFlag := flag.Bool("headless", false, "Run without interactive REPL")

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
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
			}
		}
	}()

	// Audio output loop
	go func() {
		err = driver.Play(ctx, pr)
		if err != nil && err != context.Canceled {
			log.Fatal(err)
		}
	}()

	fmt.Println("Engine running.")
	
	if *headlessFlag {
		fmt.Println("Headless mode active. Waiting for signals...")
		<-ctx.Done()
		return
	}

	fmt.Println("Commands: wpm <n>, pileup <n>, stop, exit")

	// CLI Loop
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		input := scanner.Text()
		parts := strings.Split(input, " ")
		if len(parts) == 0 {
			continue
		}

		cmd := strings.ToLower(parts[0])
		switch cmd {
		case "exit", "quit":
			cancel()
			return
		case "wpm":
			if len(parts) > 1 {
				wpm, _ := strconv.Atoi(parts[1])
				if wpm >= 15 && wpm <= 50 {
					c.MyWpm = wpm
					c.Keyer.SetWpm(wpm, wpm)
					fmt.Printf("WPM set to %d\n", wpm)
				}
			}
		case "pileup":
			count := 5
			if len(parts) > 1 {
				count, _ = strconv.Atoi(parts[1])
			}
			c.StartPileup(count)
			fmt.Printf("Started pile-up with %d stations\n", count)
		case "noise":
			if len(parts) > 1 {
				level, _ := strconv.ParseFloat(parts[1], 64)
				c.NoiseLevel = level
				fmt.Printf("Noise level set to %.2f\n", level)
			}
		case "qrm":
			if len(parts) > 1 {
				level, _ := strconv.ParseFloat(parts[1], 64)
				c.QRMLevel = level
				fmt.Printf("QRM level set to %.2f\n", level)
			}
		case "test":
			if len(parts) > 1 {
				c.TestTone = (parts[1] == "on")
				fmt.Printf("Test tone: %v\n", c.TestTone)
			}
		case "pitch":
			if len(parts) > 1 {
				pitch, _ := strconv.ParseFloat(parts[1], 64)
				c.Mixer.Pitch = pitch
				fmt.Printf("Pitch set to %.0f Hz\n", pitch)
			}
		case "bw":
			if len(parts) > 1 {
				bw, _ := strconv.ParseFloat(parts[1], 64)
				c.Bandwidth = bw
				c.Mixer.UpdateFilter(bw)
				fmt.Printf("Bandwidth set to %.0f Hz\n", bw)
			}
		case "rit":
			if len(parts) > 1 {
				rit, _ := strconv.ParseFloat(parts[1], 64)
				c.RIT = rit
				fmt.Printf("RIT set to %.0f Hz\n", rit)
			}
		case "tx":
			if len(parts) > 1 {
				msg := strings.Join(parts[1:], " ")
				c.ProcessUserTX(msg)
				fmt.Printf("TX: %s\n", msg)
			}
		case "score":
			fmt.Printf("QSOs: %d | Mults: %d | Points: %d | Total Score: %d\n",
				len(c.Log.Qsos), c.Log.TotalMults(), c.Log.TotalPoints(), c.Log.Score())
		case "lids":
			if len(parts) > 1 {
				c.LIDs = (parts[1] == "on")
				fmt.Printf("LIDs mode: %v\n", c.LIDs)
			}
		case "pota":
			park := "K-1234"
			if len(parts) > 1 {
				park = parts[1]
			}
			c.Rules = &engine.POTARules{ParkID: park}
			fmt.Printf("Switched to POTA mode. Park: %s\n", park)
		case "qsb":
			if len(parts) > 1 {
				c.QSBEnabled = (parts[1] == "on")
				fmt.Printf("QSB (Fading): %v\n", c.QSBEnabled)
			}
		case "flutter":
			if len(parts) > 1 {
				c.FlutterEnabled = (parts[1] == "on")
				fmt.Printf("Flutter (Aurora): %v\n", c.FlutterEnabled)
			}
		case "stop":
			c.Stations = nil
			c.TestTone = false
			fmt.Println("Stopped all stations and test tone.")
		default:
			fmt.Println("Unknown command.")
		}
	}
}
