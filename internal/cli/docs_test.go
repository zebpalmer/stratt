package cli

import (
	"strings"
	"testing"
)

func TestDocsCommandSelectsMkDocs(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "mkdocs.yml")
	tool, argv, err := docsCommand(dir, "build")
	if err != nil {
		t.Fatal(err)
	}
	if tool != "mkdocs" || len(argv) != 1 || argv[0] != "build" {
		t.Errorf("got %s %v", tool, argv)
	}
	tool, argv, err = docsCommand(dir, "serve")
	if err != nil {
		t.Fatal(err)
	}
	if tool != "mkdocs" || argv[0] != "serve" {
		t.Errorf("serve: got %s %v", tool, argv)
	}
}

func TestDocsCommandSelectsSphinx(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "docs/conf.py")
	tool, _, err := docsCommand(dir, "build")
	if err != nil {
		t.Fatal(err)
	}
	if tool != "sphinx-build" {
		t.Errorf("build: got %s", tool)
	}
	tool, _, err = docsCommand(dir, "serve")
	if err != nil {
		t.Fatal(err)
	}
	if tool != "sphinx-autobuild" {
		t.Errorf("serve: got %s", tool)
	}
}

func TestDocsCommandErrorsWithoutToolchain(t *testing.T) {
	if _, _, err := docsCommand(t.TempDir(), "build"); err == nil {
		t.Fatal("expected error when no docs toolchain detected")
	}
}

func TestDocsCommandSelectsHugoInDocsSubdir(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "docs/hugo.toml")

	tool, argv, err := docsCommand(dir, "build")
	if err != nil {
		t.Fatal(err)
	}
	if tool != "hugo" {
		t.Errorf("tool: got %q", tool)
	}
	// argv should include --source docs and --minify in some order.
	joined := strings.Join(argv, " ")
	if !strings.Contains(joined, "--source docs") {
		t.Errorf("expected --source docs; got %v", argv)
	}
	if !strings.Contains(joined, "--minify") {
		t.Errorf("expected --minify; got %v", argv)
	}

	tool, argv, err = docsCommand(dir, "serve")
	if err != nil {
		t.Fatal(err)
	}
	if tool != "hugo" || argv[0] != "server" {
		t.Errorf("serve: got %s %v", tool, argv)
	}
}

func TestDocsCommandSelectsHugoAtRoot(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "hugo.toml")

	tool, argv, err := docsCommand(dir, "build")
	if err != nil {
		t.Fatal(err)
	}
	if tool != "hugo" {
		t.Errorf("tool: got %q", tool)
	}
	// No --source when Hugo lives at the repo root.
	for _, a := range argv {
		if a == "--source" {
			t.Errorf("did not expect --source when Hugo is at root; got %v", argv)
		}
	}
}
