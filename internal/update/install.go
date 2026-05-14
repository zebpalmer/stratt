package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DownloadAsset fetches the asset to a temporary file inside dest dir,
// returning the local path.  Caller is responsible for cleanup if the
// install fails downstream.
func DownloadAsset(ctx context.Context, client *http.Client, a *Asset, destDir string) (string, error) {
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	out := filepath.Join(destDir, a.Name)
	req, err := http.NewRequestWithContext(ctx, "GET", a.BrowserDownloadURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download %s: status %d", a.BrowserDownloadURL, resp.StatusCode)
	}
	f, err := os.Create(out)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return out, nil
}

// ExtractBinary unpacks the named binary out of a tar.gz archive into
// destDir and returns its path.  Sets the executable bit.  Refuses to
// extract anything that would resolve outside destDir (defense against
// crafted archives).
func ExtractBinary(archivePath, destDir, binaryName string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("gunzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if filepath.Base(hdr.Name) != binaryName {
			continue
		}
		// Path traversal defense.
		clean := filepath.Clean(filepath.Join(destDir, filepath.Base(hdr.Name)))
		if !strings.HasPrefix(clean, filepath.Clean(destDir)+string(os.PathSeparator)) && clean != filepath.Clean(destDir) {
			return "", fmt.Errorf("archive entry %q resolves outside dest", hdr.Name)
		}
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return "", err
		}
		out, err := os.OpenFile(clean, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return "", err
		}
		if err := out.Close(); err != nil {
			return "", err
		}
		return clean, nil
	}
	return "", fmt.Errorf("binary %q not found in %s", binaryName, archivePath)
}

// SwapInPlace replaces the currently-running binary with the new one
// atomically.  Preserves the prior binary at backupPath so callers can
// implement rollback (R4.13).  Uses os.Rename which is atomic on POSIX
// filesystems; the kernel keeps the running process's inode alive so
// the current invocation completes without surprise.
func SwapInPlace(currentExe, newBin, backupPath string) error {
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		return err
	}

	// Preserve the current binary as the rollback target.  We copy
	// rather than rename because rename would invalidate the running
	// process's exe path on some systems and complicate diagnostics.
	if err := copyFile(currentExe, backupPath, 0o755); err != nil {
		return fmt.Errorf("backup current binary: %w", err)
	}

	// Stage the new binary in the same directory as currentExe so
	// the final rename is across the same filesystem (atomic).
	stagePath := currentExe + ".stratt-new"
	if err := copyFile(newBin, stagePath, 0o755); err != nil {
		return fmt.Errorf("stage new binary: %w", err)
	}
	if err := os.Rename(stagePath, currentExe); err != nil {
		_ = os.Remove(stagePath)
		return fmt.Errorf("atomic swap: %w", err)
	}
	return nil
}

// copyFile copies src to dst with the given mode.
func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
