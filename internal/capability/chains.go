// Resolution chains for each universal command, per requirements.md §3.
//
// Each resolveXxx returns the first matching Engine, or nil if no chain
// entry matched.  Order matters: chains are documented as first-match-wins.
package capability

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/zebpalmer/stratt/internal/detect"
)

// resolveBuild — see requirements.md §3 "build" chain.
func (r *Resolver) resolveBuild() Engine {
	switch {
	case r.HasStack("python+uv"):
		return &execEngine{tool: "uv", argv: []string{"build"}}
	case r.HasStack("go") && r.fileExists(".goreleaser.yaml", ".goreleaser.yml"):
		return &execEngine{tool: "goreleaser", argv: []string{"build", "--snapshot", "--clean"}}
	case r.HasStack("go"):
		// Without goreleaser we emit a plain `go build`. Version/commit/date
		// ldflags are injected by the runner once the bump engine knows the
		// project version — for now this is a vanilla build.
		return &execEngine{tool: "go", argv: []string{"build", "./..."}, display: "go build ./..."}
	case r.HasStack("php"):
		return &execEngine{tool: "composer", argv: []string{"install"}}
	case r.HasStack("docker"):
		return &execEngine{tool: "docker", argv: []string{"build", "."}, display: "docker build ."}
	}
	return nil
}

// resolveTest — see requirements.md §3 "test" chain.
func (r *Resolver) resolveTest() Engine {
	switch {
	case r.HasStack("python+uv"):
		return &execEngine{tool: "uv", argv: []string{"run", "pytest"}}
	case r.HasStack("go"):
		return &execEngine{tool: "go", argv: []string{"test", "./..."}}
	case r.HasStack("php"):
		// composer scripts are project-specific; the safe default is
		// `composer test` which fails clearly if no script is defined.
		return &execEngine{tool: "composer", argv: []string{"test"}}
	}
	return nil
}

// resolveLint — see requirements.md §3 "lint" chain.
//
// Stratt is opinionated: `lint` runs the repo's configured linter in
// its fixing mode where one exists.  We call the tools the repo
// already opted into, with the configuration the repo already has.
// Repos that want check-only behavior can override the task in
// stratt.toml.
func (r *Resolver) resolveLint() Engine {
	switch {
	case r.HasStack("python+uv"):
		return &execEngine{tool: "uv", argv: []string{"run", "ruff", "check", "--fix"}}
	case r.HasStack("go") && available("golangci-lint"):
		// golangci-lint's --fix only fixes a subset of linters but is
		// safe to enable by default; linters that don't support fixing
		// are no-ops with --fix on.
		return &execEngine{tool: "golangci-lint", argv: []string{"run", "--fix"}}
	case r.HasStack("go"):
		return &execEngine{tool: "go", argv: []string{"vet", "./..."}}
	case r.HasStack("php"):
		return &execEngine{tool: "composer", argv: []string{"lint"}}
	}
	return nil
}

// resolveFormat — see requirements.md §3 "format" chain.
func (r *Resolver) resolveFormat() Engine {
	switch {
	case r.HasStack("python+uv"):
		return &execEngine{tool: "uv", argv: []string{"run", "ruff", "format"}}
	case r.HasStack("go"):
		return &execEngine{tool: "gofmt", argv: []string{"-w", "."}, display: "gofmt -w ."}
	}
	return nil
}

// resolveSetup — first-time project setup.
func (r *Resolver) resolveSetup() Engine {
	switch {
	case r.HasStack("python+uv"):
		return &execEngine{tool: "uv", argv: []string{"sync", "--all-extras"}}
	case r.HasStack("go"):
		return &execEngine{tool: "go", argv: []string{"mod", "download"}}
	case r.HasStack("php"):
		return &execEngine{tool: "composer", argv: []string{"install"}}
	}
	return nil
}

// resolveSync — sync deps from lockfile.
func (r *Resolver) resolveSync() Engine {
	switch {
	case r.HasStack("python+uv"):
		return &execEngine{tool: "uv", argv: []string{"sync"}}
	case r.HasStack("go"):
		return &execEngine{tool: "go", argv: []string{"mod", "download"}}
	case r.HasStack("php"):
		return &execEngine{tool: "composer", argv: []string{"install", "--no-dev"}}
	}
	return nil
}

// resolveLock — update lockfile from manifest.
func (r *Resolver) resolveLock() Engine {
	switch {
	case r.HasStack("python+uv"):
		return &execEngine{tool: "uv", argv: []string{"lock"}}
	case r.HasStack("go"):
		return &execEngine{tool: "go", argv: []string{"mod", "tidy"}}
	case r.HasStack("php"):
		return &execEngine{tool: "composer", argv: []string{"update", "--lock"}}
	}
	return nil
}

// resolveUpgrade — upgrade all dependencies.  The Go path is a
// composite shell line because it's a two-step idiom.
func (r *Resolver) resolveUpgrade() Engine {
	switch {
	case r.HasStack("python+uv"):
		return &execEngine{tool: "uv", argv: []string{"lock", "--upgrade"}}
	case r.HasStack("go"):
		return &shellEngine{line: "go get -u ./... && go mod tidy", display: "go get -u ./... && go mod tidy"}
	case r.HasStack("php"):
		return &execEngine{tool: "composer", argv: []string{"update"}}
	}
	return nil
}

// resolveClean — multi-stack cleanup is implemented as its own
// subcommand (`stratt clean`) since it has different fan-out semantics
// from the other universal commands.  This entry is delegateEngine for
// doctor display.
func (r *Resolver) resolveClean() Engine {
	return &delegateEngine{
		display:     "remove build/cache artifacts per detected stacks",
		delegateCmd: "stratt clean",
	}
}

// resolveRelease — see requirements.md §3 "release" chain.
//
//  1. Bump-my-version config present anywhere → native bump engine
//  2. .goreleaser.yaml present (and no bump config) → tag-only mode
//  3. Otherwise → tag-only mode
//
// `stratt release` is a custom-shape subcommand, so these engines
// are display-only (delegateEngine).  Their Status reflects that the
// subcommand is available.
func (r *Resolver) resolveRelease() Engine {
	switch {
	case r.hasBumpConfig():
		return &delegateEngine{
			display:     "native bump engine (reads [tool.bumpversion])",
			delegateCmd: "stratt release",
		}
	case r.fileExists(".goreleaser.yaml", ".goreleaser.yml"):
		return &delegateEngine{
			display:     "tag-only release (CI runs goreleaser on tag-push)",
			delegateCmd: "stratt release",
		}
	case r.HasStack("go") || r.HasStack("python+uv") || r.HasStack("php"):
		return &delegateEngine{
			display:     "tag-only release",
			delegateCmd: "stratt release",
		}
	}
	return nil
}

// resolveDeploy — Kustomize image bump is the only deploy engine in v1.
// `stratt deploy` is a custom-shape subcommand (it takes positional
// args), so this is a delegateEngine for doctor display.
func (r *Resolver) resolveDeploy() Engine {
	if r.HasStack("kustomize") {
		return &delegateEngine{
			display:     "kustomize image bump (deploy/overlays/<env>/kustomization.yaml)",
			delegateCmd: "stratt deploy",
		}
	}
	return nil
}

// resolveDocs — first matching documentation toolchain.
func (r *Resolver) resolveDocs() Engine {
	switch {
	case r.HasStack("mkdocs"):
		return &execEngine{tool: "mkdocs", argv: []string{"build"}}
	case r.HasStack("sphinx"):
		return &execEngine{tool: "sphinx-build", argv: []string{"docs", "_build/html"}}
	case r.HasStack("hugo"):
		src := detect.FindHugoSource(r.root)
		argv := []string{"--minify"}
		if src != "" && src != "." {
			argv = append([]string{"--source", src}, argv...)
		}
		return &execEngine{tool: "hugo", argv: argv}
	}
	return nil
}

// resolveStyle — composite of format + lint.  Only resolves when both
// constituents have engines (i.e., the project has formatters and
// linters available).
func (r *Resolver) resolveStyle() Engine {
	if r.resolveFormat() == nil || r.resolveLint() == nil {
		return nil
	}
	return &compositeEngine{
		display: "format + lint",
		members: []string{"format", "lint"},
	}
}

// resolveAll — composite of every detected verification step that's
// applicable.  Per project policy this is "everything detected" by
// default; users override via [tasks.all] in stratt.toml when they
// want a narrower set.
//
// Membership: format, lint, test, docs (in that order, each included
// only if its constituent engine resolves).
func (r *Resolver) resolveAll() Engine {
	var members []string
	if r.resolveFormat() != nil {
		members = append(members, "format")
	}
	if r.resolveLint() != nil {
		members = append(members, "lint")
	}
	if r.resolveTest() != nil {
		members = append(members, "test")
	}
	if r.resolveDocs() != nil {
		members = append(members, "docs")
	}
	if len(members) == 0 {
		return nil
	}
	display := ""
	for i, m := range members {
		if i > 0 {
			display += " + "
		}
		display += m
	}
	return &compositeEngine{display: display, members: members}
}

// fileExists returns true if any of the given files exist in the repo root.
func (r *Resolver) fileExists(names ...string) bool {
	for _, n := range names {
		if _, err := os.Stat(filepath.Join(r.root, n)); err == nil {
			return true
		}
	}
	return false
}

// hasBumpConfig reports whether any recognized bump-my-version-style
// configuration exists in the repo.  See R2.4.7 for the full chain.
func (r *Resolver) hasBumpConfig() bool {
	if r.fileExists(".bumpversion.toml", ".bumpversion.cfg") {
		return true
	}
	// `[tool.bumpversion]` or `[tool.stratt.bump]` in pyproject.toml, or
	// `[bump]` in stratt.toml.  Done with a coarse byte scan for now;
	// the config loader will do this properly once it lands.
	for _, file := range []string{"pyproject.toml", "stratt.toml"} {
		path := filepath.Join(r.root, file)
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		body := string(b)
		switch file {
		case "pyproject.toml":
			if containsSection(body, "tool.bumpversion") || containsSection(body, "tool.stratt.bump") {
				return true
			}
		case "stratt.toml":
			if containsSection(body, "bump") {
				return true
			}
		}
	}
	return false
}

// containsSection reports whether body contains a TOML section header
// matching name.  Tolerant of whitespace around the brackets — sufficient
// as a heuristic before the real config loader lands and replaces this.
func containsSection(body, name string) bool {
	header := "[" + name + "]"
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == header {
			return true
		}
	}
	return false
}
