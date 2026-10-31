// Design: docs/architecture/bgp/structural-forwarding.md -- released receipts of an in-place import invalidation.
// RFC naming: untagged -- tests machinery RFC 8212 default reject relies on; rows RFC8212-3-1/3-2 are {gap}, which refuse a proof tag until the spec closes them (AC-11).
// Related: rfc8212_abort_restore_test.go -- the abort restore these records serve.
// Related: policy_replay_identity_test.go -- the ADD-PATH recipient rail.
package reactor

import (
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// TestRFC8212ReleasedReceiptAuthority proves the reactor, not the request's
// epoch, decides a sent-owner re-send. Method: real ingress and update-route
// rails (rfc8212SentOwnerFixture), the coordinator's own window steps, and the
// recipient's wire read after a settling marker.
func TestRFC8212ReleasedReceiptAuthority(t *testing.T) {
	t.Run("live-epoch-naming-superseded-entry-refused", func(t *testing.T) {
		// The entry stays owned under E0 while the source moves to E1; a
		// request that names its revision under the live E1 is refused.
		f, first, batch := rfc8212SentOwnerFixture(t)
		live := f.source.advanceSourcePolicyEpoch()
		batch.SourcePolicyEpoch = live
		rfc8212SentOwnerAnnounce(t, f, batch)
		rfc8212EpochMarker(t, f, "direct", 90)
		f.assertHistory(t, []byte{1}, 0)
		_, epoch, _ := rfc8212AbortEntry(t, f)
		if epoch != first.SourcePolicyEpoch {
			t.Fatalf("refused re-send re-owned the entry under epoch %d, want its admission %d", epoch, first.SourcePolicyEpoch)
		}
	})
	t.Run("finalized-record-refused-and-gone", func(t *testing.T) {
		f, _, restore := rfc8212SentOwnerFixture(t)
		inv := &sourceInvalidation{r: f.r, peer: f.source, txID: "finalized"}
		inv.window = f.source.openSourcePolicyWindow()
		removal := restore
		removal.SourcePolicyEpoch = 0
		rfc8212AbortWithdraw(t, f, removal)
		rfc8212EpochMarker(t, f, "direct", 90)
		f.assertHistory(t, []byte{1}, 1)
		if held := rfc8212ReleasedCount(f.destination); held != 1 {
			t.Fatalf("the invalidation's removal kept %d released receipts, want 1", held)
		}
		// finalize's release.
		inv.release()
		if held := rfc8212ReleasedCount(f.destination); held != 0 {
			t.Fatalf("finalize left %d released receipts", held)
		}
		restore.SourcePolicyEpoch = f.source.sourcePolicyEpoch.Load()
		rfc8212SentOwnerAnnounce(t, f, restore)
		rfc8212EpochMarker(t, f, "direct", 91)
		f.assertHistory(t, []byte{1}, 1)
	})
}

// TestRFC8212ReleasedReceiptDropped proves a released receipt does not outlive
// the moments no abort restore may name it: the source's revocation and stop.
// Method: the invalidation's removal on the update-route rail, then the call
// site, then the destination's table read under its mu.
func TestRFC8212ReleasedReceiptDropped(t *testing.T) {
	for _, tc := range []struct {
		name string
		end  func(*ownershipRailFixture)
	}{
		{"revocation", func(f *ownershipRailFixture) { f.source.unbindProcesses(f.r, []string{"bgp-rib"}) }},
		{"stop", func(f *ownershipRailFixture) { f.source.stopWithCease(0) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, _, restore := rfc8212SentOwnerFixture(t)
			f.source.mu.Lock()
			f.source.reactor = f.r
			f.source.mu.Unlock()
			f.source.openSourcePolicyWindow()
			restore.SourcePolicyEpoch = 0
			rfc8212AbortWithdraw(t, f, restore)
			rfc8212EpochMarker(t, f, "direct", 90)
			if held := rfc8212ReleasedCount(f.destination); held != 1 {
				t.Fatalf("the invalidation's removal kept %d released receipts, want 1", held)
			}
			tc.end(f)
			if held := rfc8212ReleasedCount(f.destination); held != 0 {
				t.Fatalf("%s left %d released receipts", tc.name, held)
			}
		})
	}
}

// TestRFC8212ReleasedSlotAddPath proves a released ADD-PATH receipt keeps its
// slot. Method: the recipient rail with ADD-PATH send; the source's path is
// removed by the invalidation, another source's announcement reaches the
// final writer carrying the same prefix and outgoing identifier, and the
// abort restore follows. Both paths reach the wire under distinct
// identifiers, and the finalize-free release frees every reservation.
// RFC 7911 Section 2.
func TestRFC8212ReleasedSlotAddPath(t *testing.T) {
	f := recipientIdentityRail(t, nil)
	first := recipientIdentitySend(t, f, f.source, 7, false, 11)
	if len(first) != 1 || first[0].withdraw {
		t.Fatalf("source advertisement = %+v", first)
	}
	slot := first[0].path
	revision := rfc8212AddPathRevision(t, f.destination, f.source)
	e1 := f.source.openSourcePolicyWindow()
	removal := ownershipRailBody(family.IPv4Unicast, true, pathsLimitNLRI(slot, syncOrderPrefix), 0)
	rfc8212AbortRestoreWrite(t, f.destination, removal, revision, 0)
	if held := rfc8212ReleasedCount(f.destination); held != 1 {
		t.Fatalf("the invalidation's removal kept %d released receipts, want 1", held)
	}
	// Another source's announcement of the same prefix and identifier.
	s := f.destination.currentSession()
	other := fwdItem{peer: f.destination, session: s, authority: adjOutForwarded, receivedPeer: f.other,
		receivedGeneration: f.other.forwardGeneration.Load(), sourcePolicyEpoch: f.other.sourcePolicyEpoch.Load()}
	rfc8212ReleasedWrite(t, &other, ownershipRailBody(family.IPv4Unicast, false, pathsLimitNLRI(slot, syncOrderPrefix), 13))
	e2 := f.source.abortSourcePolicyWindow(e1)
	restore := ownershipRailBody(family.IPv4Unicast, false, pathsLimitNLRI(slot, syncOrderPrefix), 11)
	rfc8212AbortRestoreWrite(t, f.destination, restore, revision, e2)
	events := recipientIdentityDrain(t, f)
	if len(events) != 4 {
		t.Fatalf("recipient history = %+v, want announce, withdraw, the other path, the restore", events)
	}
	if !events[1].withdraw || events[1].path != slot {
		t.Fatalf("the invalidation's removal = %+v, want a withdrawal of %d", events[1], slot)
	}
	if events[2].withdraw || events[2].origin != 13 || events[2].path == slot {
		t.Fatalf("the other source's path = %+v, want an announcement under an identifier other than %d", events[2], slot)
	}
	if events[3].withdraw || events[3].origin != 11 || events[3].path != slot {
		t.Fatalf("the abort restore = %+v, want the source's path back under %d", events[3], slot)
	}
	table := &f.destination.adjOut
	table.mu.Lock()
	routes, released := len(table.routes), table.releasedCount()
	table.mu.Unlock()
	if routes != 2 || released != 0 {
		t.Fatalf("destination holds %d routes and %d released receipts, want both paths and none", routes, released)
	}
}

// rfc8212ReleasedCount counts the destination's released receipts, every
// record of every key.
func rfc8212ReleasedCount(destination *Peer) int {
	destination.adjOut.mu.Lock()
	defer destination.adjOut.mu.Unlock()
	return destination.adjOut.releasedCount()
}

// rfc8212AddPathRevision reads the sent receipt of source's only path.
func rfc8212AddPathRevision(t *testing.T, destination, source *Peer) uint64 {
	t.Helper()
	destination.adjOut.mu.Lock()
	defer destination.adjOut.mu.Unlock()
	for _, entry := range destination.adjOut.routes {
		if entry.owner.source == source && entry.revision != 0 {
			return entry.revision
		}
	}
	t.Fatal("the source's advertisement installed no exact receipt")
	return 0
}

// rfc8212ReleasedWrite writes body at the final writer under item.
func rfc8212ReleasedWrite(t *testing.T, item *fwdItem, body []byte) {
	t.Helper()
	s := item.session
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.sentForward = item
	defer func() { s.sentForward = nil }()
	if err := s.writeRawUpdateBody(body); err != nil {
		t.Fatal(err)
	}
	if err := s.flushWrites(); err != nil {
		t.Fatal(err)
	}
}
