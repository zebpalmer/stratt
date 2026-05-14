package version

import (
	"strings"
	"testing"
)

func TestParseValid(t *testing.T) {
	tests := []struct {
		in        string
		wantBound string
	}{
		{">= 1.0.0", "v1.0.0"},
		{">= 1.0", "v1.0.0"},
		{">= 1", "v1.0.0"},
		{">=  1.2.3", "v1.2.3"},
		{"  >= 0.5.1  ", "v0.5.1"},
		{">=v1.2.3", "v1.2.3"},
	}
	for _, tc := range tests {
		c, err := Parse(tc.in)
		if err != nil {
			t.Errorf("Parse(%q) errored: %v", tc.in, err)
			continue
		}
		if c.Bound != tc.wantBound {
			t.Errorf("Parse(%q).Bound = %q, want %q", tc.in, c.Bound, tc.wantBound)
		}
		if c.Op != ">=" {
			t.Errorf("Parse(%q).Op = %q, want >=", tc.in, c.Op)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	tests := []string{
		"",
		"> 1.0.0",                 // unsupported operator
		"< 1.0.0",                 // ditto
		"~> 1.0",                  // ditto
		"^1.0.0",                  // ditto
		">=",                      // no version
		">= notaversion",          // garbage
		">= 1.2.3.4-extra-junk@x", // not semver
	}
	for _, in := range tests {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) should have errored", in)
		}
	}
}

func TestSatisfies(t *testing.T) {
	c, _ := Parse(">= 1.2.0")

	cases := []struct {
		got  string
		want bool
	}{
		{"1.2.0", true},
		{"1.2.1", true},
		{"1.3.0", true},
		{"2.0.0", true},
		{"1.1.9", false},
		{"0.5.0", false},
	}
	for _, tc := range cases {
		if got := c.Satisfies(tc.got); got != tc.want {
			t.Errorf("Satisfies(%q) = %v, want %v", tc.got, got, tc.want)
		}
	}
}

// TestSatisfiesDevBypass — non-semver and the literal "dev" string
// (used by goreleaser's snapshot builds) bypass the version constraint
// so contributors are never blocked by their own WIP build.
//
// A *valid* prerelease like "0.0.0-dev" does NOT bypass — it parses as
// real semver and is checked normally.
func TestSatisfiesDevBypass(t *testing.T) {
	c, _ := Parse(">= 99.0.0")
	for _, got := range []string{"dev", "", "snapshot-of-something"} {
		if !c.Satisfies(got) {
			t.Errorf("non-semver version %q should bypass the constraint", got)
		}
	}
	// Valid prerelease semver does NOT bypass.
	if c.Satisfies("0.0.0-dev") {
		t.Error("valid prerelease semver should be evaluated, not bypassed")
	}
}

// TestSatisfiesCanonicalizesGot — versions like "1.2" (no patch) are
// accepted from the binary side too.
func TestSatisfiesCanonicalizesGot(t *testing.T) {
	c, _ := Parse(">= 1.2")
	if !c.Satisfies("1.2") {
		t.Error("`1.2` should satisfy `>= 1.2`")
	}
}

func TestCheckPasses(t *testing.T) {
	if err := Check(">= 1.0.0", "1.5.0"); err != nil {
		t.Errorf("expected pass, got: %v", err)
	}
}

func TestCheckFails(t *testing.T) {
	err := Check(">= 2.0.0", "1.5.0")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), ">= 2.0.0") {
		t.Errorf("error should include constraint: %v", err)
	}
	if !strings.Contains(err.Error(), "1.5.0") {
		t.Errorf("error should include current version: %v", err)
	}
	if !strings.Contains(err.Error(), "brew upgrade") {
		t.Errorf("error should suggest upgrade command: %v", err)
	}
}

func TestCheckEmptyConstraintIsNoop(t *testing.T) {
	if err := Check("", "1.0.0"); err != nil {
		t.Errorf("empty constraint should pass: %v", err)
	}
}
