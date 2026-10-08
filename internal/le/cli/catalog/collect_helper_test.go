package clicatalog

import "testing"

// collectForTest answers Collect and fails the test on its error, the YANG
// loader's refusal of the schema. Every test here runs over the shipped
// schema, so an error is a broken tree, never an empty catalog to assert on.
func collectForTest(t testing.TB) []Entry {
	t.Helper()
	entries, err := Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	return entries
}
