package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetRequiredStrattInStrattToml(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stratt.toml")
	if err := os.WriteFile(path, []byte("# placeholder\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetRequiredStratt(path, ">= 1.5.0"); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), `required_stratt = '>= 1.5.0'`) &&
		!strings.Contains(string(body), `required_stratt = ">= 1.5.0"`) {
		t.Errorf("file should contain required_stratt; got:\n%s", body)
	}
}

func TestSetRequiredStrattInPyprojectToml(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyproject.toml")
	if err := os.WriteFile(path, []byte(`
[project]
name = "x"

[tool.stratt]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetRequiredStratt(path, ">= 2.0.0"); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	// Must appear under [tool.stratt], not at top level.
	if !strings.Contains(string(body), "[tool.stratt]") {
		t.Errorf("expected [tool.stratt] section; got:\n%s", body)
	}
	if !strings.Contains(string(body), `>= 2.0.0`) {
		t.Errorf("expected required_stratt value:\n%s", body)
	}
}

func TestSetRequiredStrattRoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stratt.toml")
	_ = os.WriteFile(path, []byte("# x\n"), 0o644)

	if err := SetRequiredStratt(path, ">= 1.0.0"); err != nil {
		t.Fatal(err)
	}
	proj, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if proj.RequiredStratt != ">= 1.0.0" {
		t.Errorf("got %q", proj.RequiredStratt)
	}
}
