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

	"github.com/mescon/muximux/v3/internal/logging"
)

var (
	reNonAlnum    = regexp.MustCompile(`[^a-z0-9-]`)
	reMultiDash   = regexp.MustCompile(`-+`)
	reThemeMeta   = regexp.MustCompile(`@theme-(\w[\w-]*):\s*(.+)`)
	reCSSVarName  = regexp.MustCompile(`^--[a-z][a-z0-9-]*$`)
	reCSSBadValue = regexp.MustCompile(`(?i)(url\s*\(|@import|expression\s*\(|javascript:|\\00)`)
)

// ThemeHandler handles custom theme CRUD operations
type ThemeHandler struct {
	themesDir string
	bundledFS fs.FS // embedded filesystem containing bundled theme CSS files (themes/*.css)
}

// NewThemeHandler creates a new theme handler.
// bundledFS should be the embedded dist filesystem (or nil if unavailable).
func NewThemeHandler(themesDir string, bundledFS fs.FS) *ThemeHandler {
	if err := os.MkdirAll(themesDir, 0755); err != nil {
		logging.Error("Failed to create themes directory", "source", "themes", "path", themesDir, "error", err)
	}
	return &ThemeHandler{themesDir: themesDir, bundledFS: bundledFS}
}

// ThemeInfo represents theme metadata for the API
type ThemeInfo struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	IsBuiltin   bool          `json:"isBuiltin"`
	IsDark      bool          `json:"isDark"`
	Family      string        `json:"family,omitempty"`
	Variant     string        `json:"variant,omitempty"`
	FamilyName  string        `json:"familyName,omitempty"`
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
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Author      string            `json:"author"`
	BaseTheme   string            `json:"baseTheme"`
	IsDark      bool              `json:"isDark"`
	Variables   map[string]string `json:"variables"`
}

// ListThemes returns all available themes (bundled + user-created).
// Bundled themes are read from the embedded filesystem, user themes from disk.
// User themes with the same ID override bundled ones.
func (h *ThemeHandler) ListThemes(w http.ResponseWriter, r *http.Request) {
	themeMap := make(map[string]ThemeInfo)

	h.loadBundledThemes(themeMap)
	h.loadUserThemes(themeMap)

	// Collect and sort by name
	themes := make([]ThemeInfo, 0, len(themeMap))
	for _, t := range themeMap {
		themes = append(themes, t)
	}
	sort.Slice(themes, func(i, j int) bool {
		return themes[i].Name < themes[j].Name
	})

	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(themes)
}

// loadBundledThemes scans bundled themes from the embedded filesystem and adds them to the map.
func (h *ThemeHandler) loadBundledThemes(themeMap map[string]ThemeInfo) {
	if h.bundledFS == nil {
		return
	}
	entries, err := fs.ReadDir(h.bundledFS, "themes")
	if err != nil {
		return
	}
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

// loadUserThemes scans user-created themes from disk and adds them to the map,
// overriding bundled themes with the same ID.
func (h *ThemeHandler) loadUserThemes(themeMap map[string]ThemeInfo) {
	entries, err := os.ReadDir(h.themesDir)
	if err != nil {
		return
	}
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

// SaveTheme creates or updates a custom theme
func (h *ThemeHandler) SaveTheme(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
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

	// Validate CSS variable names and values to prevent CSS injection
	for varName, varValue := range req.Variables {
		if !reCSSVarName.MatchString(varName) {
			http.Error(w, fmt.Sprintf("Invalid CSS variable name: %s", varName), http.StatusBadRequest)
			return
		}
		if strings.Contains(varValue, "}") || strings.Contains(varValue, "{") || reCSSBadValue.MatchString(varValue) {
			http.Error(w, fmt.Sprintf("Invalid CSS variable value for %s", varName), http.StatusBadRequest)
			return
		}
	}

	// Generate CSS content
	css := generateThemeCSS(id, &req)

	// Write to file
	filename := filepath.Join(h.themesDir, id+".css")
	if err := os.WriteFile(filename, []byte(css), 0600); err != nil {
		logging.Error("Failed to save theme file", "source", "themes", "theme", id, "error", err)
		http.Error(w, "Failed to save theme", http.StatusInternalServerError)
		return
	}

	logging.Info("Theme saved", "source", "themes", "theme", id)
	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(map[string]string{
		"id":     id,
		"status": "saved",
	})
}

// DeleteTheme removes a custom theme
func (h *ThemeHandler) DeleteTheme(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	// Extract theme name from path: /api/themes/{name}
	name := strings.TrimPrefix(r.URL.Path, "/api/themes/")
	if name == "" {
		http.Error(w, "Theme name required", http.StatusBadRequest)
		return
	}

	// Sanitize to prevent path traversal — only allow [a-z0-9-]
	name = sanitizeThemeID(name)
	if name == "" {
		http.Error(w, "Invalid theme name", http.StatusBadRequest)
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
			logging.Error("Failed to delete theme file", "source", "themes", "theme", name, "error", err)
			http.Error(w, "Failed to delete theme", http.StatusInternalServerError)
		}
		return
	}

	logging.Info("Theme deleted", "source", "themes", "theme", name)
	w.Header().Set(headerContentType, contentTypeJSON)
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
func parseThemeMetadata(content, filename string) *ThemeInfo {
	id := strings.TrimSuffix(filename, ".css")

	theme := &ThemeInfo{
		ID:        id,
		Name:      id,
		IsBuiltin: false,
		IsDark:    true,
	}

	// Parse @theme-* metadata comments
	matches := reThemeMeta.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		key := match[1]
		value := strings.TrimSpace(match[2])
		switch key {
		case "name":
			theme.Name = value
		case "description":
			theme.Description = value
		case "is-dark":
			theme.IsDark = value == "true"
		case "family":
			theme.Family = value
		case "family-name":
			theme.FamilyName = value
		case "variant":
			theme.Variant = value
		case "preview-bg", "preview-surface", "preview-accent", "preview-text":
			if theme.Preview == nil {
				theme.Preview = &ThemePreview{}
			}
			switch key {
			case "preview-bg":
				theme.Preview.BG = value
			case "preview-surface":
				theme.Preview.Surface = value
			case "preview-accent":
				theme.Preview.Accent = value
			case "preview-text":
				theme.Preview.Text = value
			}
		}
	}

	return theme
}

// sanitizeThemeID converts a theme name to a safe filesystem ID
func sanitizeThemeID(name string) string {
	id := strings.ToLower(name)
	id = reNonAlnum.ReplaceAllString(id, "-")
	id = reMultiDash.ReplaceAllString(id, "-")
	return strings.Trim(id, "-")
}

// generateThemeCSS creates a complete CSS file for a custom theme
func generateThemeCSS(id string, req *ThemeSaveRequest) string {
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

	// Build description
	description := req.Description
	if description == "" {
		description = fmt.Sprintf("Custom theme based on %s", req.BaseTheme)
	}

	// Metadata comments
	sb.WriteString(fmt.Sprintf("/**\n * %s - Custom Theme for Muximux\n", req.Name))
	if req.Author != "" {
		sb.WriteString(fmt.Sprintf(" * Author: %s\n", req.Author))
	}
	sb.WriteString(fmt.Sprintf(" * Based on: %s\n", req.BaseTheme))
	sb.WriteString(" *\n")
	sb.WriteString(fmt.Sprintf(" * @theme-id: %s\n", id))
	sb.WriteString(fmt.Sprintf(" * @theme-name: %s\n", req.Name))
	sb.WriteString(fmt.Sprintf(" * @theme-description: %s\n", description))
	sb.WriteString(fmt.Sprintf(" * @theme-is-dark: %v\n", req.IsDark))
	sb.WriteString(fmt.Sprintf(" * @theme-preview-bg: %s\n", previewBG))
	sb.WriteString(fmt.Sprintf(" * @theme-preview-surface: %s\n", previewSurface))
	sb.WriteString(fmt.Sprintf(" * @theme-preview-accent: %s\n", previewAccent))
	sb.WriteString(fmt.Sprintf(" * @theme-preview-text: %s\n", previewText))
	sb.WriteString(" */\n\n")

	// CSS selector
	fmt.Fprintf(&sb, "[data-theme=%q] {\n", id)
	sb.WriteString(fmt.Sprintf("  color-scheme: %s;\n\n", colorScheme))

	// Write all variables
	for varName, varValue := range req.Variables {
		sb.WriteString(fmt.Sprintf("  %s: %s;\n", varName, varValue))
	}

	sb.WriteString("}\n")

	return sb.String()
}
