package app

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	chronoclock "github.com/SmitUplenchwar2687/Chrono/pkg/clock"
	"github.com/SmitUplenchwar2687/Chrono/pkg/limiter"
	chronorecorder "github.com/SmitUplenchwar2687/Chrono/pkg/recorder"
	chronostorage "github.com/SmitUplenchwar2687/Chrono/pkg/storage"
)

func TestRecorderCapturesTrafficAndExportsJSON(t *testing.T) {
	vc := chronoclock.NewVirtualClock(time.Date(2026, 2, 8, 10, 0, 0, 0, time.UTC))
	cfg := Config{
		Algorithm: limiter.AlgorithmFixedWindow,
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

	resp1 := executeRequest(handler, http.MethodGet, "/public", "", "", "", "198.51.100.7:4123")
	assertStatus(t, resp1, http.StatusOK)

	resp2 := executeRequest(handler, http.MethodGet, "/api/profile", "sdk-client", "", "", "198.51.100.7:4123")
	assertStatus(t, resp2, http.StatusOK)

	if rec.Len() < 2 {
		t.Fatalf("recorder length = %d, want at least 2", rec.Len())
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
