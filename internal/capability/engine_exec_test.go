package capability

import (
	"context"
	"strings"
	"testing"
)

func TestExecEngineNameDefault(t *testing.T) {
	e := &execEngine{tool: "uv", argv: []string{"run", "pytest"}}
	if got := e.Name(); got != "uv run pytest" {
		t.Errorf("got %q, want %q", got, "uv run pytest")
	}
}

func TestExecEngineNameOverride(t *testing.T) {
	e := &execEngine{tool: "go", argv: []string{"build", "./..."}, display: "go build ./..."}
	if got := e.Name(); got != "go build ./..." {
		t.Errorf("got %q", got)
	}
}

func TestExecEngineStatusMissing(t *testing.T) {
	// A tool name that's vanishingly unlikely to exist on PATH.
	e := &execEngine{tool: "this-binary-definitely-does-not-exist-xyzzy"}
	if got := e.Status(); got != StatusMissingTool {
		t.Errorf("got %v, want StatusMissingTool", got)
	}
}

func TestExecEngineStatusReady(t *testing.T) {
	// `go` should be on PATH in any environment that built the test
	// binary in the first place.
	e := &execEngine{tool: "go", argv: []string{"version"}}
	if got := e.Status(); got != StatusReady {
		t.Errorf("got %v, want StatusReady", got)
	}
}

// TestExecEngineRun — runs a known-safe trivial command and confirms
// success / non-zero handling.  Uses the standard `go version` because
// it's guaranteed to exist on a Go test environment.
func TestExecEngineRun(t *testing.T) {
	e := &execEngine{tool: "go", argv: []string{"version"}}
	if err := e.Run(context.Background(), nil); err != nil {
		t.Errorf("Run failed unexpectedly: %v", err)
	}
}

func TestExecEngineRunFailure(t *testing.T) {
	// `go this-subcommand-does-not-exist` returns non-zero with a real
	// error.  We expect Run to surface that.
	e := &execEngine{tool: "go", argv: []string{"this-subcommand-does-not-exist"}}
	err := e.Run(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error from `go this-subcommand-does-not-exist`, got nil")
	}
	// Error should include the engine name to be useful in stratt output.
	if !strings.Contains(err.Error(), "go this-subcommand-does-not-exist") {
		t.Errorf("error should include engine name; got %q", err.Error())
	}
}

func TestNotImplementedEngine(t *testing.T) {
	e := &notImplementedEngine{display: "future thing"}
	if e.Name() != "future thing" {
		t.Errorf("name: got %q", e.Name())
	}
	if e.Status() != StatusPending {
		t.Errorf("status: got %v", e.Status())
	}
	err := e.Run(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("got %q", err.Error())
	}
}

func TestDelegateEngine(t *testing.T) {
	e := &delegateEngine{display: "release engine", delegateCmd: "stratt release"}
	if e.Name() != "release engine" {
		t.Errorf("name: got %q", e.Name())
	}
	if e.Status() != StatusReady {
		t.Errorf("delegate engines should report Ready, got %v", e.Status())
	}
	err := e.Run(context.Background(), nil)
	if err == nil {
		t.Fatal("delegate engine Run should error")
	}
	if !strings.Contains(err.Error(), "stratt release") {
		t.Errorf("error should reference the delegate command: %q", err.Error())
	}
}
