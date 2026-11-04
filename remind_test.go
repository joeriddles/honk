package main

import (
	"testing"
	"time"
)

func TestReminderDue(t *testing.T) {
	r := DefaultSettings().Reminder()                                                         // every 2 hours, Monday to Friday, 9 to 18
	tue := func(h, m int) time.Time { return time.Date(2026, 11, 3, h, m, 0, 0, time.Local) } // a Tuesday
	sat := time.Date(2026, 11, 7, 12, 0, 0, 0, time.Local)

	allWeek := r
	allWeek.Days = [7]bool{true, true, true, true, true, true, true}
	allDay := r
	allDay.Start, allDay.End = 0, 24

	cases := []struct {
		name    string
		r       Reminder
		now     time.Time
		last    time.Time
		pending int
		want    bool
	}{
		{"due", r, tue(13, 0), tue(11, 0), 3, true},
		{"too soon", r, tue(12, 59), tue(11, 0), 3, false},
		{"nothing waits", r, tue(13, 0), tue(11, 0), 0, false},
		{"before work", r, tue(8, 59), tue(1, 0), 3, false},
		{"first poll at nine", r, tue(9, 0), tue(1, 0), 3, true},
		{"after work", r, tue(18, 0), tue(11, 0), 3, false},
		{"after work, all day", allDay, tue(23, 0), tue(11, 0), 3, true},
		{"weekend", r, sat, tue(11, 0), 3, false},
		{"weekend, all week", allWeek, sat, tue(11, 0), 3, true},
		{"off", Reminder{}, tue(13, 0), tue(1, 0), 3, false},
	}
	for _, c := range cases {
		if got := c.r.Due(c.now, c.last, c.pending); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestSettingsClean(t *testing.T) {
	s := Settings{
		PollMinutes:   0,
		RemindMinutes: -5,
		RemindDays:    []int{5, 1, 9, 1, -1},
		WorkStart:     20,
		WorkEnd:       8,
		ExcludeOrgs:   []string{" acme ", "ACME", ""},
	}.clean()
	if s.PollMinutes != 2 || s.RemindMinutes != 0 {
		t.Errorf("poll %d, remind %d: want 2 and 0", s.PollMinutes, s.RemindMinutes)
	}
	if len(s.RemindDays) != 2 || s.RemindDays[0] != 1 || s.RemindDays[1] != 5 {
		t.Errorf("days %v: want [1 5]", s.RemindDays)
	}
	if s.WorkStart != 9 || s.WorkEnd != 18 {
		t.Errorf("hours %d-%d: want the default 9-18", s.WorkStart, s.WorkEnd)
	}
	if len(s.ExcludeOrgs) != 1 || s.ExcludeOrgs[0] != "acme" {
		t.Errorf("orgs %q: want [acme]", s.ExcludeOrgs)
	}
}

func TestBuildNotification(t *testing.T) {
	one := []Action{{Kind: KindReview, Repo: "o/safe", Number: 109, Title: "Fix it", URL: "https://github.com/o/safe/pull/109"}}
	n := BuildNotification(one, false)
	if n.Title != "👀 review · safe#109" || n.URL != one[0].URL {
		t.Errorf("one PR: got %+v", n)
	}
	if n := BuildNotification(one, true); n.Title != "Still waiting: 👀 review · safe#109" {
		t.Errorf("one PR reminder: got %q", n.Title)
	}

	many := append(one, Action{Kind: KindMerge, Repo: "o/ui", Number: 4, URL: "https://github.com/o/ui/pull/4"})
	n = BuildNotification(many, true)
	if n.Title != "2 PRs still wait on you" || n.URL != "" {
		t.Errorf("several PRs: got %+v, want no URL so a click opens the menu", n)
	}
}
