package dnssource

import (
	"net/netip"
	"testing"
)

// TestFollowerAcceptSnapshot: a consumer in the daemon hands the typed
// snapshot to AcceptSnapshot, which applies the same sequence rule as Accept
// and copies the records, so a later change to the producer's slice does not
// reach the consumer.
//
// VALIDATES: the RA RDNSS consumer's view of (system-dns, snapshot)
// (spec-home-cpe-23b, phase 5).
// PREVENTS: an in-process consumer aliasing the registry's snapshot, or
// rolling back to a replayed older one.
func TestFollowerAcceptSnapshot(t *testing.T) {
	var f Follower
	records := []Record{{Origin: "dhcpv6", Rank: 200, Addresses: []netip.Addr{netip.MustParseAddr("2001:db8::53")}}}
	if !f.AcceptSnapshot(&Snapshot{Sequence: 2, Records: records}) {
		t.Fatal("first snapshot not kept")
	}
	records[0].Origin = "changed"
	if got := f.Records(); got[0].Origin != "dhcpv6" {
		t.Fatalf("records aliased the producer's slice: origin %q", got[0].Origin)
	}
	if f.AcceptSnapshot(&Snapshot{Sequence: 2}) {
		t.Fatal("a repeated sequence was kept")
	}
	if f.AcceptSnapshot(&Snapshot{Sequence: 1}) {
		t.Fatal("an older sequence was kept")
	}
	if !f.AcceptSnapshot(&Snapshot{Sequence: 3}) || len(f.Records()) != 0 {
		t.Fatalf("newer empty snapshot not applied: %v", f.Records())
	}
}
