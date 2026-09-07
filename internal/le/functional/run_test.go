// Related: run.go, actions.go -- the two callers of Execute these tests hold together
//
// VALIDATES: a suite run under ZE_COVER records coverage whichever action
// started it, and the decision has one producer.
// PREVENTS: an instrumented run that collects nothing and still exits 0.

package functional

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/gotoolchain"
)

func TestSuiteCoverageIsSilentUntilZeCoverIsSet(t *testing.T) {
	suite := Suite{Name: "encode"}
	env.ResetCache()

	cover, reduce := suiteCoverage(gotoolchain.Toolchain{Root: t.TempDir()}, suite, "")
	if cover != "" {
		t.Fatalf("a run that records nothing named a coverage directory %q", cover)
	}
	reduce()

	covers := t.TempDir()
	cover, _ = suiteCoverage(gotoolchain.Toolchain{Root: t.TempDir()}, suite, covers)
	if cover != filepath.Join(covers, suite.Name) {
		t.Fatalf("coverage directory = %q, want the suite's own under %q", cover, covers)
	}
}

// TestEverySuiteRunTakesItsCoverageFromOneProducer is the test the defect asked
// for. `ZE_COVER=1 ./le functional <suite>` built instrumented binaries and
// collected nothing, because the gating loop derived GOCOVERDIR and the
// single-suite action passed a literal empty string in its place. A behavioral
// test cannot reach that: proving it would mean running a whole suite twice.
// What CAN be held is the property that made the two answers diverge, so this
// reads the source and refuses any call to Execute whose coverage argument is
// not the one suiteCoverage produced.
func TestEverySuiteRunTakesItsCoverageFromOneProducer(t *testing.T) {
	const coverArgument = 3

	found := 0
	for _, name := range []string{"run.go", "actions.go"} {
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(".", name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, isCall := node.(*ast.CallExpr)
			if !isCall {
				return true
			}
			callee, named := call.Fun.(*ast.Ident)
			if !named || callee.Name != "Execute" || len(call.Args) <= coverArgument {
				return true
			}
			found++
			argument, isIdent := call.Args[coverArgument].(*ast.Ident)
			if !isIdent || argument.Name != "cover" {
				t.Errorf("%s: Execute takes its coverage from something other than suiteCoverage", name)
			}
			return true
		})
		if !strings.Contains(sourceOf(t, name), "suiteCoverage(") {
			t.Errorf("%s calls Execute without asking suiteCoverage for the directory", name)
		}
	}
	if found != 2 {
		t.Fatalf("found %d calls to Execute, want the gating loop and the single-suite action", found)
	}
}

func sourceOf(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(".", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(raw)
}
