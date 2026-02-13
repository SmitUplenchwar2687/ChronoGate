package app

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	chronoconfig "github.com/SmitUplenchwar2687/Chrono/pkg/config"
	"github.com/SmitUplenchwar2687/Chrono/pkg/limiter"
	chronostorage "github.com/SmitUplenchwar2687/Chrono/pkg/storage"
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
	Storage        chronostorage.Config
}

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
		ConfigPath: configPath,
		Addr:       chronoCfg.Server.Addr,
		Algorithm:  chronoCfg.Limiter.Algorithm,
		Rate:       chronoCfg.Limiter.Rate,
		Window:     chronoCfg.Limiter.Window,
		Burst:      chronoCfg.Limiter.Burst,
		StorageBackend: func() string {
			if chronoCfg.Storage.Backend == "" {
				return chronostorage.BackendMemory
			}
			return chronoCfg.Storage.Backend
		}(),
		Storage: toStorageConfig(chronoCfg),
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
		cfg.Storage.Backend = raw
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func toStorageConfig(c chronoconfig.Config) chronostorage.Config {
	backend := c.Storage.Backend
	if backend == "" {
		backend = chronostorage.BackendMemory
	}

	memoryCfg := &chronostorage.MemoryConfig{
		CleanupInterval: c.Storage.Memory.CleanupInterval,
		Algorithm:       c.Storage.Memory.Algorithm,
		Burst:           c.Storage.Memory.Burst,
	}
	if memoryCfg.CleanupInterval <= 0 {
		memoryCfg.CleanupInterval = time.Minute
	}

	redisCfg := &chronostorage.RedisConfig{
		Host:         c.Storage.Redis.Host,
		Port:         c.Storage.Redis.Port,
		Password:     c.Storage.Redis.Password,
		DB:           c.Storage.Redis.DB,
		Cluster:      c.Storage.Redis.Cluster,
		ClusterNodes: append([]string(nil), c.Storage.Redis.ClusterNodes...),
		PoolSize:     c.Storage.Redis.PoolSize,
		MaxRetries:   c.Storage.Redis.MaxRetries,
		DialTimeout:  c.Storage.Redis.DialTimeout,
	}

	crdtCfg := &chronostorage.CRDTConfig{
		NodeID:         c.Storage.CRDT.NodeID,
		BindAddr:       c.Storage.CRDT.BindAddr,
		Peers:          append([]string(nil), c.Storage.CRDT.Peers...),
		GossipInterval: c.Storage.CRDT.GossipInterval,
	}

	if crdtCfg.NodeID == "" {
		crdtCfg.NodeID = "chronogate-crdt"
	}
	if crdtCfg.BindAddr == "" {
		crdtCfg.BindAddr = "127.0.0.1:0"
	}

	return chronostorage.Config{
		Backend: backend,
		Memory:  memoryCfg,
		Redis:   redisCfg,
		CRDT:    crdtCfg,
	}
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
	case chronostorage.BackendMemory, chronostorage.BackendRedis, chronostorage.BackendCRDT:
	default:
		return fmt.Errorf("invalid STORAGE_BACKEND %q", c.StorageBackend)
	}

	// Redis and CRDT storage implementations are sliding-window only.
	if (c.StorageBackend == chronostorage.BackendRedis || c.StorageBackend == chronostorage.BackendCRDT) &&
		c.Algorithm != limiter.AlgorithmSlidingWindow {
		return fmt.Errorf("algorithm %q is unsupported with %s backend; use %q", c.Algorithm, c.StorageBackend, limiter.AlgorithmSlidingWindow)
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
