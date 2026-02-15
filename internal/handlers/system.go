package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// SystemHandler serves system info and update check endpoints.
type SystemHandler struct {
	version   string
	commit    string
	buildDate string
	dataDir   string
	startedAt time.Time
}

// NewSystemHandler creates a new system handler.
func NewSystemHandler(version, commit, buildDate, dataDir string) *SystemHandler {
	return &SystemHandler{
		version:   version,
		commit:    commit,
		buildDate: buildDate,
		dataDir:   dataDir,
		startedAt: time.Now(),
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

// GetInfo returns system information.
func (h *SystemHandler) GetInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	uptime := time.Since(h.startedAt)
	resp := SystemInfoResponse{
		Version:     h.version,
		Commit:      h.commit,
		BuildDate:   h.buildDate,
		GoVersion:   runtime.Version(),
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		Environment: detectEnvironment(),
		Uptime:      formatUptime(uptime),
		UptimeSecs:  int64(uptime.Seconds()),
		StartedAt:   h.startedAt.UTC().Format(time.RFC3339),
		DataDir:     h.dataDir,
		Links:       projectLinks,
	}

	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(resp)
}

// CheckUpdate checks GitHub for the latest release.
func (h *SystemHandler) CheckUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, apiURL, nil)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("User-Agent", "Muximux/"+h.version)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	ghResp, err := client.Do(req)
	if err != nil {
		w.Header().Set(headerContentType, contentTypeJSON)
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to check for updates: " + err.Error()})
		return
	}
	defer ghResp.Body.Close()

	if ghResp.StatusCode == http.StatusNotFound {
		// No releases yet
		w.Header().Set(headerContentType, contentTypeJSON)
		json.NewEncoder(w).Encode(UpdateCheckResponse{
			CurrentVersion:  h.version,
			LatestVersion:   h.version,
			UpdateAvailable: false,
			DownloadURLs:    map[string]string{},
		})
		return
	}

	if ghResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(ghResp.Body, 1024))
		w.Header().Set(headerContentType, contentTypeJSON)
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("GitHub API error %d: %s", ghResp.StatusCode, string(body))})
		return
	}

	var release gitHubRelease
	if err := json.NewDecoder(ghResp.Body).Decode(&release); err != nil {
		http.Error(w, "Failed to parse GitHub response", http.StatusInternalServerError)
		return
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	downloads := buildDownloadURLs(release.Assets)

	resp := UpdateCheckResponse{
		CurrentVersion:  h.version,
		LatestVersion:   latestVersion,
		UpdateAvailable: compareVersions(h.version, latestVersion) < 0,
		ReleaseURL:      release.HTMLURL,
		Changelog:       release.Body,
		PublishedAt:     release.PublishedAt,
		DownloadURLs:    downloads,
	}

	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(resp)
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

	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}

	for i := 0; i < maxLen; i++ {
		var aNum, bNum int
		if i < len(aParts) {
			aNum, _ = strconv.Atoi(aParts[i])
		}
		if i < len(bParts) {
			bNum, _ = strconv.Atoi(bParts[i])
		}
		if aNum < bNum {
			return -1
		}
		if aNum > bNum {
			return 1
		}
	}
	return 0
}
