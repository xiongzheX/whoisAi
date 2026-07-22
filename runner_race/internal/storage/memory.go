package storage

import (
	"errors"
	"sync"

	"runner_race/internal/runner"
	"runner_race/internal/sumo"
)

var ErrNotFound = errors.New("not found")

type MemoryStore struct {
	mu          sync.RWMutex
	runners     map[string]runner.Runner
	matches     map[string]runner.MatchResult
	wrestlers   map[string]sumo.Wrestler
	sumoMatches map[string]sumo.MatchResult
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		runners:     make(map[string]runner.Runner),
		matches:     make(map[string]runner.MatchResult),
		wrestlers:   make(map[string]sumo.Wrestler),
		sumoMatches: make(map[string]sumo.MatchResult),
	}
}

func (s *MemoryStore) SaveRunner(r runner.Runner) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runners[r.ID] = r
}

func (s *MemoryStore) Runner(id string) (runner.Runner, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.runners[id]
	if !ok {
		return runner.Runner{}, ErrNotFound
	}
	return r, nil
}

func (s *MemoryStore) ListRunners() []runner.Runner {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]runner.Runner, 0, len(s.runners))
	for _, r := range s.runners {
		out = append(out, r)
	}
	return out
}

func (s *MemoryStore) SaveMatch(m runner.MatchResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.matches[m.MatchID] = m
}

func (s *MemoryStore) Match(id string) (runner.MatchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.matches[id]
	if !ok {
		return runner.MatchResult{}, ErrNotFound
	}
	return m, nil
}

func (s *MemoryStore) SaveWrestler(w sumo.Wrestler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wrestlers[w.ID] = w
}

func (s *MemoryStore) Wrestler(id string) (sumo.Wrestler, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	w, ok := s.wrestlers[id]
	if !ok {
		return sumo.Wrestler{}, ErrNotFound
	}
	return w, nil
}

func (s *MemoryStore) ListWrestlers() []sumo.Wrestler {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]sumo.Wrestler, 0, len(s.wrestlers))
	for _, w := range s.wrestlers {
		out = append(out, w)
	}
	return out
}

func (s *MemoryStore) SaveSumoMatch(m sumo.MatchResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sumoMatches[m.MatchID] = m
}

func (s *MemoryStore) SumoMatch(id string) (sumo.MatchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.sumoMatches[id]
	if !ok {
		return sumo.MatchResult{}, ErrNotFound
	}
	return m, nil
}
