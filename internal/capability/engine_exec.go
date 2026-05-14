package capability

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var _ = strings.TrimSpace // keep strings import in case future engines need it

// execEngine is the standard Engine implementation: a single external
// command run in the repo root.  Most engines are just this with
// different argv.
type execEngine struct {
	tool    string   // the binary on PATH, e.g. "uv" or "go"
	argv    []string // arguments after the tool name
	display string   // optional override for Name(); defaults to "tool argv..."
	cwd     string   // directory to run in; defaults to repo root
}

func (e *execEngine) Name() string {
	if e.display != "" {
		return e.display
	}
	return strings.Join(append([]string{e.tool}, e.argv...), " ")
}

func (e *execEngine) Status() EngineStatus {
	if !available(e.tool) {
		return StatusMissingTool
	}
	return StatusReady
}

func (e *execEngine) Run(ctx context.Context, _ []string) error {
	cmd := exec.CommandContext(ctx, e.tool, e.argv...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if e.cwd != "" {
		cmd.Dir = e.cwd
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", e.Name(), err)
	}
	return nil
}

// shellEngine wraps a `sh -c <line>` invocation — used when a resolved
// backend needs shell composition (pipelines, `&&`, redirects).  Status
// reports Ready unconditionally; the shell handles missing-tool errors
// at run time with its own messaging.
type shellEngine struct {
	line    string
	display string
}

func (e *shellEngine) Name() string {
	if e.display != "" {
		return e.display
	}
	return e.line
}

func (e *shellEngine) Status() EngineStatus { return StatusReady }

func (e *shellEngine) Run(ctx context.Context, _ []string) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", e.line)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", e.Name(), err)
	}
	return nil
}

// notImplementedEngine is a placeholder for engines that the requirements
// call for but aren't built yet.  It returns a clear error if invoked
// but reports a useful display name for `stratt doctor`.
type notImplementedEngine struct {
	display string
}

func (e *notImplementedEngine) Name() string         { return e.display }
func (e *notImplementedEngine) Status() EngineStatus { return StatusPending }
func (e *notImplementedEngine) Run(_ context.Context, _ []string) error {
	return fmt.Errorf("engine %q is not yet implemented", e.display)
}

// delegateEngine is a "resolver result that isn't executed by the
// runner" — used when the resolver wants to say "this is the backend
// for command X" but the actual execution path is a custom-shape
// subcommand (e.g. `stratt release`, `stratt deploy`).  Status reports
// Ready so `stratt doctor` shows no caveat.  Calling Run is a
// programming error and returns an instructive message.
type delegateEngine struct {
	display     string
	delegateCmd string // user-facing command to invoke instead
}

func (e *delegateEngine) Name() string         { return e.display }
func (e *delegateEngine) Status() EngineStatus { return StatusReady }
func (e *delegateEngine) Run(_ context.Context, _ []string) error {
	return fmt.Errorf("engine %q is dispatched by the %q subcommand, not the runner",
		e.display, e.delegateCmd)
}
