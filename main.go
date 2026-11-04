// Command honk is a menubar app that honks when a pull request waits on you.
// It talks only to api.github.com, with your gh token or a PAT.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

func main() {
	// All settings are in the Settings menu. These flags only help to test.
	check := flag.Bool("check", false, "print what waits on you, then exit (no menubar)")
	testSound := flag.Bool("test-sound", false, "play the honk and the chirp, then exit")
	flag.Parse()

	setupLog()

	settings, err := LoadSettings()
	if err != nil {
		slog.Warn("load settings; use the defaults", "err", err)
	}

	switch {
	case *testSound:
		s, err := WriteSounds()
		if err != nil {
			fatal(err)
		}
		s.Honk()
		time.Sleep(900 * time.Millisecond)
		s.Chirp()
		time.Sleep(400 * time.Millisecond)
	case *check:
		if err := runCheck(settings); err != nil {
			fatal(err)
		}
	default:
		runTray(settings)
	}
}

func runCheck(s Settings) error {
	ctx := context.Background()
	token, src, err := FindToken(ctx)
	if err != nil {
		return err
	}
	snap, err := NewClient(token).Fetch(ctx, s.ExcludeOrgs)
	if err != nil {
		return err
	}
	actions := Decide(snap, time.Now(), s.Options())
	fmt.Printf("@%s (token from %s) · %d API points left\n", snap.Me, src, snap.RateRemaining)
	fmt.Printf("fetched: %d review requests, %d reviewed, %d yours, %d mentions\n\n",
		len(snap.ReviewRequests), len(snap.Reviewed), len(snap.Mine), len(snap.Mentions))
	if len(actions) == 0 {
		fmt.Println("Nothing waits on you. 🪿💤")
		return nil
	}
	for _, a := range actions {
		sound := "chirp"
		if a.Honks(s.Options()) {
			sound = "HONK"
		}
		fmt.Printf("%-5s %s %-12s %s#%d  %s\n      %s · %s\n", sound, kindIcon[a.Kind], a.Kind, a.Repo, a.Number, a.Title, a.Reason, a.URL)
	}
	return nil
}

func setupLog() {
	var w io.Writer = os.Stderr
	dir := stateDir()
	if err := os.MkdirAll(dir, 0o700); err == nil {
		if f, err := os.OpenFile(filepath.Join(dir, "honk.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
			w = io.MultiWriter(os.Stderr, f)
		}
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(w, nil)))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "honk:", err)
	os.Exit(1)
}
