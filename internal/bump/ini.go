package bump

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// loadBumpversionINI parses the legacy `.bumpversion.cfg` format
// (bump2version, older bump-my-version).  Recognizes the top-level
// `[bumpversion]` section and `[bumpversion:file:PATH]` per-file
// sections; per-file sections inherit search/replace from the top
// level when not overridden.
//
// Sections like `[bumpversion:part:...]` (custom version components)
// are parsed but ignored.
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
			filename = strings.TrimPrefix(filename, "./")
			c.Files = append(c.Files, FileEntry{
				Filename: filename,
				Search:   s.KV["search"],
				Replace:  s.KV["replace"],
			})
		}
	}
	return c, nil
}

type iniSection struct {
	Name string
	KV   map[string]string
}

// parseINI handles section headers, key=value lines, and `#` / `;`
// comments.  Multi-line values are not supported.
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

// parseINIBool accepts the truthy/falsy strings bump-my-version and
// Python's configparser tolerate.  Empty input → defaultVal.
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

// orFirst returns the first non-empty argument.
func orFirst(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
