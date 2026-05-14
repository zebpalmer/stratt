package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zebpalmer/stratt/internal/kustomize"
)

func mustRunInDir(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

func writeOverlay(t *testing.T, root, env, body string) {
	t.Helper()
	path := kustomize.OverlayPath(root, env)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDeployUpdatesOverlay(t *testing.T) {
	dir := t.TempDir()
	withCwd(t, dir)
	writeOverlay(t, dir, "prod", `images:
  - name: app
    newTag: 1.0.0
`)

	cmd := newDeployCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"prod", "1.0.1"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	body, _ := os.ReadFile(kustomize.OverlayPath(dir, "prod"))
	if !strings.Contains(string(body), "newTag: 1.0.1") {
		t.Errorf("overlay not updated:\n%s", body)
	}
	if !strings.Contains(out.String(), "Not committed") {
		t.Errorf("should print non-committed hint by default:\n%s", out.String())
	}
}

func TestDeployErrorsOnMissingEnv(t *testing.T) {
	dir := t.TempDir()
	withCwd(t, dir)
	cmd := newDeployCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"nonexistent", "1.0.0"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing overlay")
	}
	if !strings.Contains(err.Error(), "no overlay") {
		t.Errorf("got %v", err)
	}
}

// TestDeployCommitFlag — with --commit and --yes, the change is staged
// and committed by the inline git helper.
func TestDeployCommitFlag(t *testing.T) {
	dir := t.TempDir()
	mustRunInDir(t, dir, "git", "init", "--initial-branch=main", "-q")
	mustRunInDir(t, dir, "git", "config", "user.email", "test@example.com")
	mustRunInDir(t, dir, "git", "config", "user.name", "Test User")
	mustRunInDir(t, dir, "git", "config", "commit.gpgsign", "false")

	writeOverlay(t, dir, "prod", `images:
  - name: app
    newTag: 1.0.0
`)
	mustRunInDir(t, dir, "git", "add", "-A")
	mustRunInDir(t, dir, "git", "commit", "-q", "-m", "initial")

	withCwd(t, dir)
	cmd := newDeployCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"prod", "1.0.1", "--commit", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	logOut, _ := exec.Command("git", "-C", dir, "log", "-1", "--pretty=%s").CombinedOutput()
	if !strings.Contains(string(logOut), "deploy prod app 1.0.1") {
		t.Errorf("expected deploy commit; got: %s", logOut)
	}
}

func TestDeployImageFlagForMultipleImages(t *testing.T) {
	dir := t.TempDir()
	withCwd(t, dir)
	writeOverlay(t, dir, "prod", `images:
  - name: foo
    newTag: 1.0.0
  - name: bar
    newTag: 2.0.0
`)
	cmd := newDeployCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"prod", "2.1.0", "--image=bar"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(kustomize.OverlayPath(dir, "prod"))
	if !strings.Contains(string(body), "newTag: 1.0.0") || !strings.Contains(string(body), "newTag: 2.1.0") {
		t.Errorf("expected foo unchanged and bar updated:\n%s", body)
	}
}
