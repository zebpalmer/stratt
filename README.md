# stratt

> A polyglot task runner that replaces Makefiles, manages release versions, and handles Kustomize image bumps.

Named for Eva Stratt, Project Director of the Petrova Taskforce in Andy Weir's *Project Hail Mary*.

**Pre-alpha.** Active development, expect breaking changes until v1.0.

Full docs at [stratt.sh](https://stratt.sh).

## Install

```sh
brew install zebpalmer/tap/stratt
```

Or grab a binary from the [releases page](https://github.com/zebpalmer/stratt/releases).

### macOS first-run note

Stratt binaries are not yet notarized. On first run from a direct download, Gatekeeper will quarantine the binary. Clear it with:

```sh
xattr -d com.apple.quarantine /path/to/stratt
```

(Or right-click → Open the first time, then close.) Homebrew installations are unaffected.

## License

Apache-2.0
