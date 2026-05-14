package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zebpalmer/stratt/internal/capability"
)

func newDoctorCmd(b BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Report detected stacks, resolved command backends, and binary metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			fmt.Fprintln(out, "stratt doctor")
			fmt.Fprintln(out, "─────────────")
			fmt.Fprintf(out, "version : %s\n", b.Version)
			fmt.Fprintf(out, "commit  : %s\n", b.Commit)
			fmt.Fprintf(out, "built   : %s\n", b.Date)
			fmt.Fprintln(out)

			fmt.Fprintf(out, "Scanning %s\n", cwd)
			resolver := capability.New(cwd)
			stacks := resolver.Stacks()
			if len(stacks) == 0 {
				fmt.Fprintln(out, "  no recognized stacks found")
				return nil
			}
			for _, s := range stacks {
				fmt.Fprintf(out, "  ✓ %s (via %s)\n", s.Name, s.Signal)
			}

			fmt.Fprintln(out)
			fmt.Fprintln(out, "Resolved commands:")
			tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			for _, res := range resolver.ResolveAll() {
				if res.Engine == nil {
					fmt.Fprintf(tw, "  %s\t→ —\t(no engine matched)\n", res.Command)
					continue
				}
				marker := ""
				switch res.Engine.Status() {
				case capability.StatusMissingTool:
					marker = "[tool not on PATH]"
				case capability.StatusPending:
					marker = "[not yet implemented]"
				}
				fmt.Fprintf(tw, "  %s\t→ %s\t%s\n", res.Command, res.Engine.Name(), marker)
			}
			tw.Flush()

			return nil
		},
	}
}
