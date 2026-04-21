#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOCKET_PATH="${SOCKET_PATH:-/tmp/morserunner.sock}"
ENGINE_BIN="${ENGINE_BIN:-$ROOT_DIR/morserunner-engine}"
VENV_PYTHON="${VENV_PYTHON:-$ROOT_DIR/.venv/bin/python}"
LOCAL_MODEL="${LOCAL_MODEL:-mlx-community/whisper-tiny-mlx}"
SEED_COMMAND="${SEED_COMMAND:-pileup 3}"

if [[ ! -x "$ENGINE_BIN" ]]; then
  echo "Engine binary not found at $ENGINE_BIN"
  echo "Build it first with: go build -o morserunner-engine main.go"
  exit 1
fi

if [[ ! -x "$VENV_PYTHON" ]]; then
  echo "Virtualenv Python not found at $VENV_PYTHON"
  echo "Create it first before running this smoke test."
  exit 1
fi

"$ENGINE_BIN" -headless -socket "$SOCKET_PATH" -contest WPX -wpm 30 -noise 0.03 -qrm 0.02 &
ENGINE_PID=$!

cleanup() {
  kill "$ENGINE_PID" >/dev/null 2>&1 || true
}
trap cleanup EXIT

sleep 1
"$VENV_PYTHON" "$ROOT_DIR/sidecar/send_command.py" --socket "$SOCKET_PATH" "$SEED_COMMAND"
"$VENV_PYTHON" "$ROOT_DIR/sidecar/client.py" --socket "$SOCKET_PATH" --local-model "$LOCAL_MODEL"
