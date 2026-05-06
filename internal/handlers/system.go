package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/logging"
)

// SystemHandler serves system info and update check endpoints.
type SystemHandler struct {
	version   string
	commit    string
	buildDate string
	dataDir   string
	startedAt time.Time

	// Update-check cache. GitHub's unauthenticated API rate limit is
	// 60 req/hour/IP, so a refresh interval of 1h keeps us comfortably
	// inside that even if every Settings page load triggers a check.
	updateMu      sync.Mutex
	cachedUpdate  *UpdateCheckResponse
	cachedAt      time.Time
	cacheTTL      time.Duration
	httpClient    *http.Client
	releaseAPIURL string // overridable for tests
}

// NewSystemHandler creates a new system handler.
func NewSystemHandler(version, commit, buildDate, dataDir string) *SystemHandler {
	return &SystemHandler{
		version:       version,
		commit:        commit,
		buildDate:     buildDate,
		dataDir:       dataDir,
		startedAt:     time.Now(),
		cacheTTL:      time.Hour,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		releaseAPIURL: fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo),
	}
}

// SystemInfoResponse is the JSON response for GET /api/system/info.
type SystemInfoResponse struct {
	Version     string      `json:"version"`
	Commit      string      `json:"commit"`
	BuildDate   string      `json:"build_date"`
	GoVersion   string      `json:"go_version"`
	OS          string      `json:"os"`
	Arch        string      `json:"arch"`
	Environment string      `json:"environment"`
	Uptime      string      `json:"uptime"`
	UptimeSecs  int64       `json:"uptime_seconds"`
	StartedAt   string      `json:"started_at"`
	DataDir     string      `json:"data_dir"`
	Links       SystemLinks `json:"links"`
}

// SystemLinks contains project URLs.
type SystemLinks struct {
	GitHub   string `json:"github"`
	Issues   string `json:"issues"`
	Releases string `json:"releases"`
	Wiki     string `json:"wiki"`
}

// UpdateCheckResponse is the JSON response for GET /api/system/updates.
type UpdateCheckResponse struct {
	CurrentVersion  string            `json:"current_version"`
	LatestVersion   string            `json:"latest_version"`
	UpdateAvailable bool              `json:"update_available"`
	ReleaseURL      string            `json:"release_url"`
	Changelog       string            `json:"changelog"`
	PublishedAt     string            `json:"published_at"`
	DownloadURLs    map[string]string `json:"download_urls"`
}

type gitHubRelease struct {
	TagName     string        `json:"tag_name"`
	HTMLURL     string        `json:"html_url"`
	Body        string        `json:"body"`
	PublishedAt string        `json:"published_at"`
	Assets      []gitHubAsset `json:"assets"`
}

type gitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

const (
	githubRepo    = "mescon/Muximux"
	githubBaseURL = "https://github.com/" + githubRepo
)

var projectLinks = SystemLinks{
	GitHub:   githubBaseURL,
	Issues:   githubBaseURL + "/issues",
	Releases: githubBaseURL + "/releases",
	Wiki:     githubBaseURL + "/wiki",
}

// GetInfo returns system information. The full payload (data dir,
// runtime, environment, uptime) is restricted to admin callers because
// it gives an attacker plotting next-step exploits a free map of the
// instance. Non-admin callers get the small subset the SPA shell needs
// to render the version label, with everything else stripped.
func (h *SystemHandler) GetInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	resp := SystemInfoResponse{
		Version:   h.version,
		Commit:    h.commit,
		BuildDate: h.buildDate,
		Links:     projectLinks,
	}
	if user := auth.GetUserFromContext(r.Context()); user != nil && user.Role == "admin" {
		uptime := time.Since(h.startedAt)
		resp.GoVersion = runtime.Version()
		resp.OS = runtime.GOOS
		resp.Arch = runtime.GOARCH
		resp.Environment = detectEnvironment()
		resp.Uptime = formatUptime(uptime)
		resp.UptimeSecs = int64(uptime.Seconds())
		resp.StartedAt = h.startedAt.UTC().Format(time.RFC3339)
		resp.DataDir = h.dataDir
	}

	sendJSON(w, http.StatusOK, resp)
}

// CheckUpdate checks GitHub for the latest release.
//
// Result is cached for cacheTTL (1h by default) so a Settings page that
// every admin opens repeatedly does not exhaust GitHub's unauthenticated
// 60-req/hour rate limit. The cache is process-local; restarts re-fetch.
// Admin-only at registration: the response is innocuous, but unauth'd
// users hammering this endpoint were the practical DoS vector.
func (h *SystemHandler) CheckUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	// Serve from cache if fresh.
	h.updateMu.Lock()
	if h.cachedUpdate != nil && time.Since(h.cachedAt) < h.cacheTTL {
		cached := *h.cachedUpdate
		h.updateMu.Unlock()
		sendJSON(w, http.StatusOK, cached)
		return
	}
	h.updateMu.Unlock()

	resp, status, err := h.fetchLatestRelease(r.Context())
	if err != nil {
		logging.From(r.Context()).Warn("Update check failed", "source", "system", "error", err)
		sendJSON(w, status, map[string]string{"error": err.Error()})
		return
	}

	// Cache successful responses (including the "no releases" 404 path,
	// which still produces a valid resp).
	h.updateMu.Lock()
	cachedCopy := resp
	h.cachedUpdate = &cachedCopy
	h.cachedAt = time.Now()
	h.updateMu.Unlock()

	if resp.UpdateAvailable {
		logging.From(r.Context()).Info("Update available", "source", "system", "current", h.version, "latest", resp.LatestVersion)
	} else {
		logging.From(r.Context()).Debug("Update check: up to date", "source", "system", "version", h.version)
	}

	sendJSON(w, http.StatusOK, resp)
}

// fetchLatestRelease performs the GitHub API call. Returns the response
// shape, the HTTP status to surface to the client on error, and the
// error itself. Split out so CheckUpdate can short-circuit on cache hits.
func (h *SystemHandler) fetchLatestRelease(ctx context.Context) (UpdateCheckResponse, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.releaseAPIURL, nil)
	if err != nil {
		return UpdateCheckResponse{}, http.StatusInternalServerError, fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("User-Agent", "Muximux/"+h.version)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	ghResp, err := h.httpClient.Do(req)
	if err != nil {
		return UpdateCheckResponse{}, http.StatusServiceUnavailable, fmt.Errorf("failed to check for updates: %w", err)
	}
	defer ghResp.Body.Close()

	if ghResp.StatusCode == http.StatusNotFound {
		return UpdateCheckResponse{
			CurrentVersion:  h.version,
			LatestVersion:   h.version,
			UpdateAvailable: false,
			DownloadURLs:    map[string]string{},
		}, http.StatusOK, nil
	}

	if ghResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(ghResp.Body, 1024))
		return UpdateCheckResponse{}, http.StatusServiceUnavailable, fmt.Errorf("GitHub API error %d: %s", ghResp.StatusCode, string(body))
	}

	var release gitHubRelease
	if err := json.NewDecoder(ghResp.Body).Decode(&release); err != nil {
		return UpdateCheckResponse{}, http.StatusInternalServerError, fmt.Errorf("failed to parse GitHub response: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	return UpdateCheckResponse{
		CurrentVersion:  h.version,
		LatestVersion:   latestVersion,
		UpdateAvailable: compareVersions(h.version, latestVersion) < 0,
		ReleaseURL:      release.HTMLURL,
		Changelog:       release.Body,
		PublishedAt:     release.PublishedAt,
		DownloadURLs:    buildDownloadURLs(release.Assets),
	}, http.StatusOK, nil
}

// detectEnvironment returns "docker" if running inside a container, "native" otherwise.
func detectEnvironment() string {
	// Check for /.dockerenv (Linux)
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "docker"
	}
	// Check /proc/1/cgroup for container runtimes (Linux)
	data, err := os.ReadFile("/proc/1/cgroup")
	if err == nil {
		content := string(data)
		if strings.Contains(content, "docker") || strings.Contains(content, "containerd") {
			return "docker"
		}
	}
	// Check common container environment variables (works on all platforms)
	for _, env := range []string{"DOCKER_CONTAINER", "container", "KUBERNETES_SERVICE_HOST"} {
		if os.Getenv(env) != "" {
			return "docker"
		}
	}
	return "native"
}

// formatUptime formats a duration as "Xd Yh Zm".
func formatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// buildDownloadURLs maps platform keys to asset download URLs.
func buildDownloadURLs(assets []gitHubAsset) map[string]string {
	platforms := map[string][]string{
		"linux_amd64":   {"linux-amd64", "linux_amd64"},
		"linux_arm64":   {"linux-arm64", "linux_arm64"},
		"darwin_amd64":  {"darwin-amd64", "darwin_amd64"},
		"darwin_arm64":  {"darwin-arm64", "darwin_arm64"},
		"windows_amd64": {"windows-amd64", "windows_amd64"},
	}

	result := make(map[string]string)
	for key, patterns := range platforms {
		for _, asset := range assets {
			name := strings.ToLower(asset.Name)
			for _, pat := range patterns {
				if strings.Contains(name, pat) {
					result[key] = asset.BrowserDownloadURL
					break
				}
			}
			if result[key] != "" {
				break
			}
		}
	}
	return result
}

// compareVersions compares two semver-ish version strings.
// Returns -1 if a < b, 0 if equal, 1 if a > b.
// "dev" is always considered older than any release version.
//
// Pre-release suffixes ("-rc1", "-beta", "+build") are split off
// before the numeric compare. Per SemVer 11, a pre-release is
// considered older than its release base ("1.2.3-rc1" < "1.2.3"),
// so when the numeric prefixes match, the side carrying a
// pre-release suffix sorts lower. The previous shape ran every part
// through strconv.Atoi and silently treated "3-rc1" as 0 - which
// flipped the comparison and made the update banner report
// "up to date" while the user was actually on a pre-release of an
// older base (codebase review E7).
func compareVersions(a, b string) int {
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")

	if a == b {
		return 0
	}
	// "dev" is always older than any release
	if a == "dev" {
		return -1
	}
	if b == "dev" {
		return 1
	}

	aBase, aPre := splitVersionSuffix(a)
	bBase, bPre := splitVersionSuffix(b)

	aParts := strings.Split(aBase, ".")
	bParts := strings.Split(bBase, ".")

	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}

	for i := 0; i < maxLen; i++ {
		var aNum, bNum int
		if i < len(aParts) {
			aNum = parseLeadingNumber(aParts[i])
		}
		if i < len(bParts) {
			bNum = parseLeadingNumber(bParts[i])
		}
		if aNum < bNum {
			return -1
		}
		if aNum > bNum {
			return 1
		}
	}

	// Numeric prefixes equal - a side with a pre-release suffix is
	// older than the side without one.
	switch {
	case aPre == "" && bPre != "":
		return 1
	case aPre != "" && bPre == "":
		return -1
	case aPre == bPre:
		return 0
	}
	return comparePrereleaseSegments(aPre, bPre)
}

// comparePrereleaseSegments compares two SemVer prerelease strings per
// SemVer 11.4: split on '.', then per-identifier:
//
//   - both numeric: compare numerically (so "alpha.10" > "alpha.2");
//   - numeric vs non-numeric: numeric sorts lower;
//   - both non-numeric: lexical compare.
//
// A shorter prerelease string with all identifiers equal to the
// other's prefix is older ("1.2.3-rc" < "1.2.3-rc.1"), per 11.4.4.
func comparePrereleaseSegments(a, b string) int {
	aSeg := strings.Split(a, ".")
	bSeg := strings.Split(b, ".")
	min := len(aSeg)
	if len(bSeg) < min {
		min = len(bSeg)
	}
	for i := 0; i < min; i++ {
		if c := compareOnePrereleaseIdentifier(aSeg[i], bSeg[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(aSeg) < len(bSeg):
		return -1
	case len(aSeg) > len(bSeg):
		return 1
	}
	return 0
}

// compareOnePrereleaseIdentifier compares a single dot-separated
// segment of a SemVer prerelease string. "Numeric" means the entire
// segment parses as a non-negative integer with no leading zeros;
// SemVer is strict about that (a leading zero makes it alphanumeric).
func compareOnePrereleaseIdentifier(a, b string) int {
	aNum, aOK := parseSemverNumeric(a)
	bNum, bOK := parseSemverNumeric(b)
	switch {
	case aOK && bOK:
		switch {
		case aNum < bNum:
			return -1
		case aNum > bNum:
			return 1
		}
		return 0
	case aOK && !bOK:
		// Numeric identifiers always have lower precedence than
		// non-numeric per SemVer 11.4.3.
		return -1
	case !aOK && bOK:
		return 1
	}
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// parseSemverNumeric reports whether s is a SemVer numeric identifier:
// non-empty, all digits, no leading zero unless the value is "0".
func parseSemverNumeric(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	if len(s) > 1 && s[0] == '0' {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

// splitVersionSuffix separates the numeric base from any "-pre"
// suffix. Per SemVer 10, build metadata ("+...") is ignored for
// ordering, so we strip it before looking for the prerelease
// delimiter and never include it in the returned `pre`.
func splitVersionSuffix(v string) (base, pre string) {
	if plus := strings.IndexByte(v, '+'); plus >= 0 {
		v = v[:plus]
	}
	if dash := strings.IndexByte(v, '-'); dash >= 0 {
		return v[:dash], v[dash+1:]
	}
	return v, ""
}

// parseLeadingNumber returns the numeric prefix of s, or 0 if there
// is none. Used to be lenient about "3rc1"-style components without
// throwing the whole compare off.
func parseLeadingNumber(s string) int {
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0
	}
	n, _ := strconv.Atoi(s[:end])
	return n
}
