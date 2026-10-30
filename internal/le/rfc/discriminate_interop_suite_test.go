// Design: docs/contributing/rfc-conformance-gates.md -- discrimination routes for interop carriers
// Related: discriminate_observe.go -- interopSuiteOf, interopScenarioOf, carrierArgv, environment
// Related: carriers.go -- interopTrees, derived from the interoplab catalogs
//
// The cases prove the recorder drives a checker through the suite whose
// package holds it. The scenario directory, the selector variable and the
// action all come from that suite's interoplab catalog, so an IPsec checker is
// confirmed against test/interop-ipsec/scenarios and run under
// IPSEC_INTEROP_SCENARIO, and a BGP checker still resolves as before.
//
// VALIDATES: a checker resolves to its own suite's scenario root, selector and action.
// PREVENTS: one suite's directory and selector applied to every interop checker.

package rfc

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestInteropCheckerResolvesThroughItsSuite builds a tree holding one scenario
// directory, a checker that names it, and asserts the recorder resolves the
// scenario, the argv and the selector of the suite the checker's package is.
// Before the catalog declared these, every checker was confirmed against the
// BGP directory and run under INTEROP_SCENARIO, so the IPsec row was red.
func TestInteropCheckerResolvesThroughItsSuite(t *testing.T) {
	tests := []struct {
		suite, source, scenarioDir, selector string
		argv                                 []string
	}{
		{"bgp", "internal/le/interoplab/bgp/check_fixture.go", "test/interop/scenarios",
			"INTEROP_SCENARIO", []string{"le", "test", "integration", "interop"}},
		{"ipsec", "internal/le/interoplab/ipsec/check_fixture.go", "test/interop-ipsec/scenarios",
			"IPSEC_INTEROP_SCENARIO", []string{"le", "test", "integration", "interop-ipsec"}},
	}
	for _, tt := range tests {
		t.Run(tt.suite, func(t *testing.T) {
			suite, err := interopSuiteOf(tt.source)
			if err != nil {
				t.Fatal(err)
			}
			if suite.Suite != tt.suite {
				t.Fatalf("suite of %s = %q, want %q", tt.source, suite.Suite, tt.suite)
			}
			tree := t.TempDir()
			if err := os.MkdirAll(filepath.Join(tree, filepath.FromSlash(tt.scenarioDir), "fixture-scenario"), 0o755); err != nil {
				t.Fatal(err)
			}
			reader := newTextReader(map[string]string{
				tt.source: "package " + tt.suite + "\n\nfunc checkFixture() {\n\tconst name = \"fixture-scenario\"\n}\n",
			})
			record := DiscriminationRecord{Unit: tt.source + "::checkFixture"}
			scenario, err := interopScenarioOf(tree, reader, newScopeIndex(), &record, suite)
			if err != nil {
				t.Fatal(err)
			}
			if scenario != "fixture-scenario" {
				t.Fatalf("scenario = %q, want fixture-scenario", scenario)
			}
			runner := &observationRunner{carrier: Carrier{Kind: kindInterop}, self: "le",
				scenario: scenario, suite: suite}
			if argv := runner.carrierArgv(); !slices.Equal(argv, tt.argv) {
				t.Fatalf("argv = %q, want %q", argv, tt.argv)
			}
			want := tt.selector + "=fixture-scenario"
			if environ := runner.environment(""); environ[len(environ)-1] != want {
				t.Fatalf("selector entry = %q, want %q", environ[len(environ)-1], want)
			}
		})
	}
}

// TestInteropCheckerOutsideEverySuiteIsRefused asserts a file under no
// catalog's Package is an error naming the fix, never a default suite.
func TestInteropCheckerOutsideEverySuiteIsRefused(t *testing.T) {
	_, err := interopSuiteOf("internal/le/interoplab/nosuchlab/check.go")
	if err == nil {
		t.Fatal("a file under no registered suite resolved to one")
	}
	if !strings.Contains(err.Error(), "register_catalog.go") {
		t.Fatalf("refusal names no fix: %v", err)
	}
}

// TestEveryInteropLabRegistersItsCatalogHere walks internal/le/interoplab for
// lab packages, each recognized by its register_catalog.go, and asserts this
// package links every one of them with the Package it lives in and a scenario
// root that exists. The blank imports in interoplabs.go are the one copy this
// package keeps, and this is the check that compares it to its source.
func TestEveryInteropLabRegistersItsCatalogHere(t *testing.T) {
	const labs = "internal/le/interoplab"
	root := filepath.Join("..", "..", "..")
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(labs)))
	if err != nil {
		t.Fatal(err)
	}
	declared := map[string]interoplab.Catalog{}
	for _, name := range interoplab.CatalogSuites() {
		catalog, _ := interoplab.CatalogNamed(name)
		declared[catalog.Package] = catalog
	}
	found := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(labs), entry.Name(), "register_catalog.go")); statErr != nil {
			continue
		}
		found++
		pkg := labs + "/" + entry.Name() + "/"
		catalog, held := declared[pkg]
		if !held {
			t.Errorf("%s registers a catalog, and none linked here declares Package %s: add its blank import to interoplabs.go", pkg, pkg)
			continue
		}
		if info, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(catalog.ScenarioRoot))); statErr != nil || !info.IsDir() {
			t.Errorf("suite %s declares ScenarioRoot %s, which is not a directory", catalog.Suite, catalog.ScenarioRoot)
		}
	}
	if found == 0 {
		t.Fatal("no lab package found, so the walk proves nothing")
	}
}
