package bump

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// loadBumpversionINI parses the legacy `.bumpversion.cfg` format used
// by bump2version and older bump-my-version installs.  Eight LCG repos
// (dora-metrics, wraith-{daemon,ws}, AITranscript*, lcg-data-zendesk-
// refund-bot, loginmonitor, CourseRecommendationAPI) still use this
// format as of 2026-05.
//
// The format is INI, with a top-level [bumpversion] section for global
// settings and one section per file:
//
//	[bumpversion]
//	current_version = 1.0.1
//	commit = True
//	tag = True
//	tag_name = {new_version}
//
//	[bumpversion:file:./wraithd/__init__.py]
//	search = __version__ = "{current_version}"
//	replace = __version__ = "{new_version}"
//
// Section bodies without an explicit search/replace inherit the
// defaults — same semantics as the TOML variant.
//
// This parser is deliberately minimal: it handles the well-formed
// .bumpversion.cfg files we see in the LCG fleet, not every edge
// case of the INI dialect.
func loadBumpversionINI(path string) (*Config, error) {
	if !exists(path) {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sections, err := parseINI(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	c := &Config{Source: path, Commit: true, Tag: true}
	for _, s := range sections {
		switch {
		case s.Name == "bumpversion":
			c.CurrentVersion = s.KV["current_version"]
			c.SearchTemplate = orFirst(s.KV["search"], "")
			c.ReplaceTemplate = orFirst(s.KV["replace"], "")
			c.MessageTemplate = orFirst(s.KV["message"], "")
			c.TagNameTemplate = orFirst(s.KV["tag_name"], "")
			c.Commit = parseINIBool(s.KV["commit"], true)
			c.Tag = parseINIBool(s.KV["tag"], true)
		case strings.HasPrefix(s.Name, "bumpversion:file:"):
			filename := strings.TrimPrefix(s.Name, "bumpversion:file:")
			// Normalize leading "./" since INI users frequently include it.
			filename = strings.TrimPrefix(filename, "./")
			c.Files = append(c.Files, FileEntry{
				Filename: filename,
				Search:   s.KV["search"],
				Replace:  s.KV["replace"],
			})
		}
		// Other section types (e.g. [bumpversion:part:...] for custom
		// version-component schemes) are accepted but not honored in v1.
	}
	return c, nil
}

// iniSection is one parsed [section] from an INI file.
type iniSection struct {
	Name string
	KV   map[string]string
}

// parseINI is a minimal INI reader: section headers, key=value lines,
// `#` and `;` comments.  Multi-line values are not supported.
func parseINI(data []byte) ([]iniSection, error) {
	var sections []iniSection
	var cur *iniSection
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sections = append(sections, iniSection{
				Name: line[1 : len(line)-1],
				KV:   map[string]string{},
			})
			cur = &sections[len(sections)-1]
			continue
		}
		if cur == nil {
			// Skip keys outside any section.
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		cur.KV[key] = val
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return sections, nil
}

// parseINIBool accepts the various truthy/falsy strings bump-my-version
// and configparser tolerate.  Returns defaultVal when the value is empty.
func parseINIBool(s string, defaultVal bool) bool {
	if s == "" {
		return defaultVal
	}
	if b, err := strconv.ParseBool(strings.TrimSpace(s)); err == nil {
		return b
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "yes", "on":
		return true
	case "false", "no", "off":
		return false
	}
	return defaultVal
}

// orFirst returns the first non-empty argument.  Used to apply defaults
// from a fallback chain without a switch.
func orFirst(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
