package engine

import (
	"math"
)

const (
	DefaultRate = 16000
	RiseTime    = 0.005 // 5ms
)

type Keyer struct {
	Rate       int
	WpmS       int
	WpmC       int
	RampLen    int
	RampOn     []float32
	RampOff    []float32
	MorseTable map[rune]string
}

func NewKeyer(rate int) *Keyer {
	k := &Keyer{
		Rate: rate,
		WpmS: 30,
		WpmC: 30,
		MorseTable: map[rune]string{
			'A': ".-", 'B': "-...", 'C': "-.-.", 'D': "-..", 'E': ".", 'F': "..-.",
			'G': "--.", 'H': "....", 'I': "..", 'J': ".---", 'K': "-.-", 'L': ".-..",
			'M': "--", 'N': "-.", 'O': "---", 'P': ".--.", 'Q': "--.-", 'R': ".-.",
			'S': "...", 'T': "-", 'U': "..-", 'V': "...-", 'W': ".--", 'X': "-..-",
			'Y': "-.--", 'Z': "--..",
			'1': ".----", '2': "..---", '3': "...--", '4': "....-", '5': ".....",
			'6': "-....", '7': "--...", '8': "---..", '9': "----.", '0': "-----",
			'/': "-..-.", '?': "..--..", '.': ".-.-.-", ',': "--..--",
		},
	}
	k.makeRamp()
	return k
}

func (k *Keyer) SetWpm(wpmS, wpmC int) {
	k.WpmS = wpmS
	k.WpmC = wpmC
}

func (k *Keyer) makeRamp() {
	k.RampLen = int(2.7 * RiseTime * float64(k.Rate))
	k.RampOn = k.blackmanHarrisStepResponse(k.RampLen)
	k.RampOff = make([]float32, k.RampLen)
	for i := 0; i < k.RampLen; i++ {
		k.RampOff[k.RampLen-1-i] = k.RampOn[i]
	}
}

func (k *Keyer) blackmanHarrisKernel(x float64) float64 {
	const (
		a0 = 0.35875
		a1 = 0.48829
		a2 = 0.14128
		a3 = 0.01168
	)
	return a0 - a1*math.Cos(2*math.Pi*x) + a2*math.Cos(4*math.Pi*x) - a3*math.Cos(6*math.Pi*x)
}

func (k *Keyer) blackmanHarrisStepResponse(length int) []float32 {
	res := make([]float32, length)
	for i := 0; i < length; i++ {
		res[i] = float32(k.blackmanHarrisKernel(float64(i) / float64(length)))
	}
	// integrate
	for i := 1; i < length; i++ {
		res[i] += res[i-1]
	}
	// normalize
	scale := 1.0 / res[length-1]
	for i := 0; i < length; i++ {
		res[i] *= scale
	}
	return res
}

func (k *Keyer) SamplesInUnit() int {
	// Standard Morse timing: 48 units = 1 word (MorseRunner standard)
	return int(math.Round(60.0 / 48.0 * float64(k.Rate) / float64(k.WpmS)))
}

func (k *Keyer) Encode(text string) string {
	res := ""
	for _, r := range text {
		if r == ' ' || r == '_' {
			res += " "
		} else if m, ok := k.MorseTable[r]; ok {
			res += m + " "
		}
	}
	return res
}

func (k *Keyer) GenerateEnvelope(morse string) []float32 {
	samplesPerUnit := k.SamplesInUnit()

	// Pre-calculate length
	totalUnits := 0
	for _, r := range morse {
		switch r {
		case '.':
			totalUnits += 2 // 1 dot + 1 space
		case '-':
			totalUnits += 4 // 3 dah + 1 space
		case ' ':
			totalUnits += 2 // character space (already has 1 from last element, total 3)
		}
	}

	res := make([]float32, totalUnits*samplesPerUnit)
	p := 0

	addOn := func(units int) {
		// Ramp On
		copy(res[p:], k.RampOn)
		p += k.RampLen

		// Steady state
		steadyLen := units*samplesPerUnit - 2*k.RampLen
		for i := 0; i < steadyLen; i++ {
			res[p+i] = 1.0
		}
		p += steadyLen

		// Ramp Off
		copy(res[p:], k.RampOff)
		p += k.RampLen
	}

	addOff := func(units int) {
		p += units * samplesPerUnit
	}

	for _, r := range morse {
		switch r {
		case '.':
			addOn(1)
			addOff(1)
		case '-':
			addOn(3)
			addOff(1)
		case ' ':
			addOff(2)
		}
	}

	return res
}
