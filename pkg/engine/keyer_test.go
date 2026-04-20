package engine

import (
	"testing"
)

func TestKeyerRatios(t *testing.T) {
	k := NewKeyer(16000)
	k.SetWpm(30, 30)

	morseE := k.Encode("E") // "."
	envE := k.GenerateEnvelope(morseE)

	morseT := k.Encode("T") // "-"
	envT := k.GenerateEnvelope(morseT)

	// E = dot(1) + space(1) + charSpace(2) = 4 units
	// T = dah(3) + space(1) + charSpace(2) = 6 units
	// Ratio should be 6/4 = 1.5
	if float64(len(envT)) != 1.5*float64(len(envE)) {
		t.Errorf("Dah+Space length (%d) should be 1.5x Dot+Space length (%d) for E/T comparison", len(envT), len(envE))
	}
}

func TestMorseEncoding(t *testing.T) {
	k := NewKeyer(16000)
	morse := k.Encode("K") // "-.- "

	expected := "-.- "
	if morse != expected {
		t.Errorf("K encoding = %q; want %q", morse, expected)
	}
}

func TestEnvelopeGeneration(t *testing.T) {
	k := NewKeyer(16000)
	morse := k.Encode("E") // ". "
	env := k.GenerateEnvelope(morse)

	// Check for ramp-up (allow for small float precision)
	if env[0] > 0.001 {
		t.Errorf("Envelope should start near 0, got %f", env[0])
	}

	// Check that it reaches full amplitude
	max := 0.0
	for _, v := range env {
		if float64(v) > max {
			max = float64(v)
		}
	}
	if max < 0.99 {
		t.Errorf("Envelope never reached full amplitude, max = %f", max)
	}
}

func TestHighSpeedDotIntegrity(t *testing.T) {
	rate := 16000
	k := NewKeyer(rate)
	k.SetWpm(50, 50) // High speed

	morse := k.Encode("E")
	env := k.GenerateEnvelope(morse)

	// A dot at 50 WPM is ~24ms
	// Ramps are 5ms each
	// Plateau count should be reasonable
	plateauCount := 0
	for _, v := range env {
		if v > 0.99 {
			plateauCount++
		}
	}

	if plateauCount < 20 {
		t.Errorf("50 WPM dot too distorted by ramps, plateau count = %d", plateauCount)
	}
}
