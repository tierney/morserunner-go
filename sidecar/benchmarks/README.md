# Sidecar Benchmark Dataset

Create a `manifest.jsonl` in this directory using `manifest.example.jsonl` as a template.

Manifest format (one JSON object per line):
- `id`: stable sample identifier
- `wav`: path to mono 16-bit PCM WAV (16kHz)
- `reference`: expected decoded text for quality metrics

Run local benchmark:
```bash
.venv/bin/python sidecar/benchmark_suite.py --manifest sidecar/benchmarks/manifest.jsonl --tiers local
```

Run local+cloud benchmark:
```bash
GOOGLE_API_KEY=... .venv/bin/python sidecar/benchmark_suite.py --manifest sidecar/benchmarks/manifest.jsonl --tiers both
```
