// Package detect identifies the project stacks present in a repository.
//
// Each Detector reports a single Stack (e.g., "go", "python+uv", "kustomize")
// based on the presence of well-known signal files.  A single repository can
// have multiple stacks; mixed Python + Docker + Kustomize is normal.
//
// See requirements R2.1 for the full detection signal table.
package detect

import (
	"os"
	"path/filepath"
	"sort"
)

// Stack is one detected project stack.
type Stack struct {
	Name   string // human-readable name, e.g., "python+uv"
	Signal string // the file or pattern that triggered detection
}

// Report is the result of scanning a directory.
type Report struct {
	Root   string
	Stacks []Stack
}

// detector is the predicate-shape used internally.  Each returns a non-empty
// Stack when its signal is present in root, or the zero value otherwise.
type detector func(root string) Stack

// detectors is the ordered list of stack detectors.  Order is reported order
// only — it has no effect on which detectors run.
var detectors = []detector{
	detectGo,
	detectPythonUV,
	detectPHP,
	detectDocker,
	detectKustomize,
	detectMkDocs,
	detectSphinx,
	detectHugo,
}

// Scan runs all detectors against root and returns the report.
func Scan(root string) Report {
	r := Report{Root: root}
	for _, d := range detectors {
		if s := d(root); s.Name != "" {
			r.Stacks = append(r.Stacks, s)
		}
	}
	sort.Slice(r.Stacks, func(i, j int) bool {
		return r.Stacks[i].Name < r.Stacks[j].Name
	})
	return r
}

func detectGo(root string) Stack {
	if exists(filepath.Join(root, "go.mod")) {
		return Stack{Name: "go", Signal: "go.mod"}
	}
	return Stack{}
}

func detectPythonUV(root string) Stack {
	if exists(filepath.Join(root, "pyproject.toml")) && exists(filepath.Join(root, "uv.lock")) {
		return Stack{Name: "python+uv", Signal: "pyproject.toml + uv.lock"}
	}
	return Stack{}
}

func detectPHP(root string) Stack {
	if exists(filepath.Join(root, "composer.json")) {
		return Stack{Name: "php", Signal: "composer.json"}
	}
	return Stack{}
}

func detectDocker(root string) Stack {
	if exists(filepath.Join(root, "Dockerfile")) {
		return Stack{Name: "docker", Signal: "Dockerfile"}
	}
	return Stack{}
}

func detectKustomize(root string) Stack {
	overlays := filepath.Join(root, "deploy", "overlays")
	entries, err := os.ReadDir(overlays)
	if err != nil {
		return Stack{}
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if exists(filepath.Join(overlays, e.Name(), "kustomization.yaml")) {
			return Stack{Name: "kustomize", Signal: "deploy/overlays/*/kustomization.yaml"}
		}
	}
	return Stack{}
}

func detectMkDocs(root string) Stack {
	if exists(filepath.Join(root, "mkdocs.yml")) {
		return Stack{Name: "mkdocs", Signal: "mkdocs.yml"}
	}
	return Stack{}
}

func detectSphinx(root string) Stack {
	if exists(filepath.Join(root, "docs", "conf.py")) {
		return Stack{Name: "sphinx", Signal: "docs/conf.py"}
	}
	return Stack{}
}

// detectHugo checks for a Hugo site config file either at the repo
// root (typical for dedicated docs repos) or inside a `docs/`
// subdirectory (typical for projects that ship code AND docs in the
// same repo — stratt's own layout).  Detection succeeds for any of
// hugo.{toml,yaml,yml,json}; FindHugoSource returns which directory
// to point Hugo at.
func detectHugo(root string) Stack {
	if src, name := findHugoConfigIn(root); src != "" {
		signal := name
		if src != "." {
			signal = filepath.Join(src, name)
		}
		return Stack{Name: "hugo", Signal: signal}
	}
	return Stack{}
}

// FindHugoSource returns the Hugo project directory (relative to root)
// or "" if no Hugo config is present.  Used by the docs subcommand to
// invoke `hugo --source <dir>`.
func FindHugoSource(root string) string {
	src, _ := findHugoConfigIn(root)
	return src
}

// findHugoConfigIn returns (directory, basename) of the first Hugo
// config file found, searching the repo root then docs/.  Empty
// directory string means "not found".
func findHugoConfigIn(root string) (dir, name string) {
	for _, sub := range []string{".", "docs"} {
		for _, n := range []string{"hugo.toml", "hugo.yaml", "hugo.yml", "hugo.json"} {
			if exists(filepath.Join(root, sub, n)) {
				return sub, n
			}
		}
	}
	return "", ""
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
