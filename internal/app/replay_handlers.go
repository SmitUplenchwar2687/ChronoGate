package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/SmitUplenchwar2687/Chrono/pkg/limiter"
	chronorecorder "github.com/SmitUplenchwar2687/Chrono/pkg/recorder"
)

type replayRequest struct {
	Traffic   []chronorecorder.TrafficRecord `json:"traffic"`
	Algorithm string                         `json:"algorithm"`
	Rate      int                            `json:"rate"`
	Window    string                         `json:"window"`
	Burst     int                            `json:"burst"`
	Speed     float64                        `json:"speed"`
	Keys      []string                       `json:"keys"`
	Endpoints []string                       `json:"endpoints"`
}

func parseReplayRequest(r *http.Request, defaults Config) (ReplayOptions, []chronorecorder.TrafficRecord, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return ReplayOptions{}, nil, fmt.Errorf("read request body: %w", err)
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return ReplayOptions{}, nil, fmt.Errorf("request body is required")
	}

	opts := ReplayOptions{
		Algorithm: defaults.Algorithm,
		Rate:      defaults.Rate,
		Window:    defaults.Window,
		Burst:     defaults.Burst,
		Speed:     0,
	}

	if trimmed[0] == '[' {
		var records []chronorecorder.TrafficRecord
		if err := json.Unmarshal(trimmed, &records); err != nil {
			return ReplayOptions{}, nil, fmt.Errorf("decode replay traffic array: %w", err)
		}
		if len(records) == 0 {
			return ReplayOptions{}, nil, fmt.Errorf("traffic records cannot be empty")
		}
		return opts, records, nil
	}

	var req replayRequest
	if err := json.Unmarshal(trimmed, &req); err != nil {
		return ReplayOptions{}, nil, fmt.Errorf("decode replay request: %w", err)
	}

	if strings.TrimSpace(req.Algorithm) != "" {
		algo, err := ParseAlgorithm(strings.TrimSpace(req.Algorithm))
		if err != nil {
			return ReplayOptions{}, nil, err
		}
		opts.Algorithm = algo
	}
	if req.Rate > 0 {
		opts.Rate = req.Rate
	}
	if req.Burst > 0 {
		opts.Burst = req.Burst
	}
	if strings.TrimSpace(req.Window) != "" {
		d, err := time.ParseDuration(strings.TrimSpace(req.Window))
		if err != nil {
			return ReplayOptions{}, nil, fmt.Errorf("invalid window %q: %w", req.Window, err)
		}
		opts.Window = d
	}
	if req.Speed >= 0 {
		opts.Speed = req.Speed
	}
	opts.Keys = append([]string(nil), req.Keys...)
	opts.Endpoints = append([]string(nil), req.Endpoints...)

	if len(req.Traffic) == 0 {
		return ReplayOptions{}, nil, fmt.Errorf("traffic records cannot be empty")
	}

	checkCfg := defaults
	checkCfg.Algorithm = limiter.Algorithm(opts.Algorithm)
	checkCfg.Rate = opts.Rate
	checkCfg.Window = opts.Window
	checkCfg.Burst = opts.Burst
	if err := checkCfg.Validate(); err != nil {
		return ReplayOptions{}, nil, err
	}

	return opts, req.Traffic, nil
}
