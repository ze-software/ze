// VALIDATES: the seam answers the declared distance once published, the
// bootstrap before that, and nothing for a protocol neither names.
// PREVENTS: an unset seam ranking a route at zero, the best distance there is.

package distance

import "testing"

// TestUnsetSeamDoesNotAnswerZero is the whole reason this package exists rather
// than reusing igpcost's shape. A distance of 0 is the BEST possible distance,
// the one `connected` holds, so an unset seam reporting 0 would silently make
// every route beat every other protocol. Before the declaration is published,
// Resolve answers the bootstrap value, and a protocol nothing names gets no
// answer at all.
func TestUnsetSeamDoesNotAnswerZero(t *testing.T) {
	Set(nil)

	if _, ok := Of("ebgp"); ok {
		t.Fatal("an unset seam claimed to answer")
	}
	if got, ok := Resolve("ebgp"); !ok || got != 20 {
		t.Errorf("Resolve on an unset seam = (%d,%v), want the bootstrap (20,true)", got, ok)
	}
	if _, ok := Resolve("rip"); ok {
		t.Error("Resolve answered for a protocol neither the declaration nor the bootstrap names")
	}
}

// TestDeclaredValueReachesTheRIB covers the path the whole design turns on: the
// operator's value has to arrive at the Loc-RIB, which ranks on it.
func TestDeclaredValueReachesTheRIB(t *testing.T) {
	Set(func(protocol string) (uint8, bool) {
		d, ok := map[string]uint8{"ebgp": 250, "ospf": 110, "connected": 0}[protocol]
		return d, ok
	})
	defer Set(nil)

	if got, _ := Resolve("ebgp"); got != 250 {
		t.Errorf("ebgp = %d, want the declared 250 rather than the bootstrap 20", got)
	}
	if got, _ := Resolve("ospf"); got != 110 {
		t.Errorf("ospf = %d, want 110", got)
	}

	// An operator CAN declare 0 for connected, and that is not "no answer".
	got, ok := Of("connected")
	if !ok || got != 0 {
		t.Errorf("connected = (%d,%v), want (0,true): a declared zero is an answer", got, ok)
	}

	// A protocol the declaration does not name falls back rather than zeroing.
	if got, _ := Resolve("isis"); got != 115 {
		t.Errorf("isis = %d, want the bootstrap 115 for an unnamed protocol", got)
	}
}

// TestSetReplacesRatherThanMerges pins reload behavior: a later Set is the
// whole table, so a leaf an operator removed reverts to the bootstrap rather
// than lingering at its old configured value.
func TestSetReplacesRatherThanMerges(t *testing.T) {
	Set(func(string) (uint8, bool) { return 250, true })
	if got, _ := Resolve("ebgp"); got != 250 {
		t.Fatalf("setup: ebgp = %d, want 250", got)
	}

	Set(func(string) (uint8, bool) { return 0, false })
	defer Set(nil)
	if got, _ := Resolve("ebgp"); got != 20 {
		t.Errorf("after a reload that drops the leaf, ebgp = %d, want the bootstrap 20", got)
	}
}
