package handlers

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ThemeHandler handles custom theme CRUD operations
type ThemeHandler struct {
	themesDir  string
	bundledFS  fs.FS // embedded filesystem containing bundled theme CSS files (themes/*.css)
}

// NewThemeHandler creates a new theme handler.
// bundledFS should be the embedded dist filesystem (or nil if unavailable).
func NewThemeHandler(themesDir string, bundledFS fs.FS) *ThemeHandler {
	os.MkdirAll(themesDir, 0755)
	return &ThemeHandler{themesDir: themesDir, bundledFS: bundledFS}
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

// ListThemes returns all available themes (bundled + user-created).
// Bundled themes are read from the embedded filesystem, user themes from disk.
// User themes with the same ID override bundled ones.
func (h *ThemeHandler) ListThemes(w http.ResponseWriter, r *http.Request) {
	themeMap := make(map[string]ThemeInfo)

	// 1. Scan bundled themes from embedded filesystem
	if h.bundledFS != nil {
		entries, err := fs.ReadDir(h.bundledFS, "themes")
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".css") {
					continue
				}
				data, err := fs.ReadFile(h.bundledFS, "themes/"+entry.Name())
				if err != nil {
					continue
				}
				theme := parseThemeMetadata(string(data), entry.Name())
				if theme != nil {
					theme.IsBuiltin = true
					themeMap[theme.ID] = *theme
				}
			}
		}
	}

	// 2. Scan user-created themes from disk (override bundled if same ID)
	entries, err := os.ReadDir(h.themesDir)
	if err == nil {
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
				theme.IsBuiltin = false
				themeMap[theme.ID] = *theme
			}
		}
	}

	// 3. Collect and sort by name
	themes := make([]ThemeInfo, 0, len(themeMap))
	for _, t := range themeMap {
		themes = append(themes, t)
	}
	sort.Slice(themes, func(i, j int) bool {
		return themes[i].Name < themes[j].Name
	})

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

	// Don't allow deleting builtin themes (dark/light are CSS-only, others are bundled files)
	if name == "dark" || name == "light" || h.isBundledTheme(name) {
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

// isBundledTheme checks whether a theme ID exists in the embedded filesystem
func (h *ThemeHandler) isBundledTheme(id string) bool {
	if h.bundledFS == nil {
		return false
	}
	_, err := fs.Stat(h.bundledFS, "themes/"+id+".css")
	return err == nil
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
