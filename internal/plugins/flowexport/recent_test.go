package flowexport

import (
	"net/netip"
	"testing"
)

func flowTo(dst string, dport uint16) ConntrackFlow {
	return ConntrackFlow{
		SrcAddr:  netip.MustParseAddr("198.51.100.1"),
		DstAddr:  netip.MustParseAddr(dst),
		DstPort:  dport,
		Protocol: 17,
		Packets:  1,
		Bytes:    64,
	}
}

// TestRecentRingBounded proves the ring is fixed-size and drop-oldest: appending
// more than capacity keeps exactly the newest `size` records in oldest-to-newest
// order, and counts the overwrites. (AC-11: bounded to configured ring size.)
func TestRecentRingBounded(t *testing.T) {
	r := newRecentRing(4)
	for i := range 10 {
		r.append([]ConntrackFlow{flowTo("203.0.113.9", uint16(1000+i))})
	}

	got := r.snapshot(netip.Prefix{})
	if len(got) != 4 {
		t.Fatalf("snapshot len = %d, want 4 (ring capacity)", len(got))
	}
	// Oldest-to-newest: the last 4 appended were ports 1006..1009.
	for i, want := range []uint16{1006, 1007, 1008, 1009} {
		if got[i].DstPort != want {
			t.Errorf("record[%d] dst-port = %d, want %d", i, got[i].DstPort, want)
		}
	}
	if drops := r.dropCount(); drops != 6 {
		t.Errorf("dropCount = %d, want 6 (10 appended - 4 capacity)", drops)
	}
}

// TestRecentRingNotFull covers the partially-filled ring (no wrap yet).
func TestRecentRingNotFull(t *testing.T) {
	r := newRecentRing(8)
	for i := range 3 {
		r.append([]ConntrackFlow{flowTo("203.0.113.9", uint16(2000+i))})
	}
	got := r.snapshot(netip.Prefix{})
	if len(got) != 3 {
		t.Fatalf("snapshot len = %d, want 3", len(got))
	}
	if got[0].DstPort != 2000 || got[2].DstPort != 2002 {
		t.Errorf("order wrong: got ports %d..%d, want 2000..2002", got[0].DstPort, got[2].DstPort)
	}
	if drops := r.dropCount(); drops != 0 {
		t.Errorf("dropCount = %d, want 0 (not yet full)", drops)
	}
}

// TestRecentRingDstFilter proves snapshot filters by destination prefix, which
// is how the characterizer isolates flows to the victim.
func TestRecentRingDstFilter(t *testing.T) {
	r := newRecentRing(16)
	r.append([]ConntrackFlow{
		flowTo("203.0.113.9", 1), // victim
		flowTo("198.18.0.7", 2),  // unrelated
		flowTo("203.0.113.9", 3), // victim
		flowTo("2001:db8::1", 4), // unrelated v6
	})

	victim := netip.MustParsePrefix("203.0.113.9/32")
	got := r.snapshot(victim)
	if len(got) != 2 {
		t.Fatalf("filtered snapshot len = %d, want 2", len(got))
	}
	for _, f := range got {
		if f.DstAddr != netip.MustParseAddr("203.0.113.9") {
			t.Errorf("filtered record has dst %s, want 203.0.113.9", f.DstAddr)
		}
	}

	// A /24 covering the victim also matches.
	if got := r.snapshot(netip.MustParsePrefix("203.0.113.0/24")); len(got) != 2 {
		t.Errorf("/24 filter len = %d, want 2", len(got))
	}
}

// TestRecentRingDisabled proves a nil or zero-size ring is inert and never
// panics -- the state when conntrack export is off.
func TestRecentRingDisabled(t *testing.T) {
	var nilRing *recentRing
	nilRing.append([]ConntrackFlow{flowTo("203.0.113.9", 1)})
	if got := nilRing.snapshot(netip.Prefix{}); got != nil {
		t.Errorf("nil ring snapshot = %v, want nil", got)
	}
	if d := nilRing.dropCount(); d != 0 {
		t.Errorf("nil ring dropCount = %d, want 0", d)
	}

	zero := newRecentRing(0)
	zero.append([]ConntrackFlow{flowTo("203.0.113.9", 1)})
	if got := zero.snapshot(netip.Prefix{}); got != nil {
		t.Errorf("zero-size ring snapshot = %v, want nil", got)
	}
}
