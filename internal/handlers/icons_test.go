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

// setupDashboardIconServer creates a mock server that simulates the dashboard icons CDN
// and returns the server and a DashboardIconsClient pointing to it.
func setupDashboardIconServer(t *testing.T) (*httptest.Server, *icons.DashboardIconsClient) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/svg/sonarr.svg" {
			w.Header().Set("Content-Type", "image/svg+xml")
			w.Write([]byte(`<svg>sonarr</svg>`))
			return
		}
		if r.URL.Path == "/png/sonarr.png" {
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("PNG DATA"))
			return
		}
		http.NotFound(w, r)
	}))

	cacheDir := t.TempDir()
	client := icons.NewDashboardIconsClient(cacheDir, 1*time.Hour)

	return server, client
}

// setupLucideIconServer creates a mock server for Lucide CDN.
func setupLucideIconServer(t *testing.T) (*httptest.Server, *icons.LucideClient) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/icons/home.svg" {
			w.Header().Set("Content-Type", "image/svg+xml")
			w.Write([]byte(`<svg>home</svg>`))
			return
		}
		http.NotFound(w, r)
	}))

	cacheDir := t.TempDir()
	client := icons.NewLucideClient(cacheDir, 1*time.Hour)

	return server, client
}

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
		err := os.WriteFile(filepath.Join(dir, "myicon.svg"), []byte(`<svg>test</svg>`), 0644)
		if err != nil {
			t.Fatal(err)
		}
		err = os.WriteFile(filepath.Join(dir, "another.png"), []byte("PNG"), 0644)
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
		part.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect width="100" height="100"/></svg>`))

		// Add the name field
		writer.WriteField("name", "test-icon")
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
		part.Write([]byte(`<svg></svg>`))
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
		err := os.WriteFile(filepath.Join(dir, "myicon.svg"), []byte(`<svg>test</svg>`), 0644)
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
		err := os.WriteFile(filepath.Join(dir, "myicon.svg"), []byte(`<svg>custom</svg>`), 0644)
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
}
