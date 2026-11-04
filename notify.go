package main

import (
	"fmt"
	"log/slog"
	"os/exec"
	"runtime"
	"strings"
)

// Notify shows one desktop notification for a batch of new actions.
func Notify(fresh []Action) {
	if len(fresh) == 0 {
		return
	}
	var title, body string
	if len(fresh) == 1 {
		a := fresh[0]
		title = fmt.Sprintf("%s %s · %s#%d", kindIcon[a.Kind], a.Kind, shortRepo(a.Repo), a.Number)
		body = a.Title
	} else {
		title = fmt.Sprintf("%d PRs need you", len(fresh))
		var lines []string
		for _, a := range fresh[:min(3, len(fresh))] {
			lines = append(lines, fmt.Sprintf("%s %s · %s#%d", kindIcon[a.Kind], a.Kind, shortRepo(a.Repo), a.Number))
		}
		if len(fresh) > 3 {
			lines = append(lines, fmt.Sprintf("and %d more", len(fresh)-3))
		}
		body = strings.Join(lines, "\n")
	}
	// A click opens the most urgent PR. The actions are sorted, so it is the first.
	postNotification(title, body, fresh[0].URL)
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
	if i := strings.IndexByte(repo, '/'); i >= 0 {
		return repo[i+1:]
	}
	return repo
}
