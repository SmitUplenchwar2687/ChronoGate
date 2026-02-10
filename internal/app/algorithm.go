package app

import "github.com/SmitUplenchwar2687/Chrono/pkg/limiter"

// AlgorithmFromString maps CLI strings to limiter algorithms.
func AlgorithmFromString(raw string, fallback limiter.Algorithm) limiter.Algorithm {
	switch limiter.Algorithm(raw) {
	case limiter.AlgorithmTokenBucket, limiter.AlgorithmSlidingWindow, limiter.AlgorithmFixedWindow:
		return limiter.Algorithm(raw)
	default:
		return fallback
	}
}
