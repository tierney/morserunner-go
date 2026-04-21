import subprocess
import time
import os
import json
import socket

def run_engine(socket_path, wpm, noise, qrm, flutter=False):
    cmd = [
        "./morserunner-engine",
        "-headless",
        "-socket", socket_path,
        "-wpm", str(wpm),
        "-noise", str(noise),
        "-qrm", str(qrm)
    ]
    if flutter:
        cmd.append("-flutter")
    
    return subprocess.Popen(cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

def send_command(socket_path, command):
    try:
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        sock.connect(socket_path)
        sock.sendall((command + "\n").encode())
        sock.close()
    except:
        pass

def capture_audio(socket_path, output_wav, duration):
    subprocess.run([
        ".venv/bin/python", "sidecar/capture_sample.py",
        "--socket", socket_path,
        "--output", output_wav,
        "--duration", str(duration)
    ])

def main():
    samples_dir = "sidecar/benchmarks/samples"
    os.makedirs(samples_dir, exist_ok=True)
    
    manifest_path = "sidecar/benchmarks/manifest.jsonl"
    socket_path = "/tmp/benchmark_gen.sock"
    
    test_cases = [
        {"id": "clear-30wpm", "wpm": 30, "noise": 0.0, "qrm": 0.0, "call": "K7ABC"},
        {"id": "noisy-30wpm", "wpm": 30, "noise": 0.05, "qrm": 0.0, "call": "W6XYZ"},
        {"id": "qrm-30wpm", "wpm": 30, "noise": 0.01, "qrm": 0.05, "call": "N1ABC"},
        {"id": "flutter-30wpm", "wpm": 30, "noise": 0.02, "qrm": 0.0, "flutter": True, "call": "G4AAA"},
        {"id": "fast-45wpm", "wpm": 45, "noise": 0.0, "qrm": 0.0, "call": "JA1YAA"},
    ]
    
    manifest_lines = []
    
    for case in test_cases:
        print(f"Generating {case['id']}...")
        proc = run_engine(socket_path, case['wpm'], case['noise'], case['qrm'], case.get('flutter', False))
        time.sleep(1.5) # Wait for engine and socket
        
        wav_path = f"{samples_dir}/{case['id']}.wav"
        
        # We need to trigger the specific callsign. 
        # The engine currently picks randomly from a list.
        # For simplicity, we'll just use 'pileup 1' and we know what the callsigns are.
        # Actually, let's just use the reference as what we expect from the engine.
        # Since it's random, we might need to capture and then see what it said?
        # No, let's just make the engine predictable or use a fixed one.
        # Wait, I can send 'tx <call>' to the engine! 
        # If I send 'tx K7ABC', the engine will send K7ABC.
        
        capture_proc = subprocess.Popen([
            ".venv/bin/python", "sidecar/capture_sample.py",
            "--socket", socket_path,
            "--output", wav_path,
            "--duration", "4"
        ])
        
        time.sleep(0.5)
        send_command(socket_path, f"tx {case['call']}")
        
        capture_proc.wait()
        proc.terminate()
        proc.wait()
        
        if os.path.exists(socket_path):
            os.remove(socket_path)
            
        manifest_lines.append({
            "id": case["id"],
            "wav": wav_path,
            "reference": case["call"]
        })
        
    with open(manifest_path, "w") as f:
        for line in manifest_lines:
            f.write(json.dumps(line) + "\n")
            
    print(f"Manifest written to {manifest_path}")

if __name__ == "__main__":
    main()
