package app

import (
	"fmt"
	"io"

	chronoclock "github.com/SmitUplenchwar2687/Chrono/pkg/clock"
	"github.com/SmitUplenchwar2687/Chrono/pkg/limiter"
)

// NewLimiter builds a direct limiter implementation by algorithm.
func NewLimiter(cfg Config, clk chronoclock.Clock) (limiter.Limiter, error) {
	switch cfg.Algorithm {
	case limiter.AlgorithmTokenBucket:
		return limiter.NewTokenBucket(cfg.Rate, cfg.Window, cfg.Burst, clk), nil
	case limiter.AlgorithmSlidingWindow:
		return limiter.NewSlidingWindow(cfg.Rate, cfg.Window, clk), nil
	case limiter.AlgorithmFixedWindow:
		return limiter.NewFixedWindow(cfg.Rate, cfg.Window, clk), nil
	default:
		return nil, fmt.Errorf("unknown algorithm %q", cfg.Algorithm)
	}
}

// NewStorageBackedLimiter builds the main limiter.
// The published Chrono SDK version used by this project does not yet expose
// storage-backed limiter constructors, so this returns an algorithm limiter plus
// a no-op closer for call-site compatibility.
func NewStorageBackedLimiter(cfg Config, clk chronoclock.Clock) (limiter.Limiter, io.Closer, error) {
	lim, err := NewLimiter(cfg, clk)
	if err != nil {
		return nil, nil, err
	}
	return lim, noopCloser{}, nil
}

type noopCloser struct{}

func (noopCloser) Close() error { return nil }
