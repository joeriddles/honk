package main

import (
	"bytes"
	"testing"
)

// After a clone without Git LFS, sounds/honk.wav is a small text pointer, and
// the build embeds the pointer. This test catches that.
func TestHonkIsWAV(t *testing.T) {
	if !bytes.HasPrefix(honkWAV, []byte("RIFF")) || !bytes.Equal(honkWAV[8:12], []byte("WAVE")) {
		t.Fatalf("sounds/honk.wav is not a WAV file (%d bytes); run `git lfs pull`", len(honkWAV))
	}
}
