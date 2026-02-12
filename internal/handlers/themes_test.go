package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestParseThemeMetadata(t *testing.T) {
	tests := []struct {
		name           string
		css            string
		filename       string
		expectedID     string
		expectedName   string
		expectedIsDark bool
		hasPreview     bool
	}{
		{
			name: "full metadata",
			css: `/**
 * @theme-id: nord-dark
 * @theme-name: Nord Dark
 * @theme-description: A dark Nord theme
 * @theme-is-dark: true
 * @theme-family: nord
 * @theme-family-name: Nord
 * @theme-variant: dark
 * @theme-preview-bg: #2e3440
 * @theme-preview-surface: #3b4252
 * @theme-preview-accent: #88c0d0
 * @theme-preview-text: #eceff4
 */
[data-theme="nord-dark"] { color-scheme: dark; }`,
			filename:       "nord-dark.css",
			expectedID:     "nord-dark",
			expectedName:   "Nord Dark",
			expectedIsDark: true,
			hasPreview:     true,
		},
		{
			name:           "minimal metadata",
			css:            `/* no metadata */\n[data-theme="custom"] {}`,
			filename:       "custom.css",
			expectedID:     "custom",
			expectedName:   "custom",
			expectedIsDark: true, // default
			hasPreview:     false,
		},
		{
			name: "light theme",
			css: `/**
 * @theme-name: Catppuccin Latte
 * @theme-is-dark: false
 */`,
			filename:       "catppuccin-latte.css",
			expectedID:     "catppuccin-latte",
			expectedName:   "Catppuccin Latte",
			expectedIsDark: false,
			hasPreview:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			theme := parseThemeMetadata(tt.css, tt.filename)
			if theme == nil {
				t.Fatal("expected non-nil theme")
			}
			if theme.ID != tt.expectedID {
				t.Errorf("ID = %q, want %q", theme.ID, tt.expectedID)
			}
			if theme.Name != tt.expectedName {
				t.Errorf("Name = %q, want %q", theme.Name, tt.expectedName)
			}
			if theme.IsDark != tt.expectedIsDark {
				t.Errorf("IsDark = %v, want %v", theme.IsDark, tt.expectedIsDark)
			}
			if tt.hasPreview && theme.Preview == nil {
				t.Error("expected Preview to be set")
			}
			if !tt.hasPreview && theme.Preview != nil {
				t.Error("expected Preview to be nil")
			}
		})
	}
}

func TestSanitizeThemeID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"My Theme", "my-theme"},
		{"Already-Clean", "already-clean"},
		{"  spaces  ", "spaces"},
		{"Special!@#$Chars", "special-chars"},
		{"Multiple---Dashes", "multiple-dashes"},
		{"UPPERCASE", "uppercase"},
		{"123numbers", "123numbers"},
		{"", ""},
		{"---", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeThemeID(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeThemeID(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestListThemes(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewThemeHandler(dir, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/themes", nil)
		w := httptest.NewRecorder()

		handler.ListThemes(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var themes []ThemeInfo
		if err := json.NewDecoder(w.Body).Decode(&themes); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if len(themes) != 0 {
			t.Errorf("expected 0 themes, got %d", len(themes))
		}
	})

	t.Run("bundled themes", func(t *testing.T) {
		dir := t.TempDir()
		bundledFS := fstest.MapFS{
			"themes/nord-dark.css": &fstest.MapFile{
				Data: []byte(`/* @theme-name: Nord Dark */`),
			},
			"themes/dracula.css": &fstest.MapFile{
				Data: []byte(`/* @theme-name: Dracula */`),
			},
		}
		handler := NewThemeHandler(dir, bundledFS)

		req := httptest.NewRequest(http.MethodGet, "/api/themes", nil)
		w := httptest.NewRecorder()

		handler.ListThemes(w, req)

		var themes []ThemeInfo
		if err := json.NewDecoder(w.Body).Decode(&themes); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if len(themes) != 2 {
			t.Errorf("expected 2 themes, got %d", len(themes))
		}
		for _, theme := range themes {
			if !theme.IsBuiltin {
				t.Errorf("expected theme %q to be builtin", theme.ID)
			}
		}
	})

	t.Run("user themes", func(t *testing.T) {
		dir := t.TempDir()
		err := os.WriteFile(filepath.Join(dir, "custom.css"), []byte(`/* @theme-name: Custom */`), 0644)
		if err != nil {
			t.Fatal(err)
		}

		handler := NewThemeHandler(dir, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/themes", nil)
		w := httptest.NewRecorder()

		handler.ListThemes(w, req)

		var themes []ThemeInfo
		if err := json.NewDecoder(w.Body).Decode(&themes); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if len(themes) != 1 {
			t.Errorf("expected 1 theme, got %d", len(themes))
		}
		if themes[0].IsBuiltin {
			t.Error("expected user theme not to be builtin")
		}
	})

	t.Run("user overrides bundled", func(t *testing.T) {
		dir := t.TempDir()
		// Multi-line comment format so @theme-name line ends cleanly (regex captures to EOL)
		err := os.WriteFile(filepath.Join(dir, "nord-dark.css"), []byte("/**\n * @theme-name: My Nord Dark\n */\n"), 0644)
		if err != nil {
			t.Fatal(err)
		}

		bundledFS := fstest.MapFS{
			"themes/nord-dark.css": &fstest.MapFile{
				Data: []byte(`/* @theme-name: Nord Dark */`),
			},
		}
		handler := NewThemeHandler(dir, bundledFS)

		req := httptest.NewRequest(http.MethodGet, "/api/themes", nil)
		w := httptest.NewRecorder()

		handler.ListThemes(w, req)

		var themes []ThemeInfo
		if err := json.NewDecoder(w.Body).Decode(&themes); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if len(themes) != 1 {
			t.Errorf("expected 1 theme (user override), got %d", len(themes))
		}
		if themes[0].IsBuiltin {
			t.Error("expected the user override to not be marked as builtin")
		}
		if themes[0].Name != "My Nord Dark" {
			t.Errorf("expected name 'My Nord Dark', got %q", themes[0].Name)
		}
	})
}

func TestSaveTheme(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewThemeHandler(dir, nil)

		body, _ := json.Marshal(ThemeSaveRequest{
			Name:      "My Custom Theme",
			IsDark:    true,
			BaseTheme: "dark",
			Variables: map[string]string{
				"--bg-base":        "#1a1a1a",
				"--accent-primary": "#ff6600",
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/themes", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.SaveTheme(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		// Verify file was created
		expectedPath := filepath.Join(dir, "my-custom-theme.css")
		if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
			t.Error("expected theme CSS file to be created")
		}
	})

	t.Run("CSS injection blocked", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewThemeHandler(dir, nil)

		body, _ := json.Marshal(ThemeSaveRequest{
			Name:   "Evil Theme",
			IsDark: true,
			Variables: map[string]string{
				"--bg-base": "url(javascript:alert(1))",
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/themes", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.SaveTheme(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 for CSS injection, got %d", w.Code)
		}
	})

	t.Run("invalid variable name", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewThemeHandler(dir, nil)

		body, _ := json.Marshal(ThemeSaveRequest{
			Name:   "Bad Vars",
			IsDark: true,
			Variables: map[string]string{
				"not-a-css-var": "#fff",
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/themes", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.SaveTheme(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 for invalid variable name, got %d", w.Code)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewThemeHandler(dir, nil)

		body, _ := json.Marshal(ThemeSaveRequest{
			Name:   "",
			IsDark: true,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/themes", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.SaveTheme(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewThemeHandler(dir, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/themes", nil)
		w := httptest.NewRecorder()

		handler.SaveTheme(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})

	t.Run("overwrite builtin rejected", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewThemeHandler(dir, nil)

		body, _ := json.Marshal(ThemeSaveRequest{
			Name:   "dark",
			IsDark: true,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/themes", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.SaveTheme(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("value with braces blocked", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewThemeHandler(dir, nil)

		body, _ := json.Marshal(ThemeSaveRequest{
			Name:   "Brace Theme",
			IsDark: true,
			Variables: map[string]string{
				"--bg-base": "#fff}body{background:red",
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/themes", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.SaveTheme(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 for CSS injection via braces, got %d", w.Code)
		}
	})
}

func TestDeleteTheme(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		dir := t.TempDir()
		err := os.WriteFile(filepath.Join(dir, "my-theme.css"), []byte("/* custom */"), 0644)
		if err != nil {
			t.Fatal(err)
		}

		handler := NewThemeHandler(dir, nil)

		req := httptest.NewRequest(http.MethodDelete, "/api/themes/my-theme", nil)
		w := httptest.NewRecorder()

		handler.DeleteTheme(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		// Verify file was deleted
		if _, err := os.Stat(filepath.Join(dir, "my-theme.css")); !os.IsNotExist(err) {
			t.Error("expected theme file to be deleted")
		}
	})

	t.Run("protect builtin dark", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewThemeHandler(dir, nil)

		req := httptest.NewRequest(http.MethodDelete, "/api/themes/dark", nil)
		w := httptest.NewRecorder()

		handler.DeleteTheme(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("protect builtin light", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewThemeHandler(dir, nil)

		req := httptest.NewRequest(http.MethodDelete, "/api/themes/light", nil)
		w := httptest.NewRecorder()

		handler.DeleteTheme(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("protect bundled theme", func(t *testing.T) {
		dir := t.TempDir()
		bundledFS := fstest.MapFS{
			"themes/nord-dark.css": &fstest.MapFile{
				Data: []byte(`/* @theme-name: Nord Dark */`),
			},
		}
		handler := NewThemeHandler(dir, bundledFS)

		req := httptest.NewRequest(http.MethodDelete, "/api/themes/nord-dark", nil)
		w := httptest.NewRecorder()

		handler.DeleteTheme(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("not found", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewThemeHandler(dir, nil)

		req := httptest.NewRequest(http.MethodDelete, "/api/themes/nonexistent", nil)
		w := httptest.NewRecorder()

		handler.DeleteTheme(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewThemeHandler(dir, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/themes/test", nil)
		w := httptest.NewRecorder()

		handler.DeleteTheme(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewThemeHandler(dir, nil)

		req := httptest.NewRequest(http.MethodDelete, "/api/themes/", nil)
		w := httptest.NewRecorder()

		handler.DeleteTheme(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

func TestGenerateThemeCSS(t *testing.T) {
	req := ThemeSaveRequest{
		Name:        "Test Theme",
		Description: "A test theme",
		Author:      "tester",
		BaseTheme:   "dark",
		IsDark:      true,
		Variables: map[string]string{
			"--bg-base": "#000",
		},
	}

	css := generateThemeCSS("test-theme", req)

	// Check that metadata comments are present
	if !bytes.Contains([]byte(css), []byte("@theme-id: test-theme")) {
		t.Error("expected @theme-id in CSS")
	}
	if !bytes.Contains([]byte(css), []byte("@theme-name: Test Theme")) {
		t.Error("expected @theme-name in CSS")
	}
	if !bytes.Contains([]byte(css), []byte("Author: tester")) {
		t.Error("expected Author in CSS")
	}
	if !bytes.Contains([]byte(css), []byte(`[data-theme="test-theme"]`)) {
		t.Error("expected data-theme selector in CSS")
	}
	if !bytes.Contains([]byte(css), []byte("color-scheme: dark")) {
		t.Error("expected color-scheme: dark in CSS")
	}

	// Test light theme
	req.IsDark = false
	css = generateThemeCSS("test-light", req)
	if !bytes.Contains([]byte(css), []byte("color-scheme: light")) {
		t.Error("expected color-scheme: light in CSS")
	}
}
