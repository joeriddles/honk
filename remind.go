package main

import "time"

// Reminder says when honk reminds you again about PRs that still wait on you.
type Reminder struct {
	Every time.Duration // 0 turns reminders off
	Days  [7]bool       // indexed by time.Weekday
	// Reminders come from Start:00 to before End:00 local time.
	// Start 0 and End 24 mean all day.
	Start, End int
}

// Due is true when a reminder must go out now. last is the time of the most
// recent honk, chirp, or reminder. pending is the number of actions now.
func (r Reminder) Due(now, last time.Time, pending int) bool {
	if r.Every <= 0 || pending == 0 || now.Sub(last) < r.Every {
		return false
	}
	if !r.Days[now.Weekday()] {
		return false
	}
	h := now.Hour()
	return h >= r.Start && h < r.End
}
