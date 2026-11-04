package main

import (
	"fmt"
	"log/slog"
	"os/exec"
	"runtime"
	"strings"
)

// Notification is the text of one notification, and the URL that a click opens.
// An empty URL makes a click open honk's menu.
type Notification struct{ Title, Body, URL string }

// BuildNotification writes one notification for a batch of actions. A reminder
// is about actions that you already know of, so its words differ.
func BuildNotification(as []Action, reminder bool) Notification {
	if len(as) == 1 {
		a := as[0]
		title := fmt.Sprintf("%s %s · %s#%d", kindIcon[a.Kind], a.Kind, shortRepo(a.Repo), a.Number)
		if reminder {
			title = "Still waiting: " + title
		}
		return Notification{Title: title, Body: a.Title, URL: a.URL}
	}
	title := fmt.Sprintf("%d PRs need you", len(as))
	if reminder {
		title = fmt.Sprintf("%d PRs still wait on you", len(as))
	}
	var lines []string
	for _, a := range as[:min(3, len(as))] {
		lines = append(lines, fmt.Sprintf("%s %s · %s#%d", kindIcon[a.Kind], a.Kind, shortRepo(a.Repo), a.Number))
	}
	if len(as) > 3 {
		lines = append(lines, fmt.Sprintf("and %d more", len(as)-3))
	}
	// No URL: one link cannot show several PRs, so a click opens the menu.
	return Notification{Title: title, Body: strings.Join(lines, "\n")}
}

// Notify shows one desktop notification for a batch of actions.
func Notify(as []Action, reminder bool) {
	if len(as) == 0 {
		return
	}
	n := BuildNotification(as, reminder)
	postNotification(n.Title, n.Body, n.URL)
}

// OpenURL opens a GitHub link in the default browser.
func OpenURL(u string) {
	if !strings.HasPrefix(u, "https://github.com/") {
		slog.Warn("refuse to open a non-GitHub URL", "url", u)
		return
	}
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	if err := exec.Command(name, u).Start(); err != nil {
		slog.Warn("open url", "err", err)
	}
}

func shortRepo(repo string) string {
	if _, name, ok := strings.Cut(repo, "/"); ok {
		return name
	}
	return repo
}
