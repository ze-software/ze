// Design: (none -- test utility, no architecture doc)

package golden

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
)

// DynamicPattern is a route registration whose pattern is not a string literal,
// so reading the source cannot say which route it wires.
type DynamicPattern struct {
	// Expr is the first argument, as it is written in the source.
	Expr string
	// RangeOver is the expression the enclosing range statement iterates, as it
	// is written in the source. It is empty when the call sits in no range.
	//
	// It names the SET a loop registers, which the pattern expression does not.
	// A loop repointed at another registry goes on writing route.Pattern. A
	// caller that accepts the expression alone therefore accepts a capture of a
	// registry the server no longer serves.
	RangeOver string
	// Pos is the file and line of the call.
	Pos string
}

// RoutePatterns reads one Go file and returns the route patterns it registers.
//
// The literal patterns are the ones a mux serves under a name the source
// states. The dynamic ones carry a pattern the source computes. The caller must
// then say where that pattern comes from. It is either a registry the test can
// read at run time, or a hole in the capture.
//
// Reading the source is what keeps the route list out of the test. A route
// added to that file later appears here with no edit. The coverage check then
// fails by name, instead of passing over a route nobody captured.
func RoutePatterns(t *testing.T, path string) ([]string, []DynamicPattern) {
	t.Helper()

	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s for its route registrations: %v", path, err)
	}

	scan := &routeScan{t: t, fset: fset}
	scan.walk(file, "")

	sort.Strings(scan.literal)

	if len(scan.literal) == 0 {
		t.Fatalf("%s registers no route with a literal pattern; the capture has lost its route list", path)
	}

	return scan.literal, scan.dynamic
}

// routeScan collects the registrations of one parsed file.
type routeScan struct {
	t       *testing.T
	fset    *token.FileSet
	literal []string
	dynamic []DynamicPattern
}

// walk reads one subtree. rangeOver names the set the enclosing range iterates,
// and is empty at the top level. A range statement is read by hand, in two
// parts. A registration inside the body then carries the set the loop reads,
// and the loop subject is read at the depth it sits at.
func (s *routeScan) walk(node ast.Node, rangeOver string) {
	ast.Inspect(node, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.RangeStmt:
			s.walk(v.X, rangeOver)
			s.walk(v.Body, s.source(v.X))

			return false
		case *ast.CallExpr:
			s.record(v, rangeOver)

			return true
		default:
			return true
		}
	})
}

// record keeps one call when it registers a route.
func (s *routeScan) record(call *ast.CallExpr, rangeOver string) {
	s.t.Helper()

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) == 0 {
		return
	}

	if sel.Sel.Name != "Handle" && sel.Sel.Name != "HandleFunc" {
		return
	}

	if lit, isLit := call.Args[0].(*ast.BasicLit); isLit && lit.Kind == token.STRING {
		pattern, unquoteErr := strconv.Unquote(lit.Value)
		if unquoteErr != nil {
			s.t.Fatalf("%s: route pattern %s does not unquote: %v",
				s.fset.Position(lit.Pos()), lit.Value, unquoteErr)
		}

		s.literal = append(s.literal, pattern)

		return
	}

	s.dynamic = append(s.dynamic, DynamicPattern{
		Expr:      s.source(call.Args[0]),
		RangeOver: rangeOver,
		Pos:       s.fset.Position(call.Pos()).String(),
	})
}

// source returns one node as it is written in the file.
func (s *routeScan) source(node ast.Node) string {
	s.t.Helper()

	var out bytes.Buffer
	if err := printer.Fprint(&out, s.fset, node); err != nil {
		s.t.Fatalf("%s: print the source of a route registration: %v", s.fset.Position(node.Pos()), err)
	}

	return out.String()
}

// RepoFile returns the absolute path of a repository-relative file, found by
// walking up from the test's working directory to the directory holding go.mod.
// A capture that reads a file outside its own package needs the path to survive
// the package moving.
func RepoFile(t *testing.T, rel string) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}

	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			path := filepath.Join(dir, rel)
			if _, statErr = os.Stat(path); statErr != nil {
				t.Fatalf("the capture reads %s, which is not there: %v", rel, statErr)
			}

			return path
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod above the working directory; cannot locate %s", rel)
		}

		dir = parent
	}
}
