#!/usr/bin/env bash
# Setup script for MorseRunner-Go AI Sidecar Dependencies
set -euo pipefail

echo "Creating Python virtual environment..."
python3 -m venv .venv
source .venv/bin/python -m pip install --upgrade pip

echo "Installing core dependencies..."
# Core math and signal processing
.venv/bin/pip install numpy scipy librosa

# AI Backends
.venv/bin/pip install mlx-whisper mlx-core google-generativeai

# Benchmarking and Utilities
.venv/bin/pip install argparse

echo "------------------------------------------------"
echo "Setup Complete!"
echo "To activate the environment: source .venv/bin/activate"
echo "To run the sidecar: .venv/bin/python sidecar/client.py"
echo "------------------------------------------------"
