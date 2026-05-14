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
