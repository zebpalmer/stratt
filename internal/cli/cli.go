// Package cli wires the Cobra command tree.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zebpalmer/stratt/internal/config"
	"github.com/zebpalmer/stratt/internal/update"
	"github.com/zebpalmer/stratt/internal/version"
)

// BuildInfo carries version metadata injected at link time.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// Run executes the root command and returns the exit code.
//
// The root command has SilenceErrors: true so we own the error
// presentation here.  Per R5.5: 1 = user error, 2 = system error.
// Future error types (e.g., update-available advisories) may extend
// to 3+.
func Run(b BuildInfo) int {
	root := newRootCmd(b)
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		return 1
	}
	return 0
}

func newRootCmd(b BuildInfo) *cobra.Command {
	root := &cobra.Command{
		Use:   "stratt",
		Short: "The operations chief for your repo",
		Long: `stratt is a polyglot task runner that detects your project's stack
and provides a unified CLI for build, test, release, and deploy.

It replaces Makefiles with a single, statically-linked binary that handles
the universal targets (build/test/lint/release) plus Kustomize image bumps
for Kubernetes deploys.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		// PersistentPreRunE runs before every subcommand.  We use it to
		// load project config and enforce required_stratt (R2.3.12) so
		// that no command runs unsatisfied.  `version` and `doctor` are
		// exempt because users must be able to diagnose pin issues
		// without first satisfying them.
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return runRequiredVersionCheck(cmd, b)
		},
	}

	root.AddCommand(
		newVersionCmd(b),
		newDoctorCmd(b),
	)

	// Register the universal subcommands (build, test, lint, format,
	// setup, sync, lock, upgrade) per §0.  Custom-shape commands
	// (release, deploy, clean, docs, self) get added separately as
	// their implementations land.
	for _, spec := range universalSpecs {
		root.AddCommand(newUniversalCmd(spec))
	}
	root.AddCommand(newRunCmd())
	root.AddCommand(newReleaseCmd())
	root.AddCommand(newDeployCmd())
	root.AddCommand(newCleanCmd())
	root.AddCommand(newDocsCmd())
	root.AddCommand(newSelfCmd(b))
	root.AddCommand(newConfigCmd(b))

	return root
}

// runRequiredVersionCheck loads project config, enforces
// required_stratt (R2.3.12), and (in the background) opportunistically
// pings the update notifier (R4.12).  Returns nil if either no config
// exists or the constraint passes.  Skipped for `version` and `doctor`
// so users can introspect a repo whose pin they can't yet satisfy.
func runRequiredVersionCheck(cmd *cobra.Command, b BuildInfo) error {
	switch cmd.Name() {
	case "version", "doctor", "help":
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		// Best effort: if we can't even read cwd we have bigger problems
		// downstream; let the command itself surface the issue.
		return nil
	}

	proj, err := config.Load(cwd)
	if err != nil {
		// Config errors (e.g. ErrConflict) must surface — they're
		// non-skippable per R2.3.3.
		return err
	}

	if proj != nil {
		if err := version.Check(proj.RequiredStratt, b.Version); err != nil {
			return err
		}
	}

	// Deprecation scan (R2.3.9).  We render findings to stderr without
	// blocking the command.  AutoFix-eligible findings get a "run
	// stratt config migrate" hint; pure-info findings get a plain hint.
	if findings, _ := config.Scan(cwd); len(findings) > 0 {
		for _, f := range findings {
			fmt.Fprintf(os.Stderr, "[%s] %s: %s\n", f.Severity, f.ID, f.Hint)
			if f.AutoFix != nil {
				fmt.Fprintln(os.Stderr, "       run `stratt config migrate` to fix")
			}
		}
	}

	// Update notifier (R4.12).  Two-stage:
	//   (a) print the cached advisory now — synchronous, no IO race
	//   (b) refresh the cache in the background for the next invocation
	update.NotifyIfBehind(os.Stderr, b.Version)
	go update.RefreshNotifierState(cmd.Context(), update.Options{
		Repo:           strattUpstreamRepo,
		CurrentVersion: b.Version,
	})
	return nil
}
