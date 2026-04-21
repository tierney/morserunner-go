#!/usr/bin/env python3

import json
import os
import re
import threading
import time
from dataclasses import asdict, dataclass
from typing import Optional

CALLSIGN_RE = re.compile(r"\b[A-Z0-9]{3,7}\b")


@dataclass
class DecodeMetric:
    timestamp: float
    tier: str
    text: str
    latency_ms: float
    callsign: Optional[str]


class BenchmarkLogger:
    def __init__(self, path: str):
        self.path = path
        self.lock = threading.Lock()
        directory = os.path.dirname(path)
        if directory:
            os.makedirs(directory, exist_ok=True)

    def log_decode(self, tier: str, text: str, latency_ms: float, callsign: Optional[str]) -> None:
        metric = DecodeMetric(
            timestamp=time.time(),
            tier=tier,
            text=text,
            latency_ms=round(latency_ms, 2),
            callsign=callsign,
        )
        with self.lock:
            with open(self.path, "a", encoding="utf-8") as handle:
                handle.write(json.dumps(asdict(metric), sort_keys=True) + "\n")


def normalize_text(text: str) -> str:
    compact = re.sub(r"[^A-Z0-9\s]", " ", text.upper())
    return re.sub(r"\s+", " ", compact).strip()


def extract_callsigns(text: str) -> list[str]:
    calls = []
    for token in CALLSIGN_RE.findall(normalize_text(text)):
        if any(ch.isalpha() for ch in token) and any(ch.isdigit() for ch in token):
            calls.append(token)
    return calls


def levenshtein_distance(a: list[str], b: list[str]) -> int:
    if not a:
        return len(b)
    if not b:
        return len(a)

    prev = list(range(len(b) + 1))
    for i, item_a in enumerate(a, start=1):
        curr = [i]
        for j, item_b in enumerate(b, start=1):
            insert_cost = curr[j - 1] + 1
            delete_cost = prev[j] + 1
            replace_cost = prev[j - 1] + (0 if item_a == item_b else 1)
            curr.append(min(insert_cost, delete_cost, replace_cost))
        prev = curr
    return prev[-1]


def word_error_rate(reference: str, hypothesis: str) -> float:
    ref_words = normalize_text(reference).split()
    hyp_words = normalize_text(hypothesis).split()
    if not ref_words:
        return 0.0 if not hyp_words else 1.0
    return levenshtein_distance(ref_words, hyp_words) / len(ref_words)


def char_error_rate(reference: str, hypothesis: str) -> float:
    ref_chars = list(normalize_text(reference).replace(" ", ""))
    hyp_chars = list(normalize_text(hypothesis).replace(" ", ""))
    if not ref_chars:
        return 0.0 if not hyp_chars else 1.0
    return levenshtein_distance(ref_chars, hyp_chars) / len(ref_chars)
