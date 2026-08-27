// VALIDATES: every development tool is compiled code and its tests call Go functions.
// PREVENTS: build-ignored tools or subprocess-driven go-run tests returning.
package le

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
)

func walkLeGo(t *testing.T, visit func(rel string, body string)) {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find checkout: %v", err)
	}
	dir := filepath.Join(root, "internal", "le")
	err = filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return walkErr
		}
		body, readErr := os.ReadFile(path) //nolint:gosec // path is inside this checkout
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		visit(rel, string(body))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
}

func TestNoDevelopmentToolIsBuildIgnored(t *testing.T) {
	walkLeGo(t, func(rel, body string) {
		header, _, _ := strings.Cut(body, "\npackage ")
		if strings.Contains(header, "//go:build ignore") {
			t.Errorf("%s is constrained out of every build", rel)
		}
	})
}

func TestNoDevelopmentToolTestShellsOutToGoRun(t *testing.T) {
	walkLeGo(t, func(rel, body string) {
		if !strings.HasSuffix(rel, "_test.go") {
			return
		}
		forms := []string{`"go", ` + `"run"`, `"go",` + `"run"`, `go ` + `run `}
		for _, form := range forms {
			if strings.Contains(body, form) {
				t.Errorf("%s invokes a compiled tool with %s", rel, form)
			}
		}
	})
}
