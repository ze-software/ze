// Design: docs/contributing/ze-go-style.md -- end-to-end tests for `./le go style check`
//
// VALIDATES: CheckTreePaths runs format, AST style, compound-guard,
// refuse-by-default, and error-wrap checks in one fast pass over named files,
// reporting each category cleanly and returning exit 0 on compliant files.
// PREVENTS: subagents and developers missing AST style gates until post-push CI.
package gostyle

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writeFixtureFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCheckTreePathsRunsAllFastASTChecks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFixtureFile(t, root, "go.mod", "module example.test/demo\n\ngo 1.24\n")
	writeFixtureFile(t, root, "internal/demo/bad.go", `// Design: docs/contributing/ze-go-style.md -- fixture
package demo

import (
	"errors"
	"fmt"
)

func leaf() error {
	return errors.New("parse config: bad")
}

func bad(a, b bool) error {
	if a || b {
		return errors.New("invalid")
	}
	err := leaf()
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	_ = fmt.Sprintf("%v", a)
	return nil
}
`)

	report, err := CheckTreePaths(root, []string{"internal/demo/bad.go"})
	if err != nil {
		t.Fatalf("CheckTreePaths: %v", err)
	}
	if report.Files != 1 {
		t.Fatalf("Files = %d, want 1", report.Files)
	}
	checks := make([]string, 0, len(report.Issues))
	for _, issue := range report.Issues {
		checks = append(checks, issue.Check)
	}
	for _, want := range []string{"go-style", "compound-guard", "error-wrap"} {
		if !slices.Contains(checks, want) {
			t.Errorf("missing check %q in issues %+v", want, report.Issues)
		}
	}
}

func TestCheckTreePathsPassesCompliantFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFixtureFile(t, root, "go.mod", "module example.test/demo\n\ngo 1.24\n")
	writeFixtureFile(t, root, "internal/demo/clean.go", `// Design: docs/contributing/ze-go-style.md -- fixture
package demo

import "errors"

func clean(a, b bool) error {
	if a {
		return errors.New("a is set")
	}
	if b {
		return errors.New("b is set")
	}
	return nil
}
`)

	report, err := CheckTreePaths(root, []string{"internal/demo/clean.go"})
	if err != nil {
		t.Fatalf("CheckTreePaths: %v", err)
	}
	if len(report.Issues) != 0 {
		t.Fatalf("unexpected issues:\n%s", report.Text())
	}
	if report.exitCode() != 0 {
		t.Fatalf("exitCode = %d, want 0", report.exitCode())
	}
}
