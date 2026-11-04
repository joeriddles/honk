package main

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const sampleRate = 22050

// honkWAV is a real Canada goose: two honks cut from a public-domain recording
// of the US National Park Service. The README says where it comes from.
//
//go:embed sounds/honk.wav
var honkWAV []byte

// chirpWAV is two short rising beeps for changes on your own PRs.
// It is made in code, so the repo does not need a second audio file.
func chirpWAV() []byte {
	var s []float64
	s = append(s, beep(0.07, 880, 1100)...)
	s = append(s, silence(0.04)...)
	s = append(s, beep(0.09, 1175, 1480)...)
	return wav(s)
}

func beep(dur, f0, f1 float64) []float64 {
	n := int(dur * sampleRate)
	out := make([]float64, n)
	phase := 0.0
	for i := range n {
		t := float64(i) / sampleRate
		f := f0 + (f1-f0)*t/dur
		phase += 2 * math.Pi * f / sampleRate
		out[i] = (math.Sin(phase) + 0.2*math.Sin(2*phase)) * envelope(t, dur, 0.005, 0.03)
	}
	return normalize(out, 0.5)
}

func silence(dur float64) []float64 { return make([]float64, int(dur*sampleRate)) }

func envelope(t, dur, attack, release float64) float64 {
	switch {
	case t < attack:
		return t / attack
	case t > dur-release:
		return math.Max(0, (dur-t)/release)
	}
	return 1
}

func normalize(s []float64, peak float64) []float64 {
	m := 0.0
	for _, v := range s {
		m = math.Max(m, math.Abs(v))
	}
	if m > 0 {
		for i := range s {
			s[i] *= peak / m
		}
	}
	return s
}

func wav(samples []float64) []byte {
	var b bytes.Buffer
	data := len(samples) * 2
	w := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	b.WriteString("RIFF")
	w(uint32(36 + data))
	b.WriteString("WAVEfmt ")
	w(uint32(16))
	w(uint16(1)) // PCM
	w(uint16(1)) // mono
	w(uint32(sampleRate))
	w(uint32(sampleRate * 2))
	w(uint16(2))
	w(uint16(16))
	b.WriteString("data")
	w(uint32(data))
	for _, v := range samples {
		w(int16(math.Max(-1, math.Min(1, v)) * math.MaxInt16))
	}
	return b.Bytes()
}

// Sounds holds the paths of the WAV files that the player reads.
type Sounds struct{ honk, chirp string }

func WriteSounds() (*Sounds, error) {
	dir := filepath.Join(stateDir(), "sounds")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	s := &Sounds{honk: filepath.Join(dir, "honk.wav"), chirp: filepath.Join(dir, "chirp.wav")}
	if err := os.WriteFile(s.honk, honkWAV, 0o600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(s.chirp, chirpWAV(), 0o600); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Sounds) Honk()  { play(s.honk) }
func (s *Sounds) Chirp() { play(s.chirp) }

// play starts the system player and does not wait for it.
func play(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("afplay", path)
	case "linux", "freebsd", "openbsd":
		player := "paplay"
		if _, err := exec.LookPath(player); err != nil {
			player = "aplay"
		}
		cmd = exec.Command(player, path)
	default:
		return
	}
	if err := cmd.Start(); err != nil {
		slog.Warn("play sound", "err", err)
		return
	}
	go func() { _ = cmd.Wait() }()
}
