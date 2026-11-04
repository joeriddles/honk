//go:build !darwin

package main

import (
	"log/slog"
	"os/exec"
)

func initNotify() {}

func postNotification(title, body, _ string) {
	if err := exec.Command("notify-send", "--app-name=honk", title, body).Run(); err != nil {
		slog.Warn("notify", "err", err)
	}
}
