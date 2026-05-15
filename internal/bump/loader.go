package bump

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// Load returns the bump Config for root, walking the supported config
// locations in priority order:
//
//  1. [bump] in stratt.toml
//  2. [tool.stratt.bump] in pyproject.toml
//  3. [tool.bumpversion] in pyproject.toml
//  4. .bumpversion.toml
//  5. .bumpversion.cfg  (INI; emits a deprecation warning)
//
// Returns (nil, "", nil) for repos with no bump config — the resolver
// then falls through to tag-only release mode.
func Load(root string) (*Config, string, error) {
	// 1 & 2: native locations
	if cfg, src, err := loadNative(root); err != nil {
		return nil, "", err
	} else if cfg != nil {
		cfg.Source = src
		ensureSourceInFiles(cfg, root)
		return cfg, "", nil
	}

	// 3: [tool.bumpversion] in pyproject.toml
	if cfg, src, err := loadPyprojectBumpversion(filepath.Join(root, "pyproject.toml")); err != nil {
		return nil, "", err
	} else if cfg != nil {
		cfg.Source = src
		ensureSourceInFiles(cfg, root)
		return cfg, "", nil
	}

	// 4: .bumpversion.toml standalone
	if cfg, err := loadBumpversionTOML(filepath.Join(root, ".bumpversion.toml")); err != nil {
		return nil, "", err
	} else if cfg != nil {
		cfg.Source = filepath.Join(root, ".bumpversion.toml")
		ensureSourceInFiles(cfg, root)
		return cfg, "", nil
	}

	// 5: .bumpversion.cfg (legacy INI).
	iniPath := filepath.Join(root, ".bumpversion.cfg")
	if exists(iniPath) {
		if cfg, err := loadBumpversionINI(iniPath); err != nil {
			return nil, "", err
		} else if cfg != nil {
			ensureSourceInFiles(cfg, root)
			return cfg, ".bumpversion.cfg (INI) is parsed but deprecated; migrate to .bumpversion.toml or [tool.bumpversion] in pyproject.toml with `stratt config migrate-bump`", nil
		}
	}

	return nil, "", nil
}

// ensureSourceInFiles auto-adds the bump source file to cfg.Files when
// the user hasn't listed it explicitly, so its `current_version` field
// stays in sync after every bump.  Without this, the in-file version
// freezes at the original value and the next release computes its
// starting point from a stale number.
//
// bump-my-version does this transparently; we match that behavior.
// Patterns are format-aware — TOML quotes its version string, INI
// doesn't.
func ensureSourceInFiles(cfg *Config, root string) {
	if cfg == nil || cfg.Source == "" {
		return
	}
	rel, err := filepath.Rel(root, cfg.Source)
	if err != nil {
		rel = filepath.Base(cfg.Source)
	}
	for _, f := range cfg.Files {
		if filepath.Clean(f.Filename) == filepath.Clean(rel) {
			return
		}
	}
	search, replace := defaultSearchReplaceForSource(rel)
	cfg.Files = append(cfg.Files, FileEntry{
		Filename: rel,
		Search:   search,
		Replace:  replace,
	})
}

// defaultSearchReplaceForSource picks the format-correct search/replace
// templates for the current_version line.  INI omits the quotes that
// TOML wraps strings in.
func defaultSearchReplaceForSource(filename string) (search, replace string) {
	if strings.HasSuffix(filename, ".cfg") || strings.HasSuffix(filename, ".ini") {
		return `current_version = {current_version}`, `current_version = {new_version}`
	}
	return `current_version = "{current_version}"`, `current_version = "{new_version}"`
}

func loadNative(root string) (*Config, string, error) {
	// stratt.toml [bump]
	strattPath := filepath.Join(root, "stratt.toml")
	if exists(strattPath) {
		var doc struct {
			Bump *rawBumpVersion `toml:"bump"`
		}
		if err := decodePermissive(strattPath, &doc); err != nil {
			return nil, "", err
		}
		if doc.Bump != nil {
			return doc.Bump.toConfig(), strattPath, nil
		}
	}

	// pyproject.toml [tool.stratt.bump]
	pyPath := filepath.Join(root, "pyproject.toml")
	if exists(pyPath) {
		var top map[string]any
		if err := decodePermissive(pyPath, &top); err != nil {
			return nil, "", err
		}
		if bumpRaw := dig(top, "tool", "stratt", "bump"); bumpRaw != nil {
			var rb rawBumpVersion
			if err := remarshal(bumpRaw, &rb); err != nil {
				return nil, "", fmt.Errorf("[tool.stratt.bump]: %w", err)
			}
			return rb.toConfig(), pyPath, nil
		}
	}
	return nil, "", nil
}

func loadPyprojectBumpversion(path string) (*Config, string, error) {
	if !exists(path) {
		return nil, "", nil
	}
	var top map[string]any
	if err := decodePermissive(path, &top); err != nil {
		return nil, "", err
	}
	bumpRaw := dig(top, "tool", "bumpversion")
	if bumpRaw == nil {
		return nil, "", nil
	}
	var rb rawBumpVersion
	if err := remarshal(bumpRaw, &rb); err != nil {
		return nil, "", fmt.Errorf("[tool.bumpversion]: %w", err)
	}
	return rb.toConfig(), path, nil
}

func loadBumpversionTOML(path string) (*Config, error) {
	if !exists(path) {
		return nil, nil
	}
	var rb rawBumpVersion
	if err := decodePermissive(path, &rb); err != nil {
		return nil, err
	}
	return rb.toConfig(), nil
}

// rawBumpVersion is the union of bump-my-version's [tool.bumpversion]
// schema and stratt's native [bump] schema.  Parsing is permissive so
// bump-my-version configs with fields we don't honor (parse,
// serialize, regex, pre_commit_hooks, sign_tags, allow_dirty, etc.)
// still load — those fields are read but ignored.
type rawBumpVersion struct {
	CurrentVersion string         `toml:"current_version"`
	Search         string         `toml:"search"`
	Replace        string         `toml:"replace"`
	Files          []rawFileEntry `toml:"files"`
	Tag            *bool          `toml:"tag"`
	TagName        string         `toml:"tag_name"`
	Commit         *bool          `toml:"commit"`
	Message        string         `toml:"message"`

	// Parsed but not honored; see rawBumpVersion doc.
	Parse                string   `toml:"parse"`
	Serialize            []string `toml:"serialize"`
	Regex                *bool    `toml:"regex"`
	IgnoreMissingVersion *bool    `toml:"ignore_missing_version"`
	SignTags             *bool    `toml:"sign_tags"`
	TagMessage           string   `toml:"tag_message"`
	AllowDirty           *bool    `toml:"allow_dirty"`
	CommitArgs           string   `toml:"commit_args"`
	PreCommitHooks       []string `toml:"pre_commit_hooks"`
}

type rawFileEntry struct {
	Filename string `toml:"filename"`
	Search   string `toml:"search"`
	Replace  string `toml:"replace"`
}

func (rb *rawBumpVersion) toConfig() *Config {
	c := &Config{
		CurrentVersion:  rb.CurrentVersion,
		SearchTemplate:  rb.Search,
		ReplaceTemplate: rb.Replace,
		MessageTemplate: rb.Message,
		TagNameTemplate: rb.TagName,
		Commit:          rb.Commit == nil || *rb.Commit,
		Tag:             rb.Tag == nil || *rb.Tag,
	}
	for _, fe := range rb.Files {
		c.Files = append(c.Files, FileEntry{
			Filename: fe.Filename,
			Search:   fe.Search,
			Replace:  fe.Replace,
		})
	}
	return c
}

// decodePermissive reads path as TOML into target without strict-mode.
func decodePermissive(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return toml.Unmarshal(data, target)
}

// dig walks a nested map[string]any by successive keys.  Returns nil
// if any segment is missing or isn't a map at the right depth.
func dig(root map[string]any, keys ...string) any {
	cur := any(root)
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		next, ok := m[k]
		if !ok {
			return nil
		}
		cur = next
	}
	return cur
}

// remarshal serializes v to TOML then decodes into target — used to
// type a map[string]any subtree without writing manual coercions.
// go-toml/v2 has no Decode-from-map API.
func remarshal(v any, target any) error {
	buf := &bytes.Buffer{}
	if err := toml.NewEncoder(buf).Encode(v); err != nil {
		return err
	}
	return toml.Unmarshal(buf.Bytes(), target)
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
