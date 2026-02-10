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

func TestRecorderCapturesTrafficAndExportsJSON(t *testing.T) {
	vc := chronoclock.NewVirtualClock(time.Date(2026, 2, 8, 10, 0, 0, 0, time.UTC))
	cfg := mustTestConfig(limiter.AlgorithmFixedWindow)
	cfg.Rate = 10
	cfg.Burst = 10

	mainLimiter, mainStorage, err := NewStorageBackedLimiter(cfg, vc)
	if err != nil {
		t.Fatalf("NewStorageBackedLimiter() error = %v", err)
	}
	defer mainStorage.Close()

	storageSet, cleanup := newMemoryOnlyStorageSet(t, cfg, vc)
	defer cleanup()

	rec := chronorecorder.New(nil)
	handler := NewHandler(cfg, mainLimiter, vc, rec, storageSet)

	resp1 := executeRequest(handler, http.MethodGet, "/public", "", "", "", "198.51.100.7:4123")
	assertStatus(t, resp1, http.StatusOK)

	resp2 := executeRequest(handler, http.MethodGet, "/api/profile", "sdk-client", "", "", "198.51.100.7:4123")
	assertStatus(t, resp2, http.StatusOK)

	if rec.Len() < 1 {
		t.Fatalf("recorder length = %d, want at least 1", rec.Len())
	}

	exportResp := executeRequest(handler, http.MethodGet, "/api/recordings/export", "", "", "", "198.51.100.7:4123")
	assertStatus(t, exportResp, http.StatusOK)

	var records []chronorecorder.TrafficRecord
	if err := json.Unmarshal(exportResp.Body.Bytes(), &records); err != nil {
		t.Fatalf("unmarshal export response: %v", err)
	}

	if len(records) == 0 {
		t.Fatal("exported records should not be empty")
	}

	found := false
	for _, record := range records {
		if record.Endpoint == "GET /api/profile" && record.Key == "sdk-client" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected exported records to contain GET /api/profile for key sdk-client")
	}
}
