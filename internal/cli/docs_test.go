package cli

import (
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
