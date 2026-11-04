package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
	"unicode/utf8"

	"fyne.io/systray"
)

// rowCount is the size of the fixed pool of menu rows. The app shows or hides
// rows, and does not rebuild the menu, so an open menu does not flicker.
const rowCount = 40

type row struct {
	item *systray.MenuItem
	url  string
}

type tray struct {
	state  *State
	sounds *Sounds

	mu       sync.Mutex
	settings Settings
	rows     [rowCount]*row
	seenOrgs map[string]bool // every org seen since start, for the Skip orgs menu
	orgsSub  *systray.MenuItem
	orgRows  [orgSlots]*orgSlot

	status  *systray.MenuItem
	mute    *systray.MenuItem
	refresh chan struct{}
}

func runTray(s Settings) {
	t := &tray{settings: s, seenOrgs: map[string]bool{}, refresh: make(chan struct{}, 1)}
	systray.Run(t.onReady, func() { slog.Info("quit") })
}

func (t *tray) onReady() {
	systray.SetTitle("🪿")
	systray.SetTooltip("honk: PRs that wait on you")
	initNotify()

	var err error
	if t.state, err = LoadState(); err != nil {
		slog.Warn("load state; start fresh", "err", err)
	}
	if t.sounds, err = WriteSounds(); err != nil {
		slog.Warn("write sounds", "err", err)
	}

	for i := range t.rows {
		r := &row{item: systray.AddMenuItem("", "")}
		r.item.Hide()
		t.rows[i] = r
		go func() {
			for range r.item.ClickedCh {
				t.mu.Lock()
				u := r.url
				t.mu.Unlock()
				if u != "" {
					OpenURL(u)
				}
			}
		}()
	}
	systray.AddSeparator()
	t.status = systray.AddMenuItem("Starting…", "")
	t.status.Disable()
	refresh := systray.AddMenuItem("Refresh now", "Poll GitHub now")
	t.mute = systray.AddMenuItemCheckbox("Mute", "No sounds and no notifications", t.state.Muted)
	t.buildSettings()
	quit := systray.AddMenuItem("Quit honk", "")

	go func() {
		for {
			select {
			case <-refresh.ClickedCh:
				t.poke()
			case <-t.mute.ClickedCh:
				t.mu.Lock()
				t.state.Muted = !t.state.Muted
				muted := t.state.Muted
				t.mu.Unlock()
				if muted {
					t.mute.Check()
				} else {
					t.mute.Uncheck()
				}
				t.save()
			case <-quit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()

	go t.loop()
}

func (t *tray) poke() {
	select {
	case t.refresh <- struct{}{}:
	default:
	}
}

func (t *tray) loop() {
	ctx := context.Background()
	var client *Client
	for {
		if client == nil {
			// Get the token on each failure, so `gh auth login` fixes a running app.
			token, src, err := FindToken(ctx)
			if err != nil {
				t.fail(err)
			} else {
				slog.Info("token found", "source", src)
				client = NewClient(token)
			}
		}
		if client != nil {
			if err := t.poll(ctx, client); err != nil {
				t.fail(err)
				client = nil // a bad token is the usual cause; get a new one
			}
		}
		// A new timer each time, so a change of the poll interval works at once.
		timer := time.NewTimer(t.settingsNow().Interval())
		select {
		case <-timer.C:
		case <-t.refresh:
			timer.Stop()
		}
	}
}

func (t *tray) poll(ctx context.Context, c *Client) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	s := t.settingsNow()
	snap, err := c.Fetch(ctx, s.ExcludeOrgs)
	if err != nil {
		return err
	}
	now := time.Now()
	actions := Decide(snap, now, s.Options())

	t.mu.Lock()
	for _, o := range snap.Orgs() {
		t.seenOrgs[o] = true
	}
	fresh := t.state.Update(actions, now)
	if t.state.LastAlert.IsZero() {
		t.state.LastAlert = now // the first run after an upgrade does not remind at once
	}
	remind := len(fresh) == 0 && s.Reminder().Due(now, t.state.LastAlert, len(actions))
	if len(fresh) > 0 || remind {
		t.state.LastAlert = now
	}
	muted := t.state.Muted
	t.mu.Unlock()
	t.save()

	slog.Info("poll", "user", snap.Me, "actions", len(actions), "new", len(fresh), "remind", remind, "rate_left", snap.RateRemaining)
	t.render(actions)
	t.renderOrgs()
	t.status.SetTitle(fmt.Sprintf("@%s · checked %s", snap.Me, now.Format("15:04")))

	switch {
	case muted:
	case len(fresh) > 0:
		t.alert(fresh, false, s.Options())
	case remind:
		t.alert(actions, true, s.Options())
	}
	return nil
}

// alert makes one sound and one notification for a batch of actions.
// A honk wins over a chirp.
func (t *tray) alert(as []Action, reminder bool, opt Options) {
	honk := false
	for _, a := range as {
		if a.Honks(opt) {
			honk = true
			break
		}
	}
	if t.sounds != nil {
		if honk {
			t.sounds.Honk()
		} else {
			t.sounds.Chirp()
		}
	}
	go Notify(as, reminder)
}

func (t *tray) fail(err error) {
	slog.Error("poll", "err", err)
	systray.SetTitle("🪿⚠️")
	msg := err.Error()
	if len(msg) > 80 {
		msg = msg[:80] + "…"
	}
	t.status.SetTitle("Error: " + msg)
}

func (t *tray) save() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.state.Save(); err != nil {
		slog.Warn("save state", "err", err)
	}
}

type line struct{ title, url string }

func (t *tray) render(actions []Action) {
	var in, out []Action
	for _, a := range actions {
		if a.Incoming {
			in = append(in, a)
		} else {
			out = append(out, a)
		}
	}

	var lines []line
	section := func(name string, as []Action) {
		if len(as) == 0 {
			return
		}
		lines = append(lines, line{title: fmt.Sprintf("%s (%d)", name, len(as))})
		for _, a := range as {
			lines = append(lines, line{
				title: fmt.Sprintf("%s %s · %s#%d  %s", kindIcon[a.Kind], a.Kind, shortRepo(a.Repo), a.Number, clip(a.Title, 50)),
				url:   a.URL,
			})
		}
	}
	section("Waiting on you", in)
	section("Your PRs", out)
	if len(lines) == 0 {
		lines = append(lines, line{title: "Nothing waits on you 🪿💤"})
	}
	if len(lines) > rowCount {
		extra := len(lines) - rowCount + 1
		lines = append(lines[:rowCount-1], line{title: fmt.Sprintf("…and %d more", extra)})
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	for i, r := range t.rows {
		if i >= len(lines) {
			r.url = ""
			r.item.Hide()
			continue
		}
		l := lines[i]
		r.url = l.url
		r.item.SetTitle(l.title)
		if l.url == "" {
			r.item.Disable()
		} else {
			r.item.Enable()
		}
		r.item.Show()
	}

	switch {
	case len(in) > 0:
		systray.SetTitle(fmt.Sprintf("🪿 %d", len(in)))
	default:
		systray.SetTitle("🪿")
	}
}

func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}
