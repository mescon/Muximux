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

func TestNewDashboardIconsClient(t *testing.T) {
	dir := t.TempDir()
	client := NewDashboardIconsClient(dir, 2*time.Hour)

	if client.cacheDir != dir {
		t.Errorf("expected cacheDir %q, got %q", dir, client.cacheDir)
	}
	if client.cacheTTL != 2*time.Hour {
		t.Errorf("expected cacheTTL 2h, got %v", client.cacheTTL)
	}
	if client.httpClient == nil {
		t.Error("expected httpClient to be set")
	}
}

func TestDashboardClient_GetIcon_FromCDN(t *testing.T) {
	svgContent := `<svg>dashboard</svg>`

	cacheDir := t.TempDir()
	client := NewDashboardIconsClient(cacheDir, 1*time.Hour)
	client.httpClient = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				w.Write([]byte(svgContent))
				return w.Result(), nil
			},
		},
	}

	data, contentType, err := client.GetIcon("plex", "svg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if contentType != "image/svg+xml" {
		t.Errorf("expected 'image/svg+xml', got %q", contentType)
	}
	if string(data) != svgContent {
		t.Errorf("expected %q, got %q", svgContent, string(data))
	}

	// Verify cached
	cached, err := os.ReadFile(filepath.Join(cacheDir, "plex.svg"))
	if err != nil {
		t.Fatalf("expected file to be cached: %v", err)
	}
	if string(cached) != svgContent {
		t.Errorf("cached data mismatch")
	}
}

func TestDashboardClient_GetIcon_DefaultVariant(t *testing.T) {
	cacheDir := t.TempDir()

	// Pre-populate cache
	err := os.WriteFile(filepath.Join(cacheDir, "myapp.svg"), []byte("<svg>default</svg>"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	client := NewDashboardIconsClient(cacheDir, 1*time.Hour)

	data, contentType, err := client.GetIcon("myapp", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if contentType != "image/svg+xml" {
		t.Errorf("expected 'image/svg+xml', got %q", contentType)
	}
	if string(data) != "<svg>default</svg>" {
		t.Errorf("unexpected data: %q", string(data))
	}
}

func TestDashboardClient_GetIcon_PNGVariant(t *testing.T) {
	pngContent := []byte("PNG_DATA")

	cacheDir := t.TempDir()
	client := NewDashboardIconsClient(cacheDir, 1*time.Hour)
	client.httpClient = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				w.Write(pngContent)
				return w.Result(), nil
			},
		},
	}

	data, contentType, err := client.GetIcon("icon", "png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if contentType != "image/png" {
		t.Errorf("expected 'image/png', got %q", contentType)
	}
	if string(data) != string(pngContent) {
		t.Errorf("data mismatch")
	}
}

func TestDashboardClient_GetIcon_WebPVariant(t *testing.T) {
	cacheDir := t.TempDir()
	client := NewDashboardIconsClient(cacheDir, 1*time.Hour)
	client.httpClient = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				w.Write([]byte("WEBP_DATA"))
				return w.Result(), nil
			},
		},
	}

	_, contentType, err := client.GetIcon("icon", "webp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if contentType != "image/webp" {
		t.Errorf("expected 'image/webp', got %q", contentType)
	}
}

func TestDashboardClient_GetIcon_UnknownVariant(t *testing.T) {
	cacheDir := t.TempDir()
	client := NewDashboardIconsClient(cacheDir, 1*time.Hour)
	client.httpClient = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				w.Write([]byte("<svg>unknown</svg>"))
				return w.Result(), nil
			},
		},
	}

	// Unknown variant defaults to svg
	_, contentType, err := client.GetIcon("icon", "bmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if contentType != "image/svg+xml" {
		t.Errorf("expected 'image/svg+xml' for unknown variant, got %q", contentType)
	}
}

func TestDashboardClient_GetIcon_FromCache(t *testing.T) {
	cacheDir := t.TempDir()
	content := `<svg>cached-dashboard</svg>`

	err := os.WriteFile(filepath.Join(cacheDir, "cached.svg"), []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	client := NewDashboardIconsClient(cacheDir, 1*time.Hour)

	data, contentType, err := client.GetIcon("cached", "svg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if contentType != "image/svg+xml" {
		t.Errorf("expected 'image/svg+xml', got %q", contentType)
	}
	if string(data) != content {
		t.Errorf("expected %q, got %q", content, string(data))
	}
}

func TestDashboardClient_GetIcon_CacheExpired(t *testing.T) {
	cacheDir := t.TempDir()
	freshContent := `<svg>fresh</svg>`

	// Write old cache
	cachePath := filepath.Join(cacheDir, "expiring.svg")
	err := os.WriteFile(cachePath, []byte("<svg>old</svg>"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	pastTime := time.Now().Add(-2 * time.Hour)
	if err = os.Chtimes(cachePath, pastTime, pastTime); err != nil {
		t.Fatal(err)
	}

	client := NewDashboardIconsClient(cacheDir, 1*time.Hour)
	client.httpClient = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				w.Write([]byte(freshContent))
				return w.Result(), nil
			},
		},
	}

	data, _, err := client.GetIcon("expiring", "svg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != freshContent {
		t.Errorf("expected fresh content, got %q", string(data))
	}
}

func TestDashboardClient_GetIcon_NotFound(t *testing.T) {
	cacheDir := t.TempDir()
	client := NewDashboardIconsClient(cacheDir, 1*time.Hour)
	client.httpClient = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				w.WriteHeader(http.StatusNotFound)
				return w.Result(), nil
			},
		},
	}

	_, _, err := client.GetIcon("nonexistent", "svg")
	if err == nil {
		t.Error("expected error for non-existent icon")
	}
}

func TestDashboardClient_GetIcon_HTTPError(t *testing.T) {
	cacheDir := t.TempDir()
	client := NewDashboardIconsClient(cacheDir, 1*time.Hour)
	client.httpClient = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				return nil, http.ErrHandlerTimeout
			},
		},
	}

	_, _, err := client.GetIcon("error", "svg")
	if err == nil {
		t.Error("expected error for HTTP failure")
	}
}

func TestDashboardClient_GetIconPath(t *testing.T) {
	t.Run("cached", func(t *testing.T) {
		cacheDir := t.TempDir()
		err := os.WriteFile(filepath.Join(cacheDir, "myicon.svg"), []byte("<svg/>"), 0644)
		if err != nil {
			t.Fatal(err)
		}

		client := NewDashboardIconsClient(cacheDir, 1*time.Hour)

		path, err := client.GetIconPath("myicon", "svg")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := filepath.Join(cacheDir, "myicon.svg")
		if path != expected {
			t.Errorf("expected %q, got %q", expected, path)
		}
	})

	t.Run("default variant", func(t *testing.T) {
		cacheDir := t.TempDir()
		err := os.WriteFile(filepath.Join(cacheDir, "myicon.svg"), []byte("<svg/>"), 0644)
		if err != nil {
			t.Fatal(err)
		}

		client := NewDashboardIconsClient(cacheDir, 1*time.Hour)

		path, err := client.GetIconPath("myicon", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := filepath.Join(cacheDir, "myicon.svg")
		if path != expected {
			t.Errorf("expected %q, got %q", expected, path)
		}
	})

	t.Run("downloads if not cached", func(t *testing.T) {
		cacheDir := t.TempDir()
		client := NewDashboardIconsClient(cacheDir, 1*time.Hour)
		client.httpClient = &http.Client{
			Transport: &testTransport{
				handler: func(req *http.Request) (*http.Response, error) {
					w := httptest.NewRecorder()
					w.Write([]byte("<svg>downloaded</svg>"))
					return w.Result(), nil
				},
			},
		}

		path, err := client.GetIconPath("download-me", "svg")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := filepath.Join(cacheDir, "download-me.svg")
		if path != expected {
			t.Errorf("expected %q, got %q", expected, path)
		}
	})

	t.Run("download error", func(t *testing.T) {
		cacheDir := t.TempDir()
		client := NewDashboardIconsClient(cacheDir, 1*time.Hour)
		client.httpClient = &http.Client{
			Transport: &testTransport{
				handler: func(req *http.Request) (*http.Response, error) {
					w := httptest.NewRecorder()
					w.WriteHeader(http.StatusNotFound)
					return w.Result(), nil
				},
			},
		}

		_, err := client.GetIconPath("missing", "svg")
		if err == nil {
			t.Error("expected error for download failure")
		}
	})
}

func TestDashboardClient_ListIcons(t *testing.T) {
	treeResponse := map[string]interface{}{
		"tree": []map[string]string{
			{"path": "svg/plex.svg", "type": "blob"},
			{"path": "svg/sonarr.svg", "type": "blob"},
			{"path": "png/plex.png", "type": "blob"},
			{"path": "README.md", "type": "blob"},
			{"path": "svg", "type": "tree"},
		},
	}

	cacheDir := t.TempDir()
	client := NewDashboardIconsClient(cacheDir, 1*time.Hour)
	client.httpClient = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(treeResponse)
				return w.Result(), nil
			},
		},
	}

	icons, err := client.ListIcons()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(icons) != 2 {
		t.Fatalf("expected 2 icons, got %d", len(icons))
	}

	// Should be sorted
	if icons[0].Name != "plex" {
		t.Errorf("expected first icon 'plex', got %q", icons[0].Name)
	}
	if icons[1].Name != "sonarr" {
		t.Errorf("expected second icon 'sonarr', got %q", icons[1].Name)
	}

	// All should have svg, png, webp variants
	for _, icon := range icons {
		if len(icon.Variants) != 3 {
			t.Errorf("expected 3 variants for %q, got %d", icon.Name, len(icon.Variants))
		}
	}
}

func TestDashboardClient_ListIcons_Cached(t *testing.T) {
	cacheDir := t.TempDir()
	client := NewDashboardIconsClient(cacheDir, 1*time.Hour)

	client.iconList = []IconInfo{
		{Name: "cached-app", Variants: []string{"svg"}},
	}
	client.listLoaded = time.Now()

	icons, err := client.ListIcons()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(icons) != 1 {
		t.Errorf("expected 1 icon, got %d", len(icons))
	}
}

func TestDashboardClient_ListIcons_CacheExpired(t *testing.T) {
	treeResponse := map[string]interface{}{
		"tree": []map[string]string{
			{"path": "svg/new.svg", "type": "blob"},
		},
	}

	cacheDir := t.TempDir()
	client := NewDashboardIconsClient(cacheDir, 1*time.Millisecond)
	client.httpClient = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(treeResponse)
				return w.Result(), nil
			},
		},
	}

	client.iconList = []IconInfo{{Name: "stale"}}
	client.listLoaded = time.Now().Add(-1 * time.Hour)

	icons, err := client.ListIcons()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(icons) != 1 || icons[0].Name != "new" {
		t.Errorf("expected fresh icon, got %v", icons)
	}
}

func TestDashboardClient_ListIcons_FetchError(t *testing.T) {
	cacheDir := t.TempDir()
	client := NewDashboardIconsClient(cacheDir, 1*time.Hour)
	client.httpClient = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				w.WriteHeader(http.StatusForbidden)
				return w.Result(), nil
			},
		},
	}

	_, err := client.ListIcons()
	if err == nil {
		t.Error("expected error when fetch fails")
	}
}

func TestDashboardClient_ListIcons_InvalidJSON(t *testing.T) {
	cacheDir := t.TempDir()
	client := NewDashboardIconsClient(cacheDir, 1*time.Hour)
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

	_, err := client.ListIcons()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDashboardClient_SearchIcons(t *testing.T) {
	cacheDir := t.TempDir()
	client := NewDashboardIconsClient(cacheDir, 1*time.Hour)

	client.iconList = []IconInfo{
		{Name: "plex", Variants: []string{"svg"}},
		{Name: "sonarr", Variants: []string{"svg"}},
		{Name: "radarr", Variants: []string{"svg"}},
		{Name: "plexdrive", Variants: []string{"svg"}},
	}
	client.listLoaded = time.Now()

	t.Run("matching", func(t *testing.T) {
		results, err := client.SearchIcons("plex")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("expected 2 results, got %d", len(results))
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		results, err := client.SearchIcons("SONARR")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1 result, got %d", len(results))
		}
	})

	t.Run("no results", func(t *testing.T) {
		results, err := client.SearchIcons("zzzzz")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("partial match", func(t *testing.T) {
		results, err := client.SearchIcons("arr")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("expected 2 results for 'arr', got %d", len(results))
		}
	})
}

func TestDashboardClient_SearchIcons_FetchError(t *testing.T) {
	cacheDir := t.TempDir()
	client := NewDashboardIconsClient(cacheDir, 1*time.Hour)
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

func TestDashboardClient_ClearCache(t *testing.T) {
	cacheDir := t.TempDir()

	// Write some files
	err := os.WriteFile(filepath.Join(cacheDir, "icon1.svg"), []byte("<svg/>"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(cacheDir, "icon2.png"), []byte("PNG"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	client := NewDashboardIconsClient(cacheDir, 1*time.Hour)

	err = client.ClearCache()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Directory should be removed
	_, err = os.Stat(cacheDir)
	if !os.IsNotExist(err) {
		t.Error("expected cache directory to be removed")
	}
}

func TestGetContentType(t *testing.T) {
	tests := []struct {
		variant  string
		expected string
	}{
		{"svg", "image/svg+xml"},
		{"png", "image/png"},
		{"webp", "image/webp"},
		{"unknown", "application/octet-stream"},
		{"", "application/octet-stream"},
		{"gif", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.variant, func(t *testing.T) {
			result := getContentType(tt.variant)
			if result != tt.expected {
				t.Errorf("getContentType(%q) = %q, expected %q", tt.variant, result, tt.expected)
			}
		})
	}
}

func TestSortIconInfoByName(t *testing.T) {
	icons := []IconInfo{
		{Name: "zebra"},
		{Name: "apple"},
		{Name: "mango"},
	}

	sortIconInfoByName(icons)

	if icons[0].Name != "apple" {
		t.Errorf("expected first 'apple', got %q", icons[0].Name)
	}
	if icons[1].Name != "mango" {
		t.Errorf("expected second 'mango', got %q", icons[1].Name)
	}
	if icons[2].Name != "zebra" {
		t.Errorf("expected third 'zebra', got %q", icons[2].Name)
	}
}

func TestDashboardClient_getCachePath(t *testing.T) {
	client := NewDashboardIconsClient("/cache", 1*time.Hour)

	tests := []struct {
		name, variant, expected string
	}{
		{"icon", "svg", filepath.Join("/cache", "icon.svg")},
		{"icon", "png", filepath.Join("/cache", "icon.png")},
		{"icon", "webp", filepath.Join("/cache", "icon.webp")},
	}

	for _, tt := range tests {
		path := client.getCachePath(tt.name, tt.variant)
		if path != tt.expected {
			t.Errorf("getCachePath(%q, %q) = %q, expected %q", tt.name, tt.variant, path, tt.expected)
		}
	}
}

func TestDashboardClient_getFromCache_ZeroTTL(t *testing.T) {
	cacheDir := t.TempDir()
	content := "<svg>no-expire</svg>"

	cachePath := filepath.Join(cacheDir, "old.svg")
	err := os.WriteFile(cachePath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}
	pastTime := time.Now().Add(-100 * time.Hour)
	if err = os.Chtimes(cachePath, pastTime, pastTime); err != nil {
		t.Fatal(err)
	}

	client := NewDashboardIconsClient(cacheDir, 0)

	data, ct, err := client.getFromCache("old", "svg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != content {
		t.Errorf("expected %q, got %q", content, string(data))
	}
	if ct != "image/svg+xml" {
		t.Errorf("expected 'image/svg+xml', got %q", ct)
	}
}

func TestDashboardClient_saveToCache_CreatesDir(t *testing.T) {
	baseDir := t.TempDir()
	cacheDir := filepath.Join(baseDir, "nested", "cache")
	client := NewDashboardIconsClient(cacheDir, 1*time.Hour)

	err := client.saveToCache("test", "svg", []byte("<svg/>"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = os.Stat(filepath.Join(cacheDir, "test.svg"))
	if err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
}

func TestDashboardClient_fetchIconList_HTTPError(t *testing.T) {
	cacheDir := t.TempDir()
	client := NewDashboardIconsClient(cacheDir, 1*time.Hour)
	client.httpClient = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				return nil, http.ErrHandlerTimeout
			},
		},
	}

	_, err := client.fetchIconList()
	if err == nil {
		t.Error("expected error for HTTP failure")
	}
}
