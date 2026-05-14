package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/LacalleGroup/stratt/internal/git"
	"github.com/LacalleGroup/stratt/internal/kustomize"
	"github.com/spf13/cobra"
)

// newDeployCmd implements `stratt deploy <env> <version>` (R2.5).
// Edits the kustomization.yaml under deploy/overlays/<env>/ to bump the
// image tag.  Does NOT commit or push by default (R2.5.5) — use --commit.
func newDeployCmd() *cobra.Command {
	var (
		imageName string
		commit    bool
		yes       bool
	)
	cmd := &cobra.Command{
		Use:   "deploy <env> <version>",
		Short: "Bump Kustomize image tags in deploy/overlays/<env>/",
		Long: `Update the image tag in deploy/overlays/<env>/kustomization.yaml
to <version>, preserving comments and formatting.

By default the file is modified but not committed.  Pass --commit to
stage and commit the change with a sensible message.

Examples:
  stratt deploy prod 1.14.1
  stratt deploy staging 1.15.0-rc1 --commit
  stratt deploy prod 1.14.1 --image=cartographerd  # disambiguate multi-image overlays`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, version := args[0], args[1]
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			overlay := kustomize.OverlayPath(cwd, env)
			if _, err := os.Stat(overlay); err != nil {
				return fmt.Errorf("no overlay at %s", overlay)
			}

			// Apply the image bump first (the change is reversible via
			// git if it turns out to be the wrong thing).
			change, err := kustomize.SetImage(overlay, imageName, version)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"updated %s\n  %s: %s → %s\n",
				overlay, change.Image, displayTag(change.OldTag), change.NewTag)

			if !commit {
				fmt.Fprintf(cmd.OutOrStdout(),
					"\nNot committed.  To commit:\n  git add %s\n  git commit -m \"deploy %s %s → %s\"\n",
					overlay, env, change.Image, change.NewTag)
				return nil
			}

			// Confirmation gate for commit (unless --yes).
			if !yes {
				if !confirmCommit(cmd.OutOrStdout(), cmd.InOrStdin()) {
					fmt.Fprintln(cmd.OutOrStdout(), "Skipping commit; file change remains in working tree.")
					return nil
				}
			}

			ctx := context.Background()
			repo := git.New(cwd)
			if err := repo.Add(ctx, overlay); err != nil {
				return err
			}
			msg := fmt.Sprintf("deploy %s %s %s", env, change.Image, change.NewTag)
			if err := repo.Commit(ctx, msg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Committed: %s\n", msg)
			return nil
		},
	}
	cmd.Flags().StringVar(&imageName, "image", "", "specific image name to update (required if the overlay has multiple images)")
	cmd.Flags().BoolVar(&commit, "commit", false, "stage and commit the change after editing")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the commit confirmation prompt")
	return cmd
}

func displayTag(t string) string {
	if t == "" {
		return "(unset)"
	}
	return t
}

func confirmCommit(out io.Writer, in io.Reader) bool {
	fmt.Fprint(out, "Commit this change? [Y/n] ")
	br := bufio.NewReader(in)
	line, err := br.ReadString('\n')
	if err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "", "y", "yes":
		return true
	}
	return false
}
