package app

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	chronoclock "github.com/SmitUplenchwar2687/Chrono/pkg/clock"
	"github.com/SmitUplenchwar2687/Chrono/pkg/limiter"
	chronorecorder "github.com/SmitUplenchwar2687/Chrono/pkg/recorder"
	chronostorage "github.com/SmitUplenchwar2687/Chrono/pkg/storage"
)

func TestMemoryStorageExpiration(t *testing.T) {
	vc := chronoclock.NewVirtualClock(time.Date(2026, 2, 8, 12, 0, 0, 0, time.UTC))
	store := chronostorage.NewMemoryStorage(vc)

	if err := store.Set(context.Background(), "feature", []byte("enabled"), 2*time.Second); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	value, err := store.Get(context.Background(), "feature")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(value) != "enabled" {
		t.Fatalf("Get() = %q, want enabled", string(value))
	}

	vc.Advance(3 * time.Second)

	expiredValue, err := store.Get(context.Background(), "feature")
	if err != nil {
		t.Fatalf("Get() after expiry error = %v", err)
	}
	if expiredValue != nil {
		t.Fatalf("value after expiry = %q, want nil", string(expiredValue))
	}
}

func TestStorageDemoEndpointReadWriteExpiry(t *testing.T) {
	vc := chronoclock.NewVirtualClock(time.Date(2026, 2, 8, 12, 0, 0, 0, time.UTC))
	cfg := Config{
		Algorithm: limiter.AlgorithmTokenBucket,
		Rate:      10,
		Window:    time.Minute,
		Burst:     10,
		Addr:      ":0",
	}

	lim, err := NewLimiter(cfg, vc)
	if err != nil {
		t.Fatalf("NewLimiter() error = %v", err)
	}

	rec := chronorecorder.New(nil)
	store := chronostorage.NewMemoryStorage(vc)
	handler := NewHandler(lim, vc, rec, store)

	writeResp := executeRequest(handler, http.MethodPut, "/api/storage/demo", "", "", `{"key":"demo","value":"on","ttl":"1s"}`, "198.51.100.21:5000")
	assertStatus(t, writeResp, http.StatusOK)

	readResp := executeRequest(handler, http.MethodGet, "/api/storage/demo?key=demo", "", "", "", "198.51.100.21:5000")
	assertStatus(t, readResp, http.StatusOK)

	var readBody map[string]any
	if err := json.Unmarshal(readResp.Body.Bytes(), &readBody); err != nil {
		t.Fatalf("decode read response: %v", err)
	}
	if exists, ok := readBody["exists"].(bool); !ok || !exists {
		t.Fatalf("exists = %v, want true", readBody["exists"])
	}

	vc.Advance(2 * time.Second)

	expiredResp := executeRequest(handler, http.MethodGet, "/api/storage/demo?key=demo", "", "", "", "198.51.100.21:5000")
	assertStatus(t, expiredResp, http.StatusOK)

	var expiredBody map[string]any
	if err := json.Unmarshal(expiredResp.Body.Bytes(), &expiredBody); err != nil {
		t.Fatalf("decode expired response: %v", err)
	}
	if exists, ok := expiredBody["exists"].(bool); !ok || exists {
		t.Fatalf("exists after expiry = %v, want false", expiredBody["exists"])
	}
}
