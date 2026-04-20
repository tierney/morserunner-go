import socket
import struct
import numpy as np
import threading
import time
import queue
import re
import os

class RingBuffer:
    def __init__(self, capacity_seconds, sample_rate):
        self.capacity = capacity_seconds * sample_rate
        self.buffer = np.zeros(self.capacity, dtype=np.float32)
        self.pos = 0
        self.total_samples = 0
        self.lock = threading.Lock()

    def add_samples(self, samples):
        with self.lock:
            n = len(samples)
            if n > self.capacity:
                samples = samples[-self.capacity:]
                n = self.capacity
            
            end = self.pos + n
            if end <= self.capacity:
                self.buffer[self.pos:end] = samples
            else:
                first_part = self.capacity - self.pos
                self.buffer[self.pos:] = samples[:first_part]
                self.buffer[:end % self.capacity] = samples[first_part:]
            
            self.pos = (self.pos + n) % self.capacity
            self.total_samples += n

    def get_last(self, seconds, sample_rate):
        with self.lock:
            n = int(seconds * sample_rate)
            if n > self.capacity:
                n = self.capacity
            
            start = (self.pos - n) % self.capacity
            if start < self.pos:
                return self.buffer[start:self.pos].copy()
            else:
                return np.concatenate((self.buffer[start:], self.buffer[:self.pos]))

class AISidecar:
    def __init__(self, socket_path="/tmp/morserunner.sock"):
        self.socket_path = socket_path
        self.sample_rate = 16000
        self.buffer = RingBuffer(20, self.sample_rate) # 20s ring buffer
        self.running = True
        self.command_queue = queue.Queue()
        
        # Paths
        self.fast_window = 2.0  # seconds
        self.strategic_window = 15.0 # seconds
        
        self.last_fast_decode = ""
        self.last_strategic_decode = ""

    def connect(self):
        self.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.sock.connect(self.socket_path)
        print(f"Connected to {self.socket_path}")

    def read_audio(self):
        while self.running:
            try:
                data = self.sock.recv(4096)
                if not data:
                    break
                # S16LE to Float32
                samples = np.frombuffer(data, dtype=np.int16).astype(np.float32) / 32768.0
                self.buffer.add_samples(samples)
            except Exception as e:
                print(f"Read error: {e}")
                break

    def send_command(self, cmd):
        try:
            self.sock.sendall((cmd + "\n").encode())
            print(f"Sent command: {cmd}")
        except Exception as e:
            print(f"Send error: {e}")

    def run_fast_path(self):
        """Tier 1: Fast decoding (Local MLX)"""
        try:
            import mlx_whisper
        except ImportError:
            print("mlx-whisper not installed. Fast path disabled.")
            return

        while self.running:
            time.sleep(0.5) # Stride
            audio = self.buffer.get_last(self.fast_window, self.sample_rate)
            if np.max(np.abs(audio)) < 0.01:
                continue
            
            # TODO: MLX Whisper inference
            # result = mlx_whisper.transcribe(audio, path_or_hf_repo="mlx-community/whisper-tiny-mlx")
            # self.handle_decode(result['text'], "fast")
            pass

    def run_strategic_path(self):
        """Tier 2: Strategic decoding (Gemini)"""
        try:
            import google.generativeai as genai
        except ImportError:
            print("google-generativeai not installed. Strategic path disabled.")
            return

        api_key = os.getenv("GOOGLE_API_KEY")
        if not api_key:
            print("GOOGLE_API_KEY not found. Strategic path disabled.")
            return
        
        genai.configure(api_key=api_key)
        model = genai.GenerativeModel('gemini-1.5-flash')

        while self.running:
            time.sleep(5.0) # Stride
            audio = self.buffer.get_last(self.strategic_window, self.sample_rate)
            if np.max(np.abs(audio)) < 0.01:
                continue

            # Save to temporary WAV for Gemini
            import tempfile
            import wave
            with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as tf:
                try:
                    with wave.open(tf.name, "wb") as wf:
                        wf.setnchannels(1)
                        wf.setsampwidth(2)
                        wf.setframerate(self.sample_rate)
                        wf.writeframes((audio * 32767).astype(np.int16).tobytes())
                    
                    audio_file = genai.upload_file(path=tf.name, display_name="CW_Segment")
                    response = model.generate_content([
                        "You are a CW (Morse Code) decoding expert. Decode the following audio. "
                        "Identify the callsign and any exchange information. "
                        "Return only the decoded text.",
                        audio_file
                    ])
                    self.handle_decode(response.text, "strategic")
                except Exception as e:
                    print(f"Gemini error: {e}")
                finally:
                    if os.path.exists(tf.name):
                        os.unlink(tf.name)

    def handle_decode(self, text, tier):
        text = text.strip().upper()
        if not text:
            return
        
        print(f"[{tier.upper()}] {text}")
        
        # Simple Regex for callsigns
        # CW callsigns are usually 3-7 chars, alphanumeric
        callsigns = re.findall(r'\b[A-Z0-9]{3,7}\b', text)
        
        for call in callsigns:
            # Basic validation: must have at least one number and one letter
            if not (any(c.isdigit() for c in call) and any(c.isalpha() for c in call)):
                continue
                
            if tier == "fast":
                if call not in self.confirmed_callsigns:
                    print(f"Fast Tier detected potential call: {call}")
                    self.pending_callsigns[call] = self.pending_callsigns.get(call, 0) + 1
                    
                    # If seen 3 times in fast tier, we trust it enough to send a partial TX or wait
                    if self.pending_callsigns[call] >= 3:
                        print(f"Confidence high for {call}, triggering TX...")
                        self.send_command(f"tx {call}")
                        self.confirmed_callsigns.add(call)
            
            elif tier == "strategic":
                print(f"Strategic Tier confirmed/corrected: {call}")
                if call not in self.confirmed_callsigns:
                    self.send_command(f"tx {call}")
                    self.confirmed_callsigns.add(call)
                # Strategic tier can also handle "TU" or "5NN"
                if "TU" in text or "5NN" in text:
                    print(f"QSO complete confirmed for {call}")

    def start(self):
        self.confirmed_callsigns = set()
        self.pending_callsigns = {} # call -> count
        self.connect()
        threading.Thread(target=self.read_audio, daemon=True).start()
        threading.Thread(target=self.run_fast_path, daemon=True).start()
        threading.Thread(target=self.run_strategic_path, daemon=True).start()
        
        try:
            while self.running:
                time.sleep(1)
        except KeyboardInterrupt:
            self.running = False

if __name__ == "__main__":
    sidecar = AISidecar()
    sidecar.start()
