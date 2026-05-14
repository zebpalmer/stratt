package bump

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestKindFromString(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want Kind
		ok   bool
	}{
		{"patch", Patch, true},
		{"minor", Minor, true},
		{"major", Major, true},
		{"PATCH", Patch, true},
		{" Major ", Major, true},
		{"", 0, false},
		{"bogus", 0, false},
	} {
		got, err := KindFromString(tc.in)
		if tc.ok && err != nil {
			t.Errorf("%q: unexpected error %v", tc.in, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("%q: expected error", tc.in)
		}
		if tc.ok && got != tc.want {
			t.Errorf("%q: got %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestBumpSemver(t *testing.T) {
	cases := []struct {
		current string
		kind    Kind
		want    string
	}{
		{"1.0.0", Patch, "1.0.1"},
		{"1.0.0", Minor, "1.1.0"},
		{"1.0.0", Major, "2.0.0"},
		{"1.2.3", Patch, "1.2.4"},
		{"1.2.3", Minor, "1.3.0"},
		{"1.2.3", Major, "2.0.0"},
		{"v1.2.3", Patch, "1.2.4"},
		{"1.2.3-dev", Patch, "1.2.4"}, // prerelease dropped
	}
	for _, c := range cases {
		got, err := bumpSemver(c.current, c.kind)
		if err != nil {
			t.Errorf("%s+%v: %v", c.current, c.kind, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s+%v: got %s, want %s", c.current, c.kind, got, c.want)
		}
	}
}

func TestBumpSemverInvalid(t *testing.T) {
	for _, v := range []string{"", "1", "1.2", "1.2.3.4", "abc", "1.2.x"} {
		if _, err := bumpSemver(v, Patch); err == nil {
			t.Errorf("expected error for %q", v)
		}
	}
}

func TestIsValid(t *testing.T) {
	if !IsValid("1.2.3") {
		t.Error("1.2.3 should be valid")
	}
	if !IsValid("v1.2.3") {
		t.Error("v1.2.3 should be valid (we strip the leading v)")
	}
	if IsValid("1.2") {
		t.Error("1.2 should not be valid")
	}
}

func TestComputeFindsAndReportsChanges(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", `version = "1.2.3"`)

	cfg := &Config{
		CurrentVersion: "1.2.3",
		Files: []FileEntry{
			{
				Filename: "pyproject.toml",
				Search:   `version = "{current_version}"`,
				Replace:  `version = "{new_version}"`,
			},
		},
	}
	plan, err := Compute(cfg, Patch, dir)
	if err != nil {
		t.Fatal(err)
	}
	if plan.NewVersion != "1.2.4" {
		t.Errorf("NewVersion: got %q, want 1.2.4", plan.NewVersion)
	}
	if len(plan.FileChanges) != 1 {
		t.Fatalf("FileChanges: got %d, want 1", len(plan.FileChanges))
	}
	c := plan.FileChanges[0]
	if !c.Found {
		t.Error("expected Found=true")
	}
	if c.OldChunk != `version = "1.2.3"` || c.NewChunk != `version = "1.2.4"` {
		t.Errorf("substitution wrong: old=%q new=%q", c.OldChunk, c.NewChunk)
	}
}

func TestComputeReportsMissingChunk(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", `something = "else"`)

	cfg := &Config{
		CurrentVersion: "1.2.3",
		Files: []FileEntry{
			{Filename: "pyproject.toml", Search: `version = "{current_version}"`, Replace: `version = "{new_version}"`},
		},
	}
	plan, err := Compute(cfg, Patch, dir)
	if err != nil {
		t.Fatal(err)
	}
	if plan.FileChanges[0].Found {
		t.Error("expected Found=false for missing version string")
	}
}

func TestComputeAppliesTemplateDefaults(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "VERSION", "1.0.0")

	cfg := &Config{
		CurrentVersion: "1.0.0",
		// No per-file Search/Replace and no top-level template — defaults
		// should yield search="{current_version}" replace="{new_version}".
		Files: []FileEntry{{Filename: "VERSION"}},
	}
	plan, err := Compute(cfg, Patch, dir)
	if err != nil {
		t.Fatal(err)
	}
	c := plan.FileChanges[0]
	if !c.Found || c.OldChunk != "1.0.0" || c.NewChunk != "1.0.1" {
		t.Errorf("got %+v", c)
	}
}

func TestApplyWritesChanges(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", `version = "1.0.0"
description = "x"`)

	cfg := &Config{
		CurrentVersion: "1.0.0",
		Files: []FileEntry{
			{Filename: "pyproject.toml",
				Search:  `version = "{current_version}"`,
				Replace: `version = "{new_version}"`},
		},
	}
	plan, err := Compute(cfg, Patch, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "pyproject.toml"))
	if !strings.Contains(string(got), `version = "1.0.1"`) {
		t.Errorf("file not rewritten:\n%s", string(got))
	}
	if !strings.Contains(string(got), `description = "x"`) {
		t.Errorf("unrelated content lost:\n%s", string(got))
	}
}

func TestApplyRejectsMissingChunk(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "x", "no version here")
	cfg := &Config{
		CurrentVersion: "1.0.0",
		Files:          []FileEntry{{Filename: "x"}},
	}
	plan, err := Compute(cfg, Patch, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err == nil {
		t.Fatal("expected ErrMissingVersion")
	}
}

func TestComputeCommitAndTagTemplates(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "v", "1.0.0")

	cfg := &Config{
		CurrentVersion:  "1.0.0",
		MessageTemplate: "release: {current_version} → {new_version}",
		TagNameTemplate: "release/{new_version}",
		Files:           []FileEntry{{Filename: "v"}},
	}
	plan, err := Compute(cfg, Minor, dir)
	if err != nil {
		t.Fatal(err)
	}
	if plan.CommitMessage != "release: 1.0.0 → 1.1.0" {
		t.Errorf("commit message: got %q", plan.CommitMessage)
	}
	if plan.TagName != "release/1.1.0" {
		t.Errorf("tag name: got %q", plan.TagName)
	}
}

// TestLoadFromStrattToml — native [bump] section in stratt.toml.
func TestLoadFromStrattToml(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "stratt.toml", `
[bump]
current_version = "0.5.0"
[[bump.files]]
filename = "pyproject.toml"
search = "version = \"{current_version}\""
replace = "version = \"{new_version}\""
`)
	cfg, warn, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if warn != "" {
		t.Errorf("unexpected warning: %s", warn)
	}
	if cfg == nil {
		t.Fatal("expected config from stratt.toml [bump]")
	}
	if cfg.CurrentVersion != "0.5.0" {
		t.Errorf("got %q", cfg.CurrentVersion)
	}
	if len(cfg.Files) != 1 || cfg.Files[0].Filename != "pyproject.toml" {
		t.Errorf("files: %+v", cfg.Files)
	}
}

// TestLoadFromPyprojectStratt — native [tool.stratt.bump] in pyproject.
func TestLoadFromPyprojectStratt(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", `
[tool.stratt.bump]
current_version = "1.0.0"
[[tool.stratt.bump.files]]
filename = "VERSION"
`)
	cfg, _, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || cfg.CurrentVersion != "1.0.0" {
		t.Errorf("got %+v", cfg)
	}
}

// TestLoadFromPyprojectBumpversion — legacy [tool.bumpversion] still works.
func TestLoadFromPyprojectBumpversion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", `
[tool.bumpversion]
current_version = "1.14.1"
search = "{current_version}"
replace = "{new_version}"
allow_dirty = false
tag = true
commit = true

[[tool.bumpversion.files]]
filename = "pyproject.toml"
search = 'version = "{current_version}"'
replace = 'version = "{new_version}"'
`)
	cfg, _, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected bumpversion config")
	}
	if cfg.CurrentVersion != "1.14.1" {
		t.Errorf("got %q", cfg.CurrentVersion)
	}
	if !cfg.Commit || !cfg.Tag {
		t.Errorf("commit/tag flags lost: %+v", cfg)
	}
	if len(cfg.Files) != 1 {
		t.Errorf("files: %+v", cfg.Files)
	}
}

// TestLoadFromBumpversionToml — standalone .bumpversion.toml.
func TestLoadFromBumpversionToml(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".bumpversion.toml", `
current_version = "2.0.0"
[[files]]
filename = "VERSION"
`)
	cfg, _, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || cfg.CurrentVersion != "2.0.0" {
		t.Errorf("got %+v", cfg)
	}
}

// TestLoadFromBumpversionCfgIsDeprecated — .bumpversion.cfg (INI) emits
// a deprecation warning and returns no config.
func TestLoadFromBumpversionCfgIsDeprecated(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".bumpversion.cfg", "[bumpversion]\ncurrent_version = 1.0.0\n")
	cfg, warn, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Errorf("INI cfg should not produce a usable config in v1; got %+v", cfg)
	}
	if !strings.Contains(warn, ".bumpversion.cfg") {
		t.Errorf("expected deprecation note; got %q", warn)
	}
}

// TestLoadPriorityOrder — when multiple locations exist, the first in
// the chain wins (R2.4.7).
func TestLoadPriorityOrder(t *testing.T) {
	dir := t.TempDir()
	// Both stratt.toml [bump] and [tool.bumpversion] in pyproject exist;
	// the native location wins.
	writeFile(t, dir, "stratt.toml", `
[bump]
current_version = "9.9.9"
`)
	writeFile(t, dir, "pyproject.toml", `
[tool.bumpversion]
current_version = "1.0.0"
`)
	cfg, _, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CurrentVersion != "9.9.9" {
		t.Errorf("native [bump] should win; got %q", cfg.CurrentVersion)
	}
}

// TestLoadNothingPresent — empty repo returns (nil, "", nil).
func TestLoadNothingPresent(t *testing.T) {
	cfg, warn, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Errorf("expected nil config, got %+v", cfg)
	}
	if warn != "" {
		t.Errorf("expected no warning, got %q", warn)
	}
}
