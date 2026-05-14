package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// MigrateBump consolidates legacy bump-my-version config into stratt's
// native [bump] (in stratt.toml) or [tool.stratt.bump] (in pyproject.toml)
// location, per R2.4.8.
//
// Target selection per R2.4.8:
//
//  1. stratt.toml exists → write to [bump] in stratt.toml
//  2. Else if pyproject.toml exists → write to [tool.stratt.bump]
//  3. Else → create stratt.toml with [bump]
//
// Sources tried in order; the first non-empty source is used:
//
//   - [tool.bumpversion] in pyproject.toml
//   - .bumpversion.toml at repo root
//
// `.bumpversion.cfg` (INI) is not auto-migrated — users must port that
// manually since it would require a TOML conversion this package doesn't
// perform.
//
// Returns the target file path written, and (optionally) the source
// file/section the migration consumed, for caller-side cleanup
// confirmation.
func MigrateBump(root string, out io.Writer) (target, source string, err error) {
	srcMap, source, err := readLegacyBumpSection(root)
	if err != nil {
		return "", "", err
	}
	if srcMap == nil {
		return "", "", fmt.Errorf("no legacy bump-my-version config found to migrate")
	}

	target = chooseBumpTarget(root)
	if target == "" {
		return "", "", fmt.Errorf("could not determine migration target")
	}
	if err := writeBumpSection(target, srcMap); err != nil {
		return "", "", err
	}
	fmt.Fprintf(out, "migrated bump config: %s → %s\n", source, target)
	return target, source, nil
}

// readLegacyBumpSection finds the first legacy bump source and returns
// the section's contents as a map.
func readLegacyBumpSection(root string) (map[string]any, string, error) {
	// 1. pyproject.toml [tool.bumpversion]
	pyPath := filepath.Join(root, "pyproject.toml")
	if data, err := os.ReadFile(pyPath); err == nil {
		var top map[string]any
		if err := toml.Unmarshal(data, &top); err == nil {
			if t, ok := top["tool"].(map[string]any); ok {
				if bm, ok := t["bumpversion"].(map[string]any); ok {
					return bm, pyPath + " [tool.bumpversion]", nil
				}
			}
		}
	}

	// 2. .bumpversion.toml standalone
	bvPath := filepath.Join(root, ".bumpversion.toml")
	if data, err := os.ReadFile(bvPath); err == nil {
		var top map[string]any
		if err := toml.Unmarshal(data, &top); err == nil {
			return top, bvPath, nil
		}
	}

	return nil, "", nil
}

// chooseBumpTarget picks the file to write into per R2.4.8 target rules.
// Always returns the absolute path of an existing-or-to-be-created file.
func chooseBumpTarget(root string) string {
	strattPath := filepath.Join(root, "stratt.toml")
	if _, err := os.Stat(strattPath); err == nil {
		return strattPath
	}
	pyPath := filepath.Join(root, "pyproject.toml")
	if _, err := os.Stat(pyPath); err == nil {
		return pyPath
	}
	// Neither exists — create stratt.toml.
	return strattPath
}

// writeBumpSection writes the bump map into the appropriate section of
// target.  For stratt.toml it lives at top-level [bump]; for pyproject
// it nests under [tool.stratt.bump].
//
// We do not preserve user-authored comments — the legacy config is read
// permissively and re-emitted in normalized form.  Acceptable trade-off
// for a one-shot migration; the user reviews the result before
// committing.
func writeBumpSection(target string, srcMap map[string]any) error {
	isPyproject := strings.HasSuffix(target, "pyproject.toml")

	var top map[string]any
	if data, err := os.ReadFile(target); err == nil {
		_ = toml.Unmarshal(data, &top)
	}
	if top == nil {
		top = map[string]any{}
	}

	if isPyproject {
		tool, _ := top["tool"].(map[string]any)
		if tool == nil {
			tool = map[string]any{}
		}
		stratt, _ := tool["stratt"].(map[string]any)
		if stratt == nil {
			stratt = map[string]any{}
		}
		stratt["bump"] = srcMap
		tool["stratt"] = stratt
		top["tool"] = tool
	} else {
		top["bump"] = srcMap
	}

	out, err := toml.Marshal(top)
	if err != nil {
		return err
	}
	if err := os.WriteFile(target+".tmp", out, 0o644); err != nil {
		return err
	}
	return os.Rename(target+".tmp", target)
}
