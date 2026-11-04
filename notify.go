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
		title = fmt.Sprintf("🪿 %d PRs need you", len(fresh))
		var lines []string
		for _, a := range fresh[:min(3, len(fresh))] {
			lines = append(lines, fmt.Sprintf("%s %s#%d", a.Kind, shortRepo(a.Repo), a.Number))
		}
		body = strings.Join(lines, "\n")
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		// PR titles come from other people, so pass them as arguments and
		// never paste them into the AppleScript source.
		cmd = exec.Command("osascript",
			"-e", "on run argv",
			"-e", "display notification (item 2 of argv) with title (item 1 of argv)",
			"-e", "end run",
			title, body)
	case "linux", "freebsd", "openbsd":
		cmd = exec.Command("notify-send", "--app-name=honk", title, body)
	default:
		return
	}
	if err := cmd.Run(); err != nil {
		slog.Warn("notify", "err", err)
	}
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
