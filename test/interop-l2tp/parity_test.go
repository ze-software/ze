package interop_l2tp_test

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: The L2TP fixture tree has no interpreter-backed source or cache.
// PREVENTS: Reintroducing an executable fallback beside the typed plans.
func TestL2TPTreeIsGoOnly(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && entry.Name() == "__pycache__" {
			t.Errorf("Python cache directory remains: %s", path)
			return filepath.SkipDir
		}
		if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".py") ||
			strings.HasSuffix(entry.Name(), ".pyc") || strings.HasSuffix(entry.Name(), ".sh")) {
			t.Errorf("interpreter-backed artifact remains: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
