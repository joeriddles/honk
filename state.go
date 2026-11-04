package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// State lives on disk, so a restart does not honk again for old actions.
type State struct {
	Seen      map[string]time.Time `json:"seen"` // Action.Key -> first time seen
	Muted     bool                 `json:"muted"`
	LastAlert time.Time            `json:"last_alert"` // the last honk, chirp, or reminder

	path string
}

func stateDir() string {
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "honk")
	}
	return filepath.Join(os.TempDir(), "honk")
}

func LoadState() (*State, error) {
	s := &State{Seen: map[string]time.Time{}, path: filepath.Join(stateDir(), "state.json")}
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(raw, s); err != nil {
		return s, err
	}
	if s.Seen == nil {
		s.Seen = map[string]time.Time{}
	}
	return s, nil
}

// Update records the current actions and returns the ones not seen before.
// Actions that went away are forgotten, so they honk again if they come back.
func (s *State) Update(actions []Action, now time.Time) []Action {
	var fresh []Action
	next := make(map[string]time.Time, len(actions))
	for _, a := range actions {
		k := a.Key()
		if t, ok := s.Seen[k]; ok {
			next[k] = t
			continue
		}
		next[k] = now
		fresh = append(fresh, a)
	}
	s.Seen = next
	return fresh
}

func (s *State) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(s, "", "  ")
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
