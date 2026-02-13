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

// Config is ChronoGate runtime configuration resolved from Chrono config file + env.
type Config struct {
	ConfigPath string

	Addr      string
	Algorithm limiter.Algorithm
	Rate      int
	Window    time.Duration
	Burst     int

	StorageBackend string
}

const (
	StorageBackendMemory = "memory"
	StorageBackendRedis  = "redis"
	StorageBackendCRDT   = "crdt"
)

// LoadConfig resolves configuration from Chrono defaults, optional config file,
// and environment overrides.
func LoadConfig(configPath string) (Config, error) {
	chronoCfg := chronoconfig.Default()
	if strings.TrimSpace(configPath) != "" {
		loaded, err := chronoconfig.LoadFile(configPath)
		if err != nil {
			return Config{}, fmt.Errorf("load config file: %w", err)
		}
		chronoCfg = loaded
	}

	if err := chronoCfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate chrono config: %w", err)
	}

	cfg := Config{
		ConfigPath:     configPath,
		Addr:           chronoCfg.Server.Addr,
		Algorithm:      chronoCfg.Limiter.Algorithm,
		Rate:           chronoCfg.Limiter.Rate,
		Window:         chronoCfg.Limiter.Window,
		Burst:          chronoCfg.Limiter.Burst,
		StorageBackend: StorageBackendMemory,
	}

	if raw := strings.TrimSpace(os.Getenv("ADDR")); raw != "" {
		cfg.Addr = raw
	}
	if raw := strings.TrimSpace(os.Getenv("ALGORITHM")); raw != "" {
		cfg.Algorithm = limiter.Algorithm(raw)
	}
	if raw := strings.TrimSpace(os.Getenv("WINDOW")); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid WINDOW %q: %w", raw, err)
		}
		cfg.Window = d
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

	if raw := strings.TrimSpace(os.Getenv("STORAGE_BACKEND")); raw != "" {
		cfg.StorageBackend = raw
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Validate checks app-level configuration.
func (c Config) Validate() error {
	switch c.Algorithm {
	case limiter.AlgorithmTokenBucket, limiter.AlgorithmSlidingWindow, limiter.AlgorithmFixedWindow:
	default:
		return fmt.Errorf("invalid ALGORITHM %q", c.Algorithm)
	}

	if c.Rate <= 0 {
		return fmt.Errorf("RATE must be > 0, got %d", c.Rate)
	}
	if c.Window <= 0 {
		return fmt.Errorf("WINDOW must be > 0, got %s", c.Window)
	}
	if c.Burst <= 0 {
		return fmt.Errorf("BURST must be > 0, got %d", c.Burst)
	}
	if strings.TrimSpace(c.Addr) == "" {
		return fmt.Errorf("ADDR must not be empty")
	}

	switch c.StorageBackend {
	case StorageBackendMemory, StorageBackendRedis, StorageBackendCRDT:
	default:
		return fmt.Errorf("invalid STORAGE_BACKEND %q", c.StorageBackend)
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
