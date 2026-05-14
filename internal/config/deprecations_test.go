package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanFindsBumpversionCfg(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".bumpversion.cfg"),
		[]byte("[bumpversion]\ncurrent_version = 1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].ID != "bumpversion-cfg-ini" {
		t.Errorf("id: got %q", findings[0].ID)
	}
	if findings[0].Severity != SeverityInfo {
		t.Errorf("severity: got %v", findings[0].Severity)
	}
}

func TestScanEmptyDirReturnsNone(t *testing.T) {
	findings, err := Scan(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("got %d findings, want 0", len(findings))
	}
}

func TestMigrateReportsManualForInfoOnlyDeprecations(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, ".bumpversion.cfg"), []byte("[x]\n"), 0o644)

	var buf bytes.Buffer
	fixed, manual, err := Migrate(dir, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixed) != 0 {
		t.Errorf("expected 0 fixed; got %v", fixed)
	}
	if len(manual) != 1 || manual[0] != "bumpversion-cfg-ini" {
		t.Errorf("manual: got %v", manual)
	}
	if !strings.Contains(buf.String(), "manual") {
		t.Errorf("output should label as manual; got %q", buf.String())
	}
}

func TestSeverityStringer(t *testing.T) {
	cases := map[Severity]string{
		SeverityInfo:  "info",
		SeverityWarn:  "warn",
		SeverityError: "error",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("%v.String() = %q, want %q", s, got, want)
		}
	}
}
