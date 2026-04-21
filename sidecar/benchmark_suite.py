#!/usr/bin/env python3

import argparse
import json
import os
import statistics
import time
import wave
from dataclasses import dataclass
from typing import Any

import numpy as np

from benchmarking import char_error_rate
from benchmarking import extract_callsigns
from benchmarking import word_error_rate
from client import CloudDecoder
from client import LocalDecoder


@dataclass
class BenchmarkSample:
    sample_id: str
    wav_path: str
    reference: str


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run local/cloud decoder quality-latency benchmark")
    parser.add_argument(
        "--manifest",
        default="sidecar/benchmarks/manifest.jsonl",
        help="JSONL manifest with id, wav, reference fields",
    )
    parser.add_argument(
        "--output-jsonl",
        default="sidecar/benchmark_runs.jsonl",
        help="Per-sample benchmark output file",
    )
    parser.add_argument(
        "--summary-json",
        default="sidecar/benchmark_summary.json",
        help="Aggregate benchmark summary output file",
    )
    parser.add_argument(
        "--tiers",
        default="local",
        choices=["local", "cloud", "both"],
        help="Which decode tiers to benchmark",
    )
    parser.add_argument(
        "--local-model",
        default="mlx-community/whisper-tiny-mlx",
        help="Local MLX model identifier",
    )
    parser.add_argument(
        "--cloud-model",
        default="gemini-1.5-flash",
        help="Cloud model name",
    )
    parser.add_argument(
        "--sample-rate",
        type=int,
        default=16000,
        help="Expected WAV sample rate",
    )
    parser.add_argument(
        "--cloud-prompt",
        default=(
            "You are a CW (Morse Code) decoding expert. Decode the audio, "
            "identify the callsign and exchange, and return only the decoded text."
        ),
        help="Prompt used for cloud decode",
    )
    return parser.parse_args()


def load_manifest(path: str) -> list[BenchmarkSample]:
    samples: list[BenchmarkSample] = []
    with open(path, "r", encoding="utf-8") as handle:
        for index, line in enumerate(handle, start=1):
            line = line.strip()
            if not line:
                continue
            data = json.loads(line)
            sample_id = data.get("id") or f"sample-{index}"
            wav_path = data["wav"]
            reference = data["reference"]
            samples.append(BenchmarkSample(sample_id=sample_id, wav_path=wav_path, reference=reference))
    return samples


def load_wav_mono_s16le(path: str, expected_sample_rate: int) -> np.ndarray:
    with wave.open(path, "rb") as wav_file:
        channels = wav_file.getnchannels()
        sample_width = wav_file.getsampwidth()
        sample_rate = wav_file.getframerate()
        frames = wav_file.getnframes()
        raw = wav_file.readframes(frames)

    if channels != 1:
        raise ValueError(f"{path}: expected mono WAV, got {channels} channels")
    if sample_width != 2:
        raise ValueError(f"{path}: expected 16-bit WAV, got {sample_width * 8}-bit")
    if sample_rate != expected_sample_rate:
        raise ValueError(f"{path}: expected sample rate {expected_sample_rate}, got {sample_rate}")

    return np.frombuffer(raw, dtype=np.int16).astype(np.float32) / 32768.0


def percentile(values: list[float], p: float) -> float:
    if not values:
        return 0.0
    sorted_values = sorted(values)
    idx = (len(sorted_values) - 1) * p
    low = int(idx)
    high = min(low + 1, len(sorted_values) - 1)
    frac = idx - low
    return sorted_values[low] * (1.0 - frac) + sorted_values[high] * frac


def summarize_tier(rows: list[dict[str, Any]]) -> dict[str, Any]:
    latencies = [row["latency_ms"] for row in rows]
    wers = [row["wer"] for row in rows]
    cers = [row["cer"] for row in rows]
    callsign_hits = [1.0 if row["callsign_match"] else 0.0 for row in rows]
    return {
        "count": len(rows),
        "latency_ms_avg": round(statistics.mean(latencies), 2) if latencies else 0.0,
        "latency_ms_p50": round(percentile(latencies, 0.5), 2),
        "latency_ms_p95": round(percentile(latencies, 0.95), 2),
        "wer_avg": round(statistics.mean(wers), 4) if wers else 0.0,
        "cer_avg": round(statistics.mean(cers), 4) if cers else 0.0,
        "callsign_match_rate": round(statistics.mean(callsign_hits), 4) if callsign_hits else 0.0,
    }


def ensure_parent(path: str) -> None:
    directory = os.path.dirname(path)
    if directory:
        os.makedirs(directory, exist_ok=True)


def run_decode(decoder: Any, audio: np.ndarray, sample_rate: int) -> tuple[str, float]:
    started = time.perf_counter()
    decoded = decoder.decode(audio, sample_rate)
    latency_ms = (time.perf_counter() - started) * 1000.0
    return (decoded or "").strip().upper(), latency_ms


def main() -> None:
    args = parse_args()
    samples = load_manifest(args.manifest)
    if not samples:
        raise ValueError(f"No samples found in manifest: {args.manifest}")

    run_local = args.tiers in ("local", "both")
    run_cloud = args.tiers in ("cloud", "both")

    local_decoder = LocalDecoder(args.local_model) if run_local else None
    cloud_decoder = CloudDecoder(args.cloud_model, args.cloud_prompt) if run_cloud else None

    if run_local and not local_decoder.available:
        raise RuntimeError("Local decoder not available. Install mlx-whisper and retry.")
    if run_cloud and not cloud_decoder.available:
        raise RuntimeError("Cloud decoder not available. Set GOOGLE_API_KEY and install google-generativeai.")

    ensure_parent(args.output_jsonl)
    ensure_parent(args.summary_json)

    rows: list[dict[str, Any]] = []
    with open(args.output_jsonl, "w", encoding="utf-8") as out_handle:
        for sample in samples:
            audio = load_wav_mono_s16le(sample.wav_path, args.sample_rate)
            reference_calls = extract_callsigns(sample.reference)
            reference_call = reference_calls[0] if reference_calls else None

            for tier_name, decoder in (("local", local_decoder), ("cloud", cloud_decoder)):
                if decoder is None:
                    continue

                decoded_text, latency_ms = run_decode(decoder, audio, args.sample_rate)
                decoded_calls = extract_callsigns(decoded_text)
                decoded_call = decoded_calls[0] if decoded_calls else None
                row = {
                    "timestamp": time.time(),
                    "sample_id": sample.sample_id,
                    "wav": sample.wav_path,
                    "tier": tier_name,
                    "reference": sample.reference,
                    "decoded": decoded_text,
                    "latency_ms": round(latency_ms, 2),
                    "wer": round(word_error_rate(sample.reference, decoded_text), 6),
                    "cer": round(char_error_rate(sample.reference, decoded_text), 6),
                    "reference_callsign": reference_call,
                    "decoded_callsign": decoded_call,
                    "callsign_match": (reference_call == decoded_call) if reference_call else False,
                }
                rows.append(row)
                out_handle.write(json.dumps(row, sort_keys=True) + "\n")
                print(
                    f"{tier_name:<5} {sample.sample_id:<20} latency={row['latency_ms']:>7}ms "
                    f"wer={row['wer']:.4f} cer={row['cer']:.4f} decoded={row['decoded']!r}"
                )

    by_tier: dict[str, list[dict[str, Any]]] = {"local": [], "cloud": []}
    for row in rows:
        by_tier[row["tier"]].append(row)

    summary = {
        "manifest": args.manifest,
        "output_jsonl": args.output_jsonl,
        "generated_at": time.time(),
        "tiers": {},
    }
    for tier, tier_rows in by_tier.items():
        if tier_rows:
            summary["tiers"][tier] = summarize_tier(tier_rows)

    with open(args.summary_json, "w", encoding="utf-8") as summary_handle:
        json.dump(summary, summary_handle, sort_keys=True, indent=2)
        summary_handle.write("\n")

    print(f"\nWrote per-sample results: {args.output_jsonl}")
    print(f"Wrote summary: {args.summary_json}")
    for tier, tier_summary in summary["tiers"].items():
        print(
            f"{tier}: count={tier_summary['count']} "
            f"avg={tier_summary['latency_ms_avg']}ms p95={tier_summary['latency_ms_p95']}ms "
            f"wer={tier_summary['wer_avg']} cer={tier_summary['cer_avg']} "
            f"callsign_match={tier_summary['callsign_match_rate']}"
        )


if __name__ == "__main__":
    main()
