package bump

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeCfg(t *testing.T, root, name, body string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseINIBasic(t *testing.T) {
	body := `
# comment
[bumpversion]
current_version = 1.2.3
commit = True
tag = True

[bumpversion:file:./pyproject.toml]
search = version = "{current_version}"
replace = version = "{new_version}"

; another comment
[bumpversion:file:src/__init__.py]
`
	sections, err := parseINI([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(sections))
	}
	if sections[0].Name != "bumpversion" {
		t.Errorf("section 0: %q", sections[0].Name)
	}
	if sections[0].KV["current_version"] != "1.2.3" {
		t.Errorf("current_version: %q", sections[0].KV["current_version"])
	}
	if sections[1].Name != "bumpversion:file:./pyproject.toml" {
		t.Errorf("section 1: %q", sections[1].Name)
	}
}

func TestParseINIBool(t *testing.T) {
	cases := []struct {
		in   string
		def  bool
		want bool
	}{
		{"True", false, true},
		{"true", false, true},
		{"TRUE", false, true},
		{"yes", false, true},
		{"False", true, false},
		{"no", true, false},
		{"off", true, false},
		{"", true, true},   // default applies
		{"", false, false}, // default applies
		{"garbage", true, true},
	}
	for _, c := range cases {
		if got := parseINIBool(c.in, c.def); got != c.want {
			t.Errorf("parseINIBool(%q, %v) = %v, want %v", c.in, c.def, got, c.want)
		}
	}
}

// TestLoadBumpversionINIFleetExample — mirrors the actual format
// found in wraith-daemon and other LCG repos as of 2026-05.
func TestLoadBumpversionINIFleetExample(t *testing.T) {
	dir := t.TempDir()
	writeCfg(t, dir, ".bumpversion.cfg", `[bumpversion]
current_version = 0.9.6
commit = True
tag = True
tag_name = {new_version}

[bumpversion:file:./wraithd/__init__.py]
search = __version__ = "{current_version}"
replace = __version__ = "{new_version}"
`)

	cfg, warn, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected config from .bumpversion.cfg")
	}
	if !strings.Contains(warn, ".bumpversion.cfg") {
		t.Errorf("expected deprecation warning; got %q", warn)
	}
	if cfg.CurrentVersion != "0.9.6" {
		t.Errorf("current_version: got %q", cfg.CurrentVersion)
	}
	if cfg.TagNameTemplate != "{new_version}" {
		t.Errorf("tag_name: got %q", cfg.TagNameTemplate)
	}
	if !cfg.Commit || !cfg.Tag {
		t.Errorf("commit/tag should be true; got commit=%v tag=%v", cfg.Commit, cfg.Tag)
	}

	// The user-specified file entry + the auto-added source file (.bumpversion.cfg).
	if len(cfg.Files) != 2 {
		t.Fatalf("expected 2 files (user + auto-source); got %d: %+v", len(cfg.Files), cfg.Files)
	}
	// Find the user-supplied wraithd/__init__.py entry.
	var wraithd *FileEntry
	for i := range cfg.Files {
		if strings.Contains(cfg.Files[i].Filename, "wraithd") {
			wraithd = &cfg.Files[i]
			break
		}
	}
	if wraithd == nil {
		t.Fatalf("expected wraithd entry in Files: %+v", cfg.Files)
	}
	want := FileEntry{
		Filename: "wraithd/__init__.py",
		Search:   `__version__ = "{current_version}"`,
		Replace:  `__version__ = "{new_version}"`,
	}
	if !reflect.DeepEqual(*wraithd, want) {
		t.Errorf("wraithd entry:\n  got:  %+v\n  want: %+v", *wraithd, want)
	}
}

// TestLoadBumpversionINIEndToEnd — full bump cycle against an INI repo.
func TestLoadBumpversionINIEndToEnd(t *testing.T) {
	dir := t.TempDir()
	writeCfg(t, dir, ".bumpversion.cfg", `[bumpversion]
current_version = 1.0.0
commit = True
tag = True

[bumpversion:file:./VERSION]
`)
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Compute(cfg, Minor, dir)
	if err != nil {
		t.Fatal(err)
	}
	if plan.NewVersion != "1.1.0" {
		t.Errorf("new version: got %s", plan.NewVersion)
	}
	if err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "VERSION"))
	if !strings.Contains(string(body), "1.1.0") {
		t.Errorf("VERSION not rewritten:\n%s", body)
	}
	cfgBody, _ := os.ReadFile(filepath.Join(dir, ".bumpversion.cfg"))
	if !strings.Contains(string(cfgBody), "current_version = 1.1.0") {
		t.Errorf("source .bumpversion.cfg current_version not rewritten:\n%s", cfgBody)
	}
}

// TestLoadINIDefaultsForCommitAndTag — bumpversion's defaults are
// commit=True, tag=True.  Missing keys should yield those defaults.
func TestLoadINIDefaultsForCommitAndTag(t *testing.T) {
	dir := t.TempDir()
	writeCfg(t, dir, ".bumpversion.cfg", `[bumpversion]
current_version = 1.0.0
`)
	cfg, _, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Commit || !cfg.Tag {
		t.Errorf("missing keys should default true; got commit=%v tag=%v", cfg.Commit, cfg.Tag)
	}
}
