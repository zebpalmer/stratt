package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseColorMode(t *testing.T) {
	cases := map[string]ColorMode{
		"auto":   ColorAuto,
		"":       ColorAuto,
		"AUTO":   ColorAuto,
		"always": ColorAlways,
		"force":  ColorAlways,
		"never":  ColorNever,
		"no":     ColorNever,
		"off":    ColorNever,
		"bogus":  ColorAuto, // fallback
	}
	for in, want := range cases {
		if got := ParseColorMode(in); got != want {
			t.Errorf("ParseColorMode(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestStyleNeverColor(t *testing.T) {
	var buf bytes.Buffer
	s := NewStyle(&buf, &buf, ColorNever, Normal)
	if s.UseColor() {
		t.Error("ColorNever should disable color")
	}
	got := s.Success("done")
	if strings.Contains(got, "\x1b[") {
		t.Errorf("ColorNever should emit no escape codes; got %q", got)
	}
	if !strings.Contains(got, "✓") {
		t.Errorf("Success marker missing: %q", got)
	}
}

func TestStyleAlwaysColor(t *testing.T) {
	var buf bytes.Buffer
	s := NewStyle(&buf, &buf, ColorAlways, Normal)
	if !s.UseColor() {
		t.Error("ColorAlways should enable color")
	}
	got := s.Failure("kaboom")
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("ColorAlways should emit escape codes; got %q", got)
	}
}

func TestStyleAutoDefaultsOffForNonTTY(t *testing.T) {
	var buf bytes.Buffer
	s := NewStyle(&buf, &buf, ColorAuto, Normal)
	if s.UseColor() {
		t.Error("ColorAuto on a buffer (non-TTY) should disable color")
	}
}

// TestNoColorEnvForcesOff — the NO_COLOR convention overrides
// ColorAlways.
func TestNoColorEnvForcesOff(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	s := NewStyle(&buf, &buf, ColorAlways, Normal)
	if s.UseColor() {
		t.Error("NO_COLOR should override ColorAlways")
	}
}

func TestStyleMarkers(t *testing.T) {
	var buf bytes.Buffer
	s := NewStyle(&buf, &buf, ColorNever, Normal)
	for _, tc := range []struct {
		name    string
		fn      func(string) string
		marker  string
	}{
		{"success", s.Success, "✓"},
		{"failure", s.Failure, "✗"},
		{"progress", s.Progress, "→"},
		{"task", s.Task, "▶"},
		{"warn", s.Warn, "warning:"},
		{"error", s.Error, "error:"},
	} {
		out := tc.fn("hello")
		if !strings.Contains(out, tc.marker) {
			t.Errorf("%s missing marker %q: %q", tc.name, tc.marker, out)
		}
		if !strings.Contains(out, "hello") {
			t.Errorf("%s missing message: %q", tc.name, out)
		}
	}
}
