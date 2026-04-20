# MorseRunner-Go: API & Integration Contract

This document serves as the formal specification for agents building Web Frontends or AI Decoders for the MorseRunner-Go engine.

## 1. CLI IPC (Unix Domain Socket)
**Path**: `/tmp/morserunner.sock` (Default)
**Format**: Line-delimited strings.

### Commands (Inbound to Engine)
- `wpm <n>`: Set global CW speed (15-50).
- `pileup <n>`: Start a pile-up with N stations.
- `tx <text>`: Transmit Morse code. Use `CQ` to trigger pile-up response.
- `rit <hz>`: Set Receiver Incremental Tuning.
- `bw <hz>`: Set filter bandwidth (affects noise floor).
- `stop`: Halt all active stations and test tones.

## 2. Web State Broadcast (WebSocket)
**Path**: `ws://localhost:8080/ws`
**Format**: JSON

### State Packet Schema
```json
{
  "type": "state",
  "wpm": 30,
  "pitch": 600,
  "noise": 0.05,
  "bw": 500,
  "score": 1200,
  "qsos": 5,
  "log": [...],
  "stations": [
    {
      "call": "K7ABC",
      "bfo": 0.023,
      "state": 3
    }
  ]
}
```
*Note: `state` 3 = `StSending`.*

## 3. Behavioral Nuances (For AI Agents)
- **Best Match**: The engine only replies if your `tx` callsign is the "Best Match" (highest confidence) for an active station.
- **Tail-Ending**: 5% chance of stations jumping in after a `TU` command is heard.
- **WPM Drift**: AI operators have a small random speed drift (+/- 2 WPM) over time for realism.

## 4. Audio Format
- **Sample Rate**: 16,000 Hz
- **Format**: 16-bit Signed PCM, Mono, Little-Endian.
