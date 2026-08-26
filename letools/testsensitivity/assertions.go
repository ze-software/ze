// Design: docs/architecture/testing/test-health.md -- can this test go red at all
//
// assertions.go answers one question about one Test function: is there a
// reachable call that can FAIL it? A Test with no such call executes code,
// moves coverage, and passes unconditionally -- deleting the body of the
// function under test would not turn it red.

package testsensitivity

import (
	"go/ast"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// escapeComment authorizes a genuinely assertion-free test (a "does not panic"
// smoke test, a generator that only needs to complete). It must carry a reason.
const escapeComment = "test-asserts-nothing:"

// failureSelectors are the method names that can fail a test, on *testing.T,
// *testing.B, *testing.F, or an assertion helper's receiver. Skip and SkipNow
// are deliberately absent: skipping is not failing.
var failureSelectors = map[string]bool{
	"Error": true, "Errorf": true,
	"Fatal": true, "Fatalf": true,
	"Fail": true, "FailNow": true,
}

// isTestFunc reports whether fn is an assertion site this detector can judge.
//
// Only Test functions qualify. Three neighbors are deliberately excluded
// because "has no assertion" is not a defect for them:
//
//   - TestMain is a harness, not a test.
//   - Benchmark* measures; an assertion inside the timed loop is the
//     antipattern.
//   - Fuzz* delegates its oracle to the fuzzing engine, which fails the run on
//     a panic or a crasher. A fuzz body with no explicit assertion is still
//     doing its job.
func isTestFunc(fn *ast.FuncDecl) bool {
	if fn.Recv != nil || fn.Body == nil {
		return false
	}
	name := fn.Name.Name
	if name == "TestMain" || !strings.HasPrefix(name, "Test") {
		return false
	}
	// go test only treats TestX as an entry point when X does not begin with a
	// lowercase letter.
	rest := strings.TrimPrefix(name, "Test")
	return rest == "" || rest[0] < 'a' || rest[0] > 'z'
}

// hasEscape reports whether the test carries the opt-out annotation, in its doc
// comment OR anywhere inside its body.
//
// The body case needs the file's comment list and the function's position
// range: comments are not AST nodes hanging off a FuncDecl, they live in
// File.Comments, so an ast.Inspect over the declaration can only ever re-find
// the doc comment. The earlier version did exactly that, which meant the
// natural placement (a comment on the first line of the body) was silently
// ignored while the gate's own failure message told the developer to "annotate
// the test".
func hasEscape(fn *ast.FuncDecl, file *ast.File) bool {
	if fn.Doc != nil {
		for _, comment := range fn.Doc.List {
			if strings.Contains(comment.Text, escapeComment) {
				return true
			}
		}
	}
	if file == nil {
		return false
	}
	for _, group := range file.Comments {
		if group.Pos() < fn.Pos() || group.End() > fn.End() {
			continue
		}
		for _, comment := range group.List {
			if strings.Contains(comment.Text, escapeComment) {
				return true
			}
		}
	}
	return false
}

// pkgKey identifies a Go package within a directory. A directory legally holds
// both `foo` and `foo_test`, and each may declare a helper of the same name.
type pkgKey string

// packageFuncs indexes top-level functions and methods PER PACKAGE, so a helper
// named `check` in `foo_test` never shadows a different `check` in `foo`.
//
// Flattening every file into one map made the result depend on Go's randomized
// map iteration order: with a same-named helper in both packages, whichever won
// the write decided whether canFail followed an asserting helper or an inert
// one, and the reported count flapped between runs on an unchanged tree. A
// ratchet whose count is nondeterministic fires spuriously and cannot be
// diagnosed.
//
// Methods are indexed by name too (without their receiver type, which is a
// deliberate over-approximation): a table-driven test whose case struct carries
// `func (c tcase) check(t *testing.T)` asserts for real, and treating that as a
// failure path is the safe direction for a ratchet.
func packageFuncs(parsed map[string]*ast.File, order []string) map[pkgKey]map[string]*ast.FuncDecl {
	out := map[pkgKey]map[string]*ast.FuncDecl{}
	for _, name := range order {
		file := parsed[name]
		if file == nil {
			continue
		}
		key := pkgKey(file.Name.Name)
		if out[key] == nil {
			out[key] = map[string]*ast.FuncDecl{}
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if _, seen := out[key][fn.Name.Name]; !seen {
				out[key][fn.Name.Name] = fn
			}
		}
	}
	return out
}

// testingIdents collects every identifier bound to *testing.T, *testing.B or
// *testing.F within a node, including the parameters of subtest closures
// (`t.Run("x", func(t *testing.T) {...})`), since those routinely rebind the
// name. ast.Inspect descends into FuncLit, so one pass covers them all.
func testingIdents(node ast.Node) map[string]bool {
	out := map[string]bool{}
	ast.Inspect(node, func(node ast.Node) bool {
		fnType, ok := node.(*ast.FuncType)
		if !ok || fnType.Params == nil {
			return true
		}
		for _, field := range fnType.Params.List {
			star, isStar := field.Type.(*ast.StarExpr)
			if !isStar {
				continue
			}
			selector, isSelector := star.X.(*ast.SelectorExpr)
			if !isSelector {
				continue
			}
			pkg, isIdent := selector.X.(*ast.Ident)
			if !isIdent || pkg.Name != "testing" {
				continue
			}
			switch selector.Sel.Name {
			case "T", "B", "F":
				for _, name := range field.Names {
					out[name.Name] = true
				}
			}
		}
		return true
	})
	return out
}

// union merges two identifier sets without mutating either.
func union(left, right map[string]bool) map[string]bool {
	out := make(map[string]bool, len(left)+len(right))
	for key := range left {
		out[key] = true
	}
	for key := range right {
		out[key] = true
	}
	return out
}

// assertAliases maps a file's import aliases to true when the imported package
// is an assertion library. Without this, `import req ".../require"` renders
// every req.NoError invisible and the test is wrongly reported as asserting
// nothing.
func assertAliases(file *ast.File) map[string]bool {
	// Deliberately NOT seeded with the common names. Seeding meant a local
	// variable called `assert`, `is` or `must` was treated as an assertion
	// package, so `assert := fake{}; assert.Equal(1, 2)` credited a test that
	// asserts nothing. An alias is registered only when an assertion library is
	// actually imported under it.
	out := map[string]bool{}
	for _, imported := range file.Imports {
		path := strings.Trim(imported.Path.Value, `"`)
		if !isAssertionImport(path) {
			continue
		}
		if imported.Name != nil {
			out[imported.Name.Name] = true
			continue
		}
		if index := strings.LastIndex(path, "/"); index >= 0 {
			out[path[index+1:]] = true
			continue
		}
		out[path] = true
	}
	return out
}

// isAssertionImport recognizes assertion libraries by their FINAL path element,
// never by substring.
//
// `strings.Contains(path, "/is")` matched
// `github.com/ze-software/ze/internal/plugins/isis/...`, so in every ISIS test
// file the packages `types`, `packet`, `lsdb`, `spf`, `circuit`, ... were all
// registered as assertion aliases and any call on them credited the test. 143
// live tests took that path and two genuinely inert ones hid behind it.
func isAssertionImport(path string) bool {
	if strings.Contains(path, "testify") || strings.Contains(path, "gocheck") ||
		strings.Contains(path, "quicktest") {
		return true
	}
	last := path
	if index := strings.LastIndex(path, "/"); index >= 0 {
		last = path[index+1:]
	}
	switch last {
	case "assert", "require", "is", "should", "must", "qt":
		return true
	}
	return false
}

// fileImports maps a file's local package names to the import paths behind
// them, so a call written `markupcheck.AssertNoMarkup(t, ...)` can be resolved
// to the package that declares it. An aliased import is keyed by its alias; a
// dot or blank import is skipped, since neither produces a qualified call.
func fileImports(file *ast.File) map[string]string {
	out := map[string]string{}
	for _, imported := range file.Imports {
		path := strings.Trim(imported.Path.Value, `"`)
		name := path
		if index := strings.LastIndex(path, "/"); index >= 0 {
			name = path[index+1:]
		}
		if imported.Name != nil {
			if imported.Name.Name == "_" || imported.Name.Name == "." {
				continue
			}
			name = imported.Name.Name
		}
		out[name] = path
	}
	return out
}

// crossHelper is a function in another first-party package that canFail may
// follow. Its file's assertion aliases travel with it: a helper imports its own
// assertion library, and judging its body under the CALLER's aliases would
// credit the wrong names.
type crossHelper struct {
	decl    *ast.FuncDecl
	aliases map[string]bool
}

// pkgIndex resolves a first-party import path to the functions its non-test
// files declare. Without it, a test whose only assertion is a call into a
// shared assert helper (`markupcheck.AssertNoMarkup(t, ...)`,
// `golden.AssertPortFidelity(t, ...)`) reads as asserting nothing: the helper
// index canFail consults is built per DIRECTORY, so nothing outside the test's
// own directory is ever followed. Nine live tests took that path.
//
// Parsing is lazy and cached, so only a package a test actually calls is read.
type pkgIndex struct {
	root   string
	module string
	cache  map[string]map[string]crossHelper
}

// newPkgIndex reads the module path from root's go.mod. A tree with no go.mod
// yields an index that resolves nothing, which leaves the gate exactly as
// conservative as it was before cross-package following existed.
func newPkgIndex(root string) *pkgIndex {
	index := &pkgIndex{root: root, cache: map[string]map[string]crossHelper{}}
	data, err := os.ReadFile(filepath.Join(root, "go.mod")) //nolint:gosec // fixed in-repo path
	if err != nil {
		return index
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if rest, found := strings.CutPrefix(strings.TrimSpace(line), "module "); found {
			index.module = strings.TrimSpace(rest)
			break
		}
	}
	return index
}

// funcs answers the top-level functions importPath declares, or nothing when
// the path is not first-party. Only non-test files are read: a `_test.go`
// helper is unreachable from another package.
//
// A directory that cannot be read or parsed yields an empty index rather than
// an error. That is the fail-closed direction here: nothing gets credited, so
// the caller stays on the assert-nothing list and the ratchet says so out loud.
func (p *pkgIndex) funcs(importPath string) map[string]crossHelper {
	if cached, seen := p.cache[importPath]; seen {
		return cached
	}
	out := map[string]crossHelper{}
	p.cache[importPath] = out

	if p.module == "" {
		return out
	}
	var rel string
	suffix := strings.TrimPrefix(importPath, p.module)
	switch {
	case importPath == p.module:
		rel = "."
	case strings.HasPrefix(suffix, "/"):
		rel = strings.TrimPrefix(suffix, "/")
	default:
		return out
	}

	dir := filepath.Join(p.root, filepath.FromSlash(rel))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	names := []string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		file, parseErr := parseGo(filepath.Join(dir, name))
		if parseErr != nil {
			continue
		}
		aliases := assertAliases(file)
		for _, decl := range file.Decls {
			fn, isFunc := decl.(*ast.FuncDecl)
			if !isFunc || fn.Body == nil || fn.Recv != nil {
				continue
			}
			if _, seen := out[fn.Name.Name]; !seen {
				out[fn.Name.Name] = crossHelper{decl: fn, aliases: aliases}
			}
		}
	}
	return out
}

// scope is everything canFail needs to judge one function body: the helpers it
// may follow by name inside its own package, the assertion-library aliases its
// file imports, the packages its file imports (so a shared assert helper is
// followed across the package boundary), the identifiers bound to a testing
// value, and the lazily-parsed index those cross-package lookups read.
type scope struct {
	pkgFuncs map[string]*ast.FuncDecl
	aliases  map[string]bool
	imports  map[string]string
	testing  map[string]bool
	index    *pkgIndex
}

// withTesting answers a copy whose testing identifiers also hold extra, leaving
// the receiver untouched so a sibling branch of the walk is unaffected.
func (s scope) withTesting(extra map[string]bool) scope {
	s.testing = union(s.testing, extra)
	return s
}

// canFail reports whether the body contains a reachable call that can fail the
// test. ast.Inspect descends into function literals, so subtests registered
// with t.Run and callbacks passed to t.Cleanup are covered without special
// cases.
//
// depth bounds helper following; 1 means "follow helpers one level", which is
// where the cost and benefit of this detector sit. sc.testing holds the
// identifiers bound to *testing.T/B/F in scope. It is passed in rather than
// derived from body alone, because a test's own `t` parameter is declared in
// the FuncDecl's signature, not inside its block.
func canFail(body *ast.BlockStmt, sc scope, depth int) bool {
	if body == nil {
		return false
	}
	failed := false
	ast.Inspect(body, func(node ast.Node) bool {
		if failed {
			return false
		}
		// A compile-time interface assertion (`var _ Clock = (*V)(nil)`) is a
		// real assertion: breaking it stops the package building, which is a
		// louder failure than a t.Error. Treat it as a failure path.
		if spec, ok := node.(*ast.ValueSpec); ok && spec.Type != nil {
			for _, name := range spec.Names {
				if name.Name == "_" {
					failed = true
					return false
				}
			}
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if callCanFail(call, sc, depth) {
			failed = true
			return false
		}
		return true
	})
	return failed
}

// callCanFail reports whether one call site can fail the test.
func callCanFail(call *ast.CallExpr, sc scope, depth int) bool {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		return selectorCanFail(fun, sc, depth)
	case *ast.Ident:
		if fun.Name == "panic" {
			return true
		}
		if depth > 0 {
			if helper, known := sc.pkgFuncs[fun.Name]; known &&
				canFail(helper.Body, sc.withTesting(testingIdents(helper)), depth-1) {
				return true
			}
		}
	}
	return false
}

// selectorCanFail reports whether a qualified call can fail the test.
//
// A failure method counts ONLY on a testing receiver or an assertion-library
// alias. Matching the method NAME alone credited any `fmt.Errorf(...)`,
// the standard logger's own Fatalf, or a business method called `Fail()` --
// so a test that
// constructs an error and asserts nothing was reported as asserting. 78 live
// tests took that path, 66 of them via `fmt.Errorf`, which is everyday Go
// inside a table-driven test.
func selectorCanFail(fun *ast.SelectorExpr, sc scope, depth int) bool {
	receiver, isIdent := fun.X.(*ast.Ident)
	onTestReceiver := isIdent && (sc.testing[receiver.Name] || sc.aliases[receiver.Name])
	if failureSelectors[fun.Sel.Name] && onTestReceiver {
		return true
	}
	// assert.Equal / require.NoError and friends: every exported call on an
	// assertion package can fail the test.
	if isIdent && sc.aliases[receiver.Name] && fun.Sel.Name != "Error" {
		return true
	}
	if depth == 0 {
		return false
	}

	// A method on a local value (`c.check(t)` on a table case) is a real
	// assertion site; follow it by name.
	if helper, known := sc.pkgFuncs[fun.Sel.Name]; known &&
		canFail(helper.Body, sc.withTesting(testingIdents(helper)), depth-1) {
		return true
	}
	if !isIdent {
		return false
	}

	// A call into another first-party package (`markupcheck.AssertNoMarkup(t,
	// ...)`) is followed the same way, one level, into the package the file
	// imports under that name. The helper is credited only when its own body
	// can fail, so a fixture builder that merely takes a *testing.T still counts
	// for nothing.
	path, imported := sc.imports[receiver.Name]
	if !imported {
		return false
	}
	helper, known := sc.index.funcs(path)[fun.Sel.Name]
	if !known {
		return false
	}
	inner := scope{aliases: helper.aliases, testing: testingIdents(helper.decl), index: sc.index}
	return canFail(helper.decl.Body, inner, depth-1)
}
