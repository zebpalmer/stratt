package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Severity classifies a Deprecation finding (R2.3.9).
type Severity int

const (
	SeverityInfo  Severity = iota // auto-handled; log only
	SeverityWarn                  // still readable; action recommended
	SeverityError                 // cannot proceed
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarn:
		return "warn"
	case SeverityError:
		return "error"
	}
	return "?"
}

// Deprecation describes a known-old config pattern stratt can detect.
//
// Each entry encodes a structural matcher (file existence, field-shape
// inspection) plus a severity and an optional AutoFix function used by
// `stratt config migrate`.  The Hint is user-facing — keep it short
// and tell the user exactly what to do.
type Deprecation struct {
	ID       string
	Severity Severity
	Detect   func(root string) (bool, error)
	Hint     string
	AutoFix  func(root string, out io.Writer) error // nil = manual fix only
}

// Finding is what Scan reports for one matched deprecation.
type Finding struct {
	*Deprecation
}

// Registry is the global list of known deprecations.  New entries are
// appended as schema changes accumulate over stratt's lifetime; old
// entries can be pruned when stratt drops support for very old
// configurations (R2.3.11).
//
// The registry is intentionally small in v1 — most schema fields are
// freshly minted.  The infrastructure is what matters.
var Registry = []Deprecation{
	{
		ID:       "bumpversion-cfg-ini",
		Severity: SeverityInfo,
		Detect: func(root string) (bool, error) {
			_, err := os.Stat(filepath.Join(root, ".bumpversion.cfg"))
			if os.IsNotExist(err) {
				return false, nil
			}
			return err == nil, err
		},
		Hint: ".bumpversion.cfg (INI) is read natively but the format is deprecated upstream. " +
			"Consider migrating to [tool.bumpversion] in pyproject.toml or .bumpversion.toml " +
			"for parity with the rest of the fleet.",
		AutoFix: nil, // INI → TOML migration not automated yet; bump still works against the INI source.
	},
}

// Scan runs every Deprecation matcher against root and returns the
// matching findings.  Caller (typically the CLI startup hook) decides
// how to render them based on Severity.
func Scan(root string) ([]Finding, error) {
	var out []Finding
	for i := range Registry {
		d := &Registry[i]
		matched, err := d.Detect(root)
		if err != nil {
			return nil, fmt.Errorf("deprecation %s: %w", d.ID, err)
		}
		if matched {
			out = append(out, Finding{Deprecation: d})
		}
	}
	return out, nil
}

// Migrate runs every applicable AutoFix in root.  Returns the IDs that
// were auto-fixed and the IDs that still require manual action.
//
// Pure-info deprecations with no AutoFix are reported as manual.
func Migrate(root string, out io.Writer) (fixed, manual []string, err error) {
	findings, err := Scan(root)
	if err != nil {
		return nil, nil, err
	}
	for _, f := range findings {
		if f.AutoFix == nil {
			manual = append(manual, f.ID)
			fmt.Fprintf(out, "manual: %s — %s\n", f.ID, f.Hint)
			continue
		}
		if err := f.AutoFix(root, out); err != nil {
			return fixed, manual, fmt.Errorf("autofix %s: %w", f.ID, err)
		}
		fixed = append(fixed, f.ID)
		fmt.Fprintf(out, "fixed: %s\n", f.ID)
	}
	return fixed, manual, nil
}
