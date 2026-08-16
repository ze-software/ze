// Design: docs/architecture/testing/test-health.md -- test-sensitivity ratchet
//
// inert_tests finds tests that cannot do their job, which no count of
// tests can reveal:
//
//   1. assert-nothing: a Test function with no reachable failure call. It
//      executes code, moves coverage, and passes unconditionally. Deleting the
//      body of the function under test would not turn it red.
//
//   2. tag-orphan: a _test.go file whose //go:build constraint requires a
//      project tag (ze_*) that no `go test -tags` invocation in the Makefile or
//      mk/*.mk ever supplies. The file compiles nowhere, runs nowhere, and reads
//      as coverage from every directory listing.
//
// Both are counted and ratcheted: the committed floors in
// test/health/sensitivity-baseline.json may only go DOWN, following the
// test/.ci-sleep-baseline convention (lower the floor in the same change that
// improves the number).
//
// The tag universe is DERIVED from the make files and feature-gates.txt, never
// hardcoded (ai/rules/evidence.md): a new gated feature must not
// silently make its tests orphans, and deleting a target must surface the tests
// it stranded.
//
// Populations: --check scans the WORKING TREE (the ratchet must catch an inert
// test before it is committed, not blame the next change); --tracked-only scans
// git's index (the generated page must be reproducible from a clean checkout).
//
// Usage:   go run scripts/checks/inert_tests.go [--json] [--check] [--selftest] [--tracked-only]
// Called by: make ze-test-sensitivity-check, scripts/dev/testing_health.py
//
//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// testRoots are the in-repo trees that hold first-party tests. vendor/ and
// gokrazy/modcache/ are third-party module trees: counting them is precisely the
// error that let the published test total reach six times the real one.
var testRoots = []string{"internal", "cmd", "pkg", "scripts", "test"}

const baselinePath = "test/health/sensitivity-baseline.json"

// escapeComment authorises a genuinely assertion-free test (a "does not panic"
// smoke test, a generator that only needs to complete). It must carry a reason.
const escapeComment = "test-asserts-nothing:"

// failureSelectors are the method names that can fail a test, on *testing.T,
// *testing.B, *testing.F, or an assertion helper's receiver. Skip/SkipNow are
// deliberately absent: skipping is not failing.
var failureSelectors = map[string]bool{
	"Error": true, "Errorf": true,
	"Fatal": true, "Fatalf": true,
	"Fail": true, "FailNow": true,
}

// projectTag matches the build tags this repo owns. Non-project tags (linux,
// amd64, integration, cgo, go1.x) are treated as satisfiable, so the orphan
// detector only fires on a tag whose reachability this repo actually controls.
var projectTag = regexp.MustCompile(`^ze_[a-z0-9_]+$`)

// goTestTags finds `go test ... -tags 'a b c'` (or -tags a) in the make files.
var goTestTagsRe = regexp.MustCompile(`go test[^\n]*?-tags[ =]'([^']*)'|go test[^\n]*?-tags[ =]([a-zA-Z0-9_,]+)`)

// makeVarRe finds `NAME = value` / `NAME := value` so $(GO_TEST_TAGS) expands.
var makeVarRe = regexp.MustCompile(`(?m)^([A-Z_][A-Z0-9_]*)\s*[:?]?=\s*(.*)$`)

type finding struct {
	File   string `json:"file"`
	Test   string `json:"test,omitempty"`
	Line   int    `json:"line"`
	Reason string `json:"reason"`
	Detail string `json:"detail,omitempty"`
}

type result struct {
	AssertNothing []finding `json:"assert-nothing"`
	TagOrphan     []finding `json:"tag-orphan"`
	FilesScanned  int       `json:"files-scanned"`
	TestsScanned  int       `json:"tests-scanned"`
	TagUniverse   []string  `json:"test-tag-universe"`
	Valid         bool      `json:"valid"`
}

type baseline struct {
	AssertNothing int `json:"assert-nothing"`
	TagOrphan     int `json:"tag-orphan"`
}

func main() {
	jsonOut, checkMode, selftestMode, trackedOnly := false, false, false, false
	// root is overridable so the gate's own tests can drive this entry point
	// against a fixture tree. ai/rules/evidence.md requires the guard
	// be tested from its entry point, not from its helpers, and the live tree
	// cannot be doctored to prove the ratchet fires.
	root := "."
	for _, a := range os.Args[1:] {
		switch {
		case a == "--json":
			jsonOut = true
		case a == "--check":
			checkMode = true
		case a == "--selftest":
			selftestMode = true
		case a == "--tracked-only":
			trackedOnly = true
		case strings.HasPrefix(a, "--root="):
			root = strings.TrimPrefix(a, "--root=")
		default:
			// Never ignore an argument we do not understand. A typo such as
			// `--chek` would otherwise silently drop the gate into report-only
			// mode, which exits 0 no matter how many findings exist.
			fmt.Fprintf(os.Stderr, "test-sensitivity: unknown argument %q\n", a)
			fmt.Fprintln(os.Stderr, "  usage: inert_tests.go [--json] [--check] [--selftest] [--tracked-only] [--root=DIR]")
			os.Exit(2)
		}
	}

	if selftestMode {
		if !selftest() {
			fmt.Fprintln(os.Stderr, "test-sensitivity: SELFTEST FAILED")
			os.Exit(2)
		}
		fmt.Fprintln(os.Stdout, "test-sensitivity: selftest OK")
		return
	}

	res, err := scanTree(root, trackedOnly)
	if err != nil {
		fmt.Fprintf(os.Stderr, "test-sensitivity: %v\n", err)
		os.Exit(2)
	}

	// Enforcement is decided BEFORE any output mode branches. It used to live
	// inside the non-JSON branch, keyed on a `Valid` field that scanTree always
	// set to true, so `--json --check` reported findings and exited 0 -- a guard
	// whose only enforcement path could never deny.
	if checkMode {
		base, baseErr := readBaseline(filepath.Join(root, baselinePath))
		if baseErr != nil {
			fmt.Fprintf(os.Stderr, "test-sensitivity: %v\n", baseErr)
			os.Exit(2)
		}
		res.Valid = enforce(res, base)
		if !jsonOut && res.Valid {
			fmt.Fprintf(os.Stdout,
				"test-sensitivity: OK (assert-nothing %d/%d, tag-orphan %d/%d, %d test files)\n",
				len(res.AssertNothing), base.AssertNothing,
				len(res.TagOrphan), base.TagOrphan, res.FilesScanned)
		}
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if encErr := enc.Encode(res); encErr != nil {
			fmt.Fprintf(os.Stderr, "test-sensitivity: encode: %v\n", encErr)
			os.Exit(2)
		}
	} else if !checkMode {
		printResult(res)
	}

	if checkMode && !res.Valid {
		os.Exit(1)
	}
}

// enforce applies the ratchet. Counts at or below the floor pass; above fails
// with every offender named. A floor that is now slack is reported so it gets
// lowered in the change that earned it, which is what keeps a ratchet tight.
func enforce(res *result, base baseline) bool {
	ok := true
	if len(res.AssertNothing) > base.AssertNothing {
		ok = false
		fmt.Fprintf(os.Stderr, "\ntest-sensitivity: assert-nothing count %d exceeds baseline %d\n",
			len(res.AssertNothing), base.AssertNothing)
		fmt.Fprintln(os.Stderr, "  These tests contain no reachable Error/Fatal/Fail call and cannot go red:")
		for _, f := range res.AssertNothing {
			fmt.Fprintf(os.Stderr, "    %s:%d %s\n", f.File, f.Line, f.Test)
		}
		fmt.Fprintf(os.Stderr, "  Add a real assertion, or annotate the test with a reason:\n")
		fmt.Fprintf(os.Stderr, "    // %s <why this test cannot assert>\n", escapeComment)
	}
	if len(res.TagOrphan) > base.TagOrphan {
		ok = false
		fmt.Fprintf(os.Stderr, "\ntest-sensitivity: tag-orphan count %d exceeds baseline %d\n",
			len(res.TagOrphan), base.TagOrphan)
		fmt.Fprintln(os.Stderr, "  No `go test -tags` in Makefile or mk/*.mk can build these files:")
		for _, f := range res.TagOrphan {
			fmt.Fprintf(os.Stderr, "    %s:%d requires %s\n", f.File, f.Line, f.Detail)
		}
		fmt.Fprintln(os.Stderr, "  Add the tag to a go test invocation, or delete the file.")
	}
	if !ok {
		fmt.Fprintf(os.Stderr, "\n  Refresh the floors only when the count went DOWN: make ze-test-health-update\n")
		return false
	}
	if len(res.AssertNothing) < base.AssertNothing || len(res.TagOrphan) < base.TagOrphan {
		fmt.Fprintf(os.Stderr,
			"test-sensitivity: baseline is slack (assert-nothing %d<%d, tag-orphan %d<%d). Run `make ze-test-health-update` to tighten it.\n",
			len(res.AssertNothing), base.AssertNothing, len(res.TagOrphan), base.TagOrphan)
	}
	return true
}

func readBaseline(path string) (baseline, error) {
	var b baseline
	raw, err := os.ReadFile(path) //nolint:gosec // fixed in-repo path
	if err != nil {
		return b, fmt.Errorf("read baseline %s: %w (run `make ze-test-health-update` to create it)", path, err)
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		return b, fmt.Errorf("parse baseline %s: %w", path, err)
	}
	if b.AssertNothing < 0 || b.TagOrphan < 0 {
		return b, fmt.Errorf("baseline %s has a negative floor: %+v", path, b)
	}
	return b, nil
}

// scanTree walks the in-repo test roots and runs both detectors. It fails
// closed: a scan that finds no test files at all is a broken scan, not a clean
// tree (ai/rules/evidence.md).
func scanTree(root string, trackedOnly bool) (*result, error) {
	universe, err := testTagUniverse(root)
	if err != nil {
		return nil, err
	}
	if len(universe) == 0 {
		return nil, fmt.Errorf("derived an empty test-tag universe from %s: the make files did not parse", root)
	}

	files, err := collectTestFiles(root, trackedOnly)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("found no _test.go files under %v: refusing to report a clean tree", testRoots)
	}

	res := &result{
		AssertNothing: []finding{},
		TagOrphan:     []finding{},
		FilesScanned:  len(files),
		TagUniverse:   sortedKeys(universe),
	}

	// index resolves a helper that lives in another first-party package. It is
	// built once for the whole scan so each helper package is parsed at most once.
	index := newPkgIndex(root)

	// Group by directory so a helper defined in a sibling test file resolves.
	byDir := map[string][]string{}
	for _, f := range files {
		byDir[filepath.Dir(f)] = append(byDir[filepath.Dir(f)], f)
	}

	for _, dir := range sortedKeys(byDir) {
		fset := token.NewFileSet()
		parsed := map[string]*ast.File{}
		for _, f := range byDir[dir] {
			file, perr := parser.ParseFile(fset, f, nil, parser.ParseComments)
			if perr != nil {
				// A file this gate cannot parse must not be silently skipped.
				return nil, fmt.Errorf("parse %s: %w", f, perr)
			}
			parsed[f] = file

			orphan, tags := tagOrphan(file, universe)
			if orphan {
				res.TagOrphan = append(res.TagOrphan, finding{
					File:   rel(root, f),
					Line:   buildLine(fset, file),
					Reason: "tag-orphan",
					Detail: strings.Join(tags, ", "),
				})
			}
		}

		// byDir[dir] is already sorted (files came from a sorted walk), so the
		// per-package helper index is built in a fixed order.
		pkgFuncs := packageFuncs(parsed, byDir[dir])
		for _, f := range byDir[dir] {
			file := parsed[f]
			sc := scope{
				pkgFuncs: pkgFuncs[pkgKey(file.Name.Name)],
				aliases:  assertAliases(file),
				imports:  fileImports(file),
				index:    index,
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || !isTestFunc(fn) {
					continue
				}
				res.TestsScanned++
				if hasEscape(fn, file) {
					continue
				}
				if canFail(fn.Body, sc.withTesting(testingIdents(fn)), 1) {
					continue
				}
				res.AssertNothing = append(res.AssertNothing, finding{
					File:   rel(root, f),
					Test:   fn.Name.Name,
					Line:   fset.Position(fn.Pos()).Line,
					Reason: "assert-nothing",
				})
			}
		}
	}

	sortFindings(res.AssertNothing)
	sortFindings(res.TagOrphan)
	// Valid is set by the ratchet in main when --check is given. It is NOT set
	// here: an unconditional `true` was what made `--json --check` unable to fail.
	return res, nil
}

// isTestFunc reports whether fn is an assertion site this detector can judge.
//
// Only Test functions qualify. Three neighbours are deliberately excluded
// because "has no assertion" is not a defect for them:
//
//   - TestMain is a harness, not a test.
//   - Benchmark* measures; an assertion inside the timed loop is the antipattern.
//   - Fuzz* delegates its oracle to the fuzzing engine, which fails the run on a
//     panic or a crasher. A fuzz body with no explicit assertion is still doing
//     its job.
func isTestFunc(fn *ast.FuncDecl) bool {
	if fn.Recv != nil || fn.Body == nil {
		return false
	}
	n := fn.Name.Name
	if n == "TestMain" || !strings.HasPrefix(n, "Test") {
		return false
	}
	// go test only treats TestX where X does not begin with a lowercase letter.
	rest := strings.TrimPrefix(n, "Test")
	return rest == "" || !(rest[0] >= 'a' && rest[0] <= 'z')
}

// hasEscape reports whether the test carries the opt-out annotation, in its doc
// comment OR anywhere inside its body.
//
// The body case needs the file's comment list and the function's position range:
// comments are not AST nodes hanging off a FuncDecl, they live in File.Comments,
// so an ast.Inspect over the declaration can only ever re-find the doc comment.
// The earlier version did exactly that, which meant the natural placement (a
// comment on the first line of the body) was silently ignored while the gate's
// own failure message told the developer to "annotate the test".
func hasEscape(fn *ast.FuncDecl, file *ast.File) bool {
	if fn.Doc != nil {
		for _, c := range fn.Doc.List {
			if strings.Contains(c.Text, escapeComment) {
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
		for _, c := range group.List {
			if strings.Contains(c.Text, escapeComment) {
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
// Flattening every file into one map made the result depend on Go's randomised
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
	ast.Inspect(node, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncType)
		if !ok || fn.Params == nil {
			return true
		}
		for _, field := range fn.Params.List {
			star, isStar := field.Type.(*ast.StarExpr)
			if !isStar {
				continue
			}
			sel, isSel := star.X.(*ast.SelectorExpr)
			if !isSel {
				continue
			}
			pkg, isIdent := sel.X.(*ast.Ident)
			if !isIdent || pkg.Name != "testing" {
				continue
			}
			switch sel.Sel.Name {
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
func union(a, b map[string]bool) map[string]bool {
	out := make(map[string]bool, len(a)+len(b))
	for k := range a {
		out[k] = true
	}
	for k := range b {
		out[k] = true
	}
	return out
}

// assertAliases maps a file's import aliases to true when the imported package
// is an assertion library. Without this, `import req ".../require"` renders every
// req.NoError invisible and the test is wrongly reported as asserting nothing.
func assertAliases(file *ast.File) map[string]bool {
	// Deliberately NOT seeded with the common names. Seeding meant a local
	// variable called `assert`, `is` or `must` was treated as an assertion
	// package, so `assert := fake{}; assert.Equal(1, 2)` credited a test that
	// asserts nothing. An alias is registered only when an assertion library is
	// actually imported under it.
	out := map[string]bool{}
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if !isAssertionImport(path) {
			continue
		}
		if imp.Name != nil {
			out[imp.Name.Name] = true
			continue
		}
		if idx := strings.LastIndex(path, "/"); idx >= 0 {
			out[path[idx+1:]] = true
		} else {
			out[path] = true
		}
	}
	return out
}

// isAssertionImport recognises assertion libraries by their FINAL path element,
// never by substring.
//
// `strings.Contains(path, "/is")` matched
// `github.com/ze-software/ze/internal/plugins/isis/...`, so in every ISIS
// test file the packages `types`, `packet`, `lsdb`, `spf`, `circuit`, ... were
// all registered as assertion aliases and any call on them credited the test.
// 143 live tests took that path and two genuinely inert ones hid behind it.
func isAssertionImport(path string) bool {
	if strings.Contains(path, "testify") || strings.Contains(path, "gocheck") ||
		strings.Contains(path, "quicktest") {
		return true
	}
	last := path
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		last = path[idx+1:]
	}
	switch last {
	case "assert", "require", "is", "should", "must", "qt":
		return true
	}
	return false
}

// fileImports maps a file's local package names to the import paths behind them,
// so a call written `markupcheck.AssertNoMarkup(t, ...)` can be resolved to the
// package that declares it. An aliased import is keyed by its alias; a dot or
// blank import is skipped, since neither produces a qualified call.
func fileImports(file *ast.File) map[string]string {
	out := map[string]string{}
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		name := path
		if idx := strings.LastIndex(path, "/"); idx >= 0 {
			name = path[idx+1:]
		}
		if imp.Name != nil {
			if imp.Name.Name == "_" || imp.Name.Name == "." {
				continue
			}
			name = imp.Name.Name
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
// files declare. Without it, a test whose only assertion is a call into a shared
// assert helper (`markupcheck.AssertNoMarkup(t, ...)`,
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
	idx := &pkgIndex{root: root, cache: map[string]map[string]crossHelper{}}
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return idx
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if rest, found := strings.CutPrefix(line, "module "); found {
			idx.module = strings.TrimSpace(rest)
			break
		}
	}
	return idx
}

// funcs returns the top-level functions importPath declares, or nil when the
// path is not first-party. Only non-test files are read: a `_test.go` helper is
// unreachable from another package.
//
// A directory that cannot be read or parsed yields an empty index rather than an
// error. That is the fail-closed direction here: nothing gets credited, so the
// caller stays on the assert-nothing list and the ratchet says so out loud.
func (p *pkgIndex) funcs(importPath string) map[string]crossHelper {
	if cached, seen := p.cache[importPath]; seen {
		return cached
	}
	out := map[string]crossHelper{}
	p.cache[importPath] = out

	if p.module == "" {
		return out
	}
	rel, first := "", strings.TrimPrefix(importPath, p.module)
	switch {
	case importPath == p.module:
		rel = "."
	case strings.HasPrefix(first, "/"):
		rel = strings.TrimPrefix(first, "/")
	default:
		return out
	}

	dir := filepath.Join(p.root, filepath.FromSlash(rel))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	fset := token.NewFileSet()
	names := []string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		file, perr := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ParseComments)
		if perr != nil {
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

// withTesting returns a copy whose testing identifiers also hold extra, leaving
// the receiver untouched so a sibling branch of the walk is unaffected.
func (s scope) withTesting(extra map[string]bool) scope {
	s.testing = union(s.testing, extra)
	return s
}

// canFail reports whether the body contains a reachable call that can fail the
// test. ast.Inspect descends into function literals, so subtests registered with
// t.Run and callbacks passed to t.Cleanup are covered without special cases.
// depth bounds helper following; 1 means "follow helpers one level", which is
// where the cost/benefit of this detector sits (see spec Known Limitations).
// sc.testing holds the identifiers bound to *testing.T/B/F in scope. It is
// passed in rather than derived from body alone, because a test's own `t`
// parameter is declared in the FuncDecl's signature, not inside its block.
func canFail(body *ast.BlockStmt, sc scope, depth int) bool {
	pkgFuncs, aliases, testing := sc.pkgFuncs, sc.aliases, sc.testing
	if body == nil {
		return false
	}
	failed := false
	ast.Inspect(body, func(n ast.Node) bool {
		if failed {
			return false
		}
		// A compile-time interface assertion (`var _ Clock = (*V)(nil)`) is a
		// real assertion: breaking it stops the package building, which is a
		// louder failure than a t.Error. Treat it as a failure path.
		if spec, ok := n.(*ast.ValueSpec); ok && spec.Type != nil {
			for _, name := range spec.Names {
				if name.Name == "_" {
					failed = true
					return false
				}
			}
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.SelectorExpr:
			// A failure method counts ONLY on a testing receiver or an
			// assertion-library alias. Matching the method NAME alone credited
			// any `fmt.Errorf(...)`, `log.Fatalf(...)` or a business method
			// called `Fail()` -- so a test that constructs an error and asserts
			// nothing was reported as asserting. 78 live tests took that path,
			// 66 of them via `fmt.Errorf`, which is everyday Go inside a table
			// -driven test.
			x, isIdent := fun.X.(*ast.Ident)
			onTestReceiver := isIdent && (testing[x.Name] || aliases[x.Name])
			if failureSelectors[fun.Sel.Name] && onTestReceiver {
				failed = true
				return false
			}
			if isIdent && aliases[x.Name] && fun.Sel.Name != "Error" {
				// assert.Equal / require.NoError and friends: every exported
				// call on an assertion package can fail the test.
				failed = true
				return false
			}
			// A method on a local value (`c.check(t)` on a table case) is a
			// real assertion site; follow it by name.
			if depth > 0 {
				if helper, known := pkgFuncs[fun.Sel.Name]; known &&
					canFail(helper.Body, sc.withTesting(testingIdents(helper)), depth-1) {
					failed = true
					return false
				}
			}
			// A call into another first-party package (`markupcheck.AssertNoMarkup(t, ...)`)
			// is followed the same way, one level, into the package the file
			// imports under that name. The helper is credited only when its own
			// body can fail, so a fixture builder that merely takes a *testing.T
			// still counts for nothing.
			if depth > 0 && isIdent {
				if path, imported := sc.imports[x.Name]; imported {
					if helper, known := sc.index.funcs(path)[fun.Sel.Name]; known {
						inner := scope{
							aliases: helper.aliases,
							testing: testingIdents(helper.decl),
							index:   sc.index,
						}
						if canFail(helper.decl.Body, inner, depth-1) {
							failed = true
							return false
						}
					}
				}
			}
		case *ast.Ident:
			if fun.Name == "panic" {
				failed = true
				return false
			}
			if depth > 0 {
				if helper, known := pkgFuncs[fun.Name]; known &&
					canFail(helper.Body, sc.withTesting(testingIdents(helper)), depth-1) {
					failed = true
					return false
				}
			}
		}
		return true
	})
	return failed
}

// tagOrphan reports whether a file's //go:build expression is UNSATISFIABLE
// given what the make files can supply.
//
// This is a satisfiability question, not a single evaluation, and getting that
// wrong is the obvious trap: evaluating once with "every available tag is on"
// wrongly condemns every negated constraint. `//go:build !linux` (the non-Linux
// stubs) and `//go:build ze_core && !ze_web` (the compile-out checks that
// GO_TEST_CORE_TAGS exists to run) are both reachable, and both look dead to a
// single evaluation.
//
// The model: a project tag absent from the universe can only ever be false,
// because nothing passes it. Every other tag is free, since different targets
// pass different tag sets. The file is an orphan only when no assignment of the
// free tags satisfies the expression.
func tagOrphan(file *ast.File, universe map[string]bool) (bool, []string) {
	for _, group := range file.Comments {
		for _, c := range group.List {
			// Only a comment BEFORE the package clause can be a build
			// constraint. Without this bound, a comment merely quoting a build
			// line -- which checker docs and specs in this repo do routinely --
			// was read as the file's own constraint, producing a finding
			// against a file that in fact builds everywhere. A guard must not
			// manufacture a finding it did not prove.
			if c.Pos() >= file.Package {
				continue
			}
			if !constraint.IsGoBuild(c.Text) {
				continue
			}
			expr, err := constraint.Parse(c.Text)
			if err != nil {
				// An unparseable constraint is not evidence of an orphan; the
				// Go toolchain would have rejected the file long before here.
				return false, nil
			}
			free, unreachable := classifyTags(expr, universe)
			if satisfiable(expr, free, unreachable) {
				return false, nil
			}
			if len(unreachable) == 0 {
				// Self-contradictory over otherwise-reachable tags, e.g.
				// `ze_core && !ze_core`. Report the constraint itself rather
				// than an empty "requires" list.
				return true, []string{c.Text}
			}
			return true, unreachable
		}
	}
	return false, nil
}

// classifyTags splits the expression's tags into those that may take either
// value and those pinned to false because no target supplies them.
func classifyTags(expr constraint.Expr, universe map[string]bool) (free, unreachable []string) {
	seen := map[string]bool{}
	// Eval visits every tag; the returned value is irrelevant here.
	_ = expr.Eval(func(tag string) bool {
		if !seen[tag] {
			seen[tag] = true
			if projectTag.MatchString(tag) && !universe[tag] {
				unreachable = append(unreachable, tag)
			} else {
				free = append(free, tag)
			}
		}
		return false
	})
	sort.Strings(free)
	sort.Strings(unreachable)
	return free, unreachable
}

// maxFreeTags bounds the brute-force search. Real constraints carry a handful of
// tags; anything larger is assumed satisfiable rather than reported, because a
// guard must not manufacture a finding it did not actually prove.
const maxFreeTags = 16

func satisfiable(expr constraint.Expr, free, unreachable []string) bool {
	if len(free) > maxFreeTags {
		return true
	}
	pinned := map[string]bool{}
	for _, t := range unreachable {
		pinned[t] = false
	}
	for mask := 0; mask < 1<<len(free); mask++ {
		assign := map[string]bool{}
		for k, v := range pinned {
			assign[k] = v
		}
		for i, tag := range free {
			assign[tag] = mask&(1<<i) != 0
		}
		if expr.Eval(func(tag string) bool { return assign[tag] }) {
			return true
		}
	}
	return false
}

func buildLine(fset *token.FileSet, file *ast.File) int {
	for _, group := range file.Comments {
		for _, c := range group.List {
			if constraint.IsGoBuild(c.Text) {
				return fset.Position(c.Pos()).Line
			}
		}
	}
	return 1
}

// testTagUniverse derives the set of project tags that some `go test -tags`
// invocation in the make files supplies, expanding make variables (GO_TEST_TAGS,
// ZE_FEATURES, ...) and the feature-gate manifest.
func testTagUniverse(root string) (map[string]bool, error) {
	vars := map[string]string{}
	var sources []string

	makefiles := []string{filepath.Join(root, "Makefile")}
	mks, err := filepath.Glob(filepath.Join(root, "mk", "*.mk"))
	if err != nil {
		return nil, fmt.Errorf("glob mk/*.mk: %w", err)
	}
	makefiles = append(makefiles, mks...)

	for _, mf := range makefiles {
		raw, rerr := os.ReadFile(mf) //nolint:gosec // fixed in-repo paths
		if rerr != nil {
			if mf == filepath.Join(root, "Makefile") {
				return nil, fmt.Errorf("read %s: %w", mf, rerr)
			}
			continue
		}
		text := string(raw)
		sources = append(sources, text)
		for _, m := range makeVarRe.FindAllStringSubmatch(text, -1) {
			if _, seen := vars[m[1]]; !seen {
				vars[m[1]] = m[2]
			}
		}
	}

	// ZE_FEATURES is defined as `$(shell awk ... feature-gates.txt)`, which this
	// parser cannot execute, so its value is supplied here from the manifest it
	// reads. Crucially this is bound as a make VARIABLE, not injected straight
	// into the universe: a tag reaches the universe only if some `go test -tags`
	// line actually references it.
	//
	// Seeding the universe from the manifest directly (the earlier version) made
	// the guard unable to fail. Deleting `$(ZE_FEATURES)` from GO_TEST_TAGS would
	// have stranded every feature-gated test, and the gate would still have
	// reported zero orphans -- fail-open on exactly the regression it exists to
	// catch.
	gates, err := os.ReadFile(filepath.Join(root, "feature-gates.txt")) //nolint:gosec // fixed in-repo path
	if err != nil {
		return nil, fmt.Errorf("read feature-gates.txt: %w", err)
	}
	var manifest []string
	for _, line := range strings.Split(string(gates), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && projectTag.MatchString(fields[0]) {
			manifest = append(manifest, fields[0])
		}
	}
	sort.Strings(manifest)
	vars["ZE_FEATURES"] = strings.Join(manifest, " ")

	universe := map[string]bool{}
	for _, text := range sources {
		for _, m := range goTestTagsRe.FindAllStringSubmatch(text, -1) {
			spec := m[1]
			if spec == "" {
				spec = m[2]
			}
			for _, tag := range expandTags(spec, vars, 0) {
				if projectTag.MatchString(tag) {
					universe[tag] = true
				}
			}
		}
	}
	return universe, nil
}

// expandTags splits a -tags spec into tags, resolving $(VAR) references against
// the make variables. Depth-bounded so a self-referential variable cannot loop.
func expandTags(spec string, vars map[string]string, depth int) []string {
	if depth > 4 {
		return nil
	}
	var out []string
	for _, field := range strings.FieldsFunc(spec, func(r rune) bool { return r == ' ' || r == ',' || r == '\t' }) {
		if strings.HasPrefix(field, "$(") && strings.HasSuffix(field, ")") {
			name := strings.TrimSuffix(strings.TrimPrefix(field, "$("), ")")
			if val, ok := vars[name]; ok {
				out = append(out, expandTags(val, vars, depth+1)...)
			}
			continue
		}
		if strings.ContainsAny(field, "$()'\"") {
			continue
		}
		out = append(out, field)
	}
	return out
}

// collectTestFiles gathers the test files to judge.
//
// Two populations, deliberately different, because the gate and the report
// answer different questions:
//
//   - working tree (default): what you are about to commit. This is right for
//     the ratchet -- an inert test must be caught by the `make ze-precommit-verify` run
//     that precedes its commit, not by the next one, which would blame an
//     unrelated change.
//   - tracked only (--tracked-only): what a clean checkout contains. This is
//     right for the generated page, which is byte-compared by a staleness gate:
//     if untracked work-in-progress moved the numbers, every developer with a
//     scratch test file would publish a page that CI disagrees with.
//
// A missing test root is an error in both modes. Skipping it silently would let
// a mis-set root, or an unreadable directory, shrink the count to something the
// ratchet then happily accepts -- and `--write` would bake that shrunken number
// into the floor permanently.
func collectTestFiles(root string, trackedOnly bool) ([]string, error) {
	if trackedOnly {
		return trackedTestFiles(root)
	}
	var files []string
	for _, r := range testRoots {
		dir := filepath.Join(root, r)
		if _, err := os.Stat(dir); err != nil {
			return nil, fmt.Errorf("test root %s is missing or unreadable: %w", dir, err)
		}
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				switch d.Name() {
				case "vendor", "testdata", "node_modules", ".git":
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", dir, err)
		}
	}
	sort.Strings(files)
	return files, nil
}

// trackedTestFiles lists the _test.go files git has in its index, so the result
// is reproducible from any clean checkout of the same commit.
func trackedTestFiles(root string) ([]string, error) {
	cmd := exec.Command("git", "-C", root, "ls-files", "-z", "--", "*_test.go")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files in %s: %w", root, err)
	}
	var files []string
	for _, name := range strings.Split(string(out), "\x00") {
		if name == "" {
			continue
		}
		parts := strings.Split(name, "/")
		if len(parts) == 0 {
			continue
		}
		var underRoot bool
		for _, r := range testRoots {
			if parts[0] == r {
				underRoot = true
				break
			}
		}
		if !underRoot {
			continue
		}
		skip := false
		for _, p := range parts {
			if p == "vendor" || p == "testdata" || p == "node_modules" {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		abs := filepath.Join(root, filepath.FromSlash(name))
		// git ls-files reads the INDEX; a test file deleted or moved in the
		// working tree is still listed until that deletion is staged. Parsing
		// it would fail the whole run with a bare "no such file", which any
		// developer mid-refactor would hit before they can commit. Skip the
		// absent entry -- there is no test content to judge, and on the clean
		// checkout this mode is designed to describe, the deletion is committed
		// and the entry is simply gone.
		//
		// Deliberately narrow: ONLY a not-exist error is tolerated. An
		// unreadable-but-present file still fails the run rather than silently
		// shrinking the count the ratchet accepts (ai/rules/evidence.md).
		if _, statErr := os.Stat(abs); statErr != nil {
			if os.IsNotExist(statErr) {
				fmt.Fprintf(os.Stderr,
					"test-sensitivity: skipping %s: tracked by git but absent from the working tree "+
						"(an unstaged delete or move)\n", name)
				continue
			}
			return nil, fmt.Errorf("stat tracked test file %s: %w", name, statErr)
		}
		files = append(files, abs)
	}
	sort.Strings(files)
	return files, nil
}

func printResult(r *result) {
	fmt.Fprintf(os.Stdout, "# Test Sensitivity\n\n")
	fmt.Fprintf(os.Stdout, "Test files scanned: %d\nTest functions scanned: %d\n", r.FilesScanned, r.TestsScanned)
	fmt.Fprintf(os.Stdout, "Test tag universe: %s\n\n", strings.Join(r.TagUniverse, " "))
	fmt.Fprintf(os.Stdout, "## Assert-nothing (%d)\n\n", len(r.AssertNothing))
	for _, f := range r.AssertNothing {
		fmt.Fprintf(os.Stdout, "  %s:%d %s\n", f.File, f.Line, f.Test)
	}
	fmt.Fprintf(os.Stdout, "\n## Tag-orphan (%d)\n\n", len(r.TagOrphan))
	for _, f := range r.TagOrphan {
		fmt.Fprintf(os.Stdout, "  %s:%d requires %s\n", f.File, f.Line, f.Detail)
	}
}

func rel(root, path string) string {
	if r, err := filepath.Rel(root, path); err == nil {
		return filepath.ToSlash(r)
	}
	return filepath.ToSlash(path)
}

func sortFindings(f []finding) {
	sort.Slice(f, func(i, j int) bool {
		if f[i].File != f[j].File {
			return f[i].File < f[j].File
		}
		return f[i].Line < f[j].Line
	})
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// selftestCrossPackage pins the one-level follow into another first-party
// package. It needs real files, because that resolution reads a go.mod and a
// package directory rather than one parsed source.
//
// Both directions are pinned. Crediting the caller of an asserting helper is the
// false positive this follow removes; refusing to credit the caller of a helper
// that cannot fail is what stops the follow from becoming a blanket pardon for
// every function that happens to take a *testing.T.
func selftestCrossPackage(countInert func(string, *pkgIndex) (int, bool)) bool {
	dir, err := os.MkdirTemp("", "inert-xpkg")
	if err != nil {
		fmt.Fprintf(os.Stderr, "selftest cross-package: temp dir: %v\n", err)
		return false
	}
	defer os.RemoveAll(dir)

	write := func(rel, content string) bool {
		full := filepath.Join(dir, rel)
		if mkErr := os.MkdirAll(filepath.Dir(full), 0o750); mkErr != nil {
			fmt.Fprintf(os.Stderr, "selftest cross-package: mkdir %s: %v\n", rel, mkErr)
			return false
		}
		if wErr := os.WriteFile(full, []byte(content), 0o600); wErr != nil {
			fmt.Fprintf(os.Stderr, "selftest cross-package: write %s: %v\n", rel, wErr)
			return false
		}
		return true
	}

	ok := write("go.mod", "module example.test\n\ngo 1.26\n")
	ok = ok && write("check/check.go", `package check
import "testing"
func AssertIt(t *testing.T, got int) { if got != 1 { t.Fatalf("ne") } }
func Build(t *testing.T) string { return t.TempDir() }
`)
	if !ok {
		return false
	}

	cases := []struct {
		name string
		src  string
		want int
	}{
		{"a helper in another package that can fail credits the caller", `package p
import ("testing"; "example.test/check")
func TestA(t *testing.T) { check.AssertIt(t, 1) }`, 0},
		{"a helper in another package that cannot fail credits nothing", `package p
import ("testing"; "example.test/check")
func TestA(t *testing.T) { _ = check.Build(t) }`, 1},
		{"a package outside the module is never followed", `package p
import ("testing"; "example.com/other/check")
func TestA(t *testing.T) { check.AssertIt(t, 1) }`, 1},
	}
	for _, tc := range cases {
		index := newPkgIndex(dir)
		got, valid := countInert(tc.src, index)
		if !valid {
			return false
		}
		if got != tc.want {
			fmt.Fprintf(os.Stderr, "selftest cross-package %q: got %d, want %d\n", tc.name, got, tc.want)
			return false
		}
	}
	return true
}

// selftest exercises both detectors against synthetic sources, so a broken
// detector is caught before it judges the live tree. Every case states the
// property it pins.
func selftest() bool {
	parse := func(src string) (*ast.File, *token.FileSet) {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "x_test.go", src, parser.ParseComments)
		if err != nil {
			fmt.Fprintf(os.Stderr, "selftest: parse: %v\n", err)
			return nil, nil
		}
		return file, fset
	}
	// countInert judges src under index, which is nil for every single-file case
	// and non-nil only for the cross-package ones below.
	countInert := func(src string, index *pkgIndex) (int, bool) {
		file, _ := parse(src)
		if file == nil {
			return 0, false
		}
		if index == nil {
			index = &pkgIndex{cache: map[string]map[string]crossHelper{}}
		}
		parsed := map[string]*ast.File{"x_test.go": file}
		sc := scope{
			pkgFuncs: packageFuncs(parsed, []string{"x_test.go"})[pkgKey(file.Name.Name)],
			aliases:  assertAliases(file),
			imports:  fileImports(file),
			index:    index,
		}
		n := 0
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !isTestFunc(fn) || hasEscape(fn, file) {
				continue
			}
			if !canFail(fn.Body, sc.withTesting(testingIdents(fn)), 1) {
				n++
			}
		}
		return n, true
	}
	assertNothing := func(src string) (int, bool) { return countInert(src, nil) }

	cases := []struct {
		name string
		src  string
		want int
	}{
		{"direct t.Fatal counts as a failure path", `package p
import "testing"
func TestA(t *testing.T) { if 1 == 2 { t.Fatal("x") } }`, 0},
		{"no assertion at all is assert-nothing", `package p
import "testing"
func TestA(t *testing.T) { _ = compute() }
func compute() int { return 1 }`, 1},
		{"testify assert counts", `package p
import ("testing"; "github.com/stretchr/testify/assert")
func TestA(t *testing.T) { assert.Equal(t, 1, 1) }`, 0},
		{"subtest assertion counts through the closure", `package p
import "testing"
func TestA(t *testing.T) { t.Run("s", func(t *testing.T) { t.Errorf("x") }) }`, 0},
		{"local helper is followed one level", `package p
import "testing"
func mustEq(t *testing.T, a, b int) { if a != b { t.Fatalf("ne") } }
func TestA(t *testing.T) { mustEq(t, 1, 1) }`, 0},
		{"t.Skip alone is not a failure path", `package p
import "testing"
func TestA(t *testing.T) { t.Skip("later") }`, 1},
		{"escape comment suppresses the finding", `package p
import "testing"
// test-asserts-nothing: proves the constructor does not panic
func TestA(t *testing.T) { _ = compute() }
func compute() int { return 1 }`, 0},
		{"panic counts as a failure path", `package p
import "testing"
func TestA(t *testing.T) { if false { panic("x") } }`, 0},
		{"TestMain is not an assertion site", `package p
import ("testing"; "os")
func TestMain(m *testing.M) { os.Exit(m.Run()) }`, 0},
		{"benchmarks measure and are not judged", `package p
import "testing"
func BenchmarkA(b *testing.B) { for i := 0; i < b.N; i++ { _ = compute() } }
func compute() int { return 1 }`, 0},
		{"fuzz targets delegate their oracle to the engine", `package p
import "testing"
func FuzzA(f *testing.F) { f.Fuzz(func(t *testing.T, d []byte) { _ = parse(d) }) }
func parse(d []byte) int { return len(d) }`, 0},
		{"compile-time interface assertion counts", `package p
import "testing"
func TestA(t *testing.T) { var _ Clock = (*V)(nil) }`, 0},
		{"lowercase suffix is not a go test entry point", `package p
import "testing"
func Testhelper(t *testing.T) { _ = 1 }`, 0},
		// The five below are regressions for defects found in review. Each was
		// a live false positive or false negative before its fix.
		{"testify under a non-canonical import alias still counts", `package p
import ("testing"; req "github.com/stretchr/testify/require")
func TestA(t *testing.T) { req.NoError(t, doIt()) }
func doIt() error { return nil }`, 0},
		{"assertion via a method on a table case counts", `package p
import "testing"
type tcase struct{ want int }
func (c tcase) check(t *testing.T) { if c.want != 1 { t.Fatalf("ne") } }
func TestA(t *testing.T) { t.Run("s", func(t *testing.T) { tcase{1}.check(t) }) }`, 0},
		{"escape annotation inside the body counts, not just the doc comment", `package p
import "testing"
func TestA(t *testing.T) {
	// test-asserts-nothing: the oracle is the absence of a panic
	_ = compute()
}
func compute() int { return 1 }`, 0},
		{"err.Error() is an accessor, not an assertion", `package p
import ("testing"; "errors")
func TestA(t *testing.T) { err := errors.New("x"); t.Log(err.Error()) }`, 1},
		{"t.Error with arguments is still an assertion", `package p
import "testing"
func TestA(t *testing.T) { t.Error("boom") }`, 0},
		// Regression: discriminating err.Error() from t.Error() by ARITY
		// condemned this correct test. The receiver is what distinguishes them.
		{"zero-argument t.Error is an assertion", `package p
import "testing"
func TestA(t *testing.T) { if 1 == 2 { t.Error() } }`, 0},
		{"zero-argument Error on a subtest closure receiver counts", `package p
import "testing"
func TestA(t *testing.T) { t.Run("s", func(sub *testing.T) { sub.Error() }) }`, 0},
		{"zero-argument Error on a benchmark receiver counts", `package p
import "testing"
func TestA(t *testing.T) { helper(t) }
func helper(b *testing.B) { b.Error() }`, 0},
		// Regression: matching the METHOD NAME alone credited any receiver.
		{"fmt.Errorf is not an assertion", `package p
import ("testing"; "fmt")
func TestA(t *testing.T) { err := fmt.Errorf("x %d", 1); _ = err }`, 1},
		{"a business method named Fail is not an assertion", `package p
import "testing"
type job struct{}
func (j job) Fail() {}
func TestA(t *testing.T) { job{}.Fail() }`, 1},
		{"log.Fatalf is not a test assertion", `package p
import ("testing"; "log")
func TestA(t *testing.T) { if false { log.Fatalf("x") } }`, 1},
		{"a local value named assert is not the assert package", `package p
import "testing"
type fake struct{}
func (fake) Equal(a, b int) {}
func TestA(t *testing.T) { assert := fake{}; assert.Equal(1, 2) }`, 1},
		{"an import whose path merely contains /is is not an assertion library", `package p
import ("testing"; "example.com/plugins/isis/types")
func TestA(t *testing.T) { types.New() }`, 1},
		{"a real assertion library still counts", `package p
import ("testing"; "github.com/stretchr/testify/assert")
func TestA(t *testing.T) { assert.Equal(t, 1, 1) }`, 0},
	}
	for _, tc := range cases {
		got, ok := assertNothing(tc.src)
		if !ok {
			return false
		}
		if got != tc.want {
			fmt.Fprintf(os.Stderr, "selftest assert-nothing %q: got %d, want %d\n", tc.name, got, tc.want)
			return false
		}
	}

	if !selftestCrossPackage(countInert) {
		return false
	}

	universe := map[string]bool{"ze_core": true, "ze_web": true}
	tagCases := []struct {
		name   string
		src    string
		orphan bool
	}{
		{"reachable tag is not an orphan", "//go:build ze_web\n\npackage p\n", false},
		{"unreachable tag is an orphan", "//go:build ze_nowhere\n\npackage p\n", true},
		{"GOOS-only constraint is never an orphan", "//go:build linux\n\npackage p\n", false},
		{"AND with an unreachable tag is an orphan", "//go:build ze_core && ze_nowhere\n\npackage p\n", true},
		{"OR with a reachable tag is not an orphan", "//go:build ze_nowhere || ze_core\n\npackage p\n", false},
		{"no constraint is not an orphan", "package p\n", false},
		// Regression: a comment merely QUOTING a build line, after the package
		// clause, is not the file's constraint. This fabricated findings.
		{"quoted build line in a body is not a constraint", "package p\n\nfunc f() {\n\t// the stub carries:\n\t//go:build ze_nowhere\n}\n", false},
		// Regression: classifyTags relies on constraint.Expr.Eval visiting BOTH
		// operands of && (it does; expr.go says "Eval both, to make sure ok func
		// observes all tags"). A short-circuiting Eval would leave ze_core out of
		// the assignment map, defaulting it to false, and condemn this file.
		{"conjunction of two reachable free tags is satisfiable", "//go:build ze_web && ze_core\n\npackage p\n", false},
		{"negated unreachable tag is not an orphan", "//go:build !ze_nowhere\n\npackage p\n", false},
		// The four below are the cases a single "everything available is on"
		// evaluation gets wrong. They are the regression test for that bug.
		{"negated GOOS stub is reachable", "//go:build !linux\n\npackage p\n", false},
		{"compile-out check (tag on, feature off) is reachable", "//go:build ze_core && !ze_web\n\npackage p\n", false},
		{"mutually exclusive project tags are unsatisfiable", "//go:build ze_core && !ze_core\n\npackage p\n", true},
		{"unreachable tag negated inside a reachable OR", "//go:build ze_core || ze_nowhere\n\npackage p\n", false},
		{"GOOS AND unreachable tag is an orphan", "//go:build linux && ze_nowhere\n\npackage p\n", true},
	}
	for _, tc := range tagCases {
		file, _ := parse(tc.src)
		if file == nil {
			return false
		}
		got, _ := tagOrphan(file, universe)
		if got != tc.orphan {
			fmt.Fprintf(os.Stderr, "selftest tag-orphan %q: got %v, want %v\n", tc.name, got, tc.orphan)
			return false
		}
	}

	// expandTags must resolve make variables, or the universe silently narrows
	// and every gated test file becomes a false orphan.
	vars := map[string]string{"GO_TEST_TAGS": "ze_core $(ZE_FEATURES)", "ZE_FEATURES": "ze_web ze_ssh"}
	got := expandTags("$(GO_TEST_TAGS)", vars, 0)
	if len(got) != 3 || got[0] != "ze_core" || got[1] != "ze_web" || got[2] != "ze_ssh" {
		fmt.Fprintf(os.Stderr, "selftest expandTags: got %v, want [ze_core ze_web ze_ssh]\n", got)
		return false
	}

	return true
}
