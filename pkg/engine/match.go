package engine

import (
	"strings"
)

// LevenshteinDistance calculates the edit distance between two strings.
func LevenshteinDistance(s1, s2 string) int {
	s1 = strings.ToUpper(s1)
	s2 = strings.ToUpper(s2)

	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	rows := len(s1) + 1
	cols := len(s2) + 1
	dist := make([][]int, rows)
	for i := range dist {
		dist[i] = make([]int, cols)
	}

	for i := 0; i < rows; i++ {
		dist[i][0] = i
	}
	for j := 0; j < cols; j++ {
		dist[0][j] = j
	}

	for i := 1; i < rows; i++ {
		for j := 1; j < cols; j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}
			dist[i][j] = min(
				dist[i-1][j]+1,      // deletion
				dist[i][j-1]+1,      // insertion
				dist[i-1][j-1]+cost, // substitution
			)
		}
	}
	return dist[rows-1][cols-1]
}

func min(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
	return c
}

// Confidence returns a value from 0 to 100 representing how close input matches target.
func Confidence(target, input string) int {
	if len(target) == 0 || len(input) == 0 {
		return 0
	}
	target = strings.ToUpper(target)
	input = strings.ToUpper(input)

	// Direct substring match (important for "K7" matching "K7ABC")
	if strings.Contains(target, input) {
		return 100 * len(input) / len(target)
	}

	dist := LevenshteinDistance(target, input)
	if dist >= len(target) {
		return 0
	}
	return 100 * (len(target) - dist) / len(target)
}
