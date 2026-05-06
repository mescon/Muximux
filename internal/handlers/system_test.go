package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mescon/muximux/v3/internal/auth"
)

func adminCtxRequest(method, path string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	ctx := context.WithValue(req.Context(), auth.ContextKeyUser, &auth.User{
		Username: "admin",
		Role:     "admin",
	})
	return req.WithContext(ctx)
}

// ---------------------------------------------------------------------------
// NewSystemHandler
// ---------------------------------------------------------------------------

func TestNewSystemHandler(t *testing.T) {
	h := NewSystemHandler("1.2.3", "abc123", "2025-01-01", "/data")

	if h.version != "1.2.3" {
		t.Errorf("expected version %q, got %q", "1.2.3", h.version)
	}
	if h.commit != "abc123" {
		t.Errorf("expected commit %q, got %q", "abc123", h.commit)
	}
	if h.buildDate != "2025-01-01" {
		t.Errorf("expected buildDate %q, got %q", "2025-01-01", h.buildDate)
	}
	if h.dataDir != "/data" {
		t.Errorf("expected dataDir %q, got %q", "/data", h.dataDir)
	}
	if h.startedAt.IsZero() {
		t.Error("expected startedAt to be set")
	}
	// startedAt should be very recent (within last second)
	if time.Since(h.startedAt) > time.Second {
		t.Error("expected startedAt to be within the last second")
	}
}

// ---------------------------------------------------------------------------
// compareVersions  (pure function)
// ---------------------------------------------------------------------------

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		// equal
		{"equal simple", "1.0.0", "1.0.0", 0},
		{"equal with v prefix", "v1.0.0", "v1.0.0", 0},
		{"equal mixed prefix", "v2.3.4", "2.3.4", 0},

		// a < b
		{"patch less", "1.0.0", "1.0.1", -1},
		{"minor less", "1.0.0", "1.1.0", -1},
		{"major less", "1.0.0", "2.0.0", -1},
		{"shorter version less", "1.0", "1.0.1", -1},

		// a > b
		{"patch greater", "1.0.2", "1.0.1", 1},
		{"minor greater", "1.2.0", "1.1.0", 1},
		{"major greater", "3.0.0", "2.0.0", 1},
		{"shorter version greater", "1.1", "1.0.1", 1},

		// dev handling
		{"dev is less than release", "dev", "1.0.0", -1},
		{"release is greater than dev", "1.0.0", "dev", 1},
		{"dev equals dev", "dev", "dev", 0},
		{"v prefix on release vs dev", "v0.0.1", "dev", 1},

		// edge cases
		{"two-part equal", "1.0", "1.0", 0},
		{"single-part", "3", "2", 1},
		{"zero versions", "0.0.0", "0.0.0", 0},
		{"large numbers", "10.20.30", "10.20.29", 1},

		// SemVer prerelease handling (codebase review E7).
		// 1.2.3-rc1 is older than 1.2.3 per SemVer 11. The previous
		// shape silently treated "3-rc1" as 0 and reported equality.
		{"prerelease less than release", "1.2.3-rc1", "1.2.3", -1},
		{"release greater than prerelease", "1.2.3", "1.2.3-rc1", 1},
		{"two prereleases compared lexically", "1.2.3-alpha", "1.2.3-beta", -1},
		{"prerelease vs older release", "1.2.4-rc1", "1.2.3", 1},
		{"build metadata ignored vs same base", "1.2.3+sha", "1.2.3+other", 0},
		{"prerelease against release with v prefix", "v1.2.3-rc1", "1.2.3", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareVersions(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// formatUptime  (pure function)
// ---------------------------------------------------------------------------

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"zero", 0, "0m"},
		{"seconds only (rounds down)", 45 * time.Second, "0m"},
		{"one minute", time.Minute, "1m"},
		{"minutes only", 37 * time.Minute, "37m"},
		{"one hour", time.Hour, "1h 0m"},
		{"hours and minutes", 2*time.Hour + 15*time.Minute, "2h 15m"},
		{"exactly one day", 24 * time.Hour, "1d 0h 0m"},
		{"days hours minutes", 3*24*time.Hour + 5*time.Hour + 42*time.Minute, "3d 5h 42m"},
		{"large duration", 100*24*time.Hour + 23*time.Hour + 59*time.Minute, "100d 23h 59m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatUptime(tt.duration)
			if got != tt.want {
				t.Errorf("formatUptime(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// buildDownloadURLs  (pure function)
// ---------------------------------------------------------------------------

func TestBuildDownloadURLs(t *testing.T) {
	t.Run("empty assets", func(t *testing.T) {
		result := buildDownloadURLs(nil)
		if len(result) != 0 {
			t.Errorf("expected empty map, got %v", result)
		}
	})

	t.Run("matches hyphenated patterns", func(t *testing.T) {
		assets := []gitHubAsset{
			{Name: "muximux-linux-amd64.tar.gz", BrowserDownloadURL: "https://example.com/linux-amd64"},
			{Name: "muximux-linux-arm64.tar.gz", BrowserDownloadURL: "https://example.com/linux-arm64"},
			{Name: "muximux-darwin-amd64.tar.gz", BrowserDownloadURL: "https://example.com/darwin-amd64"},
			{Name: "muximux-darwin-arm64.tar.gz", BrowserDownloadURL: "https://example.com/darwin-arm64"},
			{Name: "muximux-windows-amd64.zip", BrowserDownloadURL: "https://example.com/windows-amd64"},
		}

		result := buildDownloadURLs(assets)

		expected := map[string]string{
			"linux_amd64":   "https://example.com/linux-amd64",
			"linux_arm64":   "https://example.com/linux-arm64",
			"darwin_amd64":  "https://example.com/darwin-amd64",
			"darwin_arm64":  "https://example.com/darwin-arm64",
			"windows_amd64": "https://example.com/windows-amd64",
		}

		for key, wantURL := range expected {
			if result[key] != wantURL {
				t.Errorf("result[%q] = %q, want %q", key, result[key], wantURL)
			}
		}
	})

	t.Run("matches underscored patterns", func(t *testing.T) {
		assets := []gitHubAsset{
			{Name: "muximux_linux_amd64.tar.gz", BrowserDownloadURL: "https://example.com/linux_amd64"},
		}

		result := buildDownloadURLs(assets)

		if result["linux_amd64"] != "https://example.com/linux_amd64" {
			t.Errorf("expected underscore pattern match, got %v", result)
		}
	})

	t.Run("case insensitive matching", func(t *testing.T) {
		assets := []gitHubAsset{
			{Name: "Muximux-Linux-AMD64.tar.gz", BrowserDownloadURL: "https://example.com/linux-amd64"},
		}

		result := buildDownloadURLs(assets)

		if result["linux_amd64"] != "https://example.com/linux-amd64" {
			t.Errorf("expected case-insensitive match, got %v", result)
		}
	})

	t.Run("no matching assets", func(t *testing.T) {
		assets := []gitHubAsset{
			{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums"},
			{Name: "source.tar.gz", BrowserDownloadURL: "https://example.com/source"},
		}

		result := buildDownloadURLs(assets)

		if len(result) != 0 {
			t.Errorf("expected empty map for non-matching assets, got %v", result)
		}
	})

	t.Run("partial platform coverage", func(t *testing.T) {
		assets := []gitHubAsset{
			{Name: "muximux-linux-amd64.tar.gz", BrowserDownloadURL: "https://example.com/linux-amd64"},
		}

		result := buildDownloadURLs(assets)

		if len(result) != 1 {
			t.Errorf("expected 1 entry, got %d", len(result))
		}
		if _, ok := result["darwin_arm64"]; ok {
			t.Error("expected darwin_arm64 to be absent")
		}
	})
}

// ---------------------------------------------------------------------------
// detectEnvironment  (partially testable pure function)
// ---------------------------------------------------------------------------

func TestDetectEnvironment(t *testing.T) {
	// We can only test the result is one of the two valid values.
	// In a normal test environment (not docker), this should return "native".
	// In CI/docker it might return "docker".
	env := detectEnvironment()
	if env != "native" && env != "docker" {
		t.Errorf("detectEnvironment() = %q, want %q or %q", env, "native", "docker")
	}
}

// ---------------------------------------------------------------------------
// GetInfo  (HTTP handler)
// ---------------------------------------------------------------------------

func TestGetInfo(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		h := NewSystemHandler("1.2.3", "abc1234", "2025-06-15", "/tmp/data")

		// GetInfo only returns the runtime / dataDir / uptime fields when
		// the caller is an admin; this test seeds an admin user in the
		// context so the full payload is exercised. The "non-admin
		// scrubbed" subtest below covers the alternate branch.
		req := adminCtxRequest(http.MethodGet, "/api/system/info")
		w := httptest.NewRecorder()

		h.GetInfo(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		ct := w.Header().Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", ct)
		}

		var resp SystemInfoResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Version != "1.2.3" {
			t.Errorf("expected version %q, got %q", "1.2.3", resp.Version)
		}
		if resp.Commit != "abc1234" {
			t.Errorf("expected commit %q, got %q", "abc1234", resp.Commit)
		}
		if resp.BuildDate != "2025-06-15" {
			t.Errorf("expected build_date %q, got %q", "2025-06-15", resp.BuildDate)
		}
		if resp.GoVersion != runtime.Version() {
			t.Errorf("expected go_version %q, got %q", runtime.Version(), resp.GoVersion)
		}
		if resp.OS != runtime.GOOS {
			t.Errorf("expected os %q, got %q", runtime.GOOS, resp.OS)
		}
		if resp.Arch != runtime.GOARCH {
			t.Errorf("expected arch %q, got %q", runtime.GOARCH, resp.Arch)
		}
		if resp.Environment != "native" && resp.Environment != "docker" {
			t.Errorf("unexpected environment %q", resp.Environment)
		}
		if resp.DataDir != "/tmp/data" {
			t.Errorf("expected data_dir %q, got %q", "/tmp/data", resp.DataDir)
		}
		if resp.UptimeSecs < 0 {
			t.Errorf("expected non-negative uptime_seconds, got %d", resp.UptimeSecs)
		}
		if resp.Uptime == "" {
			t.Error("expected non-empty uptime string")
		}
		if resp.StartedAt == "" {
			t.Error("expected non-empty started_at")
		}

		// Check links
		if resp.Links.GitHub != githubBaseURL {
			t.Errorf("expected GitHub link %q, got %q", githubBaseURL, resp.Links.GitHub)
		}
		if resp.Links.Issues != githubBaseURL+"/issues" {
			t.Errorf("expected Issues link %q, got %q", githubBaseURL+"/issues", resp.Links.Issues)
		}
		if resp.Links.Releases != githubBaseURL+"/releases" {
			t.Errorf("expected Releases link %q, got %q", githubBaseURL+"/releases", resp.Links.Releases)
		}
		if resp.Links.Wiki != githubBaseURL+"/wiki" {
			t.Errorf("expected Wiki link %q, got %q", githubBaseURL+"/wiki", resp.Links.Wiki)
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		h := NewSystemHandler("1.0.0", "", "", "")

		req := httptest.NewRequest(http.MethodPost, "/api/system/info", nil)
		w := httptest.NewRecorder()

		h.GetInfo(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})

	t.Run("PUT method rejected", func(t *testing.T) {
		h := NewSystemHandler("1.0.0", "", "", "")

		req := httptest.NewRequest(http.MethodPut, "/api/system/info", nil)
		w := httptest.NewRecorder()

		h.GetInfo(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})

	t.Run("non-admin scrubbed", func(t *testing.T) {
		// A request without an admin user in the context (the SPA
		// shell needs the version to render the "About" link, but
		// only admins should see data_dir, environment, uptime, etc.)
		h := NewSystemHandler("1.2.3", "abc1234", "2025-06-15", "/tmp/data")
		req := httptest.NewRequest(http.MethodGet, "/api/system/info", nil)
		w := httptest.NewRecorder()
		h.GetInfo(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var resp SystemInfoResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Version != "1.2.3" || resp.Commit != "abc1234" || resp.BuildDate != "2025-06-15" {
			t.Errorf("public fields missing: %+v", resp)
		}
		if resp.DataDir != "" || resp.Environment != "" || resp.GoVersion != "" || resp.OS != "" || resp.Arch != "" {
			t.Errorf("non-admin response leaked private fields: %+v", resp)
		}
		if resp.UptimeSecs != 0 || resp.Uptime != "" || resp.StartedAt != "" {
			t.Errorf("non-admin response leaked uptime info: %+v", resp)
		}
	})
}

// ---------------------------------------------------------------------------
// CheckUpdate  (HTTP handler — uses a mock GitHub API server)
// ---------------------------------------------------------------------------

func TestCheckUpdate(t *testing.T) {
	t.Run("wrong method", func(t *testing.T) {
		h := NewSystemHandler("1.0.0", "", "", "")

		req := httptest.NewRequest(http.MethodPost, "/api/system/updates", nil)
		w := httptest.NewRecorder()

		h.CheckUpdate(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})

	t.Run("update available", func(t *testing.T) {
		// Mock GitHub API
		ghServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			release := gitHubRelease{
				TagName:     "v2.0.0",
				HTMLURL:     "https://github.com/mescon/Muximux/releases/tag/v2.0.0",
				Body:        "## Changelog\n- New feature",
				PublishedAt: "2025-06-15T00:00:00Z",
				Assets: []gitHubAsset{
					{Name: "muximux-linux-amd64.tar.gz", BrowserDownloadURL: "https://example.com/linux-amd64"},
					{Name: "muximux-darwin-arm64.tar.gz", BrowserDownloadURL: "https://example.com/darwin-arm64"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(release)
		}))
		defer ghServer.Close()

		h := NewSystemHandler("1.0.0", "", "", "")
		// We need to test with the real CheckUpdate which calls the real GitHub API.
		// Instead, we create a wrapper handler that talks to our mock server.
		w, resp := checkUpdateWithMockGitHub(t, h, ghServer.URL)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		if !resp.UpdateAvailable {
			t.Error("expected update_available to be true")
		}
		if resp.CurrentVersion != "1.0.0" {
			t.Errorf("expected current_version %q, got %q", "1.0.0", resp.CurrentVersion)
		}
		if resp.LatestVersion != "2.0.0" {
			t.Errorf("expected latest_version %q, got %q", "2.0.0", resp.LatestVersion)
		}
		if resp.ReleaseURL != "https://github.com/mescon/Muximux/releases/tag/v2.0.0" {
			t.Errorf("unexpected release_url %q", resp.ReleaseURL)
		}
		if resp.Changelog != "## Changelog\n- New feature" {
			t.Errorf("unexpected changelog %q", resp.Changelog)
		}
		if resp.PublishedAt != "2025-06-15T00:00:00Z" {
			t.Errorf("unexpected published_at %q", resp.PublishedAt)
		}
		if resp.DownloadURLs["linux_amd64"] != "https://example.com/linux-amd64" {
			t.Errorf("expected linux_amd64 download URL, got %v", resp.DownloadURLs)
		}
		if resp.DownloadURLs["darwin_arm64"] != "https://example.com/darwin-arm64" {
			t.Errorf("expected darwin_arm64 download URL, got %v", resp.DownloadURLs)
		}
	})

	t.Run("already up to date", func(t *testing.T) {
		ghServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			release := gitHubRelease{
				TagName:     "v1.0.0",
				HTMLURL:     "https://github.com/mescon/Muximux/releases/tag/v1.0.0",
				Body:        "Initial release",
				PublishedAt: "2025-01-01T00:00:00Z",
				Assets:      []gitHubAsset{},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(release)
		}))
		defer ghServer.Close()

		h := NewSystemHandler("1.0.0", "", "", "")
		w, resp := checkUpdateWithMockGitHub(t, h, ghServer.URL)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		if resp.UpdateAvailable {
			t.Error("expected update_available to be false")
		}
		if resp.LatestVersion != "1.0.0" {
			t.Errorf("expected latest_version %q, got %q", "1.0.0", resp.LatestVersion)
		}
	})

	t.Run("dev version always sees update", func(t *testing.T) {
		ghServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			release := gitHubRelease{
				TagName: "v0.0.1",
				HTMLURL: "https://github.com/mescon/Muximux/releases/tag/v0.0.1",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(release)
		}))
		defer ghServer.Close()

		h := NewSystemHandler("dev", "", "", "")
		w, resp := checkUpdateWithMockGitHub(t, h, ghServer.URL)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		if !resp.UpdateAvailable {
			t.Error("expected dev version to see update available")
		}
	})

	t.Run("GitHub 404 no releases", func(t *testing.T) {
		ghServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintln(w, `{"message":"Not Found"}`)
		}))
		defer ghServer.Close()

		h := NewSystemHandler("1.0.0", "", "", "")
		w, resp := checkUpdateWithMockGitHub(t, h, ghServer.URL)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200 for no releases, got %d: %s", w.Code, w.Body.String())
		}

		if resp.UpdateAvailable {
			t.Error("expected update_available to be false when no releases exist")
		}
		if resp.CurrentVersion != "1.0.0" {
			t.Errorf("expected current_version %q, got %q", "1.0.0", resp.CurrentVersion)
		}
		if resp.LatestVersion != "1.0.0" {
			t.Errorf("expected latest_version to equal current when no releases, got %q", resp.LatestVersion)
		}
	})

	t.Run("GitHub API error", func(t *testing.T) {
		ghServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintln(w, `{"message":"rate limited"}`)
		}))
		defer ghServer.Close()

		h := NewSystemHandler("1.0.0", "", "", "")
		w, _ := checkUpdateWithMockGitHub(t, h, ghServer.URL)

		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status 503 for GitHub API error, got %d", w.Code)
		}
	})

	t.Run("GitHub returns invalid JSON", func(t *testing.T) {
		ghServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{invalid json`)
		}))
		defer ghServer.Close()

		h := NewSystemHandler("1.0.0", "", "", "")
		w, _ := checkUpdateWithMockGitHub(t, h, ghServer.URL)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500 for invalid JSON, got %d", w.Code)
		}
	})

	t.Run("response is cached for cacheTTL", func(t *testing.T) {
		// Regression: CheckUpdate must serve from in-process cache so a
		// busy Settings page does not exhaust GitHub's 60-req/hour
		// unauthenticated rate limit. We count fetches against the mock.
		var fetches int32
		ghServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&fetches, 1)
			release := gitHubRelease{TagName: "v2.0.0", HTMLURL: "x", Body: "y", PublishedAt: "z", Assets: nil}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(release)
		}))
		defer ghServer.Close()

		h := NewSystemHandler("1.0.0", "", "", "")
		h.releaseAPIURL = ghServer.URL
		h.cacheTTL = time.Hour

		// First call: cache miss, must hit upstream.
		req := httptest.NewRequest(http.MethodGet, "/api/system/updates", nil)
		h.CheckUpdate(httptest.NewRecorder(), req)
		// Second + third calls: cache hits, must NOT hit upstream.
		h.CheckUpdate(httptest.NewRecorder(), req)
		h.CheckUpdate(httptest.NewRecorder(), req)

		if got := atomic.LoadInt32(&fetches); got != 1 {
			t.Errorf("expected 1 outbound fetch with cache hot, got %d", got)
		}

		// Force expiry: next call should re-fetch.
		h.cachedAt = time.Now().Add(-2 * time.Hour)
		h.CheckUpdate(httptest.NewRecorder(), req)
		if got := atomic.LoadInt32(&fetches); got != 2 {
			t.Errorf("expected 2 outbound fetches after cache expiry, got %d", got)
		}
	})
}

// checkUpdateWithMockGitHub is a test helper that exercises the CheckUpdate logic
// against a mock GitHub API server. Since CheckUpdate hard-codes the GitHub API URL,
// we create a lightweight handler that proxies through our mock server instead.
func checkUpdateWithMockGitHub(t *testing.T, h *SystemHandler, mockURL string) (*httptest.ResponseRecorder, UpdateCheckResponse) {
	t.Helper()

	// Point the handler at the mock GitHub server and reset its cache
	// so each test sees a fresh fetch.
	h.releaseAPIURL = mockURL
	h.cachedUpdate = nil
	h.cachedAt = time.Time{}

	req := httptest.NewRequest(http.MethodGet, "/api/system/updates", nil)
	w := httptest.NewRecorder()
	h.CheckUpdate(w, req)

	var resp UpdateCheckResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	return w, resp
}
