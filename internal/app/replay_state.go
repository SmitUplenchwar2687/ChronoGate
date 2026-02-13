package app

import (
	"sync"

	chronoreplay "github.com/SmitUplenchwar2687/Chrono/pkg/replay"
)

// ReplayState stores the most recent replay summary.
type ReplayState struct {
	mu      sync.RWMutex
	summary *chronoreplay.Summary
}

func NewReplayState() *ReplayState {
	return &ReplayState{}
}

func (s *ReplayState) Set(summary *chronoreplay.Summary) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.summary = cloneSummary(summary)
}

func (s *ReplayState) Get() (*chronoreplay.Summary, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.summary == nil {
		return nil, false
	}
	return cloneSummary(s.summary), true
}

func cloneSummary(in *chronoreplay.Summary) *chronoreplay.Summary {
	if in == nil {
		return nil
	}
	out := *in
	if in.PerKey != nil {
		out.PerKey = make(map[string]chronoreplay.KeySummary, len(in.PerKey))
		for k, v := range in.PerKey {
			out.PerKey[k] = v
		}
	}
	return &out
}
