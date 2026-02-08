package app

import (
	"fmt"

	chronoclock "github.com/SmitUplenchwar2687/Chrono/pkg/clock"
	"github.com/SmitUplenchwar2687/Chrono/pkg/limiter"
)

// NewLimiter builds a limiter implementation from runtime config.
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
