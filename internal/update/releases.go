package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

// Release is the minimal GitHub Releases API representation stratt needs.
type Release struct {
	TagName     string  `json:"tag_name"`
	Prerelease  bool    `json:"prerelease"`
	Draft       bool    `json:"draft"`
	HTMLURL     string  `json:"html_url"`
	PublishedAt string  `json:"published_at"`
	Assets      []Asset `json:"assets"`
}

// Asset is one downloadable file attached to a Release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	ContentType        string `json:"content_type"`
	Size               int64  `json:"size"`
}

// ChannelStable / ChannelPrerelease control which releases are considered
// "latest" by the update checker (R4.9 / R4.16).
const (
	ChannelStable     = "stable"
	ChannelPrerelease = "prerelease"
)

// LatestRelease returns the most recent release for repo (owner/name)
// that matches channel.  Returns ErrNoRelease if no eligible release
// exists.  ctx is honored throughout.
//
// HTTP client is injected for testability.  Pass nil for the default
// 10-second-timeout client.
func LatestRelease(ctx context.Context, client *http.Client, repo, channel string) (*Release, error) {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	// `/releases/latest` excludes prereleases server-side; for prerelease
	// channel users we have to enumerate.
	if channel == ChannelStable {
		return fetchLatestStable(ctx, client, repo)
	}
	return fetchLatestIncludingPrerelease(ctx, client, repo)
}

// ErrNoRelease indicates the API returned no eligible release.
var ErrNoRelease = errors.New("no release found")

func fetchLatestStable(ctx context.Context, client *http.Client, repo string) (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	rel, err := fetchRelease(ctx, client, url)
	if err != nil {
		return nil, err
	}
	if rel.Prerelease || rel.Draft {
		// Shouldn't happen because /latest filters, but defense in depth.
		return nil, ErrNoRelease
	}
	return rel, nil
}

func fetchLatestIncludingPrerelease(ctx context.Context, client *http.Client, repo string) (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=20", repo)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET %s: status %d: %s", url, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var list []*Release
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}
	for _, r := range list {
		if r.Draft {
			continue
		}
		return r, nil
	}
	return nil, ErrNoRelease
}

func fetchRelease(ctx context.Context, client *http.Client, url string) (*Release, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, ErrNoRelease
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET %s: status %d: %s", url, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// PickAsset returns the release Asset matching the running platform
// (GoReleaser archive convention: `name_<version>_<os>_<arch>.<ext>`).
// Returns nil if no matching asset is found.
func PickAsset(r *Release, suffix string) *Asset {
	for i := range r.Assets {
		a := &r.Assets[i]
		// Match by suffix-containment to tolerate version-string differences.
		// Skip attestation/checksum sidecars by extension.
		if !strings.Contains(a.Name, suffix) {
			continue
		}
		ext := strings.ToLower(a.Name)
		if strings.HasSuffix(ext, ".tar.gz") || strings.HasSuffix(ext, ".zip") {
			return a
		}
	}
	return nil
}

// IsNewer reports whether candidate is a strictly higher semver than current.
// Returns false on either parse failure (safer default — never downgrade
// to an unparseable version).  R4.8.
func IsNewer(candidate, current string) bool {
	cand := normalizeSemver(candidate)
	cur := normalizeSemver(current)
	if !semver.IsValid(cand) || !semver.IsValid(cur) {
		return false
	}
	return semver.Compare(cand, cur) > 0
}

func normalizeSemver(v string) string {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return v
}
