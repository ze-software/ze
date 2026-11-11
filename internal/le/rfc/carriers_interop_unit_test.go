// Design: docs/contributing/rfc-conformance-gates.md -- unit and interop evidence carriers.
package rfc

import "testing"

// TestInteropCatalogUnitCarrier preserves the execution boundary: Go tests in
// a known lab use the table's unit carrier; real checkers keep the catalog's
// workflow-derived tier. Unknown labs and unrelated tooling gain no evidence.
// Both scheduled and unscheduled catalogs are checked without running a lab.
func TestInteropCatalogUnitCarrier(t *testing.T) {
	trees := interopTrees()
	if len(trees) == 0 {
		t.Fatal("no interop catalogs linked; carrier controls would be vacuous")
	}
	for _, scheduled := range []bool{false, true} {
		workflows := make(map[string]string)
		if scheduled {
			for _, tree := range trees {
				workflows[tree.action] = "fixture-nightly.yml"
			}
		}
		table := carriersFor(nil, workflows)
		unit, held := CarrierFor("internal/example/example_test.go", table)
		if !held {
			t.Fatal("ordinary unit carrier missing")
		}
		if unit.Kind != kindUnit {
			t.Fatalf("ordinary unit kind=%q", unit.Kind)
		}
		if unit.Tier != tierVerify {
			t.Fatalf("ordinary unit tier=%q", unit.Tier)
		}
		for _, checker := range interopCarriers(workflows) {
			actual, found := CarrierFor(checker.Prefix+"check_example.go", table)
			if !found {
				t.Fatalf("checker for %s missing", checker.Name)
			}
			if actual != checker {
				t.Errorf("checker carrier=%+v, want unchanged catalog row %+v", actual, checker)
			}
			actual, found = CarrierFor(checker.Prefix+"check_example_test.go", table)
			if !found {
				t.Fatalf("unit for %s missing", checker.Name)
			}
			if actual != unit {
				t.Errorf("scheduled=%t catalog=%s: test carrier=%+v, want existing unit row %+v",
					scheduled, checker.Name, actual, unit)
			}
		}
		for _, suffix := range []string{"check.go", "check_test.go"} {
			unknown, found := CarrierFor("internal/le/interoplab/unregistered-fixture/"+suffix, table)
			if !found {
				t.Fatal("unknown lab lost its explicit refusal carrier")
			}
			if unknown.Kind != kindUnknown {
				t.Errorf("unknown lab kind=%q, want unknown", unknown.Kind)
			}
			if unknown.Tier != tierUnrun {
				t.Errorf("unknown lab tier=%q, want unrun", unknown.Tier)
			}
		}
		if _, found := CarrierFor("internal/le/rfc/example_test.go", table); found {
			t.Error("development-tool test admitted as RFC evidence")
		}
	}
}
