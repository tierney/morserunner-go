# MorseRunner-Go AI Sidecar

The AI Sidecar is an autonomous decoding engine for `morserunner-go`. It uses a multi-tier approach to provide real-time decoding and automated contest interaction.

## Quick Start
1. **Setup Environment**:
   ```bash
   bash sidecar/setup_env.sh
   ```
2. **Build Engine**:
   ```bash
   go build -o morserunner-engine main.go
   ```
3. **Run Sidecar**:
   ```bash
   ./morserunner-engine -headless &
   .venv/bin/python sidecar/client.py --local-mode deepmorse
   ```

## Decoding Tiers

| Tier | Model | Latency | Use Case |
|------|-------|---------|----------|
| **Fast** | DeepMorse (Local) | ~1-5ms | Instant response to callsigns. |
| **Strategic** | Gemini 1.5 (Cloud) | ~1.5s | Error correction, handling QRM/QSB. |

## Benchmarking
The sidecar includes a full benchmarking suite to compare accuracy and latency across tiers.

```bash
# Generate real CW samples from the engine
python3 sidecar/generate_manifest_samples.py

# Run benchmark
.venv/bin/python sidecar/benchmark_suite.py --manifest sidecar/benchmarks/manifest.jsonl --tiers both
```

## Real-Time Performance
On the **Apple M4 Pro (MacBook Pro)**, the sidecar achieves the following real-time performance:
- **Zero-Latency Feel**: The `deepmorse` local path operates at ~1ms per segment, which is orders of magnitude faster than the duration of a single Morse 'dit'. This allows for "instant" TX triggering.
- **Concurrent Execution**: The sidecar runs the local and cloud paths in parallel. The local path provides the immediate feedback loop, while the cloud path "overwrites" with a higher-confidence correction if it detects an error in the fast path.
- **IPC Overhead**: The Unix Domain Socket (UDS) communication adds < 0.1ms of overhead, ensuring the audio stream remains synchronized with the engine's internal state.

## Documentation
For a deep dive into the AI strategy, see [AI_DECODING.md](docs/AI_DECODING.md).
