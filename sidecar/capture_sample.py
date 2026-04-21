import socket
import wave
import sys
import os
import argparse
import numpy as np

def capture(socket_path, output_wav, duration_seconds, sample_rate=16000):
    if os.path.exists(output_wav):
        os.remove(output_wav)
    
    os.makedirs(os.path.dirname(output_wav), exist_ok=True)
    
    sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    try:
        sock.connect(socket_path)
    except Exception as e:
        print(f"Failed to connect to {socket_path}: {e}")
        return False

    print(f"Capturing {duration_seconds}s to {output_wav}...")
    
    frames_to_capture = duration_seconds * sample_rate
    captured_frames = 0
    
    with wave.open(output_wav, 'wb') as wav_file:
        wav_file.setnchannels(1)
        wav_file.setsampwidth(2) # 16-bit
        wav_file.setframerate(sample_rate)
        
        while captured_frames < frames_to_capture:
            data = sock.recv(4096)
            if not data:
                break
            wav_file.writeframes(data)
            captured_frames += len(data) // 2
            
    sock.close()
    print("Done.")
    return True

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--socket", default="/tmp/morserunner.sock")
    parser.add_argument("--output", required=True)
    parser.add_argument("--duration", type=float, default=5.0)
    args = parser.parse_args()
    
    capture(args.socket, args.output, args.duration)
