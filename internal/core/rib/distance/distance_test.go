package distance

import "testing"

// TestUnsetSeamDoesNotAnswerZero is the whole reason this package exists rather
// than reusing igpcost's shape. A distance of 0 is the BEST possible distance,
// the one `connected` holds, so an unset seam reporting 0 would silently make
// every route beat every other protocol. igpcost can report 0 for an unset seam
// because 0 there means "no interior cost known" and makes its tiebreak a
// no-op; here it would decide the tiebreak.
func TestUnsetSeamDoesNotAnswerZero(t *testing.T) {
	Set(nil)

	if _, ok := Of("ebgp"); ok {
		t.Fatal("an unset seam claimed to answer")
	}
	if got := OrDefault("ebgp", 20); got != 20 {
		t.Errorf("OrDefault on an unset seam = %d, want the caller's bootstrap 20", got)
	}
}

// TestDeclaredValueReachesTheProducer covers the path the whole spec turns on:
// the operator's value has to arrive at the producer, because locrib.selectBest
// ranks on what the producer stamped and sysrib never sees the loser.
func TestDeclaredValueReachesTheProducer(t *testing.T) {
	Set(func(protocol string) (uint8, bool) {
		d, ok := map[string]uint8{"ebgp": 250, "ospf": 110, "connected": 0}[protocol]
		return d, ok
	})
	defer Set(nil)

	if got := OrDefault("ebgp", 20); got != 250 {
		t.Errorf("ebgp = %d, want the declared 250 rather than the bootstrap 20", got)
	}
	if got := OrDefault("ospf", 110); got != 110 {
		t.Errorf("ospf = %d, want 110", got)
	}

	// An operator CAN declare 0 for connected, and that is not "no answer".
	got, ok := Of("connected")
	if !ok || got != 0 {
		t.Errorf("connected = (%d,%v), want (0,true): a declared zero is an answer", got, ok)
	}

	// A protocol the declaration does not name falls back rather than zeroing.
	if got := OrDefault("isis", 115); got != 115 {
		t.Errorf("isis = %d, want the bootstrap 115 for an unnamed protocol", got)
	}
}

// TestSetReplacesRatherThanMerges pins reload behaviour: a later Set is the
// whole table, so a leaf an operator removed reverts to the caller's bootstrap
// rather than lingering at its old configured value.
func TestSetReplacesRatherThanMerges(t *testing.T) {
	Set(func(string) (uint8, bool) { return 250, true })
	if got := OrDefault("ebgp", 20); got != 250 {
		t.Fatalf("setup: ebgp = %d, want 250", got)
	}

	Set(func(string) (uint8, bool) { return 0, false })
	defer Set(nil)
	if got := OrDefault("ebgp", 20); got != 20 {
		t.Errorf("after a reload that drops the leaf, ebgp = %d, want the bootstrap 20", got)
	}
}
