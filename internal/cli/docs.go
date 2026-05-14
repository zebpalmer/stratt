package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/LacalleGroup/stratt/internal/detect"
	"github.com/spf13/cobra"
)

// newDocsCmd implements `stratt docs build` and `stratt docs serve`,
// dispatching to mkdocs or sphinx based on detection (R3 "docs" chain).
func newDocsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Build or serve project documentation",
		Long:  `Subcommands dispatch to the detected docs toolchain (MkDocs or Sphinx).`,
	}
	cmd.AddCommand(newDocsActionCmd("build"))
	cmd.AddCommand(newDocsActionCmd("serve"))
	return cmd
}

func newDocsActionCmd(action string) *cobra.Command {
	return &cobra.Command{
		Use:   action,
		Short: fmt.Sprintf("%s docs using the detected toolchain", action),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			tool, argv, err := docsCommand(cwd, action)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "→ %s %v\n", tool, argv)
			c := exec.CommandContext(cmd.Context(), tool, argv...)
			c.Stdin = os.Stdin
			c.Stdout = cmd.OutOrStdout()
			c.Stderr = cmd.ErrOrStderr()
			c.Dir = cwd
			return c.Run()
		},
	}
}

// docsCommand returns (tool, args) for the given action against whichever
// docs toolchain is detected.  Returns an error when no docs stack is
// detected.
func docsCommand(root, action string) (string, []string, error) {
	report := detect.Scan(root)
	for _, s := range report.Stacks {
		switch s.Name {
		case "mkdocs":
			switch action {
			case "build":
				return "mkdocs", []string{"build"}, nil
			case "serve":
				return "mkdocs", []string{"serve"}, nil
			}
		case "sphinx":
			switch action {
			case "build":
				return "sphinx-build", []string{"docs", "_build/html"}, nil
			case "serve":
				return "sphinx-autobuild", []string{"docs", "_build/html"}, nil
			}
		}
	}
	return "", nil, errors.New("no docs toolchain detected (looked for mkdocs.yml or docs/conf.py)")
}
