package cli

import (
	"errors"
	"fmt"

	"github.com/LacalleGroup/stratt/internal/update"
	"github.com/spf13/cobra"
)

// newSelfCmd wires the `stratt self` subcommand group:
// `stratt self update`, `rollback`, `verify`, `check`.  Per R4.
func newSelfCmd(b BuildInfo) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "self",
		Short: "Manage the stratt binary itself (update, rollback, verify)",
	}
	cmd.AddCommand(newSelfUpdateCmd(b))
	cmd.AddCommand(newSelfRollbackCmd(b))
	cmd.AddCommand(newSelfVerifyCmd(b))
	cmd.AddCommand(newSelfCheckCmd(b))
	return cmd
}

// strattUpstreamRepo is compiled into the binary as the trust pin
// (R4.4).  Forks should override this in cmd/stratt/main.go.
const strattUpstreamRepo = "LacalleGroup/stratt"

func newSelfUpdateCmd(b BuildInfo) *cobra.Command {
	var (
		channel    string
		noVerify   bool
	)
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Download, verify, and install the latest stratt",
		Long: `Self-update the stratt binary.

The update is gated on:
  - Not running in CI ($CI / $GITHUB_ACTIONS)
  - Not installed via Homebrew (use ` + "`brew upgrade stratt`" + ` instead)
  - The latest release being strictly newer than the running version
  - Attestation verification succeeding (R4.3)`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := update.Apply(cmd.Context(), update.Options{
				Repo:             strattUpstreamRepo,
				Channel:          channel,
				CurrentVersion:   b.Version,
				Stdout:           cmd.OutOrStdout(),
				Stderr:           cmd.ErrOrStderr(),
				SkipVerification: noVerify,
			})
			if errors.Is(err, update.ErrHomebrewManaged) {
				return errors.New("this binary is managed by Homebrew; run `brew upgrade stratt` instead")
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"\nUpdated stratt %s → %s\nPrevious binary preserved at %s\n",
				res.PreviousVersion, res.NewVersion, res.BackupPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "stable", "release channel: stable | prerelease")
	cmd.Flags().BoolVar(&noVerify, "no-verify", false, "skip attestation verification (UNSAFE; for emergency recovery only)")
	return cmd
}

func newSelfRollbackCmd(b BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "rollback",
		Short: "Revert to the previously installed stratt binary",
		Long: `Restore the binary from the cache populated by the last successful
` + "`stratt self update`" + `.  Returns an error if no prior version is recorded.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			restored, err := update.Rollback(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Rolled back to %s\n", restored)
			return nil
		},
	}
}

func newSelfVerifyCmd(b BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "verify",
		Short: "Re-verify the running stratt binary against its release attestation",
		Long:  `Useful for verifying integrity after install or any time without performing an update.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			err := update.VerifyCurrent(cmd.Context(), update.Options{
				Repo:           strattUpstreamRepo,
				CurrentVersion: b.Version,
				Stderr:         cmd.ErrOrStderr(),
			})
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Attestation OK.")
			return nil
		},
	}
}

func newSelfCheckCmd(b BuildInfo) *cobra.Command {
	var channel string
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check whether a newer stratt release is available (does not install)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			latest, newer, err := update.CheckOnly(cmd.Context(), update.Options{
				Repo:           strattUpstreamRepo,
				Channel:        channel,
				CurrentVersion: b.Version,
			})
			if err != nil {
				return err
			}
			if !newer {
				fmt.Fprintf(cmd.OutOrStdout(), "Up to date (%s).\n", b.Version)
				return nil
			}
			kind, _ := update.DetectInstall()
			switch kind {
			case update.InstallHomebrew:
				fmt.Fprintf(cmd.OutOrStdout(), "Update available: %s — run `brew upgrade stratt`\n", latest.TagName)
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "Update available: %s — run `stratt self update`\n", latest.TagName)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "stable", "release channel: stable | prerelease")
	return cmd
}
