package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitInit prepares a fresh repo and returns its path.  Configures
// user.name and user.email so commits work without an ambient git
// config.
func gitInit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustRun(t, dir, "git", "init", "--initial-branch=main", "-q")
	mustRun(t, dir, "git", "config", "user.email", "test@example.com")
	mustRun(t, dir, "git", "config", "user.name", "Test User")
	mustRun(t, dir, "git", "config", "commit.gpgsign", "false")
	mustRun(t, dir, "git", "config", "tag.gpgsign", "false")
	return dir
}

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
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBranchAndCleanOnFreshRepo(t *testing.T) {
	dir := gitInit(t)
	r := New(dir)

	// Brand-new repo with no commits yet: branch is "main" (per --initial-branch).
	br, err := r.Branch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if br != "main" {
		t.Errorf("branch: got %q, want main", br)
	}

	clean, err := r.IsClean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !clean {
		t.Error("empty repo should be clean")
	}

	writeFile(t, dir, "foo.txt", "content")
	clean, _ = r.IsClean(context.Background())
	if clean {
		t.Error("repo with untracked file should not be clean")
	}
}

func TestAddCommitTag(t *testing.T) {
	dir := gitInit(t)
	writeFile(t, dir, "v", "1.0.0")
	r := New(dir)
	ctx := context.Background()

	if err := r.Add(ctx, "v"); err != nil {
		t.Fatal(err)
	}
	if err := r.Commit(ctx, "initial commit"); err != nil {
		t.Fatal(err)
	}
	clean, _ := r.IsClean(ctx)
	if !clean {
		t.Error("post-commit should be clean")
	}

	if err := r.Tag(ctx, "v1.0.0", "release"); err != nil {
		t.Fatal(err)
	}
	exists, err := r.TagExists(ctx, "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("tag should exist after creation")
	}
}

func TestCommitNothingToCommit(t *testing.T) {
	dir := gitInit(t)
	writeFile(t, dir, "x", "1")
	r := New(dir)
	ctx := context.Background()
	_ = r.Add(ctx, "x")
	_ = r.Commit(ctx, "first")

	// Second commit attempt with nothing staged.
	err := r.Commit(ctx, "no changes")
	if !errors.Is(err, ErrNothingToCommit) {
		t.Errorf("got %v, want ErrNothingToCommit", err)
	}
}

func TestTagExistsFalseWhenAbsent(t *testing.T) {
	dir := gitInit(t)
	r := New(dir)
	exists, err := r.TagExists(context.Background(), "v9.9.9")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("nonexistent tag should report false")
	}
}

// TestPushBranchFailsWithoutRemote — Push* require a configured remote;
// the helpers should surface a clean error rather than panicking.
func TestPushBranchFailsWithoutRemote(t *testing.T) {
	dir := gitInit(t)
	r := New(dir)
	err := r.PushBranch(context.Background(), "origin", "main")
	if err == nil {
		t.Fatal("push to nonexistent remote should error")
	}
	// Sanity-check the error includes context that helps users diagnose.
	if !strings.Contains(err.Error(), "git push") {
		t.Errorf("error should mention the failing command: %v", err)
	}
}
