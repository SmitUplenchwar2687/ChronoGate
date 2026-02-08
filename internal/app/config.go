package app

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	chronoconfig "github.com/SmitUplenchwar2687/Chrono/pkg/config"
	"github.com/SmitUplenchwar2687/Chrono/pkg/limiter"
)

// Config holds ChronoGate runtime settings parsed from environment variables.
type Config struct {
	Algorithm limiter.Algorithm
	Rate      int
	Window    time.Duration
	Burst     int
	Addr      string
}

// LoadConfigFromEnv loads runtime config using Chrono's public config defaults,
// then applies env var overrides for ChronoGate.
func LoadConfigFromEnv() (Config, error) {
	base := chronoconfig.Default()
	cfg := Config{
		Algorithm: base.Limiter.Algorithm,
		Rate:      base.Limiter.Rate,
		Window:    base.Limiter.Window,
		Burst:     base.Limiter.Burst,
		Addr:      base.Server.Addr,
	}

	if raw := strings.TrimSpace(os.Getenv("ALGORITHM")); raw != "" {
		cfg.Algorithm = limiter.Algorithm(raw)
	}

	var err error
	cfg.Rate, err = parsePositiveIntEnv("RATE", cfg.Rate)
	if err != nil {
		return Config{}, err
	}

	cfg.Burst, err = parsePositiveIntEnv("BURST", cfg.Burst)
	if err != nil {
		return Config{}, err
	}

	if raw := strings.TrimSpace(os.Getenv("WINDOW")); raw != "" {
		window, parseErr := time.ParseDuration(raw)
		if parseErr != nil {
			return Config{}, fmt.Errorf("invalid WINDOW %q: %w", raw, parseErr)
		}
		cfg.Window = window
	}

	if raw := strings.TrimSpace(os.Getenv("ADDR")); raw != "" {
		cfg.Addr = raw
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Validate checks configuration values.
func (c Config) Validate() error {
	switch c.Algorithm {
	case limiter.AlgorithmTokenBucket, limiter.AlgorithmSlidingWindow, limiter.AlgorithmFixedWindow:
	default:
		return fmt.Errorf("invalid ALGORITHM %q (expected token_bucket, sliding_window, or fixed_window)", c.Algorithm)
	}

	if c.Rate <= 0 {
		return fmt.Errorf("RATE must be > 0, got %d", c.Rate)
	}
	if c.Window <= 0 {
		return fmt.Errorf("WINDOW must be > 0, got %s", c.Window)
	}
	if c.Burst <= 0 {
		if c.Algorithm == limiter.AlgorithmTokenBucket {
			return fmt.Errorf("BURST must be > 0 for token_bucket, got %d", c.Burst)
		}
		return fmt.Errorf("BURST must be > 0, got %d", c.Burst)
	}
	if strings.TrimSpace(c.Addr) == "" {
		return fmt.Errorf("ADDR must not be empty")
	}

	return nil
}

func parsePositiveIntEnv(name string, defaultValue int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return defaultValue, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, raw, err)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be > 0, got %d", name, value)
	}

	return value, nil
}
