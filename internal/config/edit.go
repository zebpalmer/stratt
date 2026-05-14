package config

import (
	"os"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// SetRequiredStratt writes `required_stratt = "<value>"` into the project
// config file at path.  Detects whether path is a pyproject.toml (writes
// to [tool.stratt]) or a stratt.toml (writes to the top level) by the
// filename.
//
// Used by `stratt config require-version` (R2.3.13) and by
// `stratt config migrate` when bumping the pin after a successful migration.
func SetRequiredStratt(path, constraint string) error {
	isPyproject := strings.HasSuffix(path, "pyproject.toml")

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var top map[string]any
	if err := toml.Unmarshal(data, &top); err != nil {
		return err
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
		stratt["required_stratt"] = constraint
		tool["stratt"] = stratt
		top["tool"] = tool
	} else {
		top["required_stratt"] = constraint
	}

	out, err := toml.Marshal(top)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path+".tmp", out, 0o644); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}
