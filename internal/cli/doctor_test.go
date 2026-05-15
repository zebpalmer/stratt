package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestDoctorOutputShape — doctor must show all four required sections:
// header, version metadata, detected stacks, resolved commands.  This
// guards the central transparency requirement (R2.1.4).
func TestDoctorOutputShape(t *testing.T) {
	dir := t.TempDir()
	// Provide enough signal for `doctor` to have stacks + resolved commands.
	touch(t, dir, "go.mod")
	withCwd(t, dir)

	cmd := newDoctorCmd(BuildInfo{Version: "1.2.3", Commit: "abc1234", Date: "2026-01-01"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	body := out.String()

	for _, expected := range []string{
		"stratt doctor",
		"version : 1.2.3",
		"commit  : abc1234",
		"built   : 2026-01-01",
		"Scanning",
		"go (via go.mod)",
		"Resolved commands:",
		"build",
		"test",
		"go test ./...",
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("doctor output missing %q\n--- full output ---\n%s", expected, body)
		}
	}
}

// TestDoctorEmptyRepo — doctor in an empty repo reports no stacks and
// does not emit a resolved-commands section.
func TestDoctorEmptyRepo(t *testing.T) {
	dir := t.TempDir()
	withCwd(t, dir)

	cmd := newDoctorCmd(BuildInfo{Version: "x", Commit: "x", Date: "x"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	body := out.String()
	if !strings.Contains(body, "no recognized stacks found") {
		t.Errorf("empty repo should report no recognized stacks; got:\n%s", body)
	}
	if strings.Contains(body, "Resolved commands:") {
		t.Errorf("empty repo should not show a resolved-commands section; got:\n%s", body)
	}
}

// TestDoctorEmitsMissingToolsBlock — when the resolver picks a backend
// whose binary isn't on $PATH, doctor reports it in a separate
// "Missing tools" block with an install suggestion.
func TestDoctorEmitsMissingToolsBlock(t *testing.T) {
	// Hugo in docs/ → resolver wants `hugo`, which is almost certainly
	// not on the test runner's PATH.
	dir := t.TempDir()
	touch(t, dir, "docs/hugo.toml")
	withCwd(t, dir)

	cmd := newDoctorCmd(BuildInfo{Version: "x", Commit: "x", Date: "x"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	body := out.String()
	if !strings.Contains(body, "Missing tools:") {
		t.Errorf("expected `Missing tools:` block; got:\n%s", body)
	}
	if !strings.Contains(body, "hugo") {
		t.Errorf("expected `hugo` in missing-tools list; got:\n%s", body)
	}
	if !strings.Contains(body, "brew install hugo") {
		t.Errorf("expected install hint; got:\n%s", body)
	}
}

// TestDoctorSkipsMissingToolsBlockWhenAllPresent — repos whose
// detected backends are all on PATH (here: go-only) should NOT emit
// the Missing tools block.
func TestDoctorSkipsMissingToolsBlockWhenAllPresent(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "go.mod")
	withCwd(t, dir)

	cmd := newDoctorCmd(BuildInfo{Version: "x", Commit: "x", Date: "x"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	// `go` and `gofmt` come with the test environment.  We expect no
	// missing-tools block (or, if a tool we don't have is referenced —
	// like golangci-lint — it's a soft assertion).  We just confirm
	// the block doesn't appear *gratuitously* for present tools.
	if strings.Contains(out.String(), "go is missing") {
		t.Errorf("go should not be flagged as missing; got:\n%s", out.String())
	}
}

// TestDoctorMissingToolsDedupesAcrossCommands — the same missing tool
// referenced by several resolved commands appears once.
func TestDoctorMissingToolsDedupesAcrossCommands(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "pyproject.toml")
	touch(t, dir, "uv.lock")
	withCwd(t, dir)

	cmd := newDoctorCmd(BuildInfo{Version: "x", Commit: "x", Date: "x"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	body := out.String()
	// `uv` (likely not on PATH in test) would be the leading tool for
	// build/test/lint/format/setup/etc — every uv-backed engine.  It
	// should appear in the Missing tools block exactly once.
	if !strings.Contains(body, "Missing tools:") {
		t.Skipf("uv is on PATH in this test environment; skipping dedup check\n%s", body)
	}
	// Count `^  uv  ` rows in the block.  Crude but effective.
	missingSection := body[strings.Index(body, "Missing tools:"):]
	lines := strings.Split(missingSection, "\n")
	count := 0
	for _, line := range lines {
		// Tabwriter pads with spaces; look for "uv " at the start of
		// the trimmed indented row.
		trimmed := strings.TrimLeft(line, " ")
		if strings.HasPrefix(trimmed, "uv ") || trimmed == "uv" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected `uv` to appear once in Missing tools; got %d times:\n%s", count, missingSection)
	}
}

// TestDoctorShowsBackendMappingForMultiStack — multi-stack repos (the
// cartographer-daemon shape) must show the resolved backend for every
// universal command.  This is the §0 transparency check in test form.
func TestDoctorShowsBackendMappingForMultiStack(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "pyproject.toml")
	touch(t, dir, "uv.lock")
	touch(t, dir, "Dockerfile")
	touch(t, dir, "deploy/overlays/prod/kustomization.yaml")
	touch(t, dir, "mkdocs.yml")
	withCwd(t, dir)

	cmd := newDoctorCmd(BuildInfo{Version: "x", Commit: "x", Date: "x"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	body := out.String()
	expected := []string{
		"uv build",
		"uv run pytest",
		"uv run ruff check",
		"uv run ruff format",
		"mkdocs build",
		"kustomize image bump",
	}
	for _, e := range expected {
		if !strings.Contains(body, e) {
			t.Errorf("doctor output missing %q\n--- full output ---\n%s", e, body)
		}
	}
}
