package app

import (
	"errors"
	"fmt"

	chronoclock "github.com/SmitUplenchwar2687/Chrono/pkg/clock"
	"github.com/SmitUplenchwar2687/Chrono/pkg/limiter"
)

// StorageLimiterSet holds storage-backed limiters for memory/redis/crdt validation.
type StorageLimiterSet struct {
	Memory limiter.Limiter
	Redis  limiter.Limiter
	CRDT   limiter.Limiter

	RedisErr error
	CRDTErr  error
}

func NewStorageLimiterSet(cfg Config, clk chronoclock.Clock) *StorageLimiterSet {
	memLimiter, err := NewLimiter(cfg, clk)
	if err != nil {
		return &StorageLimiterSet{
			RedisErr: fmt.Errorf("memory limiter init failed: %w", err),
			CRDTErr:  fmt.Errorf("memory limiter init failed: %w", err),
		}
	}

	unsupported := errors.New("backend unavailable: current Chrono SDK release exposes only in-memory storage primitives")
	return &StorageLimiterSet{
		Memory:   memLimiter,
		RedisErr: unsupported,
		CRDTErr:  unsupported,
	}
}

func (s *StorageLimiterSet) Close() error {
	return nil
}
