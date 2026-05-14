package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCmd(b BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print stratt version, commit, and build date",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "stratt %s (%s, %s)\n", b.Version, b.Commit, b.Date)
			return nil
		},
	}
}
