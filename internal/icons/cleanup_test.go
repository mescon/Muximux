package icons

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanDir_RemovesOldFiles(t *testing.T) {
	dir := t.TempDir()

	// Create files with old modification times
	for _, name := range []string{"old1.svg", "old2.png"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("data"), 0600); err != nil {
			t.Fatal(err)
		}
		// Set modtime to 30 days ago
		old := time.Now().Add(-30 * 24 * time.Hour)
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}

	removed, err := cleanDir(dir, 14*24*time.Hour) // maxAge = 14 days
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 2 {
		t.Errorf("expected 2 removed, got %d", removed)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("expected empty dir, got %d files", len(entries))
	}
}

func TestCleanDir_KeepsRecentFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a recent file (default modtime is now)
	if err := os.WriteFile(filepath.Join(dir, "recent.svg"), []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create an old file
	oldPath := filepath.Join(dir, "old.svg")
	if err := os.WriteFile(oldPath, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-30 * 24 * time.Hour)
	if err := os.Chtimes(oldPath, old, old); err != nil {
		t.Fatal(err)
	}

	removed, err := cleanDir(dir, 14*24*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 remaining file, got %d", len(entries))
	}
	if entries[0].Name() != "recent.svg" {
		t.Errorf("expected recent.svg to remain, got %s", entries[0].Name())
	}
}

func TestCleanDir_NonexistentDir(t *testing.T) {
	removed, err := cleanDir("/nonexistent/path/icons", 24*time.Hour)
	if err != nil {
		t.Errorf("expected nil error for nonexistent dir, got %v", err)
	}
	if removed != 0 {
		t.Errorf("expected 0 removed, got %d", removed)
	}
}

func TestCleanDir_SkipsDirectories(t *testing.T) {
	dir := t.TempDir()

	// Create a subdirectory (should not be deleted)
	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-30 * 24 * time.Hour)
	if err := os.Chtimes(subdir, old, old); err != nil {
		t.Fatal(err)
	}

	removed, err := cleanDir(dir, 14*24*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 0 {
		t.Errorf("expected 0 removed (subdirs skipped), got %d", removed)
	}
}
