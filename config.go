package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Settings are what you change in the Settings menu. honk saves them in
// config.json and uses the new values at once.
type Settings struct {
	PollMinutes   int      `json:"poll_minutes"`
	RemindMinutes int      `json:"remind_minutes"` // 0 turns reminders off
	RemindDays    []int    `json:"remind_days"`    // time.Weekday: 0 is Sunday, 6 is Saturday
	WorkStart     int      `json:"work_start"`     // reminders come from this hour (0-23)...
	WorkEnd       int      `json:"work_end"`       // ...to before this hour (1-24)
	StaleDays     int      `json:"stale_days"`     // 0 keeps all PRs
	TeamHonk      bool     `json:"team_honk"`
	ExcludeOrgs   []string `json:"exclude_orgs"`
}

func DefaultSettings() Settings {
	return Settings{
		PollMinutes:   2,
		RemindMinutes: 120,
		RemindDays:    []int{1, 2, 3, 4, 5},
		WorkStart:     9,
		WorkEnd:       18,
		StaleDays:     60,
	}
}

func settingsPath() string { return filepath.Join(stateDir(), "config.json") }

// LoadSettings reads config.json. A missing file gives the defaults. A field
// that is missing from the file keeps its default.
func LoadSettings() (Settings, error) {
	s := DefaultSettings()
	raw, err := os.ReadFile(settingsPath())
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return DefaultSettings(), err
	}
	return s.clean(), nil
}

func (s Settings) Save() error {
	if err := os.MkdirAll(stateDir(), 0o700); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(s, "", "  ")
	tmp := settingsPath() + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, settingsPath())
}

// clean puts each value back in its range, so a bad edit of config.json
// cannot break honk.
func (s Settings) clean() Settings {
	d := DefaultSettings()
	if s.PollMinutes < 1 {
		s.PollMinutes = d.PollMinutes
	}
	s.RemindMinutes = max(s.RemindMinutes, 0)
	s.StaleDays = max(s.StaleDays, 0)
	if s.WorkStart < 0 || s.WorkEnd > 24 || s.WorkStart >= s.WorkEnd {
		s.WorkStart, s.WorkEnd = d.WorkStart, d.WorkEnd
	}
	days := []int{}
	for _, wd := range s.RemindDays {
		if wd >= 0 && wd <= 6 && !slices.Contains(days, wd) {
			days = append(days, wd)
		}
	}
	slices.Sort(days)
	s.RemindDays = days
	orgs := []string{}
	for _, o := range s.ExcludeOrgs {
		if o = strings.TrimSpace(o); o != "" && !containsFold(orgs, o) {
			orgs = append(orgs, o)
		}
	}
	s.ExcludeOrgs = orgs
	return s
}

func (s Settings) Interval() time.Duration { return time.Duration(s.PollMinutes) * time.Minute }

func (s Settings) Options() Options {
	return Options{StaleAfter: time.Duration(s.StaleDays) * 24 * time.Hour, TeamHonk: s.TeamHonk}
}

func (s Settings) Reminder() Reminder {
	r := Reminder{Every: time.Duration(s.RemindMinutes) * time.Minute, Start: s.WorkStart, End: s.WorkEnd}
	for _, wd := range s.RemindDays {
		r.Days[wd] = true
	}
	return r
}
