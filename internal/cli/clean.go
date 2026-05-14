package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/LacalleGroup/stratt/internal/detect"
	"github.com/spf13/cobra"
)

// newCleanCmd implements `stratt clean` — multi-stack cleanup.  Walks
// detected stacks and removes the conventional artifact directories
// for each.  Per requirements.md §3 "clean" chain.
func newCleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Remove build / cache artifacts for the detected stacks",
		Long: `Remove conventional build and cache artifacts for every detected stack.

Always removes:
  .stratt/cache/

Per stack:
  go        → ./bin
  python+uv → dist/, .pytest_cache/, **/__pycache__

Does not touch Docker images by default (requires --docker explicitly).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			report := detect.Scan(cwd)

			targets := []string{filepath.Join(cwd, ".stratt", "cache")}
			for _, s := range report.Stacks {
				switch s.Name {
				case "go":
					targets = append(targets, filepath.Join(cwd, "bin"))
				case "python+uv":
					targets = append(targets,
						filepath.Join(cwd, "dist"),
						filepath.Join(cwd, ".pytest_cache"),
					)
					// Recursive __pycache__ removal happens below.
				}
			}
			for _, p := range targets {
				if err := os.RemoveAll(p); err != nil {
					return fmt.Errorf("rm -rf %s: %w", p, err)
				}
				fmt.Fprintf(out, "removed %s\n", relTo(cwd, p))
			}

			// __pycache__ recursive cleanup for python+uv.
			for _, s := range report.Stacks {
				if s.Name != "python+uv" {
					continue
				}
				if err := removePycache(cwd, out); err != nil {
					return err
				}
				break
			}
			return nil
		},
	}
}

// removePycache walks cwd and removes every __pycache__ directory it
// finds.  Errors on any individual directory are surfaced; partial
// success is acceptable (the next run picks up the rest).
func removePycache(root string, log interface{ Write([]byte) (int, error) }) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries; not fatal
		}
		if info.IsDir() && info.Name() == "__pycache__" {
			if rmErr := os.RemoveAll(path); rmErr == nil {
				fmt.Fprintf(log, "removed %s\n", relTo(root, path))
			}
			return filepath.SkipDir
		}
		return nil
	})
}

func relTo(base, path string) string {
	if r, err := filepath.Rel(base, path); err == nil {
		return r
	}
	return path
}
