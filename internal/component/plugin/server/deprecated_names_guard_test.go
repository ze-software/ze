package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoProductionDeprecatedCommandNames enforces the pre-release policy that
// command migrations are clean cutovers. The alias infrastructure remains in
// place, but product command declarations must not preserve unreleased names.
//
// VALIDATES: no non-test Go source declares CommandDecl.DeprecatedNames.
// PREVENTS: accidentally keeping invalid pre-release command spellings alive.
func TestNoProductionDeprecatedCommandNames(t *testing.T) {
	for _, root := range []string{"cmd", "internal", "pkg"} {
		if err := filepath.WalkDir(repoPath(root), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				switch d.Name() {
				case "vendor", ".git":
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path) //nolint:gosec // test scans repository source files
			if err != nil {
				return err
			}
			if strings.Contains(string(data), "DeprecatedNames:") {
				t.Errorf("production command declaration uses DeprecatedNames: %s", path)
			}
			return nil
		}); err != nil {
			t.Fatalf("scan %s: %v", root, err)
		}
	}
}

func repoPath(parts ...string) string {
	elems := append([]string{"..", "..", "..", ".."}, parts...)
	return filepath.Join(elems...)
}
