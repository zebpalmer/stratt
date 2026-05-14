// Package git wraps the small subset of git porcelain stratt invokes
// during release flows.  Shelling out to the `git` binary keeps this
// package free of git-library dependencies and makes its behavior
// trivially auditable against the user's local git config (signing,
// hooks, etc.).
//
// All functions accept a context so callers can cancel hung operations
// (e.g. a remote push that's blocked on auth).
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Repo is bound to a working directory; all commands run inside it.
type Repo struct {
	Dir    string
	Stdout io.Writer // optional; defaults to discard
	Stderr io.Writer // optional; defaults to discard
}

// New returns a Repo for dir.
func New(dir string) *Repo {
	return &Repo{Dir: dir}
}

// Branch returns the current branch name.  Works even on a repo with
// no commits yet (where HEAD doesn't yet resolve to a sha) by using
// symbolic-ref instead of rev-parse.
func (r *Repo) Branch(ctx context.Context) (string, error) {
	out, err := r.captureOutput(ctx, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// IsClean reports whether the working tree has no uncommitted changes.
func (r *Repo) IsClean(ctx context.Context) (bool, error) {
	out, err := r.captureOutput(ctx, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "", nil
}

// Add stages the given paths (relative to r.Dir).
func (r *Repo) Add(ctx context.Context, paths ...string) error {
	args := append([]string{"add", "--"}, paths...)
	return r.run(ctx, args...)
}

// Commit creates a commit with the given message.  Returns
// ErrNothingToCommit when the index is empty.  We check
// `git diff --cached --quiet` first because the "nothing to commit"
// message goes to stdout (not stderr), making it awkward to detect
// from a failed `git commit` invocation alone.
func (r *Repo) Commit(ctx context.Context, message string) error {
	staged, err := r.hasStagedChanges(ctx)
	if err != nil {
		return err
	}
	if !staged {
		return ErrNothingToCommit
	}
	return r.run(ctx, "commit", "-m", message)
}

// hasStagedChanges reports whether the index differs from HEAD.  Uses
// `git diff --cached --quiet`, which exits 0 when there are no staged
// changes, 1 when there are.
func (r *Repo) hasStagedChanges(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--cached", "--quiet")
	cmd.Dir = r.Dir
	err := cmd.Run()
	if err == nil {
		return false, nil
	}
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
		return true, nil
	}
	return false, fmt.Errorf("git diff --cached --quiet: %w", err)
}

// Tag creates an annotated tag pointing at HEAD.
func (r *Repo) Tag(ctx context.Context, name, message string) error {
	return r.run(ctx, "tag", "-a", name, "-m", message)
}

// PushBranch pushes the named branch to remote (typically "origin").
func (r *Repo) PushBranch(ctx context.Context, remote, branch string) error {
	return r.run(ctx, "push", remote, branch)
}

// PushTag pushes the named tag to remote.
func (r *Repo) PushTag(ctx context.Context, remote, tag string) error {
	return r.run(ctx, "push", remote, tag)
}

// TagExists reports whether the named tag exists locally.
func (r *Repo) TagExists(ctx context.Context, name string) (bool, error) {
	out, err := r.captureOutput(ctx, "tag", "-l", name)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == name, nil
}

// ErrNothingToCommit is returned by Commit when there's nothing staged.
var ErrNothingToCommit = errors.New("nothing to commit")

// captureOutput runs git with args and returns stdout.
func (r *Repo) captureOutput(ctx context.Context, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.Dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w (stderr: %s)",
			strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// run runs git with args, streaming stdout/stderr to the Repo's
// configured writers (or discarding if unset).
func (r *Repo) run(ctx context.Context, args ...string) error {
	stdout := r.Stdout
	stderr := r.Stderr
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	// Capture stderr separately so we can surface useful errors even
	// when stderr is being discarded (e.g., in tests).
	var stderrBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.Dir
	cmd.Stdout = stdout
	cmd.Stderr = io.MultiWriter(stderr, &stderrBuf)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w (stderr: %s)",
			strings.Join(args, " "), err, strings.TrimSpace(stderrBuf.String()))
	}
	return nil
}
