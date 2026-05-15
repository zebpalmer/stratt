// Package ui handles stratt's terminal output styling: colors,
// verbosity, prefixes.  Color honors NO_COLOR and the `[display]`
// section of the user config.
package ui

import (
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// Level is the verbosity level.
type Level int

const (
	Quiet Level = iota
	Normal
	Verbose
	Debug
)

// ColorMode selects how/when to emit ANSI color codes.
type ColorMode int

const (
	ColorAuto ColorMode = iota
	ColorAlways
	ColorNever
)

// ParseColorMode reads a config string.  Unknown values fall back to Auto.
func ParseColorMode(s string) ColorMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "always", "force":
		return ColorAlways
	case "never", "no", "off":
		return ColorNever
	}
	return ColorAuto
}

// Style is the resolved output style bound to specific writers.
type Style struct {
	Out      io.Writer
	Err      io.Writer
	Level    Level
	useColor bool
}

// NewStyle returns a Style.  NO_COLOR (https://no-color.org) overrides
// everything; otherwise mode wins, with ColorAuto checking for a TTY
// on `out`.
func NewStyle(out, errW io.Writer, mode ColorMode, level Level) *Style {
	useColor := false
	switch {
	case os.Getenv("NO_COLOR") != "":
		useColor = false
	case mode == ColorAlways:
		useColor = true
	case mode == ColorNever:
		useColor = false
	default:
		useColor = isTerminal(out)
	}
	return &Style{Out: out, Err: errW, Level: level, useColor: useColor}
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

const (
	reset  = "\x1b[0m"
	dim    = "\x1b[2m"
	bold   = "\x1b[1m"
	red    = "\x1b[31m"
	green  = "\x1b[32m"
	yellow = "\x1b[33m"
	blue   = "\x1b[34m"
	cyan   = "\x1b[36m"
)

func (s *Style) wrap(code, text string) string {
	if !s.useColor {
		return text
	}
	return code + text + reset
}

// UseColor reports whether ANSI codes will be emitted.
func (s *Style) UseColor() bool { return s.useColor }

func (s *Style) Success(msg string) string  { return s.wrap(green, "✓ ") + msg + "\n" }
func (s *Style) Failure(msg string) string  { return s.wrap(red, "✗ ") + msg + "\n" }
func (s *Style) Progress(msg string) string { return s.wrap(blue, "→ ") + msg + "\n" }
func (s *Style) Task(msg string) string     { return s.wrap(cyan, "▶ ") + msg + "\n" }
func (s *Style) ShellCmd(cmd string) string { return s.wrap(dim, "+ "+cmd) + "\n" }
func (s *Style) Warn(msg string) string     { return s.wrap(yellow, "warning:") + " " + msg + "\n" }
func (s *Style) Error(msg string) string    { return s.wrap(red, "error:") + " " + msg + "\n" }
func (s *Style) Faint(text string) string   { return s.wrap(dim, text) }
func (s *Style) Bold(text string) string    { return s.wrap(bold, text) }
