package engine

import (
	"math/rand"
)

type QsbModulator struct {
	Gain      float64
	Target    float64
	Bandwidth float64 // Low = QSB, High = Flutter
	Rate      int
}

func NewQsbModulator(rate int, bandwidth float64) *QsbModulator {
	return &QsbModulator{
		Gain:      1.0,
		Target:    rand.Float64(),
		Bandwidth: bandwidth,
		Rate:      rate,
	}
}

func (q *QsbModulator) Apply(samples []float32) {
	// Simple low-pass filtered gain
	// Alpha calculation based on bandwidth
	alpha := q.Bandwidth / float64(q.Rate)
	if alpha > 1.0 {
		alpha = 1.0
	}

	for i := range samples {
		// Wander the target
		if rand.Float64() < alpha {
			q.Target = rand.Float64()
		}

		// Smooth transition
		q.Gain += (q.Target - q.Gain) * alpha
		samples[i] *= float32(q.Gain)
	}
}
