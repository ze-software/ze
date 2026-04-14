package ifacevpp

import "testing"

func TestNameMapAddLookup(t *testing.T) {
	// VALIDATES: AC-13 -- ze short name maps to VPP index
	// PREVENTS: name lookup failure after add
	m := newNameMap()
	m.Add("xe0", 1, "TenGigabitEthernet3/0/0")

	idx, ok := m.LookupIndex("xe0")
	if !ok || idx != 1 {
		t.Errorf("LookupIndex(xe0) = %d, %v", idx, ok)
	}

	name, ok := m.LookupName(1)
	if !ok || name != "xe0" {
		t.Errorf("LookupName(1) = %q, %v", name, ok)
	}

	vppName, ok := m.LookupVPPName(1)
	if !ok || vppName != "TenGigabitEthernet3/0/0" {
		t.Errorf("LookupVPPName(1) = %q, %v", vppName, ok)
	}
}

func TestNameMapRemove(t *testing.T) {
	// VALIDATES: name removed after interface deletion
	// PREVENTS: stale name entries
	m := newNameMap()
	m.Add("xe0", 1, "TenGigabitEthernet3/0/0")
	m.Remove("xe0")

	if _, ok := m.LookupIndex("xe0"); ok {
		t.Error("xe0 should be removed")
	}
	if _, ok := m.LookupName(1); ok {
		t.Error("index 1 should be removed")
	}
}

func TestNameMapNotFound(t *testing.T) {
	// VALIDATES: missing name returns false
	// PREVENTS: zero-value confusion
	m := newNameMap()

	if _, ok := m.LookupIndex("nonexistent"); ok {
		t.Error("should not find nonexistent name")
	}
	if _, ok := m.LookupName(999); ok {
		t.Error("should not find nonexistent index")
	}
}

func TestNameMapAll(t *testing.T) {
	// VALIDATES: All returns copy of all mappings
	// PREVENTS: mutation of internal state
	m := newNameMap()
	m.Add("xe0", 1, "TenGigabitEthernet3/0/0")
	m.Add("xe1", 2, "TenGigabitEthernet3/0/1")

	all := m.All()
	if len(all) != 2 {
		t.Errorf("expected 2 entries, got %d", len(all))
	}

	// Mutating returned map should not affect internal state.
	delete(all, "xe0")
	if m.Len() != 2 {
		t.Error("internal state mutated via All() return")
	}
}

func TestNameMapLen(t *testing.T) {
	m := newNameMap()
	if m.Len() != 0 {
		t.Error("new map should be empty")
	}
	m.Add("loop0", 0, "loop0")
	if m.Len() != 1 {
		t.Error("expected 1 after add")
	}
}

func TestNameMapRemoveNonexistent(t *testing.T) {
	// VALIDATES: removing nonexistent name is safe
	// PREVENTS: panic on double remove
	m := newNameMap()
	m.Remove("nonexistent") // should not panic
}
