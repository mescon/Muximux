package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mescon/muximux/v3/internal/icons"
)

func TestGetDashboardIcon(t *testing.T) {
	t.Run("empty name", func(t *testing.T) {
		cacheDir := t.TempDir()
		client := icons.NewDashboardIconsClient(cacheDir, 1*time.Hour)
		handler := NewIconHandler(client, nil, t.TempDir())

		req := httptest.NewRequest(http.MethodGet, "/api/icons/dashboard/", nil)
		w := httptest.NewRecorder()

		handler.GetDashboardIcon(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("success from cache", func(t *testing.T) {
		cacheDir := t.TempDir()
		// Pre-populate cache
		if err := os.WriteFile(filepath.Join(cacheDir, "plex.svg"), []byte("<svg>plex</svg>"), 0600); err != nil {
			t.Fatal(err)
		}

		client := icons.NewDashboardIconsClient(cacheDir, 1*time.Hour)
		handler := NewIconHandler(client, nil, t.TempDir())

		req := httptest.NewRequest(http.MethodGet, "/api/icons/dashboard/plex", nil)
		w := httptest.NewRecorder()

		handler.GetDashboardIcon(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
		if w.Header().Get("Content-Type") != "image/svg+xml" {
			t.Errorf("expected content-type 'image/svg+xml', got %q", w.Header().Get("Content-Type"))
		}
		if w.Header().Get("Cache-Control") != "public, max-age=86400" {
			t.Errorf("expected cache-control header, got %q", w.Header().Get("Cache-Control"))
		}
		if w.Body.String() != "<svg>plex</svg>" {
			t.Errorf("unexpected body: %s", w.Body.String())
		}
	})

	t.Run("with variant query param", func(t *testing.T) {
		cacheDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(cacheDir, "plex.png"), []byte("PNG_DATA"), 0600); err != nil {
			t.Fatal(err)
		}

		client := icons.NewDashboardIconsClient(cacheDir, 1*time.Hour)
		handler := NewIconHandler(client, nil, t.TempDir())

		req := httptest.NewRequest(http.MethodGet, "/api/icons/dashboard/plex?variant=png", nil)
		w := httptest.NewRecorder()

		handler.GetDashboardIcon(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
		if w.Header().Get("Content-Type") != "image/png" {
			t.Errorf("expected content-type 'image/png', got %q", w.Header().Get("Content-Type"))
		}
	})

	t.Run("default variant is svg", func(t *testing.T) {
		cacheDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(cacheDir, "radarr.svg"), []byte("<svg>radarr</svg>"), 0600); err != nil {
			t.Fatal(err)
		}

		client := icons.NewDashboardIconsClient(cacheDir, 1*time.Hour)
		handler := NewIconHandler(client, nil, t.TempDir())

		// No variant query param - should default to svg
		req := httptest.NewRequest(http.MethodGet, "/api/icons/dashboard/radarr", nil)
		w := httptest.NewRecorder()

		handler.GetDashboardIcon(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestGetLucideIcon(t *testing.T) {
	t.Run("empty name", func(t *testing.T) {
		cacheDir := t.TempDir()
		lucideClient := icons.NewLucideClient(cacheDir, 1*time.Hour)
		handler := NewIconHandler(nil, lucideClient, t.TempDir())

		req := httptest.NewRequest(http.MethodGet, "/api/icons/lucide/", nil)
		w := httptest.NewRecorder()

		handler.GetLucideIcon(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("success from cache", func(t *testing.T) {
		cacheDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(cacheDir, "home.svg"), []byte("<svg>lucide-home</svg>"), 0600); err != nil {
			t.Fatal(err)
		}

		lucideClient := icons.NewLucideClient(cacheDir, 1*time.Hour)
		handler := NewIconHandler(nil, lucideClient, t.TempDir())

		req := httptest.NewRequest(http.MethodGet, "/api/icons/lucide/home", nil)
		w := httptest.NewRecorder()

		handler.GetLucideIcon(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
		if w.Header().Get("Content-Type") != "image/svg+xml" {
			t.Errorf("expected content-type 'image/svg+xml', got %q", w.Header().Get("Content-Type"))
		}
		if w.Header().Get("Cache-Control") != "public, max-age=86400" {
			t.Errorf("expected cache-control header")
		}
		if w.Body.String() != "<svg>lucide-home</svg>" {
			t.Errorf("unexpected body: %s", w.Body.String())
		}
	})

	t.Run("with svg extension", func(t *testing.T) {
		cacheDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(cacheDir, "star.svg"), []byte("<svg>star</svg>"), 0600); err != nil {
			t.Fatal(err)
		}

		lucideClient := icons.NewLucideClient(cacheDir, 1*time.Hour)
		handler := NewIconHandler(nil, lucideClient, t.TempDir())

		// GetIcon strips .svg extension, so the path should still work
		req := httptest.NewRequest(http.MethodGet, "/api/icons/lucide/star.svg", nil)
		w := httptest.NewRecorder()

		handler.GetLucideIcon(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestListCustomIcons(t *testing.T) {
	t.Run("empty directory", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewIconHandler(nil, nil, dir)

		req := httptest.NewRequest(http.MethodGet, "/api/icons/custom", nil)
		w := httptest.NewRecorder()

		handler.ListCustomIcons(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var result []icons.CustomIconInfo
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected 0 icons, got %d", len(result))
		}
	})

	t.Run("with icons", func(t *testing.T) {
		dir := t.TempDir()
		err := os.WriteFile(filepath.Join(dir, "myicon.svg"), []byte(`<svg>test</svg>`), 0600)
		if err != nil {
			t.Fatal(err)
		}
		err = os.WriteFile(filepath.Join(dir, "another.png"), []byte("PNG"), 0600)
		if err != nil {
			t.Fatal(err)
		}

		handler := NewIconHandler(nil, nil, dir)

		req := httptest.NewRequest(http.MethodGet, "/api/icons/custom", nil)
		w := httptest.NewRecorder()

		handler.ListCustomIcons(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var result []icons.CustomIconInfo
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 icons, got %d", len(result))
		}
	})
}

func TestUploadCustomIcon(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewIconHandler(nil, nil, dir)

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		// Create a form field with explicit Content-Type for SVG
		h := make(textproto.MIMEHeader)
		h["Content-Disposition"] = []string{`form-data; name="icon"; filename="test-icon.svg"`}
		h["Content-Type"] = []string{"image/svg+xml"}
		part, err := writer.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect width="100" height="100"/></svg>`)); err != nil {
			t.Fatal(err)
		}

		// Add the name field
		if err = writer.WriteField("name", "test-icon"); err != nil {
			t.Fatal(err)
		}
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/icons/custom", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		handler.UploadCustomIcon(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]string
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["status"] != "uploaded" {
			t.Errorf("expected status 'uploaded', got %q", resp["status"])
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewIconHandler(nil, nil, dir)

		req := httptest.NewRequest(http.MethodGet, "/api/icons/custom", nil)
		w := httptest.NewRecorder()

		handler.UploadCustomIcon(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})

	t.Run("no file", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewIconHandler(nil, nil, dir)

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/icons/custom", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		handler.UploadCustomIcon(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("content type detection for octet-stream", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewIconHandler(nil, nil, dir)

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		// Create with application/octet-stream content type
		h := make(textproto.MIMEHeader)
		h["Content-Disposition"] = []string{`form-data; name="icon"; filename="test.png"`}
		h["Content-Type"] = []string{"application/octet-stream"}
		part, err := writer.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		// Write a minimal PNG header so DetectContentType identifies it
		pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
		if _, err = part.Write(pngHeader); err != nil {
			t.Fatal(err)
		}

		if err = writer.WriteField("name", "detected-icon"); err != nil {
			t.Fatal(err)
		}
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/icons/custom", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		handler.UploadCustomIcon(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("unsupported file type", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewIconHandler(nil, nil, dir)

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		h := make(textproto.MIMEHeader)
		h["Content-Disposition"] = []string{`form-data; name="icon"; filename="test.txt"`}
		h["Content-Type"] = []string{"text/plain"}
		part, err := writer.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write([]byte("not an image")); err != nil {
			t.Fatal(err)
		}

		if err = writer.WriteField("name", "bad-icon"); err != nil {
			t.Fatal(err)
		}
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/icons/custom", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		handler.UploadCustomIcon(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("name from filename", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewIconHandler(nil, nil, dir)

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		h := make(textproto.MIMEHeader)
		h["Content-Disposition"] = []string{`form-data; name="icon"; filename="auto-name.svg"`}
		h["Content-Type"] = []string{"image/svg+xml"}
		part, err := writer.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write([]byte(`<svg></svg>`)); err != nil {
			t.Fatal(err)
		}
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/icons/custom", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		handler.UploadCustomIcon(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]string
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["name"] != "auto-name" {
			t.Errorf("expected name 'auto-name', got %q", resp["name"])
		}
	})
}

func TestDeleteCustomIcon(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		dir := t.TempDir()
		err := os.WriteFile(filepath.Join(dir, "myicon.svg"), []byte(`<svg>test</svg>`), 0600)
		if err != nil {
			t.Fatal(err)
		}

		handler := NewIconHandler(nil, nil, dir)

		req := httptest.NewRequest(http.MethodDelete, "/api/icons/custom/myicon", nil)
		w := httptest.NewRecorder()

		handler.DeleteCustomIcon(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("not found", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewIconHandler(nil, nil, dir)

		req := httptest.NewRequest(http.MethodDelete, "/api/icons/custom/nonexistent", nil)
		w := httptest.NewRecorder()

		handler.DeleteCustomIcon(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewIconHandler(nil, nil, dir)

		req := httptest.NewRequest(http.MethodGet, "/api/icons/custom/myicon", nil)
		w := httptest.NewRecorder()

		handler.DeleteCustomIcon(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewIconHandler(nil, nil, dir)

		req := httptest.NewRequest(http.MethodDelete, "/api/icons/custom/", nil)
		w := httptest.NewRecorder()

		handler.DeleteCustomIcon(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

func TestServeIcon(t *testing.T) {
	t.Run("invalid path", func(t *testing.T) {
		handler := NewIconHandler(nil, nil, t.TempDir())

		req := httptest.NewRequest(http.MethodGet, "/icons/notype", nil)
		w := httptest.NewRecorder()

		handler.ServeIcon(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("unknown icon type", func(t *testing.T) {
		handler := NewIconHandler(nil, nil, t.TempDir())

		req := httptest.NewRequest(http.MethodGet, "/icons/unknown/test", nil)
		w := httptest.NewRecorder()

		handler.ServeIcon(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("custom icon found", func(t *testing.T) {
		dir := t.TempDir()
		err := os.WriteFile(filepath.Join(dir, "myicon.svg"), []byte(`<svg>custom</svg>`), 0600)
		if err != nil {
			t.Fatal(err)
		}

		handler := NewIconHandler(nil, nil, dir)

		req := httptest.NewRequest(http.MethodGet, "/icons/custom/myicon", nil)
		w := httptest.NewRecorder()

		handler.ServeIcon(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("custom icon not found", func(t *testing.T) {
		handler := NewIconHandler(nil, nil, t.TempDir())

		req := httptest.NewRequest(http.MethodGet, "/icons/custom/nonexistent", nil)
		w := httptest.NewRecorder()

		handler.ServeIcon(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})

	t.Run("dashboard icon from cache", func(t *testing.T) {
		cacheDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(cacheDir, "sonarr.svg"), []byte("<svg>sonarr</svg>"), 0600); err != nil {
			t.Fatal(err)
		}

		client := icons.NewDashboardIconsClient(cacheDir, 1*time.Hour)
		handler := NewIconHandler(client, nil, t.TempDir())

		req := httptest.NewRequest(http.MethodGet, "/icons/dashboard/sonarr", nil)
		w := httptest.NewRecorder()

		handler.ServeIcon(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
		if w.Header().Get("Content-Type") != "image/svg+xml" {
			t.Errorf("expected 'image/svg+xml', got %q", w.Header().Get("Content-Type"))
		}
	})

	t.Run("dashboard icon with variant query", func(t *testing.T) {
		cacheDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(cacheDir, "sonarr.png"), []byte("PNG"), 0600); err != nil {
			t.Fatal(err)
		}

		client := icons.NewDashboardIconsClient(cacheDir, 1*time.Hour)
		handler := NewIconHandler(client, nil, t.TempDir())

		req := httptest.NewRequest(http.MethodGet, "/icons/dashboard/sonarr?variant=png", nil)
		w := httptest.NewRecorder()

		handler.ServeIcon(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
		if w.Header().Get("Content-Type") != "image/png" {
			t.Errorf("expected 'image/png', got %q", w.Header().Get("Content-Type"))
		}
	})

	t.Run("dashboard icon with extension in name", func(t *testing.T) {
		cacheDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(cacheDir, "sonarr.png"), []byte("PNG"), 0600); err != nil {
			t.Fatal(err)
		}

		client := icons.NewDashboardIconsClient(cacheDir, 1*time.Hour)
		handler := NewIconHandler(client, nil, t.TempDir())

		// Extension in URL path, no variant query param
		req := httptest.NewRequest(http.MethodGet, "/icons/dashboard/sonarr.png", nil)
		w := httptest.NewRecorder()

		handler.ServeIcon(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
		if w.Header().Get("Content-Type") != "image/png" {
			t.Errorf("expected 'image/png', got %q", w.Header().Get("Content-Type"))
		}
	})

	t.Run("dashboard icon no extension defaults to svg", func(t *testing.T) {
		cacheDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(cacheDir, "sonarr.svg"), []byte("<svg>sonarr</svg>"), 0600); err != nil {
			t.Fatal(err)
		}

		client := icons.NewDashboardIconsClient(cacheDir, 1*time.Hour)
		handler := NewIconHandler(client, nil, t.TempDir())

		// No extension and no variant query - should default to svg
		req := httptest.NewRequest(http.MethodGet, "/icons/dashboard/sonarr", nil)
		w := httptest.NewRecorder()

		handler.ServeIcon(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("lucide icon from cache", func(t *testing.T) {
		cacheDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(cacheDir, "star.svg"), []byte("<svg>star</svg>"), 0600); err != nil {
			t.Fatal(err)
		}

		lucideClient := icons.NewLucideClient(cacheDir, 1*time.Hour)
		handler := NewIconHandler(nil, lucideClient, t.TempDir())

		req := httptest.NewRequest(http.MethodGet, "/icons/lucide/star", nil)
		w := httptest.NewRecorder()

		handler.ServeIcon(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
		if w.Header().Get("Content-Type") != "image/svg+xml" {
			t.Errorf("expected 'image/svg+xml', got %q", w.Header().Get("Content-Type"))
		}
	})

	t.Run("lucide icon with .svg extension", func(t *testing.T) {
		cacheDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(cacheDir, "star.svg"), []byte("<svg>star</svg>"), 0600); err != nil {
			t.Fatal(err)
		}

		lucideClient := icons.NewLucideClient(cacheDir, 1*time.Hour)
		handler := NewIconHandler(nil, lucideClient, t.TempDir())

		req := httptest.NewRequest(http.MethodGet, "/icons/lucide/star.svg", nil)
		w := httptest.NewRecorder()

		handler.ServeIcon(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("lucide icon cached without extension", func(t *testing.T) {
		cacheDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(cacheDir, "home.svg"), []byte("<svg>home</svg>"), 0600); err != nil {
			t.Fatal(err)
		}

		lucideClient := icons.NewLucideClient(cacheDir, 1*time.Hour)
		handler := NewIconHandler(nil, lucideClient, t.TempDir())

		// Without .svg extension
		req := httptest.NewRequest(http.MethodGet, "/icons/lucide/home", nil)
		w := httptest.NewRecorder()

		handler.ServeIcon(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestListDashboardIcons(t *testing.T) {
	cacheDir := t.TempDir()
	client := icons.NewDashboardIconsClient(cacheDir, 1*time.Hour)
	handler := NewIconHandler(client, nil, t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/api/icons/dashboard", nil)
	w := httptest.NewRecorder()

	handler.ListDashboardIcons(w, req)

	// May return empty list or error depending on cache/network state
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected 200 or 500, got %d", w.Code)
	}
}

func TestListLucideIcons(t *testing.T) {
	cacheDir := t.TempDir()
	lucideClient := icons.NewLucideClient(cacheDir, 1*time.Hour)
	handler := NewIconHandler(nil, lucideClient, t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/api/icons/lucide", nil)
	w := httptest.NewRecorder()

	handler.ListLucideIcons(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected 200 or 500, got %d", w.Code)
	}
}
