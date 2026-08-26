// The migration's two structural promises, checked over le's own trees rather
// than described in a comment: a tool is COMPILED code, and its test CALLS it.
//
// Both are green today because no tool has been ported yet, and both stay
// green only if every port keeps them. That is the point: they are cheap now
// and they are the guard for every step after this one.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// leSourceTrees are every directory this binary's code lives in.
var leSourceTrees = []string{"cmd/le", "letools"}

// walkGo calls visit for every .go file under each of le's source trees.
func walkGo(t *testing.T, visit func(rel string, body string)) {
	t.Helper()
	root := repoRoot(t)
	for _, tree := range leSourceTrees {
		dir := filepath.Join(root, tree)
		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			body, readErr := os.ReadFile(path) //nolint:gosec // path comes from a walk of this repository
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
}

// TestNoPortedToolIsBuildIgnored is AC-4. `//go:build ignore` is what keeps
// the 44 existing tool files out of `go build ./...`, out of `go vet` and out
// of the linter. A tool that arrives here carrying it has been moved, not
// ported.
func TestNoPortedToolIsBuildIgnored(t *testing.T) {
	// A build constraint is only a build constraint above the package clause,
	// so that is where this looks. It also keeps this file inside its own
	// check: the tag is named in the comment above, which is below the clause.
	walkGo(t, func(rel, body string) {
		header, _, _ := strings.Cut(body, "\npackage ")
		if strings.Contains(header, "//go:build ignore") {
			t.Errorf("%s is constrained out of every build: nothing compiles it, so nothing checks it", rel)
		}
	})
}

// TestNoTestShellsOutToGoRun is AC-5. A test that forks `go run` relinks the
// tool for every case and asserts against a process rather than a function.
// The whole point of compiling these tools is that their tests can call them.
//
// It reads TEXT, so a test that ASSERTS an argv also triggers it.
// letools/gotoolchain builds `go run` command lines, and its test says so. Spell
// that assertion without the two adjacent words. Do not soften the scan. A
// pattern that separates the two cases must understand the surrounding call.
// The scan's value comes from refusing such arguments.
func TestNoTestShellsOutToGoRun(t *testing.T) {
	walkGo(t, func(rel, body string) {
		if !strings.HasSuffix(rel, "_test.go") {
			return
		}
		// Assembled for the same reason as the tag above.
		forms := []string{`"go", ` + `"run"`, `"go",` + `"run"`, `go ` + `run `}
		for _, form := range forms {
			if strings.Contains(body, form) {
				t.Errorf("%s invokes a tool with %s: a compiled tool is called, not forked", rel, form)
			}
		}
	})
}
