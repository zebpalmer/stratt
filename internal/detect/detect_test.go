package detect

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// touch creates an empty file inside root, including any parent directories.
func touch(t *testing.T, root, rel string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestDetectGo(t *testing.T) {
	dir := t.TempDir()
	if got := detectGo(dir); got.Name != "" {
		t.Fatalf("empty repo should not match: got %+v", got)
	}
	touch(t, dir, "go.mod")
	if got := detectGo(dir); got.Name != "go" || got.Signal != "go.mod" {
		t.Fatalf("go.mod present: got %+v", got)
	}
}

func TestDetectPythonUV(t *testing.T) {
	dir := t.TempDir()

	touch(t, dir, "pyproject.toml")
	if got := detectPythonUV(dir); got.Name != "" {
		t.Fatalf("pyproject without uv.lock should not match: got %+v", got)
	}

	touch(t, dir, "uv.lock")
	if got := detectPythonUV(dir); got.Name != "python+uv" {
		t.Fatalf("pyproject + uv.lock: got %+v", got)
	}
}

func TestDetectPHP(t *testing.T) {
	dir := t.TempDir()
	if got := detectPHP(dir); got.Name != "" {
		t.Fatalf("empty repo: got %+v", got)
	}
	touch(t, dir, "composer.json")
	if got := detectPHP(dir); got.Name != "php" {
		t.Fatalf("composer.json present: got %+v", got)
	}
}

func TestDetectDocker(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "Dockerfile")
	if got := detectDocker(dir); got.Name != "docker" {
		t.Fatalf("Dockerfile present: got %+v", got)
	}
}

func TestDetectKustomize(t *testing.T) {
	dir := t.TempDir()

	// Just the overlays directory with no kustomization.yaml does not match.
	if err := os.MkdirAll(filepath.Join(dir, "deploy", "overlays", "prod"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := detectKustomize(dir); got.Name != "" {
		t.Fatalf("empty overlay should not match: got %+v", got)
	}

	touch(t, dir, "deploy/overlays/prod/kustomization.yaml")
	if got := detectKustomize(dir); got.Name != "kustomize" {
		t.Fatalf("overlay with kustomization.yaml: got %+v", got)
	}
}

func TestDetectMkDocs(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "mkdocs.yml")
	if got := detectMkDocs(dir); got.Name != "mkdocs" {
		t.Fatalf("mkdocs.yml present: got %+v", got)
	}
}

func TestDetectSphinx(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "docs/conf.py")
	if got := detectSphinx(dir); got.Name != "sphinx" {
		t.Fatalf("docs/conf.py present: got %+v", got)
	}
}

// TestScanMultiStack mirrors the cartographer-daemon shape:
// python+uv + docker + kustomize + mkdocs.  Names should come back
// sorted alphabetically per Scan's contract.
func TestScanMultiStack(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "pyproject.toml")
	touch(t, dir, "uv.lock")
	touch(t, dir, "Dockerfile")
	touch(t, dir, "deploy/overlays/prod/kustomization.yaml")
	touch(t, dir, "mkdocs.yml")

	got := Scan(dir)
	if got.Root != dir {
		t.Errorf("Root: got %q, want %q", got.Root, dir)
	}

	names := make([]string, len(got.Stacks))
	for i, s := range got.Stacks {
		names[i] = s.Name
	}
	want := []string{"docker", "kustomize", "mkdocs", "python+uv"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("stack names: got %v, want %v", names, want)
	}

	// Confirm sorted ordering is stable.
	if !sort.StringsAreSorted(names) {
		t.Errorf("stacks should be returned in sorted order, got %v", names)
	}
}

func TestScanEmpty(t *testing.T) {
	dir := t.TempDir()
	got := Scan(dir)
	if len(got.Stacks) != 0 {
		t.Errorf("empty repo: expected 0 stacks, got %v", got.Stacks)
	}
}
