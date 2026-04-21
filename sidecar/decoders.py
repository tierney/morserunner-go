import numpy as np
import scipy.signal as signal
from typing import Optional

class DeepMorseDecoder:
    """
    A CNN-inspired decoder that uses matched-filter convolution on spectrograms.
    This mimics the first layer of DeepMorse but uses pre-defined kernels
    for dits and dahs, making it robust without needing external weights.
    """
    def __init__(self, sample_rate=16000, wpm=30):
        self.sample_rate = sample_rate
        # CW Timing at given WPM
        self.dit_len_sec = 1.2 / wpm
        self.target_freq = None
        
    def decode(self, audio: np.ndarray, sample_rate: int) -> str:
        if len(audio) == 0:
            return ""
            
        # 1. FFT Spectrogram
        f, t, Sxx = signal.spectrogram(audio, sample_rate, nperseg=256, noverlap=192)
        dt = t[1] - t[0]
        
        # 2. Find Carrier
        if self.target_freq is None:
            peak_bin = np.argmax(np.mean(Sxx, axis=1)[1:]) + 1
            self.target_freq = f[peak_bin]
            
        target_bin = np.argmin(np.abs(f - self.target_freq))
        envelope = Sxx[target_bin, :]
        envelope = envelope / (np.max(envelope) + 1e-9)
        
        # 3. 'CNN' Matched Filter Layer
        # Define kernels for dit and dah
        dit_steps = int(self.dit_len_sec / dt)
        if dit_steps < 1: dit_steps = 1
        
        dit_kernel = np.ones(dit_steps)
        dah_kernel = np.ones(dit_steps * 3)
        
        # Convolve envelope with kernels (Probability map)
        dit_prob = np.convolve(envelope, dit_kernel, mode='same') / dit_steps
        dah_prob = np.convolve(envelope, dah_kernel, mode='same') / (dit_steps * 3)
        
        # 4. Decoding Logic
        result = []
        curr_morse = ""
        
        # Sliding window through probabilities
        i = 0
        while i < len(envelope):
            if envelope[i] > 0.4:
                # Decide if dit or dah based on local max probability
                if dah_prob[i] > 0.6 and dah_prob[i] > dit_prob[i]:
                    curr_morse += "-"
                    i += dit_steps * 3
                elif dit_prob[i] > 0.5:
                    curr_morse += "."
                    i += dit_steps
                else:
                    i += 1
            else:
                # Gap detection
                gap_len = 0
                while i < len(envelope) and envelope[i] <= 0.4:
                    gap_len += 1
                    i += 1
                
                if gap_len * dt > self.dit_len_sec * 2: # End of char
                    char = self._morse_to_char(curr_morse)
                    if char: result.append(char)
                    curr_morse = ""
                if gap_len * dt > self.dit_len_sec * 5: # End of word
                    if result and result[-1] != " ":
                        result.append(" ")
        
        # Final char
        char = self._morse_to_char(curr_morse)
        if char: result.append(char)
        
        return "".join(result).strip()

    def _morse_to_char(self, m):
        morse_map = {
            '.-': 'A', '-...': 'B', '-.-.': 'C', '-..': 'D', '.': 'E',
            '..-.': 'F', '--.': 'G', '....': 'H', '..': 'I', '.---': 'J',
            '-.-': 'K', '.-..': 'L', '--': 'M', '-.': 'N', '---': 'O',
            '.--.': 'P', '--.-': 'Q', '.-.': 'R', '...': 'S', '-': 'T',
            '..-': 'U', '...-': 'V', '.--': 'W', '-..-': 'X', '-.--': 'Y',
            '--..': 'Z', '-----': '0', '.----': '1', '..---': '2', '...--': '3',
            '....-': '4', '.....': '5', '-....': '6', '--...': '7', '---..': '8',
            '----.': '9', '.-.-.-': '.', '--..--': ',', '..--..': '?'
        }
        return morse_map.get(m, "")
