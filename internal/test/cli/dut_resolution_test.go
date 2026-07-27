package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// VALIDATES: every suite runner in this package obtains the ze binary under test
// from buildZe (which honors ZE_BIN, falls back to this session's bin/, and
// builds only when asked to), rather than composing a path to <baseDir>/bin/ze
// itself.
// PREVENTS: the failure that made every CI run red for the whole web suite.
// cmd_web.go built its own <baseDir>/bin/ze path, but the functional flow builds
// its isolated set into tmp/testbin-*/bin and exports ZE_BIN there
// (mk/test-functional.mk), leaving <baseDir>/bin empty. On a fresh checkout all
// 87 .wb tests died in ~4ms each on "fork/exec bin/ze: no such file or
// directory"; on a developer host a leftover bin/ze hid it AND meant the suite
// tested a binary that was not the one under test.
//
// The check is structural because the behavioral path is not reachable from a
// unit test: cmdWebMain resolves baseDir with FindBaseDir() (the real repo root)
// and then needs agent-browser and a browser to go further.
func TestSuiteRunnersResolveDUTThroughBuildZe(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	fset := token.NewFileSet()
	found := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, name, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", name, parseErr)
		}
		found++

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isFilepathJoin(call.Fun) {
				return true
			}
			for i, arg := range call.Args {
				if stringLit(arg) != "bin" || i+1 >= len(call.Args) {
					continue
				}
				// filepath.Join(x, "bin") alone is a directory, which is what
				// internal/test/sessionpath legitimately computes. And a "bin"
				// segment under the test tree names a fixture wrapper
				// (test/exabgp/bin/exabgp), not the DUT. Only a ze binary name
				// after it is this rule's subject.
				if next := stringLit(call.Args[i+1]); isZeBinary(next) {
					t.Errorf("%s: hardcodes the %s path via filepath.Join(..., \"bin\", %q); "+
						"call buildZe(ctx, baseDir) instead so ZE_BIN is honored",
						fset.Position(call.Pos()), next, next)
				}
			}
			return true
		})
	}

	if found == 0 {
		t.Fatal("no non-test Go files parsed: the guard would pass vacuously")
	}
}

// stringLit returns the value of a string literal expression, or "" when the
// expression is not one.
func stringLit(e ast.Expr) string {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return ""
	}
	return value
}

// isZeBinary reports whether name is one of the binaries a suite runs as the
// device under test. These are the ones mk/test-functional.mk builds into its
// isolated dir and points ZE_BIN/ZE_TEST_BIN at.
func isZeBinary(name string) bool {
	switch name {
	case "ze", "ze-test", "ze-stripped":
		return true
	}
	return false
}

func isFilepathJoin(fun ast.Expr) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Join" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "filepath"
}

// VALIDATES: cmd_web.go names buildZe, so the guard above cannot be satisfied by
// deleting the binary lookup altogether.
// PREVENTS: a future refactor that drops the resolver and reintroduces an
// implicit dependency on a stale <repo>/bin/ze.
func TestWebSuiteCallsBuildZe(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(".", "cmd_web.go"))
	if err != nil {
		t.Fatalf("read cmd_web.go: %v", err)
	}
	if !strings.Contains(string(src), "buildZe(ctx, baseDir)") {
		t.Error("cmd_web.go must resolve the ze binary with buildZe(ctx, baseDir)")
	}
}
