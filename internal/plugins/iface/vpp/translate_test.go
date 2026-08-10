// VPP interface translate: pure conversions with no VPP channel -- the ze<->VPP
// name map, VPP fixed-length C-string trimming, SwInterfaceDetails->InterfaceInfo
// decoding, and the FIB-source / neighbor-state enum name lookups.
package ifacevpp

import (
	"testing"

	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ip_neighbor"
)

func TestNameMapAddLookup(t *testing.T) {
	// VALIDATES: AC-13 -- ze short name maps to VPP index
	// PREVENTS: name lookup failure after add
	m := newNameMap()
	m.Add("xe0", 1, "TenGigabitEthernet3/0/0")

	idx, ok := m.lookupIndex("xe0")
	if !ok || idx != 1 {
		t.Errorf("LookupIndex(xe0) = %d, %v", idx, ok)
	}

	name, ok := m.lookupName(1)
	if !ok || name != "xe0" {
		t.Errorf("LookupName(1) = %q, %v", name, ok)
	}

	vppName, ok := m.lookupVPPName(1)
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

	if _, ok := m.lookupIndex("xe0"); ok {
		t.Error("xe0 should be removed")
	}
	if _, ok := m.lookupName(1); ok {
		t.Error("index 1 should be removed")
	}
}

func TestNameMapNotFound(t *testing.T) {
	// VALIDATES: missing name returns false
	// PREVENTS: zero-value confusion
	m := newNameMap()

	if _, ok := m.lookupIndex("nonexistent"); ok {
		t.Error("should not find nonexistent name")
	}
	if _, ok := m.lookupName(999); ok {
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

// --- neighborStateName ---

func TestNeighborStateNameStatic(t *testing.T) {
	if got := neighborStateName(ip_neighbor.IP_API_NEIGHBOR_FLAG_STATIC); got != "permanent" {
		t.Errorf("STATIC: got %q, want permanent", got)
	}
}

func TestNeighborStateNameNone(t *testing.T) {
	if got := neighborStateName(0); got != "reachable" {
		t.Errorf("0: got %q, want reachable (default)", got)
	}
}

func TestNeighborStateNameNoFibEntry(t *testing.T) {
	// NO_FIB_ENTRY alone (no STATIC) is still considered a resolved
	// cache entry, so the state remains "reachable".
	if got := neighborStateName(ip_neighbor.IP_API_NEIGHBOR_FLAG_NO_FIB_ENTRY); got != "reachable" {
		t.Errorf("NO_FIB_ENTRY: got %q, want reachable", got)
	}
}

// --- fibSourceName ---

func TestFibSourceNameKnown(t *testing.T) {
	if got := fibSourceName(19); got != "bgp" {
		t.Errorf("fibSourceName(19): got %q, want bgp", got)
	}
	if got := fibSourceName(10); got != "dhcp" {
		t.Errorf("fibSourceName(10): got %q, want dhcp", got)
	}
}

func TestFibSourceNameUnknown(t *testing.T) {
	// 200 is outside the well-known range; expect decimal string.
	if got := fibSourceName(200); got != "200" {
		t.Errorf("fibSourceName(200): got %q, want 200", got)
	}
}

// --- trimCString ---

func TestTrimCStringNULTerminated(t *testing.T) {
	// VALIDATES: AC-10 -- VPP fixed-length strings parsed correctly
	// PREVENTS: returning strings with embedded NULs to consumers
	got := trimCString("TenGigabitEthernet3/0/0\x00\x00\x00")
	if got != "TenGigabitEthernet3/0/0" {
		t.Errorf("trimCString: got %q, want %q", got, "TenGigabitEthernet3/0/0")
	}
}

func TestTrimCStringNoNUL(t *testing.T) {
	// VALIDATES: strings without NUL are returned verbatim
	got := trimCString("loop0")
	if got != "loop0" {
		t.Errorf("trimCString: got %q, want loop0", got)
	}
}

// --- detailsToInfo ---

func TestDetailsToInfoAdminUp(t *testing.T) {
	// VALIDATES: AC-10 -- admin state "up" derived from ADMIN_UP flag
	d := &interfaces.SwInterfaceDetails{
		SwIfIndex:     1,
		InterfaceName: asciiName("xe0"),
		Flags:         interface_types.IF_STATUS_API_FLAG_ADMIN_UP,
		Mtu:           []uint32{9000, 0, 0, 0},
	}
	info := detailsToInfo(d)
	if info.Name != "xe0" {
		t.Errorf("Name: got %q, want xe0", info.Name)
	}
	if info.State != "up" {
		t.Errorf("State: got %q, want up", info.State)
	}
	if info.MTU != 9000 {
		t.Errorf("MTU: got %d, want 9000", info.MTU)
	}
	if info.Index != 1 {
		t.Errorf("Index: got %d, want 1", info.Index)
	}
}

func TestDetailsToInfoAdminDown(t *testing.T) {
	// VALIDATES: AC-10 -- absence of ADMIN_UP flag reports state "down"
	d := &interfaces.SwInterfaceDetails{
		SwIfIndex:     2,
		InterfaceName: asciiName("loop0"),
		Flags:         0,
	}
	info := detailsToInfo(d)
	if info.State != "down" {
		t.Errorf("State: got %q, want down", info.State)
	}
}
