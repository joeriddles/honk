package main

import (
	"errors"
	"fmt"
	"html"
	"io/fs"
	"os"
	"path/filepath"
)

// The label is the same as in the Makefile, so `make login` and the menu
// change the same LaunchAgent.
const agentLabel = "dev.joeriddles.honk"

func agentPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", agentLabel+".plist")
}

func loginSupported() bool { return true }

func loginEnabled() bool {
	_, err := os.Stat(agentPath())
	return err == nil
}

// setLogin writes or removes the LaunchAgent. It does not load or unload it:
// a load would start a second honk now, and an unload would stop this one.
// launchd reads the change at the next login.
func setLogin(on bool) error {
	if !on {
		err := os.Remove(agentPath())
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>%s</string>
<key>ProgramArguments</key><array><string>%s</string></array>
<key>RunAtLoad</key><true/>
<key>LimitLoadToSessionType</key><string>Aqua</string>
</dict></plist>
`, agentLabel, html.EscapeString(exe))
	if err := os.MkdirAll(filepath.Dir(agentPath()), 0o755); err != nil {
		return err
	}
	return os.WriteFile(agentPath(), []byte(plist), 0o644)
}
