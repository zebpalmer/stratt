package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/LacalleGroup/stratt/internal/bump"
	"github.com/LacalleGroup/stratt/internal/release"
	"github.com/spf13/cobra"
)

// newReleaseCmd wires the `stratt release` flow.  This is a custom-shape
// command rather than a generic universal subcommand because it accepts
// positional args (`stratt release patch`) and many flags governing the
// interactive prompts and push behavior.
func newReleaseCmd() *cobra.Command {
	var (
		typeFlag      string
		ciFlag        bool
		yesFlag       bool
		branchFlag    string
		remoteFlag    string
		noPushFlag    bool
	)
	cmd := &cobra.Command{
		Use:   "release [patch|minor|major]",
		Short: "Bump version, commit, tag, and (optionally) push",
		Long: `Run the release flow: bump version per [bump]/[tool.bumpversion] config,
commit, tag, and push.  stratt does NOT build release artifacts —
GitHub Actions takes over after the tag is pushed.

Examples:
  stratt release                # interactive: prompt for patch|minor|major
  stratt release patch          # non-interactive shortcut
  stratt release --type=minor   # equivalent
  stratt release patch --ci     # CI mode: no prompts, fail on missing decisions
  stratt release patch --no-push  # local only; print the push command

See requirements R2.4 for the full design.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			// Reconcile positional arg vs --type flag.  They're equivalent.
			kindStr := strings.TrimSpace(typeFlag)
			if len(args) == 1 {
				if kindStr != "" && args[0] != kindStr {
					return fmt.Errorf("conflicting release types: positional %q vs --type=%q", args[0], kindStr)
				}
				kindStr = args[0]
			}

			opts := release.Options{
				CWD:    cwd,
				CI:     ciFlag,
				AssumeYes: yesFlag,
				Branch: branchFlag,
				Remote: remoteFlag,
				Push:   !noPushFlag,
				Stdin:  cmd.InOrStdin(),
				Stdout: cmd.OutOrStdout(),
				Stderr: cmd.ErrOrStderr(),
			}

			if kindStr != "" {
				k, err := bump.KindFromString(kindStr)
				if err != nil {
					return err
				}
				opts.Kind = k
				opts.HasKind = true
			}

			if err := release.Run(cmd.Context(), opts); err != nil {
				// Surface bump-config errors with extra context about the
				// chain so users know what to add.
				if errors.Is(err, bump.ErrMissingVersion) {
					return fmt.Errorf("%w (check your [tool.bumpversion] files block matches the file's current content)", err)
				}
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&typeFlag, "type", "", "release type: patch | minor | major (alternative to positional arg)")
	cmd.Flags().BoolVar(&ciFlag, "ci", false, "non-interactive mode: no prompts, fail loudly on missing decisions")
	cmd.Flags().BoolVarP(&yesFlag, "yes", "y", false, "skip final confirmation (major-bump gate still requires explicit input)")
	cmd.Flags().StringVar(&branchFlag, "branch", "main", "release branch")
	cmd.Flags().StringVar(&remoteFlag, "remote", "origin", "git remote for push")
	cmd.Flags().BoolVar(&noPushFlag, "no-push", false, "do not push commit/tag to remote (default is to push)")
	return cmd
}
