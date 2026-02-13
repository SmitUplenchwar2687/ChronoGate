package app

import (
	"io"
	"sync"

	chronorecorder "github.com/SmitUplenchwar2687/Chrono/pkg/recorder"
)

// RecordingState controls request recording lifecycle for ChronoGate.
type RecordingState struct {
	mu      sync.RWMutex
	enabled bool
	rec     *chronorecorder.Recorder
}

func NewRecordingState(initial *chronorecorder.Recorder, enabled bool) *RecordingState {
	if initial == nil {
		initial = chronorecorder.New(nil)
	}
	return &RecordingState{enabled: enabled, rec: initial}
}

func (s *RecordingState) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rec = chronorecorder.New(nil)
	s.enabled = true
}

func (s *RecordingState) Stop() []chronorecorder.TrafficRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = false
	if s.rec == nil {
		return nil
	}
	return s.rec.Records()
}

func (s *RecordingState) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

func (s *RecordingState) Record(rec chronorecorder.TrafficRecord) error {
	s.mu.RLock()
	enabled := s.enabled
	active := s.rec
	s.mu.RUnlock()

	if !enabled || active == nil {
		return nil
	}
	return active.Record(rec)
}

func (s *RecordingState) ExportJSON(w io.Writer) error {
	s.mu.RLock()
	active := s.rec
	s.mu.RUnlock()
	if active == nil {
		return nil
	}
	return active.ExportJSON(w)
}

func (s *RecordingState) Records() []chronorecorder.TrafficRecord {
	s.mu.RLock()
	active := s.rec
	s.mu.RUnlock()
	if active == nil {
		return nil
	}
	return active.Records()
}

func (s *RecordingState) Len() int {
	s.mu.RLock()
	active := s.rec
	s.mu.RUnlock()
	if active == nil {
		return 0
	}
	return active.Len()
}
