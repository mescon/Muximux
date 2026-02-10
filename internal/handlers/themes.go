package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ThemeHandler handles custom theme CRUD operations
type ThemeHandler struct {
	themesDir string
}

// NewThemeHandler creates a new theme handler
func NewThemeHandler(themesDir string) *ThemeHandler {
	// Ensure directory exists
	os.MkdirAll(themesDir, 0755)
	return &ThemeHandler{themesDir: themesDir}
}

// ThemeInfo represents theme metadata for the API
type ThemeInfo struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	IsBuiltin   bool          `json:"isBuiltin"`
	IsDark      bool          `json:"isDark"`
	Preview     *ThemePreview `json:"preview,omitempty"`
}

// ThemePreview holds color swatches for the theme grid
type ThemePreview struct {
	BG      string `json:"bg"`
	Surface string `json:"surface"`
	Accent  string `json:"accent"`
	Text    string `json:"text"`
}

// ThemeSaveRequest is the JSON body for creating/updating a theme
type ThemeSaveRequest struct {
	Name      string            `json:"name"`
	BaseTheme string            `json:"baseTheme"`
	IsDark    bool              `json:"isDark"`
	Variables map[string]string `json:"variables"`
}

// ListThemes returns all custom themes from the themes directory
func (h *ThemeHandler) ListThemes(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(h.themesDir)
	if err != nil {
		// Directory might not exist yet
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]ThemeInfo{})
		return
	}

	themes := make([]ThemeInfo, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".css") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(h.themesDir, entry.Name()))
		if err != nil {
			continue
		}

		theme := parseThemeMetadata(string(data), entry.Name())
		if theme != nil {
			themes = append(themes, *theme)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(themes)
}

// SaveTheme creates or updates a custom theme
func (h *ThemeHandler) SaveTheme(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ThemeSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Theme name is required", http.StatusBadRequest)
		return
	}

	// Sanitize the ID from the name
	id := sanitizeThemeID(req.Name)
	if id == "" {
		http.Error(w, "Invalid theme name", http.StatusBadRequest)
		return
	}

	// Don't allow overwriting builtin themes
	if id == "dark" || id == "light" {
		http.Error(w, "Cannot overwrite builtin themes", http.StatusBadRequest)
		return
	}

	// Generate CSS content
	css := generateThemeCSS(id, req)

	// Write to file
	filename := filepath.Join(h.themesDir, id+".css")
	if err := os.WriteFile(filename, []byte(css), 0644); err != nil {
		http.Error(w, "Failed to save theme", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id":     id,
		"status": "saved",
	})
}

// DeleteTheme removes a custom theme
func (h *ThemeHandler) DeleteTheme(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract theme name from path: /api/themes/{name}
	name := strings.TrimPrefix(r.URL.Path, "/api/themes/")
	if name == "" {
		http.Error(w, "Theme name required", http.StatusBadRequest)
		return
	}

	// Don't allow deleting builtin themes
	if name == "dark" || name == "light" || name == "nord" || name == "catppuccin" {
		http.Error(w, "Cannot delete builtin themes", http.StatusBadRequest)
		return
	}

	filename := filepath.Join(h.themesDir, name+".css")
	if err := os.Remove(filename); err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Theme not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to delete theme", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "deleted",
	})
}

// parseThemeMetadata extracts theme info from CSS file comments
func parseThemeMetadata(content string, filename string) *ThemeInfo {
	id := strings.TrimSuffix(filename, ".css")

	theme := &ThemeInfo{
		ID:        id,
		Name:      id,
		IsBuiltin: false,
		IsDark:    true,
	}

	// Parse @theme-* metadata comments
	metaPattern := regexp.MustCompile(`@theme-(\w[\w-]*):\s*(.+)`)
	matches := metaPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		key := match[1]
		value := strings.TrimSpace(match[2])
		switch key {
		case "id":
			theme.ID = value
		case "name":
			theme.Name = value
		case "description":
			theme.Description = value
		case "is-dark":
			theme.IsDark = value == "true"
		case "preview-bg":
			if theme.Preview == nil {
				theme.Preview = &ThemePreview{}
			}
			theme.Preview.BG = value
		case "preview-surface":
			if theme.Preview == nil {
				theme.Preview = &ThemePreview{}
			}
			theme.Preview.Surface = value
		case "preview-accent":
			if theme.Preview == nil {
				theme.Preview = &ThemePreview{}
			}
			theme.Preview.Accent = value
		case "preview-text":
			if theme.Preview == nil {
				theme.Preview = &ThemePreview{}
			}
			theme.Preview.Text = value
		}
	}

	return theme
}

// sanitizeThemeID converts a theme name to a safe filesystem ID
func sanitizeThemeID(name string) string {
	// Lowercase, replace spaces/special chars with hyphens
	id := strings.ToLower(name)
	id = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(id, "-")
	id = regexp.MustCompile(`-+`).ReplaceAllString(id, "-")
	id = strings.Trim(id, "-")
	return id
}

// generateThemeCSS creates a complete CSS file for a custom theme
func generateThemeCSS(id string, req ThemeSaveRequest) string {
	// Determine preview colors from variables
	previewBG := req.Variables["--bg-base"]
	previewSurface := req.Variables["--bg-surface"]
	previewAccent := req.Variables["--accent-primary"]
	previewText := req.Variables["--text-primary"]

	if previewBG == "" {
		previewBG = "#09090b"
	}
	if previewSurface == "" {
		previewSurface = "#111114"
	}
	if previewAccent == "" {
		previewAccent = "#2dd4bf"
	}
	if previewText == "" {
		previewText = "#fafafa"
	}

	colorScheme := "dark"
	if !req.IsDark {
		colorScheme = "light"
	}

	var sb strings.Builder

	// Metadata comments
	sb.WriteString(fmt.Sprintf("/**\n * %s - Custom Theme for Muximux\n", req.Name))
	sb.WriteString(fmt.Sprintf(" * Based on: %s\n", req.BaseTheme))
	sb.WriteString(" *\n")
	sb.WriteString(fmt.Sprintf(" * @theme-id: %s\n", id))
	sb.WriteString(fmt.Sprintf(" * @theme-name: %s\n", req.Name))
	sb.WriteString(fmt.Sprintf(" * @theme-description: Custom theme based on %s\n", req.BaseTheme))
	sb.WriteString(fmt.Sprintf(" * @theme-is-dark: %v\n", req.IsDark))
	sb.WriteString(fmt.Sprintf(" * @theme-preview-bg: %s\n", previewBG))
	sb.WriteString(fmt.Sprintf(" * @theme-preview-surface: %s\n", previewSurface))
	sb.WriteString(fmt.Sprintf(" * @theme-preview-accent: %s\n", previewAccent))
	sb.WriteString(fmt.Sprintf(" * @theme-preview-text: %s\n", previewText))
	sb.WriteString(" */\n\n")

	// CSS selector
	sb.WriteString(fmt.Sprintf("[data-theme=\"%s\"] {\n", id))
	sb.WriteString(fmt.Sprintf("  color-scheme: %s;\n\n", colorScheme))

	// Write all variables
	for varName, varValue := range req.Variables {
		sb.WriteString(fmt.Sprintf("  %s: %s;\n", varName, varValue))
	}

	sb.WriteString("}\n")

	return sb.String()
}
