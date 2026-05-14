package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTest(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestMigrateBumpPyprojectIntoStrattToml — if stratt.toml exists, the
// migration writes [bump] there.
func TestMigrateBumpPyprojectIntoStrattToml(t *testing.T) {
	dir := t.TempDir()
	writeTest(t, dir, "stratt.toml", "# placeholder\n")
	writeTest(t, dir, "pyproject.toml", `
[tool.bumpversion]
current_version = "1.0.0"
[[tool.bumpversion.files]]
filename = "VERSION"
`)
	var buf bytes.Buffer
	target, source, err := MigrateBump(dir, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(target, "stratt.toml") {
		t.Errorf("target should be stratt.toml; got %q", target)
	}
	if !strings.Contains(source, "pyproject.toml") {
		t.Errorf("source should be pyproject; got %q", source)
	}
	body, _ := os.ReadFile(target)
	if !strings.Contains(string(body), `current_version = '1.0.0'`) &&
		!strings.Contains(string(body), `current_version = "1.0.0"`) {
		t.Errorf("target should contain current_version; got:\n%s", body)
	}
}

// TestMigrateBumpPyprojectInPlace — no stratt.toml, but pyproject exists:
// the migration writes [tool.stratt.bump] into pyproject.
func TestMigrateBumpPyprojectInPlace(t *testing.T) {
	dir := t.TempDir()
	writeTest(t, dir, "pyproject.toml", `
[tool.bumpversion]
current_version = "2.0.0"
`)
	var buf bytes.Buffer
	target, _, err := MigrateBump(dir, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(target, "pyproject.toml") {
		t.Errorf("target should be pyproject; got %q", target)
	}
	body, _ := os.ReadFile(target)
	if !strings.Contains(string(body), "[tool.stratt.bump]") {
		t.Errorf("target should contain [tool.stratt.bump]; got:\n%s", body)
	}
}

// TestMigrateBumpFromStandaloneBumpversionToml — .bumpversion.toml as source.
func TestMigrateBumpFromStandaloneBumpversionToml(t *testing.T) {
	dir := t.TempDir()
	writeTest(t, dir, ".bumpversion.toml", `
current_version = "0.5.0"
`)
	var buf bytes.Buffer
	target, source, err := MigrateBump(dir, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(source, ".bumpversion.toml") {
		t.Errorf("source: got %q", source)
	}
	if !strings.HasSuffix(target, "stratt.toml") {
		t.Errorf("target should be a new stratt.toml; got %q", target)
	}
	body, _ := os.ReadFile(target)
	if !strings.Contains(string(body), "[bump]") {
		t.Errorf("target should contain [bump]; got:\n%s", body)
	}
}

func TestMigrateBumpNoLegacyConfigErrors(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	_, _, err := MigrateBump(dir, &buf)
	if err == nil {
		t.Fatal("expected error when no legacy config exists")
	}
}

// TestMigrateBumpRoundTripIsLoadable — after migration, the new
// location parses as a valid stratt bump source.
func TestMigrateBumpRoundTripIsLoadable(t *testing.T) {
	dir := t.TempDir()
	writeTest(t, dir, "pyproject.toml", `
[tool.bumpversion]
current_version = "3.4.5"
[[tool.bumpversion.files]]
filename = "VERSION"
`)
	if _, _, err := MigrateBump(dir, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	// stratt.toml didn't exist, so target was pyproject.toml.
	// Reload Project; required_stratt is empty but the file parses.
	if _, err := Load(dir); err != nil {
		t.Fatalf("post-migration Load failed: %v", err)
	}
}
