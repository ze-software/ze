package hookcheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"
)

// ungroundedCategories are the fixture categories whose verdict RESTATES the
// rule instead of asking the producer for it. Each one is a place where the
// fixtures can go on passing after the producer stops doing what they describe.
//
// It is a ratchet, not an allowance. A category may leave this list, never join
// it, and a category that leaves must not come back.
//
// The list exists because that failure has already happened once and nothing
// saw it. `eae2825926` replaced the Python hooks with Go, and the design-gate
// behaviour did not come across: writeDesignEvidence
// (internal/le/hookruntime/writeedit.go) now clears on ANY LSP marker, or
// failing that ANY source-read marker, with no per-kind demand. The 52
// design-gate fixtures stayed green throughout, because their verdict is
// `strings.Cut(value, ":")` compared with itself and never calls the gate.
// plan/spec-finish-ci-coverage.md cited those fixtures as its proof, and the
// hook-source-drift digest did not help: it says the producer MOVED, and a
// deliberate rewrite re-baselines it, which is exactly when a simulation
// silently stops matching.
//
//nolint:gochecknoglobals // a ratchet baseline is data, and it is read by one test
var ungroundedCategories = []string{
	categoryCISleepMarker,
	categoryCommitGate,
	categoryDelegation,
	categoryDelegationReminder,
	categoryDesignGate,
	categoryDesignRef,
	categoryDraftIncubator,
	categoryFormatAlloc,
	categoryPhaseGates,
	categoryRFCChangedLedger,
	categoryRFCLanguage,
	categoryRFCTestGuard,
	categoryRenderedRule,
	categoryScriptWeakeningArms,
	categorySessionState,
	categorySessionStateLocation,
	categorySubagentContext,
	categoryTestFirst,
	categoryValidateSpec,
	categoryWeakenedHatch,
}

// Twenty of twenty-five, measured on 2026-09-03 when this guard was written.
// Five categories call their producer and cannot drift: mark-source-read asks
// sourceKind, governed-doc-edit asks governedWrite, session-id asks
// safeSessionID, and raw-job-admission and journal-row-shape do the same. The
// other twenty describe a rule the producer is supposed to follow, and nothing
// makes the producer follow it.

// groundedByConstruction are the identifiers that do NOT ground a verdict: the
// standard-library string and regexp work any restatement is built from. A
// verdict calling only these has reimplemented its producer's rule inline.
//
//nolint:gochecknoglobals // read by one test
var groundedByConstruction = []string{
	"Contains", "HasSuffix", "HasPrefix", "Cut", "TrimSpace", "EqualFold",
	"Split", "Fields", "Index", "Count", "ReplaceAll", "ToLower", "ToUpper",
	"MatchString", "len", "append",
}

// TestEveryFixtureCategoryReachesItsProducer holds the rule that a fixture
// naming a producer must ASK it, because a fixture that restates the rule
// cannot go red when the producer stops following it.
//
// VALIDATES: categoryVerdict's case for each category calls something other
// than string matching, so the fixtures are bound to the code they describe.
// PREVENTS: the design-gate failure recorded in
// plan/journal/refactor-removes-feature.md, where a migration kept 52 fixture
// names, dropped the behaviour they exercised, and left a green suite that a
// spec cited as proof.
//
// It reads this package's own source, which is the only way to see whether a
// verdict CALLS the producer: no runtime observation can distinguish a correct
// simulation from a correct producer.
func TestEveryFixtureCategoryReachesItsProducer(t *testing.T) {
	calls := categoryVerdictCalls(t)

	for _, category := range &fixtureCategories {
		called, seen := calls[category.name]
		if !seen {
			t.Errorf("category %q has a row in fixtureCategories and no case in categoryVerdict, "+
				"so its fixtures assert nothing", category.name)
			continue
		}
		grounded := false
		for _, name := range called {
			if !slices.Contains(groundedByConstruction, name) {
				grounded = true
				break
			}
		}
		listed := slices.Contains(ungroundedCategories, category.name)
		switch {
		case grounded && listed:
			t.Errorf("category %q now calls its producer, so remove it from ungroundedCategories: "+
				"the ratchet only tightens", category.name)
		case !grounded && !listed:
			t.Errorf("category %q decides its verdict with string matching alone (%v), so it restates "+
				"%s rather than asking it. A fixture that restates its producer cannot go red when the "+
				"producer stops following the rule, which is how 52 design-gate fixtures stayed green "+
				"through a rewrite that dropped what they proved. Call the producer, or state why not by "+
				"adding the category to ungroundedCategories with a reason",
				category.name, called, category.evidence)
		}
	}

	for _, name := range ungroundedCategories {
		if !slices.ContainsFunc(fixtureCategories[:], func(c fixtureCategory) bool { return c.name == name }) {
			t.Errorf("ungroundedCategories names %q, which is not a fixture category: "+
				"a baseline that outlives its subject stops being read", name)
		}
	}
}

// categoryVerdictCalls maps each case of categoryVerdict to the function names
// its body calls. A case that calls nothing yields an empty slice, which is
// still an answer: it decides on the value alone.
func categoryVerdictCalls(t *testing.T) map[string][]string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixtures.go", nil, 0)
	if err != nil {
		t.Fatalf("parse fixtures.go: %v", err)
	}

	out := map[string][]string{}
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "categoryVerdict" {
			return true
		}
		ast.Inspect(fn.Body, func(inner ast.Node) bool {
			clause, ok := inner.(*ast.CaseClause)
			if !ok {
				return true
			}
			var names []string
			ast.Inspect(clause, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch fun := call.Fun.(type) {
				case *ast.Ident:
					names = append(names, fun.Name)
				case *ast.SelectorExpr:
					names = append(names, fun.Sel.Name)
				}
				return true
			})
			for _, expr := range clause.List {
				ident, ok := expr.(*ast.Ident)
				if !ok {
					continue
				}
				out[categoryConstantValue(t, file, ident.Name)] = names
			}
			return true
		})
		return false
	})
	if len(out) == 0 {
		t.Fatal("categoryVerdict has no case clauses, so this guard reads nothing")
	}
	return out
}

// categoryConstantValue resolves a category constant's identifier to its string
// value, so the map is keyed by the same name fixtureCategories carries.
func categoryConstantValue(t *testing.T, file *ast.File, ident string) string {
	t.Helper()
	var value string
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, name := range spec.Names {
			if name.Name != ident || i >= len(spec.Values) {
				continue
			}
			if lit, ok := spec.Values[i].(*ast.BasicLit); ok {
				value = strings.Trim(lit.Value, `"`)
			}
		}
		return true
	})
	if value == "" {
		t.Fatalf("cannot resolve category constant %q to its string value", ident)
	}
	return value
}
