package main

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"fyne.io/systray"
)

// The Settings submenu. Each group shows its value in its title, and a check
// mark on the chosen item. A change is saved and used at once.

type choice struct {
	label string
	value int
}

var (
	pollChoices = []choice{{"1 minute", 1}, {"2 minutes", 2}, {"5 minutes", 5}, {"10 minutes", 10}, {"15 minutes", 15}}

	remindChoices = []choice{{"Never", 0}, {"15 minutes", 15}, {"30 minutes", 30}, {"1 hour", 60},
		{"2 hours", 120}, {"4 hours", 240}, {"Day", 1440}}

	startChoices = []choice{{"Midnight", 0}, {"6 AM", 6}, {"7 AM", 7}, {"8 AM", 8}, {"9 AM", 9},
		{"10 AM", 10}, {"11 AM", 11}, {"Noon", 12}}

	endChoices = []choice{{"1 PM", 13}, {"2 PM", 14}, {"3 PM", 15}, {"4 PM", 16}, {"5 PM", 17}, {"6 PM", 18},
		{"7 PM", 19}, {"8 PM", 20}, {"9 PM", 21}, {"10 PM", 22}, {"Midnight", 24}}

	staleChoices = []choice{{"1 week", 7}, {"2 weeks", 14}, {"30 days", 30}, {"60 days", 60}, {"90 days", 90}, {"Never", 0}}

	// Monday first, as on a work calendar. Values are time.Weekday.
	dayChoices = []choice{{"Monday", 1}, {"Tuesday", 2}, {"Wednesday", 3}, {"Thursday", 4},
		{"Friday", 5}, {"Saturday", 6}, {"Sunday", 0}}
)

// orgSlots is the size of the fixed pool of rows in the Skip orgs menu.
const orgSlots = 25

type orgSlot struct {
	item *systray.MenuItem
	org  string
}

func (t *tray) buildSettings() {
	menu := systray.AddMenuItem("Settings", "")
	s := t.settingsNow()

	t.radio(menu, "Check GitHub every", pollChoices, s.PollMinutes, true,
		func(s *Settings, v int) { s.PollMinutes = v })
	menu.AddSeparator()
	t.radio(menu, "Remind me every", remindChoices, s.RemindMinutes, false,
		func(s *Settings, v int) { s.RemindMinutes = v })
	t.days(menu, s)
	t.radio(menu, "Workday starts at", startChoices, s.WorkStart, false,
		func(s *Settings, v int) { s.WorkStart = v })
	t.radio(menu, "Workday ends at", endChoices, s.WorkEnd, false,
		func(s *Settings, v int) { s.WorkEnd = v })
	menu.AddSeparator()
	t.radio(menu, "Skip PRs idle for", staleChoices, s.StaleDays, true,
		func(s *Settings, v int) { s.StaleDays = v })
	t.orgsMenu(menu)
	t.toggle(menu, "Honk for team reviews", s.TeamHonk, func(s *Settings, on bool) { s.TeamHonk = on })

	if loginSupported() {
		menu.AddSeparator()
		login := menu.AddSubMenuItemCheckbox("Start at login", "", loginEnabled())
		go func() {
			for range login.ClickedCh {
				on := !login.Checked()
				if err := setLogin(on); err != nil {
					slog.Warn("start at login", "err", err)
					continue
				}
				if on {
					login.Check()
				} else {
					login.Uncheck()
				}
				slog.Info("start at login", "on", on)
			}
		}()
	}
}

// radio adds a submenu where exactly one choice has a check mark.
func (t *tray) radio(parent *systray.MenuItem, name string, choices []choice, current int, refetch bool, set func(*Settings, int)) {
	sub := parent.AddSubMenuItem("", "")
	items := make([]*systray.MenuItem, len(choices))
	mark := func(v int) {
		label := fmt.Sprint(v)
		for i, c := range choices {
			if c.value == v {
				items[i].Check()
				label = c.label
			} else {
				items[i].Uncheck()
			}
		}
		sub.SetTitle(name + ": " + label)
	}
	for i, c := range choices {
		items[i] = sub.AddSubMenuItemCheckbox(c.label, "", false)
	}
	mark(current)
	for i, c := range choices {
		go func() {
			for range items[i].ClickedCh {
				t.change(func(s *Settings) { set(s, c.value) }, refetch)
				mark(c.value)
			}
		}()
	}
}

func (t *tray) toggle(parent *systray.MenuItem, name string, on bool, set func(*Settings, bool)) {
	item := parent.AddSubMenuItemCheckbox(name, "", on)
	go func() {
		for range item.ClickedCh {
			on := !item.Checked()
			t.change(func(s *Settings) { set(s, on) }, true)
			if on {
				item.Check()
			} else {
				item.Uncheck()
			}
		}
	}()
}

// days adds a submenu where each day of the week turns on or off.
func (t *tray) days(parent *systray.MenuItem, s Settings) {
	sub := parent.AddSubMenuItem("", "")
	title := func(days []int) { sub.SetTitle("Remind on: " + daySummary(days)) }
	title(s.RemindDays)
	for _, c := range dayChoices {
		item := sub.AddSubMenuItemCheckbox(c.label, "", slices.Contains(s.RemindDays, c.value))
		go func() {
			for range item.ClickedCh {
				on := !item.Checked()
				t.change(func(s *Settings) {
					if on {
						s.RemindDays = append(s.RemindDays, c.value)
					} else {
						s.RemindDays = slices.DeleteFunc(s.RemindDays, func(d int) bool { return d == c.value })
					}
				}, false)
				if on {
					item.Check()
				} else {
					item.Uncheck()
				}
				title(t.settingsNow().RemindDays)
			}
		}()
	}
}

// daySummary names a set of days in a few words: "Weekdays", "Mon, Wed".
func daySummary(days []int) string {
	has := func(ds ...int) bool {
		for _, d := range ds {
			if !slices.Contains(days, d) {
				return false
			}
		}
		return true
	}
	switch {
	case len(days) == 0:
		return "No days"
	case len(days) == 7:
		return "Every day"
	case len(days) == 5 && has(1, 2, 3, 4, 5):
		return "Weekdays"
	case len(days) == 2 && has(0, 6):
		return "Weekends"
	}
	var names []string
	for _, c := range dayChoices {
		if slices.Contains(days, c.value) {
			names = append(names, c.label[:3])
		}
	}
	return strings.Join(names, ", ")
}

// orgsMenu adds a submenu that lists the orgs of your PRs. A check mark
// means honk skips that org. The rows fill in after each poll.
func (t *tray) orgsMenu(parent *systray.MenuItem) {
	t.orgsSub = parent.AddSubMenuItem("Skip orgs", "")
	for i := range t.orgRows {
		r := &orgSlot{item: t.orgsSub.AddSubMenuItemCheckbox("", "", false)}
		r.item.Hide()
		t.orgRows[i] = r
		go func() {
			for range r.item.ClickedCh {
				t.mu.Lock()
				org := r.org
				t.mu.Unlock()
				if org == "" {
					continue
				}
				t.change(func(s *Settings) {
					if containsFold(s.ExcludeOrgs, org) {
						s.ExcludeOrgs = slices.DeleteFunc(s.ExcludeOrgs, func(o string) bool { return strings.EqualFold(o, org) })
					} else {
						s.ExcludeOrgs = append(s.ExcludeOrgs, org)
					}
				}, true)
				t.renderOrgs()
			}
		}()
	}
	t.renderOrgs()
}

// renderOrgs fills the Skip orgs rows: every org seen since start, plus the
// orgs that you skip (a skipped org has no PRs in the results).
func (t *tray) renderOrgs() {
	t.mu.Lock()
	defer t.mu.Unlock()
	orgs := slices.Clone(t.settings.ExcludeOrgs)
	for o := range t.seenOrgs {
		if !containsFold(orgs, o) {
			orgs = append(orgs, o)
		}
	}
	slices.SortFunc(orgs, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })

	skipped := len(t.settings.ExcludeOrgs)
	if skipped == 0 {
		t.orgsSub.SetTitle("Skip orgs: None")
	} else {
		t.orgsSub.SetTitle(fmt.Sprintf("Skip orgs: %d", skipped))
	}
	for i, r := range t.orgRows {
		switch {
		case i < len(orgs):
			r.org = orgs[i]
			r.item.SetTitle(orgs[i])
			r.item.Enable()
			if containsFold(t.settings.ExcludeOrgs, orgs[i]) {
				r.item.Check()
			} else {
				r.item.Uncheck()
			}
			r.item.Show()
		case i == 0:
			r.org = ""
			r.item.SetTitle("No orgs yet")
			r.item.Uncheck()
			r.item.Disable()
			r.item.Show()
		default:
			r.org = ""
			r.item.Hide()
		}
	}
}

// change applies fn to the settings, saves them, and polls again when the
// change affects what honk gets from GitHub.
func (t *tray) change(fn func(*Settings), refetch bool) {
	t.mu.Lock()
	s := t.settings
	s.RemindDays = slices.Clone(s.RemindDays)
	s.ExcludeOrgs = slices.Clone(s.ExcludeOrgs)
	fn(&s)
	s = s.clean()
	t.settings = s
	t.mu.Unlock()
	if err := s.Save(); err != nil {
		slog.Warn("save settings", "err", err)
	}
	slog.Info("settings changed", "settings", fmt.Sprintf("%+v", s))
	if refetch {
		t.poke()
	}
}

func (t *tray) settingsNow() Settings {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.settings
}
