package app

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	chronoclock "github.com/SmitUplenchwar2687/Chrono/pkg/clock"
	"github.com/SmitUplenchwar2687/Chrono/pkg/limiter"
	chronorecorder "github.com/SmitUplenchwar2687/Chrono/pkg/recorder"
)

func TestStorageMemoryEndpointRateLimit(t *testing.T) {
	vc := chronoclock.NewVirtualClock(time.Date(2026, 2, 8, 12, 0, 0, 0, time.UTC))
	cfg := mustTestConfig(limiter.AlgorithmFixedWindow)
	cfg.Rate = 1

	mainLimiter, mainStorage, err := NewStorageBackedLimiter(cfg, vc)
	if err != nil {
		t.Fatalf("NewStorageBackedLimiter() error = %v", err)
	}
	defer mainStorage.Close()

	storageSet, cleanup := newMemoryOnlyStorageSet(t, cfg, vc)
	defer cleanup()

	handler := NewHandler(cfg, mainLimiter, vc, chronorecorder.New(nil), storageSet)

	resp1 := executeRequest(handler, http.MethodGet, "/api/storage/memory", "storage-key", "", "", "198.51.100.21:5000")
	if resp1.Code != http.StatusOK && resp1.Code != http.StatusTooManyRequests {
		t.Fatalf("unexpected status: %d", resp1.Code)
	}

	resp2 := executeRequest(handler, http.MethodGet, "/api/storage/memory", "storage-key", "", "", "198.51.100.21:5000")
	assertStatus(t, resp2, http.StatusTooManyRequests)
}

func TestStorageCompareCRDTNotePresent(t *testing.T) {
	vc := chronoclock.NewVirtualClock(time.Date(2026, 2, 8, 12, 0, 0, 0, time.UTC))
	cfg := mustTestConfig(limiter.AlgorithmFixedWindow)

	mainLimiter, mainStorage, err := NewStorageBackedLimiter(cfg, vc)
	if err != nil {
		t.Fatalf("NewStorageBackedLimiter() error = %v", err)
	}
	defer mainStorage.Close()

	storageSet, cleanup := newMemoryOnlyStorageSet(t, cfg, vc)
	defer cleanup()

	handler := NewHandler(cfg, mainLimiter, vc, chronorecorder.New(nil), storageSet)
	resp := executeRequest(handler, http.MethodGet, "/api/storage/compare", "cmp-user", "", "", "198.51.100.21:5000")
	assertStatus(t, resp, http.StatusOK)

	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode compare response: %v", err)
	}

	results, ok := body["results"].(map[string]any)
	if !ok {
		t.Fatalf("results has wrong type: %T", body["results"])
	}
	crdt, ok := results["crdt"].(map[string]any)
	if !ok {
		t.Fatalf("crdt has wrong type: %T", results["crdt"])
	}
	note, _ := crdt["note"].(string)
	if note == "" {
		t.Fatal("expected crdt compare note to be present")
	}
}
