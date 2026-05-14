// Package version handles semver comparison for the `required_stratt`
// project-level pin (R2.3.12 / R2.3.14).
//
// Stratt only supports the `>= X.Y[.Z]` form today.  Anything else is
// rejected with a clear error.  More operators (`<`, `>`, `~`, range
// expressions) can be added on demand — R2.3.12 explicitly leaves the
// grammar minimal until needs prove otherwise.
package version

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

// Constraint represents a parsed `required_stratt` value, e.g. `">= 1.2"`.
type Constraint struct {
	// Op is currently always ">=".  Kept as a field so future operators
	// can extend without changing the struct shape.
	Op string

	// Bound is the right-hand version, canonicalized to leading-v form
	// (golang.org/x/mod/semver requires this).  e.g. "v1.2.0".
	Bound string

	// Raw is the original user-supplied string, used in error messages.
	Raw string
}

// Parse turns a constraint string into a Constraint.  Surrounding
// whitespace is ignored.  The supported grammar is:
//
//	">=" SPACE+ MAJOR "." MINOR [ "." PATCH ]
//
// Missing minor/patch components default to 0.
func Parse(raw string) (*Constraint, error) {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, ">=") {
		return nil, fmt.Errorf(
			"unsupported version constraint %q: only \">= X.Y[.Z]\" is supported in v1", raw)
	}
	rest := strings.TrimSpace(strings.TrimPrefix(s, ">="))
	if rest == "" {
		return nil, fmt.Errorf("version constraint %q is missing the version", raw)
	}
	canonical, err := canonicalize(rest)
	if err != nil {
		return nil, fmt.Errorf("version constraint %q: %w", raw, err)
	}
	return &Constraint{Op: ">=", Bound: canonical, Raw: raw}, nil
}

// canonicalize normalizes "1.2" → "v1.2.0", "1.2.3" → "v1.2.3", etc.
func canonicalize(v string) (string, error) {
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	// Fill in missing components.  semver.Canonical refuses to add a
	// missing patch; we do it ourselves so callers can write ">= 1.2".
	parts := strings.SplitN(strings.TrimPrefix(v, "v"), ".", 4)
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	v = "v" + strings.Join(parts[:3], ".")
	if rest := parts[3:]; len(rest) > 0 {
		v += "." + strings.Join(rest, ".")
	}
	if !semver.IsValid(v) {
		return "", fmt.Errorf("not a valid semver: %s", v)
	}
	return v, nil
}

// Satisfies reports whether got satisfies the constraint.  got may be
// supplied without a leading 'v'; it is canonicalized internally.
//
// A version of "dev" (or anything else that doesn't parse as semver) is
// treated as satisfying any constraint — local dev builds bypass the
// pin so contributors aren't blocked by their own work-in-progress
// builds.
func (c *Constraint) Satisfies(got string) bool {
	if got == "" || got == "dev" {
		return true
	}
	if !strings.HasPrefix(got, "v") {
		got = "v" + got
	}
	if !semver.IsValid(got) {
		// Treat unparseable versions as dev/local; don't block.
		return true
	}
	switch c.Op {
	case ">=":
		return semver.Compare(got, c.Bound) >= 0
	default:
		return false
	}
}

// Check is the convenience wrapper used by the CLI startup hook: parse
// the constraint string (which may be empty == no constraint), compare
// against current, and return a friendly error if not satisfied.
func Check(constraint, current string) error {
	if strings.TrimSpace(constraint) == "" {
		return nil
	}
	c, err := Parse(constraint)
	if err != nil {
		return err
	}
	if c.Satisfies(current) {
		return nil
	}
	return fmt.Errorf(
		"this repo requires stratt %s (you have %s); upgrade with `brew upgrade stratt` "+
			"or `stratt self update`", c.Raw, current)
}
