package icons

import (
	"strings"
	"testing"
)

func TestGetBuiltinIcon(t *testing.T) {
	t.Run("existing icon", func(t *testing.T) {
		data, contentType, err := GetBuiltinIcon("home")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if contentType != "image/svg+xml" {
			t.Errorf("expected 'image/svg+xml', got %q", contentType)
		}
		if len(data) == 0 {
			t.Error("expected non-empty SVG data")
		}
		if !strings.Contains(string(data), "<svg") {
			t.Error("expected SVG content")
		}
	})

	t.Run("existing icon with .svg extension", func(t *testing.T) {
		data, contentType, err := GetBuiltinIcon("home.svg")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if contentType != "image/svg+xml" {
			t.Errorf("expected 'image/svg+xml', got %q", contentType)
		}
		if len(data) == 0 {
			t.Error("expected non-empty data")
		}
	})

	t.Run("non-existent icon", func(t *testing.T) {
		_, _, err := GetBuiltinIcon("does-not-exist-12345")
		if err == nil {
			t.Error("expected error for non-existent icon")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got %q", err.Error())
		}
	})

	t.Run("multiple icons", func(t *testing.T) {
		names := []string{"settings", "globe", "terminal", "star", "search"}
		for _, name := range names {
			data, _, err := GetBuiltinIcon(name)
			if err != nil {
				t.Errorf("expected icon %q to exist: %v", name, err)
				continue
			}
			if len(data) == 0 {
				t.Errorf("expected non-empty data for %q", name)
			}
		}
	})
}

func TestListBuiltinIcons(t *testing.T) {
	icons, err := ListBuiltinIcons()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(icons) == 0 {
		t.Error("expected at least one builtin icon")
	}

	// Verify known icons are present
	knownIcons := map[string]bool{
		"home":     false,
		"settings": false,
		"globe":    false,
		"star":     false,
		"terminal": false,
	}

	for _, icon := range icons {
		if icon.Name == "" {
			t.Error("expected non-empty icon name")
		}
		if _, ok := knownIcons[icon.Name]; ok {
			knownIcons[icon.Name] = true
		}
	}

	for name, found := range knownIcons {
		if !found {
			t.Errorf("expected builtin icon %q to be in the list", name)
		}
	}
}

func TestSearchBuiltinIcons(t *testing.T) {
	t.Run("matching", func(t *testing.T) {
		results, err := SearchBuiltinIcons("home")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) == 0 {
			t.Error("expected at least one result for 'home'")
		}
		found := false
		for _, r := range results {
			if r.Name == "home" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected 'home' to be in results")
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		results, err := SearchBuiltinIcons("HOME")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) == 0 {
			t.Error("expected at least one result for 'HOME'")
		}
	})

	t.Run("partial match", func(t *testing.T) {
		results, err := SearchBuiltinIcons("ar")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should match "star", "chart", "calendar", "camera", "share", "shopping-cart"
		if len(results) == 0 {
			t.Error("expected at least one result for 'ar'")
		}
	})

	t.Run("no results", func(t *testing.T) {
		results, err := SearchBuiltinIcons("zzzzzzzzz")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})
}
