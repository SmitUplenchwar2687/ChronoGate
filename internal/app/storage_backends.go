package app

import (
	"fmt"
	"sync"
	"time"

	chronoclock "github.com/SmitUplenchwar2687/Chrono/pkg/clock"
	"github.com/SmitUplenchwar2687/Chrono/pkg/limiter"
	chronostorage "github.com/SmitUplenchwar2687/Chrono/pkg/storage"
)

// StorageLimiterSet holds storage-backed limiters for memory/redis/crdt validation.
type StorageLimiterSet struct {
	Memory limiter.Limiter
	Redis  limiter.Limiter
	CRDT   limiter.Limiter

	memoryStore chronostorage.Storage
	redisStore  chronostorage.Storage
	crdtStore   chronostorage.Storage

	RedisErr error
	CRDTErr  error
}

func NewStorageLimiterSet(cfg Config, clk chronoclock.Clock) *StorageLimiterSet {
	set := &StorageLimiterSet{}

	memoryCfg := cfg.Storage
	memoryCfg.Backend = chronostorage.BackendMemory
	if memoryCfg.Memory == nil {
		memoryCfg.Memory = &chronostorage.MemoryConfig{}
	}
	memoryCfg.Memory.Algorithm = string(cfg.Algorithm)
	memoryCfg.Memory.Burst = cfg.Burst
	injectClockIntoStorageConfig(&memoryCfg, clk)

	memStore, err := chronostorage.NewStorage(memoryCfg)
	if err == nil {
		set.memoryStore = memStore
		memLimiter, limErr := limiter.NewStorageLimiter(memStore, cfg.Rate, cfg.Window, clk)
		if limErr == nil {
			set.Memory = memLimiter
		}
	}

	redisCfg := cfg.Storage
	redisCfg.Backend = chronostorage.BackendRedis
	injectClockIntoStorageConfig(&redisCfg, clk)
	redisStore, redisErr := chronostorage.NewStorage(redisCfg)
	if redisErr != nil {
		set.RedisErr = redisErr
	} else {
		set.redisStore = redisStore
		redisLimiter, limErr := limiter.NewStorageLimiter(redisStore, cfg.Rate, cfg.Window, clk)
		if limErr != nil {
			set.RedisErr = limErr
		} else {
			set.Redis = redisLimiter
		}
	}

	crdtCfg := cfg.Storage
	crdtCfg.Backend = chronostorage.BackendCRDT
	if crdtCfg.CRDT == nil {
		crdtCfg.CRDT = &chronostorage.CRDTConfig{}
	}
	if crdtCfg.CRDT.NodeID == "" {
		crdtCfg.CRDT.NodeID = fmt.Sprintf("chronogate-%d", time.Now().UnixNano())
	}
	if crdtCfg.CRDT.BindAddr == "" {
		crdtCfg.CRDT.BindAddr = "127.0.0.1:0"
	}
	injectClockIntoStorageConfig(&crdtCfg, clk)
	crdtStore, crdtErr := chronostorage.NewStorage(crdtCfg)
	if crdtErr != nil {
		set.CRDTErr = crdtErr
	} else {
		set.crdtStore = crdtStore
		crdtLimiter, limErr := limiter.NewStorageLimiter(crdtStore, cfg.Rate, cfg.Window, clk)
		if limErr != nil {
			set.CRDTErr = limErr
		} else {
			set.CRDT = crdtLimiter
		}
	}

	return set
}

func (s *StorageLimiterSet) Close() error {
	stores := []chronostorage.Storage{s.memoryStore, s.redisStore, s.crdtStore}
	var (
		mu   sync.Mutex
		errL []error
		wg   sync.WaitGroup
	)

	for _, st := range stores {
		if st == nil {
			continue
		}
		wg.Add(1)
		go func(store chronostorage.Storage) {
			defer wg.Done()
			if err := store.Close(); err != nil {
				mu.Lock()
				errL = append(errL, err)
				mu.Unlock()
			}
		}(st)
	}
	wg.Wait()

	if len(errL) == 0 {
		return nil
	}
	return fmt.Errorf("close storage backends: %v", errL)
}
