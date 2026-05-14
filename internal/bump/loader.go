package bump

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

// Load returns the bump Config for root, reading from the first matching
// location in the R2.4.7 compat chain:
//
//   1. [bump] in stratt.toml
//   2. [tool.stratt.bump] in pyproject.toml
//   3. [tool.bumpversion] in pyproject.toml
//   4. .bumpversion.toml
//   5. .bumpversion.cfg  (deprecation note emitted via the returned warning)
//
// Returns (nil, nil) when no bump config is present anywhere — the
// resolver then uses tag-only mode.
//
// Returns a non-nil warning string when the config came from a
// deprecated location; callers should surface it via the deprecation
// registry path once that lands.
func Load(root string) (*Config, string, error) {
	// 1 & 2: native locations
	if cfg, src, err := loadNative(root); err != nil {
		return nil, "", err
	} else if cfg != nil {
		cfg.Source = src
		return cfg, "", nil
	}

	// 3: [tool.bumpversion] in pyproject.toml
	if cfg, src, err := loadPyprojectBumpversion(filepath.Join(root, "pyproject.toml")); err != nil {
		return nil, "", err
	} else if cfg != nil {
		cfg.Source = src
		return cfg, "", nil
	}

	// 4: .bumpversion.toml standalone
	if cfg, err := loadBumpversionTOML(filepath.Join(root, ".bumpversion.toml")); err != nil {
		return nil, "", err
	} else if cfg != nil {
		cfg.Source = filepath.Join(root, ".bumpversion.toml")
		return cfg, "", nil
	}

	// 5: .bumpversion.cfg — INI format, emits deprecation warning
	if exists(filepath.Join(root, ".bumpversion.cfg")) {
		// We don't implement INI parsing in v1 — the file's presence is
		// reported as a deprecation that asks the user to migrate.
		return nil, ".bumpversion.cfg detected; INI format is not parsed natively, please migrate to .bumpversion.toml or [tool.bumpversion] in pyproject.toml", nil
	}

	return nil, "", nil
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

// rawBumpVersion captures the union of bump-my-version's [tool.bumpversion]
// schema and stratt's native [bump] schema.  Unknown fields are
// permitted (loader is intentionally non-strict here) so existing
// bump-my-version configs with fields we don't honor still parse.
//
// Fields we *don't* support in v1 (parse, serialize, regex,
// pre_commit_hooks, sign_tags, allow_dirty, ignore_missing_version)
// are accepted but not acted on.  See R2.4.10.
type rawBumpVersion struct {
	CurrentVersion string         `toml:"current_version"`
	Search         string         `toml:"search"`
	Replace        string         `toml:"replace"`
	Files          []rawFileEntry `toml:"files"`
	Tag            *bool          `toml:"tag"`
	TagName        string         `toml:"tag_name"`
	Commit         *bool          `toml:"commit"`
	Message        string         `toml:"message"`

	// Accepted but not honored in v1 — silently parsed so existing
	// configs work without error.
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

// decodePermissive reads path as TOML into target.  Unknown fields are
// allowed — see rawBumpVersion docstring.
func decodePermissive(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return toml.Unmarshal(data, target)
}

// dig walks a nested map[string]any tree by successive keys.  Returns
// nil if any segment is missing or not a map at the right depth.  Used
// to extract sub-tables from a permissively-parsed pyproject.toml.
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

// remarshal serializes v to TOML, then decodes into target.  This is
// our crutch for "I have a map[string]any subtree, give me a typed
// struct" — go-toml/v2 doesn't expose a generic Decode-from-map API.
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
