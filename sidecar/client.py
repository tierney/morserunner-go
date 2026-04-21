#!/usr/bin/env python3

import argparse
import os
import queue
import socket
import tempfile
import threading
import time
import wave
from dataclasses import dataclass
from typing import Optional

import numpy as np

from benchmarking import BenchmarkLogger
from benchmarking import extract_callsigns
from decoders import DeepMorseDecoder


@dataclass
class SidecarConfig:
    socket_path: str = "/tmp/morserunner.sock"
    sample_rate: int = 16000
    fast_window_seconds: float = 2.0
    fast_stride_seconds: float = 0.5
    strategic_window_seconds: float = 15.0
    strategic_stride_seconds: float = 5.0
    silence_threshold: float = 0.01
    local_confirmations: int = 2
    local_decoder_mode: str = "deepmorse"  # "whisper" or "deepmorse"
    local_model: str = "mlx-community/distil-whisper-large-v3"
    cloud_model: str = "gemini-1.5-flash"
    benchmark_path: str = "sidecar/benchmark_results.jsonl"
    strategic_prompt: str = (
        "You are a CW (Morse Code) decoding expert. Decode the audio, "
        "identify the callsign and exchange, and return only the decoded text."
    )


class RingBuffer:
    def __init__(self, capacity_seconds: float, sample_rate: int):
        self.capacity = int(capacity_seconds * sample_rate)
        self.buffer = np.zeros(self.capacity, dtype=np.float32)
        self.pos = 0
        self.total_samples = 0
        self.lock = threading.Lock()

    def add_samples(self, samples: np.ndarray) -> None:
        with self.lock:
            n = len(samples)
            if n > self.capacity:
                samples = samples[-self.capacity :]
                n = self.capacity

            end = self.pos + n
            if end <= self.capacity:
                self.buffer[self.pos:end] = samples
            else:
                first = self.capacity - self.pos
                self.buffer[self.pos:] = samples[:first]
                self.buffer[: end % self.capacity] = samples[first:]

            self.pos = (self.pos + n) % self.capacity
            self.total_samples += n

    def has_window(self, seconds: float, sample_rate: int) -> bool:
        with self.lock:
            return self.total_samples >= int(seconds * sample_rate)

    def get_last(self, seconds: float, sample_rate: int) -> np.ndarray:
        with self.lock:
            n = min(int(seconds * sample_rate), self.capacity, self.total_samples)
            if n <= 0:
                return np.zeros(0, dtype=np.float32)

            start = (self.pos - n) % self.capacity
            if start < self.pos:
                return self.buffer[start:self.pos].copy()
            return np.concatenate((self.buffer[start:], self.buffer[: self.pos]))


class LocalDecoder:
    def __init__(self, mode: str, model_name: str):
        self.mode = mode
        self.model_name = model_name
        self.module = None
        self.deepmorse_decoder = None
        self.available = False

        if mode == "whisper":
            try:
                import mlx_whisper
                self.module = mlx_whisper
                self.available = True
            except ImportError:
                print("mlx-whisper not installed. Falling back to deepmorse.")
                self.mode = "deepmorse"

        if self.mode == "deepmorse":
            self.deepmorse_decoder = DeepMorseDecoder()
            self.available = True

    def decode(self, audio: np.ndarray, sample_rate: int) -> Optional[str]:
        if not self.available:
            return None

        try:
            if self.mode == "whisper":
                result = self.module.transcribe(
                    audio,
                    path_or_hf_repo=self.model_name,
                    language="en",
                )
                if isinstance(result, dict):
                    return str(result.get("text", "")).strip()
                return str(result).strip()
            
            elif self.mode == "deepmorse":
                return self.deepmorse_decoder.decode(audio, sample_rate)

        except Exception as exc:
            print(f"Local decoder error ({self.mode}): {exc}")
            return None


class CloudDecoder:
    def __init__(self, model_name: str, prompt: str):
        self.model_name = model_name
        self.prompt = prompt
        self.available = False
        self.genai = None
        self.model = None

        api_key = os.getenv("GOOGLE_API_KEY")
        if not api_key:
            print("GOOGLE_API_KEY not found. Cloud path disabled.")
            return

        try:
            import google.generativeai as genai

            genai.configure(api_key=api_key)
            self.genai = genai
            self.model = genai.GenerativeModel(model_name)
            self.available = True
        except ImportError:
            print("google-generativeai not installed. Cloud path disabled.")
        except Exception as exc:
            print(f"Cloud decoder init error: {exc}")

    def decode(self, audio: np.ndarray, sample_rate: int) -> Optional[str]:
        if not self.available:
            return None

        with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as tmp:
            wav_path = tmp.name

        try:
            with wave.open(wav_path, "wb") as wav_file:
                wav_file.setnchannels(1)
                wav_file.setsampwidth(2)
                wav_file.setframerate(sample_rate)
                wav_file.writeframes((audio * 32767).astype(np.int16).tobytes())

            audio_file = self.genai.upload_file(path=wav_path, display_name="CW_Segment")
            response = self.model.generate_content([self.prompt, audio_file])
            return str(getattr(response, "text", "")).strip()
        except Exception as exc:
            print(f"Cloud decoder error: {exc}")
            return None
        finally:
            if os.path.exists(wav_path):
                os.unlink(wav_path)


class AISidecar:
    def __init__(self, config: SidecarConfig):
        self.config = config
        self.buffer = RingBuffer(20.0, config.sample_rate)
        self.running = True
        self.sock = None
        self.pending_callsigns = {}
        self.confirmed_callsigns = set()
        self.latest_fast_text = ""
        self.latest_cloud_text = ""
        self.command_queue = queue.Queue()
        self.benchmarks = BenchmarkLogger(config.benchmark_path)
        self.local_decoder = LocalDecoder(config.local_decoder_mode, config.local_model)
        self.cloud_decoder = CloudDecoder(config.cloud_model, config.strategic_prompt)

    def connect(self) -> None:
        self.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.sock.connect(self.config.socket_path)
        print(f"Connected to {self.config.socket_path}")

    def read_audio(self) -> None:
        while self.running:
            try:
                data = self.sock.recv(4096)
                if not data:
                    self.running = False
                    break
                samples = np.frombuffer(data, dtype=np.int16).astype(np.float32) / 32768.0
                self.buffer.add_samples(samples)
            except Exception as exc:
                print(f"Read error: {exc}")
                self.running = False
                break

    def send_command(self, cmd: str) -> None:
        try:
            self.sock.sendall((cmd + "\n").encode("utf-8"))
            self.command_queue.put(cmd)
            print(f"Sent command: {cmd}")
        except Exception as exc:
            print(f"Send error: {exc}")

    def maybe_send_tx(self, call: str, tier: str) -> None:
        if call in self.confirmed_callsigns:
            return

        if tier == "fast":
            self.pending_callsigns[call] = self.pending_callsigns.get(call, 0) + 1
            if self.pending_callsigns[call] < self.config.local_confirmations:
                return

        self.confirmed_callsigns.add(call)
        self.send_command(f"tx {call}")

    def handle_decode(self, text: Optional[str], tier: str, latency_ms: float) -> None:
        if not text:
            return

        normalized = text.strip().upper()
        if not normalized:
            return

        print(f"[{tier.upper()}] {normalized}")

        if tier == "fast":
            self.latest_fast_text = normalized
        else:
            self.latest_cloud_text = normalized

        callsigns = extract_callsigns(normalized)
        chosen_call = callsigns[0] if callsigns else None
        self.benchmarks.log_decode(
            tier=tier,
            text=normalized,
            latency_ms=latency_ms,
            callsign=chosen_call,
        )

        for call in callsigns:
            self.maybe_send_tx(call, tier)

    def run_fast_path(self) -> None:
        while self.running:
            time.sleep(self.config.fast_stride_seconds)
            if not self.local_decoder.available:
                continue
            if not self.buffer.has_window(self.config.fast_window_seconds, self.config.sample_rate):
                continue

            audio = self.buffer.get_last(self.config.fast_window_seconds, self.config.sample_rate)
            if audio.size == 0 or np.max(np.abs(audio)) < self.config.silence_threshold:
                continue

            start = time.perf_counter()
            text = self.local_decoder.decode(audio, self.config.sample_rate)
            latency_ms = (time.perf_counter() - start) * 1000.0
            self.handle_decode(text, "fast", latency_ms)

    def run_strategic_path(self) -> None:
        while self.running:
            time.sleep(self.config.strategic_stride_seconds)
            if not self.cloud_decoder.available:
                continue
            if not self.buffer.has_window(self.config.strategic_window_seconds, self.config.sample_rate):
                continue

            audio = self.buffer.get_last(self.config.strategic_window_seconds, self.config.sample_rate)
            if audio.size == 0 or np.max(np.abs(audio)) < self.config.silence_threshold:
                continue

            start = time.perf_counter()
            text = self.cloud_decoder.decode(audio, self.config.sample_rate)
            latency_ms = (time.perf_counter() - start) * 1000.0
            self.handle_decode(text, "strategic", latency_ms)

    def start(self) -> None:
        self.connect()
        threads = [
            threading.Thread(target=self.read_audio, daemon=True),
            threading.Thread(target=self.run_fast_path, daemon=True),
            threading.Thread(target=self.run_strategic_path, daemon=True),
        ]
        for thread in threads:
            thread.start()

        try:
            while self.running:
                time.sleep(1.0)
        except KeyboardInterrupt:
            self.running = False
        finally:
            if self.sock is not None:
                self.sock.close()


def parse_args() -> SidecarConfig:
    parser = argparse.ArgumentParser(description="MorseRunner-Go AI decoding sidecar")
    parser.add_argument("--socket", default="/tmp/morserunner.sock", help="Unix socket path")
    parser.add_argument(
        "--local-mode",
        default="deepmorse",
        choices=["whisper", "deepmorse"],
        help="Local decoder mode",
    )
    parser.add_argument(
        "--local-model",
        default="mlx-community/distil-whisper-large-v3",
        help="MLX Whisper model identifier",
    )
    parser.add_argument(
        "--cloud-model",
        default="gemini-1.5-flash",
        help="Gemini model name for strategic decoding",
    )
    parser.add_argument(
        "--benchmark-path",
        default="sidecar/benchmark_results.jsonl",
        help="JSONL file for latency and decode logs",
    )
    args = parser.parse_args()
    return SidecarConfig(
        socket_path=args.socket,
        local_decoder_mode=args.local_mode,
        local_model=args.local_model,
        cloud_model=args.cloud_model,
        benchmark_path=args.benchmark_path,
    )


if __name__ == "__main__":
    AISidecar(parse_args()).start()
