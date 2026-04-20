package engine

import (
	"testing"
)

func TestConfidence(t *testing.T) {
	tests := []struct {
		target   string
		input    string
		expected int
	}{
		{"K7ABC", "K7ABC", 100}, // Full match
		{"K7ABC", "k7abc", 100}, // Case insensitive
		{"K7ABC", "K7", 40},    // Partial prefix (2/5)
		{"K7ABC", "ABC", 60},   // Partial suffix (3/5)
		{"K7ABC", "K7XBC", 80},  // One character error (4/5)
		{"K7ABC", "XYZ", 0},     // No match
		{"K7ABC", "", 0},        // Empty input
		{"W6XYZ", "W6X", 60},    // Substring match
		{"K7ABC", "K7ABCDE", 60}, // Input longer than target (penalized)
	}

	for _, tt := range tests {
		got := Confidence(tt.target, tt.input)
		if got != tt.expected {
			t.Errorf("Confidence(%q, %q) = %d; want %d", tt.target, tt.input, got, tt.expected)
		}
	}
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		s1       string
		s2       string
		expected int
	}{
		{"ABC", "ABC", 0},
		{"ABC", "ABD", 1},
		{"ABC", "AB", 1},
		{"ABC", "ABCD", 1},
		{"K7ABC", "K7XBC", 1},
		{"", "ABC", 3},
	}

	for _, tt := range tests {
		got := LevenshteinDistance(tt.s1, tt.s2)
		if got != tt.expected {
			t.Errorf("LevenshteinDistance(%q, %q) = %d; want %d", tt.s1, tt.s2, got, tt.expected)
		}
	}
}
