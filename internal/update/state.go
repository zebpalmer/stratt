package update

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// State is the persisted update-checker state at ~/.stratt/cache/state.json.
// Tracks the last check time (R4.12) and the previously installed
// version (R4.13) for rollback.
type State struct {
	LastCheck         time.Time `json:"last_check"`
	LatestSeenVersion string    `json:"latest_seen_version"`
	PreviousVersion   string    `json:"previous_version"`
}

// StatePath returns the canonical state-file path under HOME.
func StatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".stratt", "cache", "state.json"), nil
}

// LoadState reads State.  Missing files return a zero-value State and
// no error — that's a fresh stratt install.
func LoadState() (*State, error) {
	path, err := StatePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &State{}, nil
	}
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		// Corrupted state file: log via the returned zero-value State
		// and the caller's choice to either reset or surface the issue.
		return &State{}, nil
	}
	return &s, nil
}

// SaveState writes s atomically.
func SaveState(s *State) error {
	path, err := StatePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// CacheDir returns the per-version binary cache directory used by Apply
// (for rollback support) — `~/.stratt/cache/binaries/<version>/`.
func CacheDir(version string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".stratt", "cache", "binaries", version), nil
}
