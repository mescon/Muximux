package icons

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

//go:embed builtin/*.svg
var builtinIcons embed.FS

// BuiltinIconInfo represents a builtin icon
type BuiltinIconInfo struct {
	Name string `json:"name"`
}

// GetBuiltinIcon returns a builtin icon by name
func GetBuiltinIcon(name string) ([]byte, string, error) {
	// Normalize name - remove extension if present
	name = strings.TrimSuffix(name, ".svg")

	path := fmt.Sprintf("builtin/%s.svg", name)
	data, err := builtinIcons.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("builtin icon not found: %s", name)
	}

	return data, "image/svg+xml", nil
}

// ListBuiltinIcons returns all available builtin icons
func ListBuiltinIcons() ([]BuiltinIconInfo, error) {
	var icons []BuiltinIconInfo

	err := fs.WalkDir(builtinIcons, "builtin", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".svg" {
			name := strings.TrimSuffix(filepath.Base(path), ".svg")
			icons = append(icons, BuiltinIconInfo{Name: name})
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return icons, nil
}

// SearchBuiltinIcons searches builtin icons by name
func SearchBuiltinIcons(query string) ([]BuiltinIconInfo, error) {
	all, err := ListBuiltinIcons()
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var results []BuiltinIconInfo
	for _, icon := range all {
		if strings.Contains(strings.ToLower(icon.Name), query) {
			results = append(results, icon)
		}
	}

	return results, nil
}
