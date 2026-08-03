package mcp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// dispatchSwitchName is the function whose `switch req.Method` IS this server's
// JSON-RPC method surface.
const dispatchSwitchName = "dispatchMethod"

// dispatchSwitchMethods returns every method name dispatchMethod's switch names,
// read out of the package source.
//
// WHY PARSE THE SOURCE. The probe tables below are hand-written. Until now they
// were also the only description of the method surface. A method added to the
// switch and not to a table was invisible to every test that iterates them.
//
// TestCacheableMethodsMatchSpecification was the casualty. Its PREVENTS promises
// it catches "a new cacheable method landing in the dispatch switch with no
// hints". It cannot: "dispatched" was defined by the very table that would be
// missing the method, so the check saw implemented=false, hinted=false, and
// reported nothing. Reading the switch makes the tables answerable to the code.
//
// Every step below fails the test rather than returning an empty set. A rename
// that made this silently find nothing would restore the blind spot it exists to
// close (ai/rules/evidence.md).
func dispatchSwitchMethods(t *testing.T) map[string]struct{} {
	t.Helper()

	// Every non-test .go file of this package. Read as a directory, not by
	// filename, so moving dispatchMethod to a sibling file does not break the
	// gate. Parsed one file at a time because parser.ParseDir is deprecated.
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		parsed, parseErr := parser.ParseFile(fset, name, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", name, parseErr)
		}
		files = append(files, parsed)
	}
	if len(files) == 0 {
		t.Fatal("no non-test .go files found in the package directory")
	}

	// Every string constant in the package, so a `case methodToolsList:` can be
	// resolved to the wire name it stands for.
	consts := map[string]string{}
	for _, file := range files {
		collectStringConsts(file, consts)
	}

	var found *ast.SwitchStmt
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, isFunc := decl.(*ast.FuncDecl)
			if !isFunc || fn.Name.Name != dispatchSwitchName || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				sw, isSwitch := n.(*ast.SwitchStmt)
				if !isSwitch || !isMethodTag(sw.Tag) {
					return true
				}
				found = sw
				return false
			})
		}
	}
	if found == nil {
		t.Fatalf("no `switch req.Method` found in %s: this test reads the dispatch surface out of it, "+
			"so a rename or a restructure must be reflected here rather than left to silently find nothing",
			dispatchSwitchName)
	}

	methods := map[string]struct{}{}
	for _, stmt := range found.Body.List {
		clause, isCase := stmt.(*ast.CaseClause)
		if !isCase {
			continue
		}
		for _, expr := range clause.List {
			name, resolveErr := resolveStringExpr(expr, consts)
			if resolveErr != "" {
				t.Fatalf("%s: case expression is not a resolvable string: %s", dispatchSwitchName, resolveErr)
			}
			methods[name] = struct{}{}
		}
	}
	if len(methods) == 0 {
		t.Fatalf("%s's method switch yielded no cases", dispatchSwitchName)
	}
	return methods
}

// collectStringConsts records every `const name = "literal"` in a file.
func collectStringConsts(file *ast.File, into map[string]string) {
	for _, decl := range file.Decls {
		gen, isGen := decl.(*ast.GenDecl)
		if !isGen || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, isValue := spec.(*ast.ValueSpec)
			if !isValue {
				continue
			}
			for i, name := range value.Names {
				if i >= len(value.Values) {
					continue
				}
				lit, isLit := value.Values[i].(*ast.BasicLit)
				if !isLit || lit.Kind != token.STRING {
					continue
				}
				unquoted, err := strconv.Unquote(lit.Value)
				if err != nil {
					continue
				}
				into[name.Name] = unquoted
			}
		}
	}
}

// isMethodTag reports whether a switch tag is `<something>.Method`, which is how
// the dispatch switch selects on the JSON-RPC method.
func isMethodTag(tag ast.Expr) bool {
	sel, isSel := tag.(*ast.SelectorExpr)
	return isSel && sel.Sel != nil && sel.Sel.Name == "Method"
}

// resolveStringExpr turns a case expression into the wire method it names,
// returning a non-empty reason when it cannot.
func resolveStringExpr(expr ast.Expr, consts map[string]string) (string, string) {
	switch node := expr.(type) {
	case *ast.BasicLit:
		if node.Kind != token.STRING {
			return "", "a non-string literal"
		}
		unquoted, err := strconv.Unquote(node.Value)
		if err != nil {
			return "", "an unparseable string literal " + node.Value
		}
		return unquoted, ""
	case *ast.Ident:
		value, known := consts[node.Name]
		if !known {
			return "", node.Name + " is not a package-level string constant"
		}
		return value, ""
	default:
		return "", "an expression this test cannot evaluate"
	}
}

// errorOnlyDispatchCases are the switch cases that answer with a JSON-RPC ERROR
// rather than a result. They have no row in resultBearingMethods and no envelope
// invariant to assert.
//
// One member, and it needs a reason to be here. `initialize` is dispatched only
// for the diagnostic. A client still sending the removed handshake gets a 404
// that names the protocol version this server does speak, rather than a bare
// "method not found".
// TestInitializeWithHeadersIsUnknownMethod (headers_test.go) covers it.
//
// This set is the one way to keep a new method out of the envelope, caching and
// serverInfo tables. That is deliberate. It takes an edit here, with a written
// reason, rather than happening by omission.
var errorOnlyDispatchCases = map[string]struct{}{
	initializeMethod: {},
}

// TestResultBearingMethodsMatchDispatchSwitch is the gate that makes the
// hand-written probe table answerable to the code.
//
// Five tests iterate resultBearingMethods -- TestEveryResultCarriesResultType,
// TestEveryResultCarriesServerInfo, TestNonCacheableResultsCarryNoHints,
// TestCacheScopeIsNeverPublic and TestCacheableMethodsMatchSpecification (via
// dispatchedMethods) -- and every one of them silently skipped any method the
// table did not list. That is the shape of a guard that does not guard: it
// reports on what it was told about, not on what the server does.
//
// VALIDATES: every case in dispatchMethod's switch is either a row of
// resultBearingMethods or a declared error-only case, and nothing in either set
// is absent from the switch.
// PREVENTS: a new dispatched method shipping with no envelope assertions, no
// serverInfo assertion and no cache-hint verdict, while five green tests imply
// it was covered.
func TestResultBearingMethodsMatchDispatchSwitch(t *testing.T) {
	inSwitch := dispatchSwitchMethods(t)

	tabled := map[string]struct{}{}
	for _, probe := range resultBearingMethods("unused") {
		if _, duplicate := tabled[probe.method]; duplicate {
			t.Errorf("resultBearingMethods lists %s twice", probe.method)
		}
		tabled[probe.method] = struct{}{}
	}

	for method := range inSwitch {
		_, isTabled := tabled[method]
		_, isErrorOnly := errorOnlyDispatchCases[method]
		switch {
		case isTabled && isErrorOnly:
			t.Errorf("%s is both a resultBearingMethods row and an errorOnlyDispatchCases member; it cannot be both", method)
		case !isTabled && !isErrorOnly:
			t.Errorf("%s is dispatched but has no resultBearingMethods row, so the envelope, serverInfo and "+
				"cache-hint tables never reach it. Add a row, or declare it in errorOnlyDispatchCases with a reason", method)
		}
	}

	for method := range tabled {
		if _, dispatched := inSwitch[method]; !dispatched {
			t.Errorf("resultBearingMethods names %s, which dispatchMethod's switch does not handle: "+
				"the probe would be answered -32601 and the table would be asserting nothing", method)
		}
	}
	for method := range errorOnlyDispatchCases {
		if _, dispatched := inSwitch[method]; !dispatched {
			t.Errorf("errorOnlyDispatchCases names %s, which dispatchMethod's switch no longer handles; drop the entry", method)
		}
	}
}

// TestDispatchSwitchMethodsReadsRealNames guards the reader itself.
//
// A source parser that quietly returned the wrong thing would make the gate
// above vacuous. The failure would then look like a passing test, not a broken
// one.
//
// VALIDATES: the parsed set resolves named constants to wire names and contains
// the methods this server certainly dispatches.
// PREVENTS: the const-resolution step regressing to returning identifier names
// (`methodToolsList` instead of `tools/list`), which would make every membership
// check above compare two disjoint vocabularies and report nothing.
func TestDispatchSwitchMethodsReadsRealNames(t *testing.T) {
	inSwitch := dispatchSwitchMethods(t)
	for _, method := range []string{
		methodServerDiscover, methodToolsList, methodToolsCall,
		methodTasksGet, methodTasksUpdate, methodTasksCancel,
		methodResourcesList, methodResourcesRead, initializeMethod,
	} {
		if _, present := inSwitch[method]; !present {
			t.Errorf("dispatchSwitchMethods did not report %q; parsed set = %v", method, slices.Sorted(maps.Keys(inSwitch)))
		}
	}
	// Names the switch does not carry must not appear: tasks/list and
	// tasks/result were removed this revision and answer -32601.
	for _, absent := range []string{"tasks/list", "tasks/result", "prompts/get"} {
		if _, present := inSwitch[absent]; present {
			t.Errorf("dispatchSwitchMethods reported %q, which the switch does not handle", absent)
		}
	}
}
