# AI Decoding Strategy: Morse Code (CW)

## Overview
Traditional speech-to-text models (like OpenAI Whisper) are designed for human vocal characteristics (phonemes, harmonics). Morse code (CW) is fundamentally different: it is a monochromatic pulse-width modulated (PWM) signal.

Generic AI models often fail on CW because:
1. **Rhythm vs. Phonemes**: CW relies on precise timing ratios (1:3:7), not vocal shapes.
2. **Bandwidth**: CW is extremely narrow-band (typically < 100Hz), while speech is wide-band (300Hz - 3400Hz+).
3. **Noise Profile**: HF noise (QRM/QRN) is different from the acoustic noise models used to train Whisper.

## Our Multi-Path Approach

### 1. Cloud Path (High Reasoning)
Uses multi-modal models (like Gemini 1.5). These models are "smart" enough to recognize the rhythmic patterns in audio even without specific CW training, acting like a human operator "ear-copying" the signal.

### 2. Local Path (FFT-Optimized CNN)
Instead of raw waveform analysis, we use **Spectrogram-based Convolutional Neural Networks (CNNs)**.

**Pipeline:**
1. **FFT Windowing**: Convert the 16kHz PCM stream into a 2D spectrogram.
2. **Frequency Isolation**: Automatically center on the CW pitch (e.g., 600Hz) to eliminate out-of-band noise.
3. **CNN Feature Extraction**: A 2D CNN identifies the visual patterns of "dits" and "dahs" in the spectrogram.
4. **CTC Decoding**: A Connectionist Temporal Classification (CTC) layer converts the pulse patterns into characters without needing exact timing alignment.

## Training with MorseRunner
One of the unique advantages of this codebase is the ability to generate **infinite synthetic training data**.
By running the engine in headless mode with various `-noise`, `-qrm`, and `-flutter` flags, we can generate perfectly labeled WAV samples to train the local CNN model.

## Recommended Models (OSS)
- **DeepMorse (ag1le)**: CNN + LSTM architecture. Highly robust.
- **MorseAngel**: Envelope-based deep learning.
- **Our MLX-CNN**: A native Apple Silicon implementation of the spectrogram-to-text pipeline.
