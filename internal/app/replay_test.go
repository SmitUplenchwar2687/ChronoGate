package app

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SmitUplenchwar2687/Chrono/pkg/limiter"
	chronorecorder "github.com/SmitUplenchwar2687/Chrono/pkg/recorder"
)

func TestRunReplaySummary(t *testing.T) {
	start := time.Date(2026, 2, 8, 12, 0, 0, 0, time.UTC)
	records := []chronorecorder.TrafficRecord{
		{Timestamp: start, Key: "k1", Endpoint: "GET /api/profile"},
		{Timestamp: start.Add(time.Second), Key: "k1", Endpoint: "GET /api/profile"},
		{Timestamp: start.Add(2 * time.Second), Key: "k1", Endpoint: "GET /api/profile"},
		{Timestamp: start.Add(3 * time.Second), Key: "k2", Endpoint: "GET /api/profile"},
	}

	var file bytes.Buffer
	rec := chronorecorder.New(nil)
	for _, record := range records {
		if err := rec.Record(record); err != nil {
			t.Fatalf("Record() error = %v", err)
		}
	}
	if err := rec.ExportJSON(&file); err != nil {
		t.Fatalf("ExportJSON() error = %v", err)
	}

	tempFile := t.TempDir() + "/records.json"
	if err := os.WriteFile(tempFile, file.Bytes(), 0o600); err != nil {
		t.Fatalf("write temp replay file: %v", err)
	}

	var output bytes.Buffer
	summary, err := RunReplay(context.Background(), ReplayOptions{
		File:      tempFile,
		Algorithm: limiter.AlgorithmFixedWindow,
		Rate:      2,
		Window:    time.Minute,
		Burst:     2,
		Speed:     0,
	}, &output)
	if err != nil {
		t.Fatalf("RunReplay() error = %v", err)
	}

	if summary.TotalRecords != 4 {
		t.Fatalf("TotalRecords = %d, want 4", summary.TotalRecords)
	}
	if summary.Replayed != 4 {
		t.Fatalf("Replayed = %d, want 4", summary.Replayed)
	}
	if summary.Allowed != 3 || summary.Denied != 1 {
		t.Fatalf("Allowed/Denied = %d/%d, want 3/1", summary.Allowed, summary.Denied)
	}
	if summary.PerKey["k1"].Denied != 1 {
		t.Fatalf("PerKey[k1].Denied = %d, want 1", summary.PerKey["k1"].Denied)
	}
	if summary.PerKey["k2"].Allowed != 1 {
		t.Fatalf("PerKey[k2].Allowed = %d, want 1", summary.PerKey["k2"].Allowed)
	}

	out := output.String()
	if !strings.Contains(out, "Total: 4") || !strings.Contains(out, "k1: allowed=2 denied=1") {
		t.Fatalf("unexpected replay output:\n%s", out)
	}
}
