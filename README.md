# MorseRunner Go Engine

A headless Go implementation of the core MorseRunner engine, optimized for macOS (M4 Pro) and Linux. This engine reproduces the Morse generation and competition logic from the original Delphi project, providing low-latency audio and an IPC sidecar for AI decoding.

## Core Features
- **Deterministic Timing**: Sample-accurate inter-element and inter-character spacing.
- **Soft Keying**: Blackman-Harris envelope generation (5ms rise/fall) to eliminate digital clicks.
- **Advanced DSP**:
    - **Mixer**: Complex baseband mixing with configurable Pitch.
    - **Filtering**: 31-tap Sinc FIR filter for adjustable Bandwidth.
    - **QSB**: Independent signal fading for each station.
    - **AGC**: Automatic Gain Control for consistent output levels.
- **AI Sidecar**: PCM stream output via Unix Domain Socket (`/tmp/morserunner.sock`) at 16kHz mono.
- **Headless CLI**: Real-time control of the engine state.

## Installation
Ensure you have Go installed, then:
```bash
go mod tidy
go build -o morserunner-engine main.go
```

## AI & Automated Usage (CLI Flags)
The engine supports command-line flags for easy integration with AI scripts or automated testing:
```bash
./morserunner-engine -wpm 40 -noise 0.1 -qsb -contest POTA -park K-1234 -headless
```
### Supported Flags:
- `-wpm <int>`: Initial CW speed (default 30).
- `-pitch <float>`: Side-tone frequency in Hz (default 600.0).
- `-bw <float>`: Filter bandwidth in Hz (default 500.0).
- `-noise <float>`: Background noise level (default 0.05).
- `-qrm <float>`: QRM interference level (default 0.0).
- `-qsb`: Enable signal fading (QSB).
- `-flutter`: Enable Aurora distortion (Flutter).
- `-lids`: Enable operator imperfections.
- `-contest <string>`: Contest type (WPX, ARRLDX, POTA).
- `-park <string>`: Park ID for POTA mode.
- `-socket <path>`: Path for the IPC PCM socket (default `/tmp/morserunner.sock`).
- `-headless`: Run without the interactive CLI prompt (wait for signals).

## CLI Commands (Interactive)
If running without `-headless`, use the following commands at the `> ` prompt:

### Advanced AI Realism
- **Competitive Matching**: Stations only reply if they are the "Best Match" for your transmission. Partial matches will defer to better ones.
- **CQ Support**: Stations intelligently respond to `CQ` calls.
- **Operator Imperfections (LIDs)**: Enable simulated operator errors and cut numbers.
- **DSP Modulators**: Native implementation of QSB (fading) and Flutter (Aurora) from original Delphi source.

## Testing
The engine includes a full test suite for timing and logic:
```bash
go test ./pkg/engine/...
```
Tested for **50 WPM** integrity and sub-sample Morse timing accuracy.

### Competition Logic
- `pileup <n>`: Start a pile-up with `<n>` stations. Stations will loop their callsigns every 3 seconds.
- `wpm <n>`: Set the CW speed (15-50 WPM).
- `pota <ParkID>`: Switch to Parks on the Air mode (e.g., `pota K-1234`).
- `stop`: Stop all active transmissions and clear the station list.
- `exit`: Gracefully shut down the engine.

### Radio & Environment Controls
- `noise <level>`: Set background white noise level (e.g., `0.05`).
- `qrm <level>`: Add wandering interference carriers (e.g., `0.1`).
- `pitch <Hz>`: Change the side-tone pitch (default `600`).
- `bw <Hz>`: Set the receiver bandwidth (e.g., `200` to `1000`).
- `rit <Hz>`: Offset the receiver tuning (Receiver Incremental Tuning).
- `test on/off`: Toggle a continuous test tone at the current pitch.
- `lids on/off`: Toggle simulated operator imperfections (LIDs mode).

## IPC Sidecar (AI Decoding)
The engine streams raw 16-bit PCM (16kHz, Mono) to `/tmp/morserunner.sock`. You can test the stream using `netcat` and `ffplay`:
```bash
nc -U /tmp/morserunner.sock | ffplay -f s16le -ar 16000 -ac 1 -
```

## Architecture
- `pkg/audio`: Handles the `Oto` driver and the Unix Domain Socket server.
- `pkg/engine`:
    - `Keyer`: Morse encoding and envelope generation.
    - `Mixer`: DSP effects, filtering, and modulation.
    - `Station`: State machine for CW stations.
    - `Contest`: Orchestration of the competition and pile-up logic.
