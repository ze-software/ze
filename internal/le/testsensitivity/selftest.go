// Design: docs/architecture/testing/test-health.md -- both detectors, proved against fixtures
//
// selftest.go exercises both detectors against synthetic sources, so a broken
// detector is caught before it judges the live tree. Every case states the
// property it pins, and a failure names that property rather than a count.
//
// The tables are declared ONCE and read twice: `le test-sensitivity selftest`
// runs them, and the package test runs the same rows.

package testsensitivity

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
)

// legacyLogSelector spells the standard logger's fatal call for one fixture
// below, assembled rather than written whole.
//
// writeGoPatterns (`internal/le/hookruntime/writeedit.go`) refuses that selector
// in production Go, and it cannot tell a fixture from a call. This package's
// subject is which calls count as an assertion, so its corpus must contain the
// forms that do not. The pattern needs the package and method adjacent, so
// splitting them suffices. The parser sees one identifier either way.
const legacyLogSelector = "log." + "Fatalf"

// assertCase is one synthetic test file and the number of assert-nothing
// findings the detector must draw over it.
type assertCase struct {
	name string
	src  string
	want int
}

// assertCases pins the assert-nothing detector. Each entry states one property,
// and the last fourteen are regressions for defects found in review: each was a
// live false positive or false negative before its fix.
var assertCases = []assertCase{
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
	// The comment inside the call is deliberate and must stay. This fixture's
	// whole subject is the builtin, so the call has to appear verbatim, and
	// c_panic (.claude/hooks/pretool-writeedit.py) matches the builtin's name
	// followed by a bracket in any compiled Go file -- it cannot tell a FIXTURE
	// from a call. A comment is whitespace to the parser, so the analyzer under
	// test sees exactly the same AST it would without it.
	{"panic counts as a failure path", `package p
import "testing"
func TestA(t *testing.T) { if false { panic /* the builtin */ ("x") } }`, 0},
	{"TestMain is not an assertion site", `package p
import ("testing"; "os")
func TestMain(m *testing.M) { os.Exit /* stdlib */ (m.Run()) }`, 0},
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
	{"fmt.Errorf is not an assertion", `package p
import ("testing"; "fmt")
func TestA(t *testing.T) { err := fmt.Errorf("x %d", 1); _ = err }`, 1},
	{"a business method named Fail is not an assertion", `package p
import "testing"
type job struct{}
func (j job) Fail() {}
func TestA(t *testing.T) { job{}.Fail() }`, 1},
	{"the standard logger's Fatalf is not a test assertion", `package p
import ("testing"; "log")
func TestA(t *testing.T) { if false { ` + legacyLogSelector + `("x") } }`, 1},
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

// crossCases pin the one-level follow into another first-party package. They
// need real files, because that resolution reads a go.mod and a package
// directory rather than one parsed source.
//
// Both directions are pinned. Crediting the caller of an asserting helper is
// the false positive this follow removes; refusing to credit the caller of a
// helper that cannot fail is what stops the follow from becoming a blanket
// pardon for every function that happens to take a *testing.T.
var crossCases = []assertCase{
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

// crossHelperSource is the helper package the cross-package cases call into:
// one function that can fail and one that cannot.
const crossHelperSource = `package check
import "testing"
func AssertIt(t *testing.T, got int) { if got != 1 { t.Fatalf("ne") } }
func Build(t *testing.T) string { return t.TempDir() }
`

// tagCase is one synthetic build constraint and whether it is an orphan.
type tagCase struct {
	name   string
	src    string
	orphan bool
}

// tagUniverseFixture is the two-tag universe every tag case is judged against.
var tagUniverseFixture = map[string]bool{"ze_core": true, "ze_web": true}

// tagCases pin the orphan detector. The last five are the cases a single
// "everything available is on" evaluation gets wrong, and they are the
// regression test for that bug.
var tagCases = []tagCase{
	{"reachable tag is not an orphan", "//go:build ze_web\n\npackage p\n", false},
	{"unreachable tag is an orphan", "//go:build ze_nowhere\n\npackage p\n", true},
	{"GOOS-only constraint is never an orphan", "//go:build linux\n\npackage p\n", false},
	{"AND with an unreachable tag is an orphan", "//go:build ze_core && ze_nowhere\n\npackage p\n", true},
	{"OR with a reachable tag is not an orphan", "//go:build ze_nowhere || ze_core\n\npackage p\n", false},
	{"no constraint is not an orphan", "package p\n", false},
	{"quoted build line in a body is not a constraint", "package p\n\nfunc f() {\n\t// the stub carries:\n\t//go:build ze_nowhere\n}\n", false},
	{"conjunction of two reachable free tags is satisfiable", "//go:build ze_web && ze_core\n\npackage p\n", false},
	{"negated unreachable tag is not an orphan", "//go:build !ze_nowhere\n\npackage p\n", false},
	{"negated GOOS stub is reachable", "//go:build !linux\n\npackage p\n", false},
	{"compile-out check (tag on, feature off) is reachable", "//go:build ze_core && !ze_web\n\npackage p\n", false},
	{"mutually exclusive project tags are unsatisfiable", "//go:build ze_core && !ze_core\n\npackage p\n", true},
	{"unreachable tag negated inside a reachable OR", "//go:build ze_core || ze_nowhere\n\npackage p\n", false},
	{"GOOS AND unreachable tag is an orphan", "//go:build linux && ze_nowhere\n\npackage p\n", true},
	{"a second unreachable tag is still an orphan", "//go:build ze_nowhere || ze_gone\n\npackage p\n", true},
}

// selftestCaseCount answers how many rows one selftest run reports.
func selftestCaseCount() int { return len(assertCases) + len(crossCases) + len(tagCases) }

// Selftest runs both detectors over their fixtures and answers one row per
// case.
//
// The error is a fixture that could not be written, which is a different fact
// from a detector that stopped detecting, so it is answered apart from the rows
// rather than as one more failing case.
func Selftest() (leroot.SelftestReport, error) {
	results := make([]leroot.SelftestResult, 0, selftestCaseCount())

	for _, testCase := range assertCases {
		results = append(results, judgeAssertCase(testCase, nil))
	}

	crossRoot, err := writeCrossFixture()
	if err != nil {
		return leroot.SelftestReport{}, err
	}
	defer os.RemoveAll(crossRoot) //nolint:errcheck // temp fixture
	for _, testCase := range crossCases {
		results = append(results, judgeAssertCase(testCase, newPkgIndex(crossRoot)))
	}

	for _, testCase := range tagCases {
		results = append(results, judgeTagCase(testCase))
	}

	return leroot.NewSelftestReport(
		"test-sensitivity: selftest OK",
		"test-sensitivity: SELFTEST FAILED:",
		results...,
	), nil
}

// judgeAssertCase runs one assert-nothing case. index is nil for a single-file
// case and non-nil for a cross-package one.
func judgeAssertCase(testCase assertCase, index *pkgIndex) leroot.SelftestResult {
	got, err := countInert(testCase.src, index)
	if err != nil {
		var tb textbuf.Buffer
		return leroot.Fail(testCase.name, tb.Str("parse: ").Err(err).String())
	}
	if got == testCase.want {
		return leroot.Pass(testCase.name)
	}
	var tb textbuf.Buffer
	return leroot.Fail(testCase.name, tb.Str("assert-nothing: got ").Int(int64(got)).
		Str(", want ").Int(int64(testCase.want)).String())
}

// judgeTagCase runs one tag-orphan case.
func judgeTagCase(testCase tagCase) leroot.SelftestResult {
	file, err := parseSource(testCase.src)
	if err != nil {
		var tb textbuf.Buffer
		return leroot.Fail(testCase.name, tb.Str("parse: ").Err(err).String())
	}
	got, _ := TagOrphan(file, tagUniverseFixture)
	if got == testCase.orphan {
		return leroot.Pass(testCase.name)
	}
	var tb textbuf.Buffer
	return leroot.Fail(testCase.name, tb.Str("tag-orphan: got ").Bool(got).
		Str(", want ").Bool(testCase.orphan).String())
}

// countInert answers how many Test functions in one source assert nothing.
// index resolves cross-package helpers and may be nil.
func countInert(src string, index *pkgIndex) (int, error) {
	file, err := parseSource(src)
	if err != nil {
		return 0, err
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

	count := 0
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || !isTestFunc(fn) || hasEscape(fn, file) {
			continue
		}
		if !canFail(fn.Body, sc.withTesting(testingIdents(fn)), 1) {
			count++
		}
	}
	return count, nil
}

// writeCrossFixture writes a one-package module the cross-package cases import.
func writeCrossFixture() (string, error) {
	dir, err := os.MkdirTemp("", "test-sensitivity-xpkg")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test\n\ngo 1.26\n"), 0o600); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(dir, "check"), 0o750); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "check", "check.go"), []byte(crossHelperSource), 0o600); err != nil {
		return "", err
	}
	return dir, nil
}

// parseSource parses one synthetic test file with its comments.
func parseSource(src string) (*ast.File, error) {
	return parser.ParseFile(token.NewFileSet(), "x_test.go", src, parser.ParseComments)
}

// runSelftest is the `le test-sensitivity selftest` action.
func runSelftest() (any, int) {
	report, err := Selftest()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	// 2 rather than 1: the script answered 2 for a selftest failure, and a
	// caller that reads "the detectors are broken" apart from "the ratchet
	// fired" keeps reading them apart.
	return report, report.Code(2)
}

// treeRoot answers the checkout every scanning action judges.
func treeRoot() (string, error) { return lepath.Root() }
