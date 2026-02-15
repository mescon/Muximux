package icons

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mescon/muximux/v3/internal/logging"
)

// CustomIconInfo represents a custom uploaded icon
type CustomIconInfo struct {
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

// CustomIconsManager handles custom icon uploads and serving
type CustomIconsManager struct {
	storageDir string
}

// AllowedMimeTypes defines which file types can be uploaded
var AllowedMimeTypes = map[string]string{
	"image/svg+xml": ".svg",
	"image/png":     ".png",
	"image/jpeg":    ".jpg",
	"image/webp":    ".webp",
	"image/gif":     ".gif",
}

// MaxIconSize is the maximum allowed icon file size (2MB)
const MaxIconSize = 2 * 1024 * 1024

// NewCustomIconsManager creates a new custom icons manager
func NewCustomIconsManager(storageDir string) *CustomIconsManager {
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		logging.Error("Failed to create custom icons directory", "source", "icons", "path", storageDir, "error", err)
	}
	return &CustomIconsManager{
		storageDir: storageDir,
	}
}

// SaveIcon saves an uploaded icon file
func (m *CustomIconsManager) SaveIcon(name string, data []byte, contentType string) error {
	// Validate content type
	ext, ok := AllowedMimeTypes[contentType]
	if !ok {
		return fmt.Errorf("unsupported file type: %s", contentType)
	}

	// Validate size
	if len(data) > MaxIconSize {
		return fmt.Errorf("file too large: max size is 2MB")
	}

	// Sanitize name - only allow alphanumeric, dash, underscore
	name = sanitizeIconName(name)
	if name == "" {
		return fmt.Errorf("invalid icon name")
	}

	// Save file
	filename := name + ext
	path := filepath.Join(m.storageDir, filename)

	return os.WriteFile(path, data, 0644)
}

// GetIcon retrieves a custom icon by name
func (m *CustomIconsManager) GetIcon(name string) ([]byte, string, error) {
	name = sanitizeIconName(name)

	// Try each supported extension
	for contentType, ext := range AllowedMimeTypes {
		path := filepath.Join(m.storageDir, name+ext)
		if data, err := os.ReadFile(path); err == nil {
			return data, contentType, nil
		}
	}

	// Also try with extension included in name
	path := filepath.Join(m.storageDir, name)
	if data, err := os.ReadFile(path); err == nil {
		contentType := guessContentType(name)
		return data, contentType, nil
	}

	return nil, "", fmt.Errorf("custom icon not found: %s", name)
}

// ListIcons returns all custom icons
func (m *CustomIconsManager) ListIcons() ([]CustomIconInfo, error) {
	entries, err := os.ReadDir(m.storageDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []CustomIconInfo{}, nil
		}
		return nil, err
	}

	var icons []CustomIconInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		name := entry.Name()
		ext := filepath.Ext(name)
		baseName := strings.TrimSuffix(name, ext)

		icons = append(icons, CustomIconInfo{
			Name:        baseName,
			ContentType: guessContentType(name),
			Size:        info.Size(),
		})
	}

	return icons, nil
}

// DeleteIcon removes a custom icon
func (m *CustomIconsManager) DeleteIcon(name string) error {
	name = sanitizeIconName(name)

	// Try each supported extension
	for _, ext := range AllowedMimeTypes {
		path := filepath.Join(m.storageDir, name+ext)
		if err := os.Remove(path); err == nil {
			return nil
		}
	}

	return fmt.Errorf("custom icon not found: %s", name)
}

// SaveIconFromReader saves an icon from a reader
func (m *CustomIconsManager) SaveIconFromReader(name string, r io.Reader, contentType string) error {
	data, err := io.ReadAll(io.LimitReader(r, MaxIconSize+1))
	if err != nil {
		return fmt.Errorf("failed to read icon data: %w", err)
	}

	if len(data) > MaxIconSize {
		return fmt.Errorf("file too large: max size is 2MB")
	}

	return m.SaveIcon(name, data, contentType)
}

// sanitizeIconName removes unsafe characters from icon names
func sanitizeIconName(name string) string {
	// Remove extension if present
	name = strings.TrimSuffix(name, filepath.Ext(name))

	// Only allow alphanumeric, dash, underscore
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			result.WriteRune(r)
		}
	}
	return strings.ToLower(result.String())
}

// guessContentType returns content type based on file extension
func guessContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}
