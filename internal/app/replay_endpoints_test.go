package app

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	chronoclock "github.com/SmitUplenchwar2687/Chrono/pkg/clock"
	"github.com/SmitUplenchwar2687/Chrono/pkg/limiter"
	chronorecorder "github.com/SmitUplenchwar2687/Chrono/pkg/recorder"
	chronoreplay "github.com/SmitUplenchwar2687/Chrono/pkg/replay"
)

func TestReplayLastNotFoundBeforeAnyReplay(t *testing.T) {
	vc := chronoclock.NewVirtualClock(time.Date(2026, 2, 8, 14, 0, 0, 0, time.UTC))
	cfg := mustTestConfig(limiter.AlgorithmFixedWindow)

	mainLimiter, mainStorage, err := NewStorageBackedLimiter(cfg, vc)
	if err != nil {
		t.Fatalf("NewStorageBackedLimiter() error = %v", err)
	}
	defer mainStorage.Close()

	storageSet, cleanup := newMemoryOnlyStorageSet(t, cfg, vc)
	defer cleanup()

	handler := NewHandler(cfg, mainLimiter, vc, chronorecorder.New(nil), storageSet)
	resp := executeRequest(handler, http.MethodGet, "/api/replay/last", "", "", "", "198.51.100.40:8080")
	assertStatus(t, resp, http.StatusNotFound)
}

func TestRecordStopReplayAndReplayLast(t *testing.T) {
	vc := chronoclock.NewVirtualClock(time.Date(2026, 2, 8, 14, 0, 0, 0, time.UTC))
	cfg := mustTestConfig(limiter.AlgorithmFixedWindow)
	cfg.Rate = 5

	mainLimiter, mainStorage, err := NewStorageBackedLimiter(cfg, vc)
	if err != nil {
		t.Fatalf("NewStorageBackedLimiter() error = %v", err)
	}
	defer mainStorage.Close()

	storageSet, cleanup := newMemoryOnlyStorageSet(t, cfg, vc)
	defer cleanup()

	handler := NewHandler(cfg, mainLimiter, vc, chronorecorder.New(nil), storageSet)

	startResp := executeRequest(handler, http.MethodPost, "/api/record/start", "", "", "", "198.51.100.41:8080")
	assertStatus(t, startResp, http.StatusOK)

	profileResp := executeRequest(handler, http.MethodGet, "/api/profile", "replay-key", "", "", "198.51.100.41:8080")
	assertStatus(t, profileResp, http.StatusOK)

	ordersResp := executeRequest(handler, http.MethodPost, "/api/orders", "replay-key", "", `{"item":"book"}`, "198.51.100.41:8080")
	assertStatus(t, ordersResp, http.StatusCreated)

	stopResp := executeRequest(handler, http.MethodPost, "/api/record/stop", "", "", "", "198.51.100.41:8080")
	assertStatus(t, stopResp, http.StatusOK)

	var stopBody struct {
		Recording bool                           `json:"recording"`
		Count     int                            `json:"count"`
		Records   []chronorecorder.TrafficRecord `json:"records"`
	}
	if err := json.Unmarshal(stopResp.Body.Bytes(), &stopBody); err != nil {
		t.Fatalf("decode /api/record/stop response: %v", err)
	}
	if stopBody.Recording {
		t.Fatal("recording should be false after /api/record/stop")
	}
	if stopBody.Count == 0 || len(stopBody.Records) == 0 {
		t.Fatal("expected non-empty recording export from /api/record/stop")
	}

	replayPayload := map[string]any{
		"traffic":   stopBody.Records,
		"algorithm": "fixed_window",
		"rate":      1,
		"window":    "1m",
		"burst":     1,
		"speed":     0,
		"endpoints": []string{"GET /api/profile"},
	}
	replayBody, err := json.Marshal(replayPayload)
	if err != nil {
		t.Fatalf("marshal replay payload: %v", err)
	}

	replayResp := executeRequest(handler, http.MethodPost, "/api/replay", "", "", string(replayBody), "198.51.100.41:8080")
	assertStatus(t, replayResp, http.StatusOK)

	var replayResult struct {
		Summary chronoreplay.Summary `json:"summary"`
	}
	if err := json.Unmarshal(replayResp.Body.Bytes(), &replayResult); err != nil {
		t.Fatalf("decode /api/replay response: %v", err)
	}
	if replayResult.Summary.Replayed != 1 {
		t.Fatalf("replay filtered count = %d, want 1", replayResult.Summary.Replayed)
	}
	if replayResult.Summary.Allowed != 1 || replayResult.Summary.Denied != 0 {
		t.Fatalf("unexpected replay allowed/denied = %d/%d", replayResult.Summary.Allowed, replayResult.Summary.Denied)
	}

	lastResp := executeRequest(handler, http.MethodGet, "/api/replay/last", "", "", "", "198.51.100.41:8080")
	assertStatus(t, lastResp, http.StatusOK)

	var lastBody struct {
		Summary chronoreplay.Summary `json:"summary"`
	}
	if err := json.Unmarshal(lastResp.Body.Bytes(), &lastBody); err != nil {
		t.Fatalf("decode /api/replay/last response: %v", err)
	}
	if lastBody.Summary.Replayed != replayResult.Summary.Replayed {
		t.Fatalf("last replay replayed=%d, want %d", lastBody.Summary.Replayed, replayResult.Summary.Replayed)
	}
}
