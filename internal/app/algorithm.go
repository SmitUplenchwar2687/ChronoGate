package app

import (
	"fmt"

	"github.com/SmitUplenchwar2687/Chrono/pkg/limiter"
)

// AlgorithmFromString maps CLI strings to limiter algorithms.
func AlgorithmFromString(raw string, fallback limiter.Algorithm) limiter.Algorithm {
	switch limiter.Algorithm(raw) {
	case limiter.AlgorithmTokenBucket, limiter.AlgorithmSlidingWindow, limiter.AlgorithmFixedWindow:
		return limiter.Algorithm(raw)
	default:
		return fallback
	}
}

// ParseAlgorithm validates and parses a limiter algorithm string.
func ParseAlgorithm(raw string) (limiter.Algorithm, error) {
	algo := limiter.Algorithm(raw)
	switch algo {
	case limiter.AlgorithmTokenBucket, limiter.AlgorithmSlidingWindow, limiter.AlgorithmFixedWindow:
		return algo, nil
	default:
		return "", fmt.Errorf("invalid algorithm %q", raw)
	}
}
