package engine

import (
	"math"
)

type Mixer struct {
	Rate      int
	Pitch     float64
	Amplitude float64
	Phase     float64
	TwoPi     float64
	Bandwidth float64
	Filter    []float64 // Filter coefficients
	History   []complex128
}

func NewMixer(rate int, pitch float64) *Mixer {
	m := &Mixer{
		Rate:      rate,
		Pitch:     pitch,
		Amplitude: 0.5,
		TwoPi:     2.0 * math.Pi,
		History:   make([]complex128, 64),
	}
	m.UpdateFilter(500.0)
	return m
}

func (m *Mixer) UpdateFilter(bw float64) {
	m.Bandwidth = bw
	// Simple moving average or FIR filter coefficients
	// For now, let's use a 31-tap Sinc filter
	taps := 31
	m.Filter = make([]float64, taps)
	fc := bw / float64(m.Rate)
	for i := 0; i < taps; i++ {
		n := float64(i - (taps-1)/2)
		if n == 0 {
			m.Filter[i] = 2 * fc
		} else {
			m.Filter[i] = math.Sin(2*math.Pi*fc*n) / (math.Pi * n)
		}
		// Hamming window
		m.Filter[i] *= 0.54 - 0.46*math.Cos(2*math.Pi*float64(i)/float64(taps-1))
	}
}

func (m *Mixer) Mix(baseband []complex128) []float32 {
	res := make([]float32, len(baseband))
	deltaPhi := m.TwoPi * m.Pitch / float64(m.Rate)

	for i, s := range baseband {
		// Apply filter (convolution)
		// Shift history
		copy(m.History[1:], m.History[:len(m.History)-1])
		m.History[0] = s

		filtered := complex(0, 0)
		for j, coeff := range m.Filter {
			filtered += m.History[j] * complex(coeff, 0)
		}

		// Modulate to pitch
		val := real(filtered)*math.Cos(m.Phase) - imag(filtered)*math.Sin(m.Phase)
		res[i] = float32(val * m.Amplitude)

		m.Phase += deltaPhi
		for m.Phase > m.TwoPi {
			m.Phase -= m.TwoPi
		}
	}

	return res
}

// Simple AGC implementation
type AGC struct {
	Level   float64
	Attack  float64
	Release float64
}

func NewAGC() *AGC {
	return &AGC{
		Level:   1.0,
		Attack:  0.01,
		Release: 0.001,
	}
}

func (a *AGC) Process(samples []float32) {
	for i := range samples {
		abs := math.Abs(float64(samples[i]))
		if abs*a.Level > 1.0 {
			a.Level = 1.0 / abs
		} else {
			a.Level += a.Release * (1.0 - a.Level)
		}
		samples[i] *= float32(a.Level)
	}
}
