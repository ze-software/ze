package config

import (
	"testing"
)

// TestSetLeafInactive verifies that Tree tracks leaf-level inactivity
// in a sibling map separate from the leaf value, so the value is
// preserved verbatim while the leaf is marked deactivated.
//
// VALIDATES: AC-5 (loaded config with deactivated leaf returns absent
// after PruneInactive) -- this test covers only the Tree primitive;
// the prune integration is covered separately in prune_test.go.
//
// PREVENTS: encoding inactivity inside the value string (collision
// risk if a real value starts with "inactive:") and pollution of
// Tree.values with synthetic markers.
func TestSetLeafInactive(t *testing.T) {
	tree := NewTree()
	tree.Set("router-id", "10.0.0.1")

	if tree.IsLeafInactive("router-id") {
		t.Fatalf("fresh leaf must report active")
	}

	tree.SetLeafInactive("router-id", true)

	if !tree.IsLeafInactive("router-id") {
		t.Fatalf("after SetLeafInactive(true) IsLeafInactive must be true")
	}

	v, ok := tree.Get("router-id")
	if !ok || v != "10.0.0.1" {
		t.Fatalf("value must survive deactivation: got (%q,%v)", v, ok)
	}

	tree.SetLeafInactive("router-id", false)
	if tree.IsLeafInactive("router-id") {
		t.Fatalf("after SetLeafInactive(false) IsLeafInactive must be false")
	}
}

// TestSetLeafInactiveUnknownLeaf verifies marking a name that has no
// value present is permitted (idempotent; pre-mark before set is fine).
// Avoids forcing a strict ordering between Set and SetLeafInactive.
func TestSetLeafInactiveUnknownLeaf(t *testing.T) {
	tree := NewTree()
	tree.SetLeafInactive("router-id", true)
	if !tree.IsLeafInactive("router-id") {
		t.Fatalf("SetLeafInactive on absent leaf must still record state")
	}
}

// TestClearLeafInactive verifies the explicit clear method removes
// the entry rather than leaving a `false` zombie in the map.
func TestClearLeafInactive(t *testing.T) {
	tree := NewTree()
	tree.SetLeafInactive("router-id", true)
	tree.ClearLeafInactive("router-id")
	if tree.IsLeafInactive("router-id") {
		t.Fatalf("after ClearLeafInactive IsLeafInactive must be false")
	}
}

// TestCloneLeafInactive verifies leaf inactivity is deep-copied so
// mutating the clone does not leak back to the original.
func TestCloneLeafInactive(t *testing.T) {
	tree := NewTree()
	tree.Set("router-id", "10.0.0.1")
	tree.SetLeafInactive("router-id", true)

	clone := tree.Clone()
	if !clone.IsLeafInactive("router-id") {
		t.Fatalf("clone must preserve leaf-inactive state")
	}

	clone.SetLeafInactive("router-id", false)
	if !tree.IsLeafInactive("router-id") {
		t.Fatalf("mutation on clone must not leak to original")
	}
}
