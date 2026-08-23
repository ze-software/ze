// Design: docs/architecture/traffic/fw-7b-backend-hardening.md -- the vppOps seam's construction invariant
// Related: timeout_linux.go -- newGovppOps, the one construction site this guard permits

package trafficvpp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// The one place a govppOps may be built. Both the file and the function are
// checked, so moving the constructor is a deliberate edit here rather than a
// silent widening of where the deadline can be skipped.
const (
	govppOpsTypeName   = "govppOps"
	govppOpsCtorFile   = "timeout_linux.go"
	govppOpsCtorFunc   = "newGovppOps"
	govppOpsCtorAdvice = "call newGovppOps instead: it installs the reply deadline, and a govppOps built any other way sends on a channel govpp left at core.DefaultReplyTimeout, which is 0 and means no deadline at all"
)

// govppOpsSite is one place in this package's sources that builds a govppOps.
type govppOpsSite struct {
	file string
	line int
	fn   string
}

// TestGovppOpsIsBuiltOnlyByItsConstructor reads this package's own sources and
// fails when a govppOps is built anywhere but inside newGovppOps.
//
// VALIDATES: the invariant the reply deadline rests on. newGovppOps installs
// the deadline before it returns, so the bound holds for every request exactly
// as long as the constructor is the only way an ops facade comes into being.
//
// PREVENTS: that invariant decaying into convention. govppOps is an unexported
// struct with an unexported field, so `&govppOps{ch: ch}` stays legal anywhere
// in package trafficvpp and neither the compiler nor any other test refuses
// one. Before this guard the property held because a single literal happened to
// sit in a single file, which is precisely what a reader who believes "there is
// no way to skip it" stops checking.
//
// SCOPE, because this guard is a ratchet and not a proof: it sees the three
// forms that name the type directly, and govppOpsSitesIn says which and lists
// what a parse cannot reach. It catches the regression that actually occurred,
// an inline literal at the call site. It does not make an unbounded facade
// unconstructible.
//
// The scan is deliberately build-tag blind: it parses every .go file in the
// package directory rather than the files this GOOS compiles, so it runs on
// darwin as well as linux and sees a file no current build tag selects.
func TestGovppOpsIsBuiltOnlyByItsConstructor(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the package directory: %v", err)
	}

	var sites []govppOpsSite
	parsed := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		// A file this scan cannot parse FAILS the test; it is never skipped.
		// The whole guard is "look at every source file and find at most one
		// site", so a file that is not looked at is a file that could hold the
		// site. Skipping unparseable files would make the guard report the
		// invariant intact for sources it never read, which is the quiet
		// failure that a guard exists to prevent. Proven the hard way while
		// mutation-testing this test: a harness bug injected malformed Go into
		// three scratch copies, and this Fatal is what said so. A skipping
		// version would have reported the invariant intact instead. Measured,
		// not assumed: with backend_linux.go malformed this test fails naming
		// the file, and with that same file simply ABSENT -- the limit case of
		// skipping it -- the scan finds one site and PASSES.
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		parsed++
		sites = append(sites, govppOpsSitesIn(fset, file, name)...)
	}

	// A scan that finds nothing must not read as a pass. Zero sites means the
	// constructor stopped building a govppOps, or the walk stopped matching
	// one, and both of those are this guard failing rather than succeeding.
	if len(sites) == 0 {
		t.Fatalf("no %s construction found across %d parsed files: the constructor stopped building one, or this scan no longer matches it",
			govppOpsTypeName, parsed)
	}

	for _, site := range sites {
		if site.file == govppOpsCtorFile && site.fn == govppOpsCtorFunc {
			continue
		}
		where := site.fn
		if where == "" {
			where = "package scope"
		}
		t.Errorf("%s:%d: %s builds a %s outside %s in %s -- %s",
			site.file, site.line, where, govppOpsTypeName, govppOpsCtorFunc, govppOpsCtorFile, govppOpsCtorAdvice)
	}

	if len(sites) != 1 {
		t.Errorf("found %d %s construction sites, want exactly 1 (inside %s in %s)",
			len(sites), govppOpsTypeName, govppOpsCtorFunc, govppOpsCtorFile)
	}
}

// govppOpsSitesIn returns the sites in one parsed file that build a govppOps
// DIRECTLY, in the three forms that name the type:
//
//	govppOps{...}, &govppOps{...}   a composite literal
//	new(govppOps)                   an allocation
//	var ops govppOps                a declaration with no initializer
//
// A selector form cannot occur: the type is unexported, so only this package
// names it.
//
// What the scan does NOT see, said here so the guard reads as the ratchet it is
// rather than a proof of impossibility. A govppOps that comes into being as
// part of some OTHER value is invisible to a parse: `[]govppOps{{ch: ch}}`
// elides the inner literal's type, and a struct carrying a govppOps field
// builds one whenever the outer value is built. Seeing those needs the type of
// an expression, which means go/types and a full package load rather than
// go/parser on one file.
//
// That bound is acceptable because of what this guard is FOR. The regression it
// exists to catch is the inline `&govppOps{ch: ch}` that (*backend).Apply
// carried before the constructor existed, and every direct form of it is
// covered. Neither indirect form has ever appeared in this package.
func govppOpsSitesIn(fset *token.FileSet, file *ast.File, name string) []govppOpsSite {
	var out []govppOpsSite
	record := func(pos token.Pos) {
		out = append(out, govppOpsSite{
			file: name,
			line: fset.Position(pos).Line,
			fn:   enclosingFuncName(file, pos),
		})
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch expr := node.(type) {
		case *ast.CompositeLit:
			if id, ok := expr.Type.(*ast.Ident); ok && id.Name == govppOpsTypeName {
				record(expr.Pos())
			}
		case *ast.CallExpr:
			if isNewGovppOps(expr) {
				record(expr.Pos())
			}
		case *ast.ValueSpec:
			if isBareGovppOpsDecl(expr) {
				record(expr.Pos())
			}
		}
		return true
	})
	return out
}

// isNewGovppOps reports whether the call is `new(govppOps)`.
func isNewGovppOps(call *ast.CallExpr) bool {
	fn, ok := call.Fun.(*ast.Ident)
	if !ok || fn.Name != "new" || len(call.Args) != 1 {
		return false
	}
	arg, ok := call.Args[0].(*ast.Ident)
	return ok && arg.Name == govppOpsTypeName
}

// isBareGovppOpsDecl reports whether the spec is `var ops govppOps` with no
// initializer, which zero-values the struct into being and yields a usable
// facade on a nil channel.
//
// A spec WITH an initializer is skipped rather than missed: the initializer is
// what constructs the value, and the composite-literal and new() arms already
// see it, so counting both would report one site twice.
func isBareGovppOpsDecl(spec *ast.ValueSpec) bool {
	if len(spec.Values) > 0 {
		return false
	}
	id, ok := spec.Type.(*ast.Ident)
	return ok && id.Name == govppOpsTypeName
}

// enclosingFuncName returns the name of the function whose body contains pos,
// or an empty string when pos sits at package scope.
func enclosingFuncName(file *ast.File, pos token.Pos) string {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if fn.Body.Pos() <= pos && pos <= fn.Body.End() {
			return fn.Name.Name
		}
	}
	return ""
}
