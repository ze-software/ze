package reactor

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// freeCalls names the calls that hand an entry's memory back. Each one makes
// the slot available to another goroutine's getReadBuf, so every one of them
// must come after the walk that reads it.
var freeCalls = [...]string{"ReturnReadBuffer", "returnFwdHandles"}

// walkCall is the identifier release. It parses the entry's Withdrawn Routes
// and MP_UNREACH sections, which are slices into the same pooled buffer.
const walkCall = "fwdReleaseWithdrawnPathIDs"

// TestEvictionWalksTheBodyBeforeItFreesTheBuffer holds the ordering that keeps
// the RFC 7911 identifier release reading the UPDATE it belongs to.
//
// VALIDATES: in both eviction paths, fwdReleaseWithdrawnPathIDs is called
// before ReturnReadBuffer and before returnFwdHandles.
// PREVENTS: the order this repository shipped until 2026-09-03, where the read
// buffer went back to the pool first. Another goroutine can take that slot
// between the two statements, so the walk parsed the next UPDATE's bytes: it
// released identifiers belonging to no path, and the paths this UPDATE really
// withdrew were never freed. A -race run caught it as a write/read pair on one
// address; the order is what the defect actually is, so the order is what this
// asserts.
//
// It reads the source rather than running the code because the failure is an
// interleaving. A behavioral test would need the pool to hand the slot to a
// second goroutine inside a two-statement window, which no assertion can make
// happen on demand.
func TestEvictionWalksTheBodyBeforeItFreesTheBuffer(t *testing.T) {
	const path = "recent_cache.go"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	checked := 0
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if fn.Name.Name != "evictLocked" && fn.Name.Name != "Delete" {
			continue
		}
		checked++
		checkReleaseOrder(t, fset, fn)
	}
	if checked != 2 {
		t.Fatalf("found %d of the 2 eviction paths in %s, so this guard reads less than it claims", checked, path)
	}
}

// checkReleaseOrder reports a free that a body performs before its walk.
func checkReleaseOrder(t *testing.T, fset *token.FileSet, fn *ast.FuncDecl) {
	t.Helper()

	walkAt := token.NoPos
	frees := map[string]token.Pos{}
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := calleeName(call.Fun)
		if name == walkCall && walkAt == token.NoPos {
			walkAt = call.Pos()
			return true
		}
		for _, free := range &freeCalls {
			if name == free {
				if _, seen := frees[free]; !seen {
					frees[free] = call.Pos()
				}
			}
		}
		return true
	})

	if walkAt == token.NoPos {
		t.Errorf("%s no longer calls %s, so a withdrawn path's identifier is never freed", fn.Name.Name, walkCall)
		return
	}
	for _, free := range &freeCalls {
		at, present := frees[free]
		if !present {
			t.Errorf("%s no longer calls %s, so the entry's memory leaks", fn.Name.Name, free)
			continue
		}
		if at < walkAt {
			t.Errorf("%s calls %s at %s, before %s at %s. The walk reads slices into that memory, "+
				"so a goroutine that takes the freed slot in between makes it read another UPDATE.",
				fn.Name.Name, free, fset.Position(at), walkCall, fset.Position(walkAt))
		}
	}
}

// calleeName returns the called identifier for a plain call and for a method
// call, and an empty string for anything this guard does not judge.
func calleeName(expr ast.Expr) string {
	switch fun := expr.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	default:
		return ""
	}
}

// TestTheOrderGuardNamesTheCallsTheSourceUses fails if a rename leaves the
// guard above searching for calls that no longer exist, which would make it
// pass over any order at all.
func TestTheOrderGuardNamesTheCallsTheSourceUses(t *testing.T) {
	body, err := os.ReadFile("recent_cache.go")
	if err != nil {
		t.Fatalf("read recent_cache.go: %v", err)
	}
	source := string(body)
	for _, name := range append([]string{walkCall}, freeCalls[:]...) {
		if !strings.Contains(source, name+"(") {
			t.Errorf("recent_cache.go calls no %s, so the order guard judges nothing", name)
		}
	}
}
