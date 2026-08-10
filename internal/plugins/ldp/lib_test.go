package ldp

import (
	"net/netip"
	"testing"
)

func TestLIBAddAndLookup(t *testing.T) {
	lib := newLIB()
	fec := netip.MustParsePrefix("10.1.0.0/24")
	peerKey := "10.0.0.1:0"
	peerAddr := netip.MustParseAddr("10.0.0.1")

	lib.AddRemote(fec, 1000, peerKey, peerAddr, peerAddr)

	b, ok := lib.LookupRemote(fec, peerKey)
	if !ok {
		t.Fatal("LookupRemote failed")
	}
	if b.Label != 1000 {
		t.Errorf("Label = %d, want 1000", b.Label)
	}
	if b.FEC != fec {
		t.Errorf("FEC = %s, want %s", b.FEC, fec)
	}
	if b.PeerAddr != peerAddr {
		t.Errorf("PeerAddr = %s, want %s", b.PeerAddr, peerAddr)
	}
}

func TestLIBRemoveRemote(t *testing.T) {
	lib := newLIB()
	fec := netip.MustParsePrefix("10.1.0.0/24")
	peerKey := "10.0.0.1:0"

	lib.AddRemote(fec, 1000, peerKey, netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.1"))

	removed := lib.removeRemote(fec, peerKey)
	if removed == nil {
		t.Fatal("RemoveRemote returned nil")
	}
	if removed.Label != 1000 {
		t.Errorf("removed Label = %d, want 1000", removed.Label)
	}

	_, ok := lib.LookupRemote(fec, peerKey)
	if ok {
		t.Fatal("LookupRemote succeeded after remove")
	}
	if lib.Len() != 0 {
		t.Errorf("Len = %d, want 0", lib.Len())
	}
}

func TestLIBRemoveAllForPeer(t *testing.T) {
	lib := newLIB()
	peer := "10.0.0.1:0"
	addr := netip.MustParseAddr("10.0.0.1")

	lib.AddRemote(netip.MustParsePrefix("10.1.0.0/24"), 1000, peer, addr, addr)
	lib.AddRemote(netip.MustParsePrefix("10.2.0.0/24"), 1001, peer, addr, addr)
	lib.AddRemote(netip.MustParsePrefix("10.3.0.0/24"), 2000, "10.0.0.2:0", netip.MustParseAddr("10.0.0.2"), netip.MustParseAddr("10.0.0.2"))

	removed := lib.removeAllForPeer(peer)
	if len(removed) != 2 {
		t.Fatalf("RemoveAllForPeer returned %d, want 2", len(removed))
	}
	if lib.Len() != 1 {
		t.Errorf("Len = %d, want 1 (other peer's binding)", lib.Len())
	}
}

func TestLIBAllocateLabel(t *testing.T) {
	lib := newLIB()
	l1 := lib.AllocateLabel()
	l2 := lib.AllocateLabel()
	if l1 < 16 {
		t.Errorf("first label = %d, want >= 16 (reserved range)", l1)
	}
	if l2 <= l1 {
		t.Errorf("labels not monotonic: %d then %d", l1, l2)
	}
}

func TestLIBAllBindings(t *testing.T) {
	lib := newLIB()
	lib.AddRemote(netip.MustParsePrefix("10.1.0.0/24"), 1000, "peer1", netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.1"))
	lib.AddRemote(netip.MustParsePrefix("10.2.0.0/24"), 1001, "peer1", netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.1"))
	lib.AddRemote(netip.MustParsePrefix("10.1.0.0/24"), 2000, "peer2", netip.MustParseAddr("10.0.0.2"), netip.MustParseAddr("10.0.0.2"))

	all := lib.allBindings()
	if len(all) != 3 {
		t.Errorf("AllBindings returned %d, want 3", len(all))
	}
}

func TestLIBMultiplePeersSameFEC(t *testing.T) {
	lib := newLIB()
	fec := netip.MustParsePrefix("10.1.0.0/24")

	lib.AddRemote(fec, 1000, "peer1", netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.1"))
	lib.AddRemote(fec, 2000, "peer2", netip.MustParseAddr("10.0.0.2"), netip.MustParseAddr("10.0.0.2"))

	b1, ok := lib.LookupRemote(fec, "peer1")
	if !ok || b1.Label != 1000 {
		t.Errorf("peer1 binding: ok=%v label=%d", ok, b1.Label)
	}
	b2, ok := lib.LookupRemote(fec, "peer2")
	if !ok || b2.Label != 2000 {
		t.Errorf("peer2 binding: ok=%v label=%d", ok, b2.Label)
	}
}

func TestLIBRemoveNonExistent(t *testing.T) {
	lib := newLIB()
	fec := netip.MustParsePrefix("10.1.0.0/24")

	removed := lib.removeRemote(fec, "nonexistent")
	if removed != nil {
		t.Error("RemoveRemote should return nil for non-existent")
	}
}

// VALIDATES: AC-3 -- EnsureLocal allocates a local label on first use and is
// idempotent, so a FEC keeps one stable label across repeated calls.
func TestLIBEnsureLocalIdempotent(t *testing.T) {
	lib := newLIB()
	fec := netip.MustParsePrefix("10.1.0.0/24")

	first := lib.EnsureLocal(fec)
	if first.Label < 16 {
		t.Errorf("local label = %d, want >= 16 (reserved range)", first.Label)
	}
	if first.FEC != fec {
		t.Errorf("FEC = %s, want %s", first.FEC, fec)
	}

	second := lib.EnsureLocal(fec)
	if second.Label != first.Label {
		t.Errorf("EnsureLocal not idempotent: %d then %d", first.Label, second.Label)
	}
}

// VALIDATES: AC-3 -- distinct FECs get distinct local labels and all surface in
// the LocalBindings snapshot (feeds `show ldp binding`, AC-8).
func TestLIBEnsureLocalDistinct(t *testing.T) {
	lib := newLIB()
	a := lib.EnsureLocal(netip.MustParsePrefix("10.1.0.0/24"))
	b := lib.EnsureLocal(netip.MustParsePrefix("10.2.0.0/24"))
	if a.Label == b.Label {
		t.Errorf("distinct FECs share label %d", a.Label)
	}

	locals := lib.localBindings()
	if len(locals) != 2 {
		t.Fatalf("LocalBindings returned %d, want 2", len(locals))
	}
}

func TestLIBLocalBindingsEmpty(t *testing.T) {
	lib := newLIB()
	if got := lib.localBindings(); len(got) != 0 {
		t.Errorf("LocalBindings on empty LIB returned %d, want 0", len(got))
	}
}

// VALIDATES: label allocation skips labels already in use, so the wrap from
// MaxLabel back to 16 cannot hand out a duplicate local label.
func TestLIBAllocateSkipsUsed(t *testing.T) {
	lib := newLIB()
	// Simulate 16 and 17 already allocated (e.g. after a wraparound).
	lib.usedLabels[16] = true
	lib.usedLabels[17] = true

	b := lib.EnsureLocal(netip.MustParsePrefix("10.1.0.0/24"))
	if b.Label != 18 {
		t.Errorf("allocated label = %d, want 18 (16 and 17 in use)", b.Label)
	}
	// The newly allocated label is now also marked used.
	c := lib.EnsureLocal(netip.MustParsePrefix("10.2.0.0/24"))
	if c.Label == b.Label {
		t.Errorf("reused in-use label %d", c.Label)
	}
}
