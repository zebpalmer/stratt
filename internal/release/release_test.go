package release

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LacalleGroup/stratt/internal/bump"
)

func mustRun(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// setupRepo creates a fresh git repo with a pyproject.toml,
// [tool.bumpversion] config, and an initial committed state on main.
// Push is intentionally not configured — tests pass Push: false.
func setupRepo(t *testing.T, currentVersion string) string {
	t.Helper()
	dir := t.TempDir()
	mustRun(t, dir, "git", "init", "--initial-branch=main", "-q")
	mustRun(t, dir, "git", "config", "user.email", "test@example.com")
	mustRun(t, dir, "git", "config", "user.name", "Test User")
	mustRun(t, dir, "git", "config", "commit.gpgsign", "false")
	mustRun(t, dir, "git", "config", "tag.gpgsign", "false")

	pyproject := `[project]
name = "x"
version = "` + currentVersion + `"

[tool.bumpversion]
current_version = "` + currentVersion + `"
commit = true
tag = true

[[tool.bumpversion.files]]
filename = "pyproject.toml"
search = "version = \"{current_version}\""
replace = "version = \"{new_version}\""
`
	writeFile(t, dir, "pyproject.toml", pyproject)
	mustRun(t, dir, "git", "add", "-A")
	mustRun(t, dir, "git", "commit", "-q", "-m", "initial")
	return dir
}

// TestReleaseNonInteractivePatchSuccess — the happy path with explicit
// --type=patch and --ci.  Verifies file rewrite, commit, and tag.
func TestReleaseNonInteractivePatchSuccess(t *testing.T) {
	dir := setupRepo(t, "1.2.3")
	var stdout, stderr bytes.Buffer

	err := Run(context.Background(), Options{
		CWD:     dir,
		Kind:    bump.Patch,
		HasKind: true,
		CI:      true,
		Push:    false, // can't push in a test repo with no remote
		Stdin:   strings.NewReader(""),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if err != nil {
		t.Fatalf("release failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	// File rewrite.
	body, _ := os.ReadFile(filepath.Join(dir, "pyproject.toml"))
	if !strings.Contains(string(body), `version = "1.2.4"`) {
		t.Errorf("pyproject not rewritten:\n%s", body)
	}

	// Tag created.
	tagOut, _ := exec.Command("git", "-C", dir, "tag", "-l").CombinedOutput()
	if !strings.Contains(string(tagOut), "v1.2.4") {
		t.Errorf("expected v1.2.4 tag, got: %s", tagOut)
	}

	// Commit created with the templated message.
	logOut, _ := exec.Command("git", "-C", dir, "log", "-1", "--pretty=%s").CombinedOutput()
	if !strings.Contains(string(logOut), "1.2.3") || !strings.Contains(string(logOut), "1.2.4") {
		t.Errorf("commit message: %s", logOut)
	}
}

func TestReleasePreflightWrongBranchFails(t *testing.T) {
	dir := setupRepo(t, "1.0.0")
	mustRun(t, dir, "git", "checkout", "-b", "feature")

	err := Run(context.Background(), Options{
		CWD:     dir,
		Kind:    bump.Patch,
		HasKind: true,
		CI:      true,
		Branch:  "main",
		Push:    false,
		Stdin:   strings.NewReader(""),
		Stdout:  &bytes.Buffer{},
		Stderr:  &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected branch-check failure")
	}
	if !strings.Contains(err.Error(), "feature") || !strings.Contains(err.Error(), "main") {
		t.Errorf("error should cite both branches: %v", err)
	}
}

func TestReleasePreflightDirtyTreeFails(t *testing.T) {
	dir := setupRepo(t, "1.0.0")
	writeFile(t, dir, "dirty.txt", "uncommitted")

	err := Run(context.Background(), Options{
		CWD:     dir,
		Kind:    bump.Patch,
		HasKind: true,
		CI:      true,
		Push:    false,
		Stdin:   strings.NewReader(""),
		Stdout:  &bytes.Buffer{},
		Stderr:  &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected clean-tree check to fail")
	}
	if !strings.Contains(err.Error(), "clean") {
		t.Errorf("error should mention cleanliness: %v", err)
	}
}

func TestReleaseCIRequiresKind(t *testing.T) {
	dir := setupRepo(t, "1.0.0")
	err := Run(context.Background(), Options{
		CWD:    dir,
		CI:     true,
		Push:   false,
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected --ci-without-kind error")
	}
	if !strings.Contains(err.Error(), "--ci") {
		t.Errorf("error should reference --ci: %v", err)
	}
}

func TestReleaseInteractivePromptPatch(t *testing.T) {
	dir := setupRepo(t, "1.0.0")
	// Provide "p\n" for the prompt, then "y\n" for the final confirm.
	stdin := strings.NewReader("p\ny\n")
	var stdout, stderr bytes.Buffer

	err := Run(context.Background(), Options{
		CWD:    dir,
		Push:   false,
		Stdin:  stdin,
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		t.Fatalf("release failed: %v\nstdout: %s", err, stdout.String())
	}
	body, _ := os.ReadFile(filepath.Join(dir, "pyproject.toml"))
	if !strings.Contains(string(body), `version = "1.0.1"`) {
		t.Errorf("expected 1.0.1, got:\n%s", body)
	}
}

func TestReleaseInteractiveUserAborts(t *testing.T) {
	dir := setupRepo(t, "1.0.0")
	// Type 'patch', then 'n' to reject the final confirmation.
	stdin := strings.NewReader("patch\nn\n")
	err := Run(context.Background(), Options{
		CWD:    dir,
		Push:   false,
		Stdin:  stdin,
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected aborted-by-user error")
	}
	// Original file unchanged.
	body, _ := os.ReadFile(filepath.Join(dir, "pyproject.toml"))
	if !strings.Contains(string(body), `version = "1.0.0"`) {
		t.Errorf("abort should leave file unchanged; got:\n%s", body)
	}
}

func TestReleaseMajorRequiresExtraConfirmation(t *testing.T) {
	dir := setupRepo(t, "1.0.0")
	// Type 'major', then 'n' to the major confirmation.
	stdin := strings.NewReader("major\nn\n")
	err := Run(context.Background(), Options{
		CWD:    dir,
		Push:   false,
		Stdin:  stdin,
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected major-bump abort")
	}
	if !strings.Contains(err.Error(), "major") {
		t.Errorf("error should reference major: %v", err)
	}
}

func TestReleaseNoBumpConfigErrors(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "git", "init", "--initial-branch=main", "-q")
	mustRun(t, dir, "git", "config", "user.email", "test@example.com")
	mustRun(t, dir, "git", "config", "user.name", "Test User")
	// No commits = clean tree, branch = main, but no bump config.
	err := Run(context.Background(), Options{
		CWD:     dir,
		Kind:    bump.Patch,
		HasKind: true,
		CI:      true,
		Push:    false,
		Stdin:   strings.NewReader(""),
		Stdout:  &bytes.Buffer{},
		Stderr:  &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected no-bump-config error")
	}
	if !strings.Contains(err.Error(), "bump") {
		t.Errorf("error should reference bump config: %v", err)
	}
}

// TestReleaseLocalOnlyPrintsPushHint — Push: false should leave the
// commit + tag locally and print the push command.
func TestReleaseLocalOnlyPrintsPushHint(t *testing.T) {
	dir := setupRepo(t, "1.0.0")
	var stdout bytes.Buffer

	err := Run(context.Background(), Options{
		CWD:     dir,
		Kind:    bump.Patch,
		HasKind: true,
		CI:      true,
		Push:    false,
		Stdin:   strings.NewReader(""),
		Stdout:  &stdout,
		Stderr:  &bytes.Buffer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Push with:") {
		t.Errorf("expected push hint; got:\n%s", out)
	}
	if !strings.Contains(out, "git push origin main") {
		t.Errorf("hint should mention `git push origin main`; got:\n%s", out)
	}
}
