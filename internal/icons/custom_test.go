package icons

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewCustomIconsManager(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "custom-icons")

	mgr := NewCustomIconsManager(subDir)

	if mgr.storageDir != subDir {
		t.Errorf("expected storageDir %q, got %q", subDir, mgr.storageDir)
	}

	// Directory should be created
	info, err := os.Stat(subDir)
	if err != nil {
		t.Fatalf("expected directory to be created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected a directory")
	}
}

func TestCustomIconsManager_SaveIcon(t *testing.T) {
	t.Run("SVG", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewCustomIconsManager(dir)

		err := mgr.SaveIcon("myicon", []byte("<svg>test</svg>"), "image/svg+xml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(dir, "myicon.svg"))
		if err != nil {
			t.Fatalf("expected file to exist: %v", err)
		}
		if string(data) != "<svg>test</svg>" {
			t.Errorf("expected '<svg>test</svg>', got %q", string(data))
		}
	})

	t.Run("PNG", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewCustomIconsManager(dir)

		err := mgr.SaveIcon("icon", []byte("PNG_DATA"), "image/png")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = os.Stat(filepath.Join(dir, "icon.png"))
		if err != nil {
			t.Fatalf("expected png file: %v", err)
		}
	})

	t.Run("JPEG", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewCustomIconsManager(dir)

		err := mgr.SaveIcon("photo", []byte("JPEG_DATA"), "image/jpeg")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = os.Stat(filepath.Join(dir, "photo.jpg"))
		if err != nil {
			t.Fatalf("expected jpg file: %v", err)
		}
	})

	t.Run("WebP", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewCustomIconsManager(dir)

		err := mgr.SaveIcon("webpicon", []byte("WEBP_DATA"), "image/webp")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = os.Stat(filepath.Join(dir, "webpicon.webp"))
		if err != nil {
			t.Fatalf("expected webp file: %v", err)
		}
	})

	t.Run("GIF", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewCustomIconsManager(dir)

		err := mgr.SaveIcon("animated", []byte("GIF_DATA"), "image/gif")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = os.Stat(filepath.Join(dir, "animated.gif"))
		if err != nil {
			t.Fatalf("expected gif file: %v", err)
		}
	})

	t.Run("unsupported content type", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewCustomIconsManager(dir)

		err := mgr.SaveIcon("bad", []byte("data"), "application/pdf")
		if err == nil {
			t.Error("expected error for unsupported content type")
		}
		if !strings.Contains(err.Error(), "unsupported file type") {
			t.Errorf("expected 'unsupported file type' error, got %q", err.Error())
		}
	})

	t.Run("file too large", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewCustomIconsManager(dir)

		// Create data larger than MaxIconSize (2MB)
		bigData := make([]byte, MaxIconSize+1)
		err := mgr.SaveIcon("big", bigData, "image/svg+xml")
		if err == nil {
			t.Error("expected error for file too large")
		}
		if !strings.Contains(err.Error(), "file too large") {
			t.Errorf("expected 'file too large' error, got %q", err.Error())
		}
	})

	t.Run("invalid name", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewCustomIconsManager(dir)

		err := mgr.SaveIcon("!@#$%", []byte("<svg/>"), "image/svg+xml")
		if err == nil {
			t.Error("expected error for invalid icon name")
		}
		if !strings.Contains(err.Error(), "invalid icon name") {
			t.Errorf("expected 'invalid icon name' error, got %q", err.Error())
		}
	})

	t.Run("name with extension stripped", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewCustomIconsManager(dir)

		err := mgr.SaveIcon("icon.svg", []byte("<svg/>"), "image/svg+xml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should strip .svg from name and add it back from content type
		_, err = os.Stat(filepath.Join(dir, "icon.svg"))
		if err != nil {
			t.Fatalf("expected file: %v", err)
		}
	})

	t.Run("name sanitization", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewCustomIconsManager(dir)

		err := mgr.SaveIcon("My Icon Name", []byte("<svg/>"), "image/svg+xml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Name should be sanitized (lowercase, only allowed chars)
		_, err = os.Stat(filepath.Join(dir, "myiconname.svg"))
		if err != nil {
			t.Fatalf("expected sanitized file: %v", err)
		}
	})
}

func TestCustomIconsManager_GetIcon(t *testing.T) {
	t.Run("found SVG", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewCustomIconsManager(dir)

		content := "<svg>found</svg>"
		if err := os.WriteFile(filepath.Join(dir, "found.svg"), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}

		data, ct, err := mgr.GetIcon("found")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != content {
			t.Errorf("expected %q, got %q", content, string(data))
		}
		if ct != "image/svg+xml" {
			t.Errorf("expected 'image/svg+xml', got %q", ct)
		}
	})

	t.Run("found PNG", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewCustomIconsManager(dir)

		if err := os.WriteFile(filepath.Join(dir, "photo.png"), []byte("PNG"), 0600); err != nil {
			t.Fatal(err)
		}

		data, ct, err := mgr.GetIcon("photo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != "PNG" {
			t.Errorf("expected 'PNG', got %q", string(data))
		}
		if ct != "image/png" {
			t.Errorf("expected 'image/png', got %q", ct)
		}
	})

	t.Run("found with extension in name", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewCustomIconsManager(dir)

		content := "<svg>ext</svg>"
		if err := os.WriteFile(filepath.Join(dir, "withext.svg"), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}

		data, _, err := mgr.GetIcon("withext.svg")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != content {
			t.Errorf("expected %q, got %q", content, string(data))
		}
	})

	t.Run("not found", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewCustomIconsManager(dir)

		_, _, err := mgr.GetIcon("nonexistent")
		if err == nil {
			t.Error("expected error for non-existent icon")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got %q", err.Error())
		}
	})
}

func TestCustomIconsManager_ListIcons(t *testing.T) {
	t.Run("empty directory", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewCustomIconsManager(dir)

		icons, err := mgr.ListIcons()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(icons) != 0 {
			t.Errorf("expected 0 icons, got %d", len(icons))
		}
	})

	t.Run("with files", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewCustomIconsManager(dir)

		if err := os.WriteFile(filepath.Join(dir, "icon1.svg"), []byte("<svg/>"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "icon2.png"), []byte("PNG"), 0600); err != nil {
			t.Fatal(err)
		}

		icons, err := mgr.ListIcons()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(icons) != 2 {
			t.Errorf("expected 2 icons, got %d", len(icons))
		}

		for _, icon := range icons {
			if icon.Name == "" {
				t.Error("expected non-empty icon name")
			}
			if icon.ContentType == "" {
				t.Error("expected non-empty content type")
			}
			if icon.Size == 0 {
				t.Error("expected non-zero size")
			}
		}
	})

	t.Run("skips directories", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewCustomIconsManager(dir)

		if err := os.WriteFile(filepath.Join(dir, "icon.svg"), []byte("<svg/>"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0755); err != nil {
			t.Fatal(err)
		}

		icons, err := mgr.ListIcons()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(icons) != 1 {
			t.Errorf("expected 1 icon (skipping dir), got %d", len(icons))
		}
	})

	t.Run("non-existent directory", func(t *testing.T) {
		mgr := &CustomIconsManager{storageDir: "/nonexistent/path"}

		icons, err := mgr.ListIcons()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(icons) != 0 {
			t.Errorf("expected 0 icons for non-existent dir, got %d", len(icons))
		}
	})
}

func TestCustomIconsManager_DeleteIcon(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewCustomIconsManager(dir)

		if err := os.WriteFile(filepath.Join(dir, "todelete.svg"), []byte("<svg/>"), 0600); err != nil {
			t.Fatal(err)
		}

		err := mgr.DeleteIcon("todelete")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = os.Stat(filepath.Join(dir, "todelete.svg"))
		if !os.IsNotExist(err) {
			t.Error("expected file to be deleted")
		}
	})

	t.Run("delete PNG", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewCustomIconsManager(dir)

		if err := os.WriteFile(filepath.Join(dir, "topng.png"), []byte("PNG"), 0600); err != nil {
			t.Fatal(err)
		}

		err := mgr.DeleteIcon("topng")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewCustomIconsManager(dir)

		err := mgr.DeleteIcon("nonexistent")
		if err == nil {
			t.Error("expected error for non-existent icon")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got %q", err.Error())
		}
	})
}

func TestCustomIconsManager_SaveIconFromReader(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewCustomIconsManager(dir)

		reader := strings.NewReader("<svg>from-reader</svg>")
		err := mgr.SaveIconFromReader("reader-icon", reader, "image/svg+xml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(dir, "reader-icon.svg"))
		if err != nil {
			t.Fatalf("expected file: %v", err)
		}
		if string(data) != "<svg>from-reader</svg>" {
			t.Errorf("expected '<svg>from-reader</svg>', got %q", string(data))
		}
	})

	t.Run("too large", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewCustomIconsManager(dir)

		// Create a reader with data larger than MaxIconSize
		bigData := strings.Repeat("x", MaxIconSize+1)
		reader := strings.NewReader(bigData)

		err := mgr.SaveIconFromReader("big", reader, "image/svg+xml")
		if err == nil {
			t.Error("expected error for file too large")
		}
		if !strings.Contains(err.Error(), "file too large") {
			t.Errorf("expected 'file too large' error, got %q", err.Error())
		}
	})

	t.Run("invalid content type", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewCustomIconsManager(dir)

		reader := strings.NewReader("data")
		err := mgr.SaveIconFromReader("bad", reader, "text/plain")
		if err == nil {
			t.Error("expected error for invalid content type")
		}
	})
}

func TestSanitizeIconName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with-dash", "with-dash"},
		{"with_underscore", "with_underscore"},
		{"MixedCase", "mixedcase"},
		{"with.extension.svg", "withextension"},
		{"Special!@#$Chars", "specialchars"},
		{"123numbers", "123numbers"},
		{"My Icon", "myicon"},
		{"", ""},
		{"file.png", "file"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeIconName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeIconName(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGuessContentType(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"icon.svg", "image/svg+xml"},
		{"icon.png", "image/png"},
		{"icon.jpg", "image/jpeg"},
		{"icon.jpeg", "image/jpeg"},
		{"icon.webp", "image/webp"},
		{"icon.gif", "image/gif"},
		{"icon.bmp", "application/octet-stream"},
		{"icon.SVG", "image/svg+xml"},
		{"icon.PNG", "image/png"},
		{"noext", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := guessContentType(tt.filename)
			if result != tt.expected {
				t.Errorf("guessContentType(%q) = %q, expected %q", tt.filename, result, tt.expected)
			}
		})
	}
}
