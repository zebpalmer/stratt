// Package ui owns stratt's terminal output styling: colors,
// verbosity, prefixes.
//
// Color rules (R5.4):
//   - "auto"   — colored only when the destination is a TTY.  Default.
//   - "always" — force colored output even when piped.
//   - "never"  — strip all color codes.
//
// User override: ~/.stratt/config.toml [display] color = "..." plus
// the standard NO_COLOR convention.
//
// Verbosity rules (R5.7):
//   - quiet   — only errors and final summary lines
//   - normal  — also progress markers (→ and ▶)
//   - verbose — also command echoes (+ <shell command>)
//   - debug   — also internal step logging
//
// `-v` on the CLI bumps one level (normal → verbose); `-vv` bumps two
// (normal → debug).
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

// ParseColorMode turns a config string into a ColorMode.  Unknown
// values fall back to Auto.
func ParseColorMode(s string) ColorMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "always", "force":
		return ColorAlways
	case "never", "no", "off":
		return ColorNever
	}
	return ColorAuto
}

// Style is the resolved output style: color mode + verbosity, bound
// to specific writers.  Pass this around rather than recomputing color
// every print.
type Style struct {
	Out      io.Writer
	Err      io.Writer
	Level    Level
	useColor bool
}

// NewStyle returns a Style for the given writers and policy.
//
// Color enable rules, in order:
//  1. NO_COLOR env var set (per https://no-color.org)            → no color
//  2. mode = ColorNever                                          → no color
//  3. mode = ColorAlways                                         → color
//  4. mode = ColorAuto AND `out` is a TTY                        → color
//  5. otherwise                                                  → no color
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

// isTerminal reports whether w is a *os.File backed by a TTY.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// ANSI escape sequences for the colors we use.  Keeping these as
// constants instead of a library dep makes the binary 100KB smaller
// and the code grep-able.
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

// wrap applies an ANSI color to s if color is enabled, otherwise
// returns s untouched.
func (s *Style) wrap(code, text string) string {
	if !s.useColor {
		return text
	}
	return code + text + reset
}

// UseColor reports whether ANSI codes are being emitted.  Useful for
// tests and one-off conditional rendering.
func (s *Style) UseColor() bool { return s.useColor }

// Success returns a green "✓" marker followed by msg.  Newline included.
func (s *Style) Success(msg string) string { return s.wrap(green, "✓ ") + msg + "\n" }

// Failure returns a red "✗" marker followed by msg.  Newline included.
func (s *Style) Failure(msg string) string { return s.wrap(red, "✗ ") + msg + "\n" }

// Progress returns a blue "→" marker followed by msg.  Newline included.
// Use for "doing this now" status lines.
func (s *Style) Progress(msg string) string { return s.wrap(blue, "→ ") + msg + "\n" }

// Task returns a "▶" task-announcement marker followed by msg.  Newline included.
func (s *Style) Task(msg string) string { return s.wrap(cyan, "▶ ") + msg + "\n" }

// ShellCmd returns the "+ <command>" line shown when echoing user-task
// shell commands.  Dim styling so it doesn't dominate output.
func (s *Style) ShellCmd(cmd string) string { return s.wrap(dim, "+ "+cmd) + "\n" }

// Warn returns a yellow "warning:" prefix.  Newline included.
func (s *Style) Warn(msg string) string { return s.wrap(yellow, "warning:") + " " + msg + "\n" }

// Error returns a red "error:" prefix.  Newline included.
func (s *Style) Error(msg string) string { return s.wrap(red, "error:") + " " + msg + "\n" }

// Faint wraps text in dim styling — used for low-priority annotations
// (e.g., "[tool not on PATH]" suffixes in doctor).
func (s *Style) Faint(text string) string { return s.wrap(dim, text) }

// Bold wraps text in bold styling.
func (s *Style) Bold(text string) string { return s.wrap(bold, text) }
