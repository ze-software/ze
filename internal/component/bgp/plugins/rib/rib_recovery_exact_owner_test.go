// Design: docs/architecture/plugin/rib-storage-design.md -- exact recovery ownership.
package rib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

func TestRecoverySameAddressOwnersRemainIndependent(t *testing.T) {
	address := netip.MustParseAddr("192.0.2.1")
	r := &RIBManager{peerUp: map[uint64]bool{41: true, 42: true, 43: false}}
	request := ribevents.RecoveryRequest{Destination: address, DestinationOwner: 41}
	key := ribevents.RecoveryKey{Excluded: []uint64{42}}
	for _, tc := range []struct {
		owner uint64
		want  bool
	}{{41, false}, {42, false}, {43, false}, {44, true}} {
		candidate := Candidate{PeerIP: address, PeerOwner: tc.owner}
		if got := r.recoveryEligibleLocked(&request, &key, &candidate); got != tc.want {
			t.Errorf("owner %d at shared address: eligible=%v, want %v", tc.owner, got, tc.want)
		}
	}
	if recoveryExcludes(&key, 44) || recoveryExcludes(&key, 0) || !recoveryExcludes(&key, 42) {
		t.Fatal("SENT source exclusion confused original owners")
	}
}

func TestRecoverySentInventoryExcludesExactOwnerWithPathZero(t *testing.T) {
	prefix := netip.MustParsePrefix("203.0.113.0/24")
	identity := ribOutKey{Prefix: prefix}
	request := ribevents.RecoveryRequest{Family: family.IPv4Unicast,
		Keys: []ribevents.RecoveryKey{{Excluded: []uint64{42}}}}
	entries := map[ribOutKey]ribOutEntry{
		{Prefix: prefix, PathID: 0}: {SourcePeer: "192.0.2.1", SourceOwner: 42, AddPath: true},
		{Prefix: prefix, PathID: 7}: {SourcePeer: "192.0.2.1", SourceOwner: 43, AddPath: true},
	}
	outcomes := make([]ribevents.RecoveryOutcome, 1)
	if err := recoverySentAddPathLocked(&request, []ribOutKey{identity}, entries, outcomes); err != nil {
		t.Fatal(err)
	}
	if len(outcomes[0].Sent) != 1 {
		t.Fatalf("excluded alias withdrew %d paths, want only original owner's path zero", len(outcomes[0].Sent))
	}
	if got := outcomes[0].Sent[0]; len(got) < 4 || got[0]|got[1]|got[2]|got[3] != 0 {
		t.Fatalf("lost destination's explicit ADD-PATH zero: %x", got)
	}
}
