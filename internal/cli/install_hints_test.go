package cli

import (
	"strings"
	"testing"
)

// TestInstallHintCovers — sanity check that every binary stratt
// resolves to in a default chain has an install hint registered.  This
// catches the case where someone adds a new chain entry but forgets
// the install registry.
func TestInstallHintCovers(t *testing.T) {
	// The set of tools that appear as the leading binary in any
	// default chain entry across chains.go.  If you add or rename a
	// backend, add it here.
	tools := []string{
		"hugo",
		"mkdocs",
		"sphinx-build",
		"sphinx-autobuild",
		"uv",
		"go",
		"gofmt",
		"golangci-lint",
		"goreleaser",
		"composer",
		"docker",
	}
	for _, tool := range tools {
		if InstallHint(tool) == "" {
			t.Errorf("InstallHint(%q) returned empty — add it to install_hints.go", tool)
		}
	}
}

// TestInstallHintUnknownReturnsEmpty — non-tool callers should get
// an empty string back, not a misleading suggestion.
func TestInstallHintUnknownReturnsEmpty(t *testing.T) {
	if got := InstallHint("totally-not-a-real-tool"); got != "" {
		t.Errorf("got %q for unknown tool, want empty", got)
	}
}

// TestInstallHintHugoBrew — the headline use case from the user.
func TestInstallHintHugoBrew(t *testing.T) {
	got := InstallHint("hugo")
	if !strings.Contains(got, "brew install hugo") {
		t.Errorf("hugo hint should mention brew install hugo; got %q", got)
	}
}
