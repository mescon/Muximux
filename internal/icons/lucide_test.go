package icons

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewLucideClient(t *testing.T) {
	dir := t.TempDir()
	client := NewLucideClient(dir, 1*time.Hour)

	if client.cacheDir != dir {
		t.Errorf("expected cacheDir %q, got %q", dir, client.cacheDir)
	}
	if client.cacheTTL != 1*time.Hour {
		t.Errorf("expected cacheTTL 1h, got %v", client.cacheTTL)
	}
	if client.httpClient == nil {
		t.Error("expected httpClient to be set")
	}
}

func TestLucideClient_GetIcon_FromCDN(t *testing.T) {
	svgContent := `<svg xmlns="http://www.w3.org/2000/svg"><circle r="10"/></svg>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write([]byte(svgContent))
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	client := NewLucideClient(cacheDir, 1*time.Hour)
	client.httpClient = server.Client()

	// Override the CDN URL by making the client use the test server
	// We need to override the downloadIcon to use our test server
	// Instead, we'll test by setting the httpClient and then manually calling
	// with a name that maps to our server URL

	// Actually, the client uses a hardcoded URL format. Let's test the cache path instead
	// and test downloadIcon indirectly by overriding httpClient transport.

	// Use a custom round-tripper to intercept requests
	client.httpClient.Transport = &testTransport{
		handler: func(req *http.Request) (*http.Response, error) {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", "image/svg+xml")
			w.Write([]byte(svgContent))
			return w.Result(), nil
		},
	}

	data, contentType, err := client.GetIcon("test-icon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if contentType != "image/svg+xml" {
		t.Errorf("expected content type 'image/svg+xml', got %q", contentType)
	}
	if string(data) != svgContent {
		t.Errorf("expected SVG content %q, got %q", svgContent, string(data))
	}

	// Verify it was cached
	cachedData, err := os.ReadFile(filepath.Join(cacheDir, "test-icon.svg"))
	if err != nil {
		t.Fatalf("expected icon to be cached: %v", err)
	}
	if string(cachedData) != svgContent {
		t.Errorf("cached data mismatch")
	}
}

func TestLucideClient_GetIcon_FromCache(t *testing.T) {
	cacheDir := t.TempDir()
	svgContent := `<svg>cached</svg>`

	// Pre-populate cache
	err := os.WriteFile(filepath.Join(cacheDir, "cached-icon.svg"), []byte(svgContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	client := NewLucideClient(cacheDir, 1*time.Hour)

	data, contentType, err := client.GetIcon("cached-icon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if contentType != "image/svg+xml" {
		t.Errorf("expected content type 'image/svg+xml', got %q", contentType)
	}
	if string(data) != svgContent {
		t.Errorf("expected %q, got %q", svgContent, string(data))
	}
}

func TestLucideClient_GetIcon_StripsSvgExtension(t *testing.T) {
	cacheDir := t.TempDir()
	svgContent := `<svg>ext-test</svg>`

	err := os.WriteFile(filepath.Join(cacheDir, "myicon.svg"), []byte(svgContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	client := NewLucideClient(cacheDir, 1*time.Hour)

	// Request with .svg extension should still work
	data, _, err := client.GetIcon("myicon.svg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != svgContent {
		t.Errorf("expected %q, got %q", svgContent, string(data))
	}
}

func TestLucideClient_GetIcon_CacheExpired(t *testing.T) {
	cacheDir := t.TempDir()
	svgContent := `<svg>old</svg>`
	freshContent := `<svg>fresh</svg>`

	// Write cached file
	cachePath := filepath.Join(cacheDir, "expiring.svg")
	err := os.WriteFile(cachePath, []byte(svgContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Set modification time to the past
	pastTime := time.Now().Add(-2 * time.Hour)
	if err = os.Chtimes(cachePath, pastTime, pastTime); err != nil {
		t.Fatal(err)
	}

	client := NewLucideClient(cacheDir, 1*time.Hour)
	client.httpClient = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				w.Header().Set("Content-Type", "image/svg+xml")
				w.Write([]byte(freshContent))
				return w.Result(), nil
			},
		},
	}

	data, _, err := client.GetIcon("expiring")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != freshContent {
		t.Errorf("expected fresh content %q, got %q", freshContent, string(data))
	}
}

func TestLucideClient_GetIcon_CDNNotFound(t *testing.T) {
	cacheDir := t.TempDir()
	client := NewLucideClient(cacheDir, 1*time.Hour)
	client.httpClient = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				w.WriteHeader(http.StatusNotFound)
				return w.Result(), nil
			},
		},
	}

	_, _, err := client.GetIcon("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent icon")
	}
}

func TestLucideClient_GetIcon_CDNError(t *testing.T) {
	cacheDir := t.TempDir()
	client := NewLucideClient(cacheDir, 1*time.Hour)
	client.httpClient = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				return nil, http.ErrHandlerTimeout
			},
		},
	}

	_, _, err := client.GetIcon("error-icon")
	if err == nil {
		t.Error("expected error for HTTP failure")
	}
}

func TestLucideClient_ListIcons(t *testing.T) {
	treeResponse := map[string]interface{}{
		"tree": []map[string]string{
			{"path": "icons/home.svg", "type": "blob"},
			{"path": "icons/star.svg", "type": "blob"},
			{"path": "icons/search.svg", "type": "blob"},
			{"path": "README.md", "type": "blob"},
			{"path": "icons", "type": "tree"},
			{"path": "icons/subfolder", "type": "tree"},
		},
	}

	categoriesResponse := map[string][]string{
		"navigation": {"home"},
		"social":     {"star"},
		"general":    {"home", "search"},
	}

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		// Differentiate based on URL path
		if r.URL.Path == "/repos/lucide-icons/lucide/git/trees/main" {
			json.NewEncoder(w).Encode(treeResponse)
		} else {
			json.NewEncoder(w).Encode(categoriesResponse)
		}
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	client := NewLucideClient(cacheDir, 1*time.Hour)
	client.httpClient = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				w.Header().Set("Content-Type", "application/json")

				if req.URL.Path == "/repos/lucide-icons/lucide/git/trees/main" {
					json.NewEncoder(w).Encode(treeResponse)
				} else {
					json.NewEncoder(w).Encode(categoriesResponse)
				}
				return w.Result(), nil
			},
		},
	}

	icons, err := client.ListIcons()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(icons) != 3 {
		t.Fatalf("expected 3 icons, got %d", len(icons))
	}

	// Icons should be sorted
	if icons[0].Name != "home" {
		t.Errorf("expected first icon 'home', got %q", icons[0].Name)
	}
	if icons[1].Name != "search" {
		t.Errorf("expected second icon 'search', got %q", icons[1].Name)
	}
	if icons[2].Name != "star" {
		t.Errorf("expected third icon 'star', got %q", icons[2].Name)
	}

	// Check categories
	homeCats := icons[0].Categories
	if len(homeCats) != 2 {
		t.Errorf("expected 2 categories for 'home', got %d", len(homeCats))
	}
}

func TestLucideClient_ListIcons_Cached(t *testing.T) {
	cacheDir := t.TempDir()
	client := NewLucideClient(cacheDir, 1*time.Hour)

	// Pre-populate the in-memory cache
	client.iconList = []LucideIconInfo{
		{Name: "cached-icon", Categories: []string{"test"}},
	}
	client.listLoaded = time.Now()

	icons, err := client.ListIcons()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(icons) != 1 {
		t.Errorf("expected 1 cached icon, got %d", len(icons))
	}
	if icons[0].Name != "cached-icon" {
		t.Errorf("expected 'cached-icon', got %q", icons[0].Name)
	}
}

func TestLucideClient_ListIcons_CacheExpired(t *testing.T) {
	treeResponse := map[string]interface{}{
		"tree": []map[string]string{
			{"path": "icons/fresh.svg", "type": "blob"},
		},
	}
	categoriesResponse := map[string][]string{}

	cacheDir := t.TempDir()
	client := NewLucideClient(cacheDir, 1*time.Millisecond) // Very short TTL
	client.httpClient = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				w.Header().Set("Content-Type", "application/json")
				if req.URL.Path == "/repos/lucide-icons/lucide/git/trees/main" {
					json.NewEncoder(w).Encode(treeResponse)
				} else {
					json.NewEncoder(w).Encode(categoriesResponse)
				}
				return w.Result(), nil
			},
		},
	}

	// Pre-populate stale cache
	client.iconList = []LucideIconInfo{{Name: "stale"}}
	client.listLoaded = time.Now().Add(-1 * time.Hour)

	icons, err := client.ListIcons()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(icons) != 1 || icons[0].Name != "fresh" {
		t.Errorf("expected fresh icon from API, got %v", icons)
	}
}

func TestLucideClient_ListIcons_TreeError(t *testing.T) {
	cacheDir := t.TempDir()
	client := NewLucideClient(cacheDir, 1*time.Hour)
	client.httpClient = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				w.WriteHeader(http.StatusInternalServerError)
				return w.Result(), nil
			},
		},
	}

	_, err := client.ListIcons()
	if err == nil {
		t.Error("expected error when tree fetch fails")
	}
}

func TestLucideClient_SearchIcons(t *testing.T) {
	cacheDir := t.TempDir()
	client := NewLucideClient(cacheDir, 1*time.Hour)

	// Pre-populate cache
	client.iconList = []LucideIconInfo{
		{Name: "home", Categories: []string{"navigation"}},
		{Name: "search", Categories: []string{"general"}},
		{Name: "star", Categories: []string{"social"}},
		{Name: "heart", Categories: []string{"social"}},
	}
	client.listLoaded = time.Now()

	t.Run("search by name", func(t *testing.T) {
		results, err := client.SearchIcons("home")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1 result, got %d", len(results))
		}
		if results[0].Name != "home" {
			t.Errorf("expected 'home', got %q", results[0].Name)
		}
	})

	t.Run("search by category", func(t *testing.T) {
		results, err := client.SearchIcons("social")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("expected 2 results for 'social' category, got %d", len(results))
		}
	})

	t.Run("search case insensitive", func(t *testing.T) {
		results, err := client.SearchIcons("SEARCH")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1 result, got %d", len(results))
		}
	})

	t.Run("search no results", func(t *testing.T) {
		results, err := client.SearchIcons("zzzzz")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("search partial name match", func(t *testing.T) {
		results, err := client.SearchIcons("ear")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// "search" and "heart" both contain "ear"
		if len(results) != 2 {
			t.Errorf("expected 2 results for 'ear', got %d", len(results))
		}
	})
}

func TestLucideClient_SearchIcons_FetchError(t *testing.T) {
	cacheDir := t.TempDir()
	client := NewLucideClient(cacheDir, 1*time.Hour)
	client.httpClient = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				return nil, http.ErrHandlerTimeout
			},
		},
	}

	_, err := client.SearchIcons("test")
	if err == nil {
		t.Error("expected error when list fetch fails")
	}
}

func TestLucideClient_getCachePath(t *testing.T) {
	client := NewLucideClient("/cache/dir", 1*time.Hour)
	path := client.getCachePath("test-icon")
	expected := filepath.Join("/cache/dir", "test-icon.svg")
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestLucideClient_saveToCache(t *testing.T) {
	cacheDir := t.TempDir()
	client := NewLucideClient(cacheDir, 1*time.Hour)

	err := client.saveToCache("save-test", []byte("<svg>test</svg>"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cacheDir, "save-test.svg"))
	if err != nil {
		t.Fatalf("failed to read cached file: %v", err)
	}
	if string(data) != "<svg>test</svg>" {
		t.Errorf("expected '<svg>test</svg>', got %q", string(data))
	}
}

func TestLucideClient_saveToCache_CreatesDir(t *testing.T) {
	baseDir := t.TempDir()
	cacheDir := filepath.Join(baseDir, "nested", "cache")
	client := NewLucideClient(cacheDir, 1*time.Hour)

	err := client.saveToCache("nested-test", []byte("<svg/>"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = os.Stat(filepath.Join(cacheDir, "nested-test.svg"))
	if err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
}

func TestLucideClient_getFromCache_NotExist(t *testing.T) {
	cacheDir := t.TempDir()
	client := NewLucideClient(cacheDir, 1*time.Hour)

	_, err := client.getFromCache("does-not-exist")
	if err == nil {
		t.Error("expected error for non-existent cache entry")
	}
}

func TestLucideClient_getFromCache_ZeroTTL(t *testing.T) {
	cacheDir := t.TempDir()
	content := `<svg>no-expire</svg>`

	// Write a cache file with old modification time
	cachePath := filepath.Join(cacheDir, "old-icon.svg")
	err := os.WriteFile(cachePath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}
	pastTime := time.Now().Add(-100 * time.Hour)
	if err = os.Chtimes(cachePath, pastTime, pastTime); err != nil {
		t.Fatal(err)
	}

	// With zero TTL, cache never expires
	client := NewLucideClient(cacheDir, 0)

	data, err := client.getFromCache("old-icon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != content {
		t.Errorf("expected %q, got %q", content, string(data))
	}
}

func TestLucideClient_fetchCategories_Error(t *testing.T) {
	cacheDir := t.TempDir()
	client := NewLucideClient(cacheDir, 1*time.Hour)
	client.httpClient = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				w.WriteHeader(http.StatusInternalServerError)
				return w.Result(), nil
			},
		},
	}

	_, err := client.fetchCategories()
	if err == nil {
		t.Error("expected error for failed categories fetch")
	}
}

func TestLucideClient_fetchCategories_InvalidJSON(t *testing.T) {
	cacheDir := t.TempDir()
	client := NewLucideClient(cacheDir, 1*time.Hour)
	client.httpClient = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("not json"))
				return w.Result(), nil
			},
		},
	}

	_, err := client.fetchCategories()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLucideClient_fetchTreeNames_InvalidJSON(t *testing.T) {
	cacheDir := t.TempDir()
	client := NewLucideClient(cacheDir, 1*time.Hour)
	client.httpClient = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("not json"))
				return w.Result(), nil
			},
		},
	}

	_, err := client.fetchTreeNames()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLucideClient_fetchTreeNames_StatusError(t *testing.T) {
	cacheDir := t.TempDir()
	client := NewLucideClient(cacheDir, 1*time.Hour)
	client.httpClient = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				w.WriteHeader(http.StatusForbidden)
				return w.Result(), nil
			},
		},
	}

	_, err := client.fetchTreeNames()
	if err == nil {
		t.Error("expected error for non-200 status")
	}
}

// testTransport is a custom http.RoundTripper for testing
type testTransport struct {
	handler func(req *http.Request) (*http.Response, error)
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.handler(req)
}
