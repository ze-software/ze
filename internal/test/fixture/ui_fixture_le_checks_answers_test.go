// Design: docs/architecture/testing/runner-architecture.md -- semantic UI answer contracts.

package fixture

import "testing"

// TestLEChecksPassedRowsAcceptsAdditionalCases proves a growing selftest corpus
// preserves the contract while reporting its actual population to count checks.
func TestLEChecksPassedRowsAcceptsAdditionalCases(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows string
		want int
	}{
		{"one", `[{"case":"original","passed":true}]`, 1},
		{"extended", `[{"case":"original","passed":true},{"case":"new regression","passed":true}]`, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := leChecksPassedRows(tc.rows, "fixture")
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("row population = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestLEChecksPassedRowsRejectsInvalidEvidence proves removing a count pin does
// not admit missing evidence, duplicate identities, failed cases, or bad JSON.
func TestLEChecksPassedRowsRejectsInvalidEvidence(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows string
	}{
		{"empty", `[]`},
		{"null", `null`},
		{"object", `{}`},
		{"non-object row", `[true]`},
		{"missing name", `[{"passed":true}]`},
		{"empty name", `[{"case":" ","passed":true}]`},
		{"duplicate", `[{"case":"same","passed":true},{"case":"same","passed":true}]`},
		{"failed", `[{"case":"regression","passed":false}]`},
		{"missing verdict", `[{"case":"regression"}]`},
		{"wrong verdict type", `[{"case":"regression","passed":"true"}]`},
		{"trailing payload", `[{"case":"regression","passed":true}] []`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := leChecksPassedRows(tc.rows, "fixture"); err == nil {
				t.Fatalf("invalid selftest evidence was accepted: %s", tc.rows)
			}
		})
	}
}
