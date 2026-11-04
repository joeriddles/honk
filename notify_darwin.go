package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit -framework Foundation -framework UserNotifications
#include <stdlib.h>

int honkNotifyInit(void);
void honkNotify(const char *title, const char *body, const char *url);
*/
import "C"

import (
	"log/slog"
	"os/exec"
	"sync"
	"unsafe"
)

var (
	nativeOnce sync.Once
	native     bool
)

// initNotify sets up native notifications. The tray calls it at start-up, so
// macOS asks for permission once, and so a click works after a restart.
func initNotify() {
	nativeOnce.Do(func() { native = C.honkNotifyInit() == 1 })
	slog.Info("notifications", "native", native)
}

func postNotification(title, body, url string) {
	nativeOnce.Do(func() { native = C.honkNotifyInit() == 1 })
	if native {
		ct, cb, cu := C.CString(title), C.CString(body), C.CString(url)
		defer C.free(unsafe.Pointer(ct))
		defer C.free(unsafe.Pointer(cb))
		defer C.free(unsafe.Pointer(cu))
		C.honkNotify(ct, cb, cu)
		return
	}
	// Outside honk.app, fall back to AppleScript. PR titles come from other
	// people, so pass them as arguments and never paste them into the source.
	cmd := exec.Command("osascript",
		"-e", "on run argv",
		"-e", "display notification (item 2 of argv) with title (item 1 of argv)",
		"-e", "end run",
		title, body)
	if err := cmd.Run(); err != nil {
		slog.Warn("notify", "err", err)
	}
}

//export honkNotifyLog
func honkNotifyLog(msg *C.char) {
	slog.Info("notifications: " + C.GoString(msg))
}
