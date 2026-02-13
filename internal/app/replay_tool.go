package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	chronoclock "github.com/SmitUplenchwar2687/Chrono/pkg/clock"
	"github.com/SmitUplenchwar2687/Chrono/pkg/limiter"
	chronorecorder "github.com/SmitUplenchwar2687/Chrono/pkg/recorder"
	chronoreplay "github.com/SmitUplenchwar2687/Chrono/pkg/replay"
)

// ReplayOptions configures replay execution.
type ReplayOptions struct {
	File      string
	Algorithm limiter.Algorithm
	Rate      int
	Window    time.Duration
	Burst     int
	Speed     float64
	Keys      []string
	Endpoints []string
}

// RunReplay loads recorded traffic from file, replays it through the selected limiter,
// and prints summary stats.
func RunReplay(ctx context.Context, opts ReplayOptions, out io.Writer) (*chronoreplay.Summary, error) {
	if strings.TrimSpace(opts.File) == "" {
		return nil, fmt.Errorf("replay file is required")
	}

	f, err := os.Open(opts.File)
	if err != nil {
		return nil, fmt.Errorf("open replay file: %w", err)
	}
	defer f.Close()

	records, err := chronorecorder.LoadJSON(f)
	if err != nil {
		return nil, fmt.Errorf("load records: %w", err)
	}

	return RunReplayRecords(ctx, records, opts, out)
}

// RunReplayRecords replays in-memory traffic records and prints summary stats.
func RunReplayRecords(ctx context.Context, records []chronorecorder.TrafficRecord, opts ReplayOptions, out io.Writer) (*chronoreplay.Summary, error) {
	if len(records) == 0 {
		return nil, fmt.Errorf("no records provided")
	}

	sorted := append([]chronorecorder.TrafficRecord(nil), records...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	vc := chronoclock.NewVirtualClock(sorted[0].Timestamp)
	lim, err := NewLimiter(Config{
		Algorithm:      opts.Algorithm,
		Rate:           opts.Rate,
		Window:         opts.Window,
		Burst:          opts.Burst,
		Addr:           ":0",
		StorageBackend: "memory",
	}, vc)
	if err != nil {
		return nil, fmt.Errorf("create limiter: %w", err)
	}

	replayer := chronoreplay.New(lim, vc, opts.Speed, &chronoreplay.Filter{
		Keys:      opts.Keys,
		Endpoints: opts.Endpoints,
	})
	replayer.LoadRecords(sorted)

	summary, err := replayer.Run(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("run replay: %w", err)
	}

	if out == nil {
		out = io.Discard
	}

	fmt.Fprintf(out, "Total: %d\n", summary.TotalRecords)
	fmt.Fprintf(out, "Replayed: %d\n", summary.Replayed)
	fmt.Fprintf(out, "Allowed: %d\n", summary.Allowed)
	fmt.Fprintf(out, "Denied: %d\n", summary.Denied)
	fmt.Fprintf(out, "Per-key:\n")

	keys := make([]string, 0, len(summary.PerKey))
	for key := range summary.PerKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		ks := summary.PerKey[key]
		fmt.Fprintf(out, "  %s: allowed=%d denied=%d\n", key, ks.Allowed, ks.Denied)
	}

	return summary, nil
}
