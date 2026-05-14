// Package release wires the bump engine, git operations, and pre-flight
// gates into the user-facing `stratt release` flow (R2.4).
//
// The flow is:
//
//   1. Pre-flight gates (R2.4.1): on the configured branch, working tree
//      clean, lockfile in sync, optionally tests/lint pass.
//   2. Determine bump Kind: either supplied via Options.Kind (non-
//      interactive) or prompted (interactive).
//   3. Confirmation gate for Major releases.
//   4. Compute plan (dry-run; show file-by-file diff).
//   5. Final confirmation (skip with AssumeYes or in --ci mode).
//   6. Apply: write files, stage, commit, tag.
//   7. Push (default ON per R2.4.5; configurable off).
package release

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/LacalleGroup/stratt/internal/bump"
	"github.com/LacalleGroup/stratt/internal/git"
)

// Options drives a single release.  Zero-value Options is *not* safe —
// at minimum the IO streams must be populated.  CWD defaults to the
// process working directory.
type Options struct {
	// CWD is the repo root.  Required.
	CWD string

	// Kind selects the bump granularity.  When zero-value (Patch with
	// the zero check fall-through), the runner falls back to interactive
	// prompting in non-CI mode.  Use HasKind to distinguish "explicitly
	// patch" from "unset".
	Kind    bump.Kind
	HasKind bool

	// Branch is the release branch.  Default "main".  Pre-flight aborts
	// if HEAD is on a different branch.
	Branch string

	// Push controls whether to push commit + tag to origin after the
	// local bump.  Default true per R2.4.5.
	Push bool

	// Remote is the git remote to push to.  Default "origin".
	Remote string

	// CI disables interactive prompts.  Combined with HasKind=true this
	// produces a fully non-interactive release.
	CI bool

	// AssumeYes skips final confirmation prompts (but not the major-bump
	// confirmation gate, which requires explicit input per R2.4.2.4).
	AssumeYes bool

	// Stdin / Stdout / Stderr — required.  Stdin must be a terminal-like
	// reader for the interactive prompts to work.
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Run executes one release per Options.  Returns nil on success, or a
// rich error explaining which gate failed.
func Run(ctx context.Context, opts Options) error {
	if opts.CWD == "" {
		return errors.New("CWD must be set")
	}
	if opts.Branch == "" {
		opts.Branch = "main"
	}
	if opts.Remote == "" {
		opts.Remote = "origin"
	}

	// Wrap stdin once so prompts share a buffer.  Multiple bufio.Readers
	// on the same underlying io.Reader silently lose data.
	stdin := bufio.NewReader(opts.Stdin)

	// Pre-flight gates (R2.4.1).
	repo := git.New(opts.CWD)
	if err := preflight(ctx, repo, opts); err != nil {
		return err
	}

	// Load bump configuration.
	cfg, warn, err := bump.Load(opts.CWD)
	if err != nil {
		return fmt.Errorf("loading bump config: %w", err)
	}
	if warn != "" {
		fmt.Fprintf(opts.Stderr, "warning: %s\n", warn)
	}
	if cfg == nil {
		return errors.New(
			"no version-bump configuration found; add [bump] to stratt.toml " +
				"or [tool.bumpversion] to pyproject.toml " +
				"(see R2.4.7 for the supported locations)")
	}

	// Determine kind: explicit > prompt.
	kind, err := resolveKind(opts, stdin)
	if err != nil {
		return err
	}

	// Confirmation gate for Major.
	if kind == bump.Major {
		if err := confirmMajor(opts, stdin); err != nil {
			return err
		}
	}

	// Compute and display plan.
	plan, err := bump.Compute(cfg, kind, opts.CWD)
	if err != nil {
		return fmt.Errorf("computing bump plan: %w", err)
	}
	if err := printPlan(opts.Stdout, plan); err != nil {
		return err
	}

	// Refuse to proceed if any file change is "not found".
	for _, c := range plan.FileChanges {
		if !c.Found {
			return fmt.Errorf("%w: %s (file does not contain the search string %q)",
				bump.ErrMissingVersion, c.Path, c.OldChunk)
		}
	}

	// Final confirmation (skipped with --yes or in CI).
	if !opts.AssumeYes && !opts.CI {
		if !confirm(opts, stdin, fmt.Sprintf("\nProceed with bump %s → %s?", plan.OldVersion, plan.NewVersion), true) {
			return errors.New("aborted by user")
		}
	}

	// Apply: write files.
	if err := bump.Apply(plan); err != nil {
		return fmt.Errorf("applying bump: %w", err)
	}

	// Stage and commit.
	if cfg.Commit {
		paths := make([]string, 0, len(plan.FileChanges))
		for _, c := range plan.FileChanges {
			paths = append(paths, c.Path)
		}
		if err := repo.Add(ctx, paths...); err != nil {
			return err
		}
		if err := repo.Commit(ctx, plan.CommitMessage); err != nil {
			return err
		}
	}

	// Tag (R2.4.5 controls push, not tag; tag follows the bump config).
	if cfg.Tag {
		if err := repo.Tag(ctx, plan.TagName, plan.CommitMessage); err != nil {
			return err
		}
	}

	// Push commit + tag.
	if opts.Push {
		if err := repo.PushBranch(ctx, opts.Remote, opts.Branch); err != nil {
			return fmt.Errorf("push branch: %w", err)
		}
		if cfg.Tag {
			if err := repo.PushTag(ctx, opts.Remote, plan.TagName); err != nil {
				return fmt.Errorf("push tag: %w", err)
			}
		}
	} else {
		fmt.Fprintf(opts.Stdout, "\nLocal release complete.  Push with:\n  git push %s %s\n",
			opts.Remote, opts.Branch)
		if cfg.Tag {
			fmt.Fprintf(opts.Stdout, "  git push %s %s\n", opts.Remote, plan.TagName)
		}
	}

	return nil
}

// preflight runs R2.4.1's branch/clean checks.  Other gates (tests, lint,
// lockfile sync) will be added as their integrations mature; this is the
// minimum viable set that protects users from the worst footguns.
func preflight(ctx context.Context, repo *git.Repo, opts Options) error {
	branch, err := repo.Branch(ctx)
	if err != nil {
		return fmt.Errorf("preflight: %w", err)
	}
	if branch != opts.Branch {
		return fmt.Errorf("preflight: on branch %q, expected %q (use a different `branch` in [release] config if intentional)",
			branch, opts.Branch)
	}
	clean, err := repo.IsClean(ctx)
	if err != nil {
		return fmt.Errorf("preflight: %w", err)
	}
	if !clean {
		return errors.New("preflight: working tree is not clean (commit or stash changes before releasing)")
	}
	return nil
}

// resolveKind picks the bump granularity.  Explicit (Options.HasKind)
// wins; otherwise prompt the user, or fail in CI mode.
func resolveKind(opts Options, stdin *bufio.Reader) (bump.Kind, error) {
	if opts.HasKind {
		return opts.Kind, nil
	}
	if opts.CI {
		return 0, errors.New("--ci requires --type=patch|minor|major (no interactive prompts)")
	}
	fmt.Fprintln(opts.Stdout, "Release type: [p]atch  [m]inor  [M]ajor")
	fmt.Fprint(opts.Stdout, "Choose: ")
	line, err := stdin.ReadString('\n')
	if err != nil {
		return 0, err
	}
	trimmed := strings.TrimSpace(line)
	// Single-char forms: case-sensitive to disambiguate 'm'/'M'.
	switch trimmed {
	case "p":
		return bump.Patch, nil
	case "m":
		return bump.Minor, nil
	case "M":
		return bump.Major, nil
	}
	// Long forms: case-insensitive.
	switch strings.ToLower(trimmed) {
	case "patch":
		return bump.Patch, nil
	case "minor":
		return bump.Minor, nil
	case "major":
		return bump.Major, nil
	}
	return 0, fmt.Errorf("invalid choice %q", trimmed)
}

// confirmMajor enforces the explicit confirmation gate for Major bumps
// per R2.4.2.4.  In CI mode, the gate is satisfied implicitly by the
// user having typed --type=major.
func confirmMajor(opts Options, stdin *bufio.Reader) error {
	if opts.CI {
		return nil
	}
	if !confirm(opts, stdin, "MAJOR release.  This is a breaking-change bump.  Are you sure?", false) {
		return errors.New("major release aborted")
	}
	return nil
}

// confirm prompts the user with a yes/no question.  defaultYes selects
// the default when the user presses enter alone.
func confirm(opts Options, stdin *bufio.Reader, prompt string, defaultYes bool) bool {
	choices := " [Y/n] "
	if !defaultYes {
		choices = " [y/N] "
	}
	fmt.Fprint(opts.Stdout, prompt+choices)
	line, err := stdin.ReadString('\n')
	if err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "":
		return defaultYes
	case "y", "yes":
		return true
	default:
		return false
	}
}

// printPlan renders a dry-run preview to w.
func printPlan(w io.Writer, p *bump.Plan) error {
	fmt.Fprintf(w, "\nBump plan (%s → %s):\n", p.OldVersion, p.NewVersion)
	for _, c := range p.FileChanges {
		fmt.Fprintln(w, c.PreviewLine())
	}
	if p.Cfg.Commit {
		fmt.Fprintf(w, "Commit message: %q\n", p.CommitMessage)
	} else {
		fmt.Fprintln(w, "Commit: disabled in config")
	}
	if p.Cfg.Tag {
		fmt.Fprintf(w, "Tag:            %s\n", p.TagName)
	} else {
		fmt.Fprintln(w, "Tag: disabled in config")
	}
	return nil
}

