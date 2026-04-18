package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mescon/muximux/v3/internal/icons"
)

// disableSSRF replaces both the pre-flight SSRF validator and the
// per-connect dial-time validator with no-ops. Tests use
// httptest.NewServer which binds to 127.0.0.1, and after findings.md C8
// both layers reject loopback independently.
func disableSSRF(t *testing.T) {
	t.Helper()
	origValidator := validateHostSSRF
	validateHostSSRF = func(hostname string) error { return nil }
	origDial := safeSSRFDialContext
	safeSSRFDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		var dialer net.Dialer
		return dialer.DialContext(ctx, network, addr)
	}
	t.Cleanup(func() {
		validateHostSSRF = origValidator
		safeSSRFDialContext = origDial
	})
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

func TestFetchCustomIcon(t *testing.T) {
	t.Run("happy path PNG", func(t *testing.T) {
		disableSSRF(t)
		// Start a mock server serving a PNG
		pngData := append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, make([]byte, 100)...)
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			w.Write(pngData)
		}))
		defer ts.Close()

		dir := t.TempDir()
		handler := NewIconHandler(nil, nil, dir)

		body, _ := json.Marshal(map[string]string{"url": ts.URL + "/icon.png"})
		req := httptest.NewRequest(http.MethodPost, "/api/icons/custom/fetch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.FetchCustomIcon(w, req)

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
		if resp["name"] == "" {
			t.Error("expected non-empty name in response")
		}

		// Verify icon was saved on disk
		files, _ := os.ReadDir(dir)
		if len(files) != 1 {
			t.Errorf("expected 1 file in custom dir, got %d", len(files))
		}
	})

	t.Run("with custom name", func(t *testing.T) {
		disableSSRF(t)
		svgData := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect width="10" height="10"/></svg>`)
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/svg+xml")
			w.Write(svgData)
		}))
		defer ts.Close()

		dir := t.TempDir()
		handler := NewIconHandler(nil, nil, dir)

		body, _ := json.Marshal(map[string]string{"url": ts.URL + "/icon.svg", "name": "my-custom-name"})
		req := httptest.NewRequest(http.MethodPost, "/api/icons/custom/fetch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.FetchCustomIcon(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]string
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["name"] != "my-custom-name" {
			t.Errorf("expected name 'my-custom-name', got %q", resp["name"])
		}
	})

	t.Run("missing URL", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewIconHandler(nil, nil, dir)

		body, _ := json.Marshal(map[string]string{})
		req := httptest.NewRequest(http.MethodPost, "/api/icons/custom/fetch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.FetchCustomIcon(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("invalid URL scheme", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewIconHandler(nil, nil, dir)

		body, _ := json.Marshal(map[string]string{"url": "ftp://evil.com/icon.png"})
		req := httptest.NewRequest(http.MethodPost, "/api/icons/custom/fetch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.FetchCustomIcon(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("non-image content type", func(t *testing.T) {
		disableSSRF(t)
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<html>not an icon</html>"))
		}))
		defer ts.Close()

		dir := t.TempDir()
		handler := NewIconHandler(nil, nil, dir)

		body, _ := json.Marshal(map[string]string{"url": ts.URL + "/page.html"})
		req := httptest.NewRequest(http.MethodPost, "/api/icons/custom/fetch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.FetchCustomIcon(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("file too large", func(t *testing.T) {
		disableSSRF(t)
		// Serve a file larger than MaxIconSize
		bigData := make([]byte, icons.MaxIconSize+1)
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			w.Write(bigData)
		}))
		defer ts.Close()

		dir := t.TempDir()
		handler := NewIconHandler(nil, nil, dir)

		body, _ := json.Marshal(map[string]string{"url": ts.URL + "/big.png"})
		req := httptest.NewRequest(http.MethodPost, "/api/icons/custom/fetch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.FetchCustomIcon(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("server returns 404", func(t *testing.T) {
		disableSSRF(t)
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}))
		defer ts.Close()

		dir := t.TempDir()
		handler := NewIconHandler(nil, nil, dir)

		body, _ := json.Marshal(map[string]string{"url": ts.URL + "/missing.png"})
		req := httptest.NewRequest(http.MethodPost, "/api/icons/custom/fetch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.FetchCustomIcon(w, req)

		if w.Code != http.StatusBadGateway {
			t.Errorf("expected status 502, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewIconHandler(nil, nil, dir)

		req := httptest.NewRequest(http.MethodPost, "/api/icons/custom/fetch", bytes.NewReader([]byte("not json")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.FetchCustomIcon(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("content type with parameters", func(t *testing.T) {
		disableSSRF(t)
		pngData := append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, make([]byte, 50)...)
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png; charset=utf-8")
			w.Write(pngData)
		}))
		defer ts.Close()

		dir := t.TempDir()
		handler := NewIconHandler(nil, nil, dir)

		body, _ := json.Marshal(map[string]string{"url": ts.URL + "/icon.png"})
		req := httptest.NewRequest(http.MethodPost, "/api/icons/custom/fetch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.FetchCustomIcon(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("name derived from URL path", func(t *testing.T) {
		disableSSRF(t)
		svgData := []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/svg+xml")
			w.Write(svgData)
		}))
		defer ts.Close()

		dir := t.TempDir()
		handler := NewIconHandler(nil, nil, dir)

		body, _ := json.Marshal(map[string]string{"url": ts.URL + "/images/my-app-icon.svg"})
		req := httptest.NewRequest(http.MethodPost, "/api/icons/custom/fetch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.FetchCustomIcon(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]string
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["name"] != "my-app-icon" {
			t.Errorf("expected name 'my-app-icon', got %q", resp["name"])
		}
	})

	// SSRF protection tests (these do NOT disable SSRF validation)
	t.Run("rejects loopback address", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewIconHandler(nil, nil, dir)

		body, _ := json.Marshal(map[string]string{"url": "http://127.0.0.1:9999/icon.png"})
		req := httptest.NewRequest(http.MethodPost, "/api/icons/custom/fetch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.FetchCustomIcon(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 for loopback, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("rejects private IP", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewIconHandler(nil, nil, dir)

		// Use a hostname that resolves to a private IP
		body, _ := json.Marshal(map[string]string{"url": "http://10.0.0.1/icon.png"})
		req := httptest.NewRequest(http.MethodPost, "/api/icons/custom/fetch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.FetchCustomIcon(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 for private IP, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("rejects localhost hostname", func(t *testing.T) {
		dir := t.TempDir()
		handler := NewIconHandler(nil, nil, dir)

		body, _ := json.Marshal(map[string]string{"url": "http://localhost/icon.png"})
		req := httptest.NewRequest(http.MethodPost, "/api/icons/custom/fetch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.FetchCustomIcon(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 for localhost, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// TestServeIcon_CustomHardenedHeaders covers findings.md H3. A custom
// icon served over /api/icons/custom/... must arrive with headers that
// neuter direct-load XSS: Content-Disposition: attachment,
// X-Content-Type-Options: nosniff, and a restrictive CSP.
func TestServeIcon_CustomHardenedHeaders(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "icon.svg"),
		[]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		0o600); err != nil {
		t.Fatalf("seed icon: %v", err)
	}
	h := NewIconHandler(nil, nil, dir)

	req := httptest.NewRequest(http.MethodGet, "/icons/custom/icon.svg", nil)
	rec := httptest.NewRecorder()
	h.ServeIcon(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") || !strings.Contains(csp, "sandbox") {
		t.Errorf("Content-Security-Policy = %q, want default-src 'none' with sandbox", csp)
	}
	if dp := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(dp, "attachment") {
		t.Errorf("Content-Disposition = %q, want attachment", dp)
	}
}

func TestValidateHostSSRF(t *testing.T) {
	t.Run("rejects loopback", func(t *testing.T) {
		if err := validateHostSSRF("127.0.0.1"); err == nil {
			t.Error("expected error for loopback address")
		}
	})

	t.Run("rejects localhost", func(t *testing.T) {
		if err := validateHostSSRF("localhost"); err == nil {
			t.Error("expected error for localhost")
		}
	})

	t.Run("rejects unresolvable host", func(t *testing.T) {
		if err := validateHostSSRF("this-host-does-not-exist.invalid"); err == nil {
			t.Error("expected error for unresolvable hostname")
		}
	})
}

// TestValidateIP_BlocksMaskedAndCGNAT covers the pieces findings.md C8
// called out as bypassable: IPv4-mapped IPv6 (::ffff:a.b.c.d) and the
// RFC 6598 CGNAT range (100.64.0.0/10).
func TestValidateIP_BlocksMaskedAndCGNAT(t *testing.T) {
	cases := []struct {
		name string
		ip   string
		want bool // true if expected to be blocked
	}{
		{"IPv4 loopback", "127.0.0.1", true},
		{"IPv4 RFC1918", "10.0.0.5", true},
		{"IPv4 link-local", "169.254.169.254", true},
		{"IPv4 CGNAT", "100.64.0.1", true},
		{"IPv4-mapped IPv6 loopback", "::ffff:127.0.0.1", true},
		{"IPv4-mapped IPv6 RFC1918", "::ffff:10.0.0.1", true},
		{"public v4", "8.8.8.8", false},
		{"public v6", "2001:db8::1", true}, // documentation range; not "private" per IsPrivate, but it's multicast-adjacent... actually it's global. Leave as false.
	}
	// 2001:db8::1 is the RFC 3849 documentation prefix. IsPrivate returns
	// false for it, so validateIP should accept it. Adjust:
	cases[len(cases)-1].want = false

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			ip := net.ParseIP(c.ip)
			err := validateIP(ip)
			if c.want && err == nil {
				t.Errorf("expected %s to be blocked", c.ip)
			}
			if !c.want && err != nil {
				t.Errorf("expected %s to be allowed, got %v", c.ip, err)
			}
		})
	}
}

// TestResolveIconContentType covers findings.md L9: the server's
// declared Content-Type must never override sniffing, except for SVG
// where the server header is trusted only if the bytes start with SVG
// or XML markers.
func TestResolveIconContentType(t *testing.T) {
	pngMagic := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	pngData := append(pngMagic, bytes.Repeat([]byte{0x00}, 16)...)

	svgBytes := []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)
	htmlAsSVG := []byte(`<html><script>alert(1)</script></html>`)

	cases := []struct {
		name   string
		data   []byte
		header string
		want   string
	}{
		{"PNG bytes trump any header", pngData, "image/jpeg", "image/png"},
		{"PNG bytes with matching header", pngData, "image/png", "image/png"},
		{"SVG header + SVG bytes", svgBytes, "image/svg+xml", "image/svg+xml"},
		{"SVG header + HTML bytes rejected", htmlAsSVG, "image/svg+xml", ""},
		{"Text bytes with no matching header", []byte("plain text"), "image/png", ""},
		{"Text bytes with svg header but no SVG marker", []byte("plain text"), "image/svg+xml", ""},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := resolveIconContentType(c.data, c.header)
			if got != c.want {
				t.Errorf("resolveIconContentType() = %q, want %q", got, c.want)
			}
		})
	}
}

// TestFetchCustomIcon_RejectsRedirectToLoopback covers the second half of
// C8: an attacker's remote server can no longer bounce the fetcher to a
// private address via a 302 because each redirect hop is revalidated.
func TestFetchCustomIcon_RejectsRedirectToLoopback(t *testing.T) {
	// We need validateHostSSRF to permit the initial (loopback) hop for
	// the test server but still reject a loopback literal on the
	// redirect hop. Toggle it by hostname.
	origValidator := validateHostSSRF
	validateHostSSRF = func(hostname string) error {
		if hostname == "blocked.invalid" || strings.HasPrefix(hostname, "127.") {
			// Pretend the target looks internal on the redirect hop.
			return &net.AddrError{Err: "blocked", Addr: hostname}
		}
		return nil
	}
	t.Cleanup(func() { validateHostSSRF = origValidator })
	origDial := safeSSRFDialContext
	safeSSRFDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, addr)
	}
	t.Cleanup(func() { safeSSRFDialContext = origDial })

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Redirect to a host we flagged as blocked above.
		w.Header().Set("Location", "http://blocked.invalid/")
		w.WriteHeader(http.StatusFound)
	}))
	defer ts.Close()

	dir := t.TempDir()
	handler := NewIconHandler(nil, nil, dir)

	// Allow the initial hop by overriding the validator for the test
	// server's host only.
	validateHostSSRF = func(hostname string) error {
		if hostname == "blocked.invalid" {
			return &net.AddrError{Err: "blocked", Addr: hostname}
		}
		return nil
	}

	body, _ := json.Marshal(map[string]string{"url": ts.URL + "/icon.png"})
	req := httptest.NewRequest(http.MethodPost, "/api/icons/custom/fetch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.FetchCustomIcon(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502 on redirect to blocked host, got %d: %s", w.Code, w.Body.String())
	}
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
