package icons

import (
	"os"
	"path/filepath"
	"time"

	"github.com/mescon/muximux/v3/internal/logging"
)

// StartCacheCleanup runs a periodic goroutine that removes expired icon cache files.
// It checks cacheDirs for files with ModTime older than maxAge and deletes them.
// The goroutine stops when done is closed.
func StartCacheCleanup(cacheDirs []string, maxAge time.Duration, done <-chan struct{}) {
	go func() {
		// Initial delay to avoid competing with startup I/O
		select {
		case <-time.After(30 * time.Second):
		case <-done:
			return
		}

		runCleanup(cacheDirs, maxAge)

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				runCleanup(cacheDirs, maxAge)
			case <-done:
				return
			}
		}
	}()
}

func runCleanup(cacheDirs []string, maxAge time.Duration) {
	total := 0
	for _, dir := range cacheDirs {
		n, err := cleanDir(dir, maxAge)
		if err != nil {
			logging.Warn("Icon cache cleanup error", "source", "icons", "dir", dir, "error", err)
		}
		total += n
	}
	if total > 0 {
		logging.Info("Icon cache cleanup: removed expired files", "source", "icons", "count", total)
	} else {
		logging.Debug("Icon cache cleanup: nothing to remove", "source", "icons")
	}
}

// cleanDir removes files in dir whose ModTime is older than maxAge.
// Returns the number of files removed.
func cleanDir(dir string, maxAge time.Duration) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(filepath.Join(dir, e.Name())); err == nil {
				removed++
			}
		}
	}

	return removed, nil
}
