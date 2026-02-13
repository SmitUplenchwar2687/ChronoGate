package app

import (
	"fmt"

	chronoclock "github.com/SmitUplenchwar2687/Chrono/pkg/clock"
	"github.com/SmitUplenchwar2687/Chrono/pkg/limiter"
	chronostorage "github.com/SmitUplenchwar2687/Chrono/pkg/storage"
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

// NewStorageBackedLimiter builds a main limiter using Chrono storage factory + StorageLimiter.
func NewStorageBackedLimiter(cfg Config, clk chronoclock.Clock) (limiter.Limiter, chronostorage.Storage, error) {
	storageCfg := cfg.Storage
	storageCfg.Backend = cfg.StorageBackend
	injectClockIntoStorageConfig(&storageCfg, clk)

	backend, err := chronostorage.NewStorage(storageCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("create storage backend %q: %w", storageCfg.Backend, err)
	}

	lim, err := limiter.NewStorageLimiter(backend, cfg.Rate, cfg.Window, clk)
	if err != nil {
		_ = backend.Close()
		return nil, nil, fmt.Errorf("create storage limiter: %w", err)
	}

	return lim, backend, nil
}

func injectClockIntoStorageConfig(cfg *chronostorage.Config, clk chronoclock.Clock) {
	if cfg == nil {
		return
	}
	if cfg.Memory != nil {
		cfg.Memory.Clock = clk
		if cfg.Memory.Algorithm == "" {
			cfg.Memory.Algorithm = chronostorage.AlgorithmFixedWindow
		}
	}
	if cfg.Redis != nil {
		cfg.Redis.Clock = clk
	}
	if cfg.CRDT != nil {
		cfg.CRDT.Clock = clk
	}
}
