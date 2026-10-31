// Design: docs/architecture/bgp/structural-forwarding.md -- released receipts of an in-place import invalidation.
// RFC naming: untagged -- tests machinery RFC 8212 default reject relies on; rows RFC8212-3-1/3-2 are {gap}, which refuse a proof tag until the spec closes them (AC-11).
// Related: rfc8212_abort_restore_test.go -- the single-source abort restores these extend.
// Related: rfc8212_released_receipt_test.go -- the released-record lifecycle.
package reactor

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// TestRFC8212AbortRestoresBothReleasedSources proves one reload that
// invalidates two sources holding the same key, at a destination without
// ADD-PATH send, keeps both released receipts, so both abort restores reach
// the destination. Method: source A's copy of P is installed; A's
// invalidation removes it and the recovery round elects source B's P; B's
// invalidation then removes B's copy and the round finds no survivor. The
// destination holds no entry and one released record per source. A's abort
// restore is written and re-owns P; B's abort restore then meets A's entry
// and is owed, so the recovery election runs among the two restored paths
// and writes its winner, B. RFC 8212 Section 3 and RFC 4271 Section 9.1.
func TestRFC8212AbortRestoresBothReleasedSources(t *testing.T) {
	rail := newOwedRail(t)
	other := adjOutTestOtherSource(rail.r, rail.source)
	attachRecoveryOwner(t, rail.r, rail.source, other)
	p := syncOrderPrefixWire
	rail.forward(t, false, p)
	installedA, held := adjOutTestEntry(rail.destination)
	if !held || installedA.revision == 0 {
		t.Fatal("the forwarded announcement installed no exact receipt")
	}
	// RFC 8212 Section 3: A's apply withdraws its denied copy, and the round
	// elects B's path.
	e1A := rail.source.openSourcePolicyWindow()
	owedRailExpected(t, rail.destination, ownershipRailBody(family.IPv4Unicast, true, p, 0), installedA.revision)
	rail.request(t)
	rail.release <- owedSurvivorMessage(other.Settings().Address, other.sourcePolicyEpoch.Load(), p, 21)
	lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the round elected B")
	installedB, held := adjOutTestEntry(rail.destination)
	if !held || installedB.owner.source != other || installedB.revision == 0 {
		t.Fatalf("after the round the destination holds %v from %p, want B %p with a receipt", held, installedB.owner.source, other)
	}
	// The same reload's invalidation of B withdraws B's copy, and no
	// survivor remains.
	e1B := other.openSourcePolicyWindow()
	owedRailExpected(t, rail.destination, ownershipRailBody(family.IPv4Unicast, true, p, 0), installedB.revision)
	rail.request(t)
	rail.release <- nil
	lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the round withdrew P")
	if _, held := adjOutTestEntry(rail.destination); held {
		t.Fatal("the round left B's denied entry in place")
	}
	if released := rfc8212ReleasedCount(rail.destination); released != 2 {
		t.Fatalf("the destination keeps %d released receipts of P, want one for A and one for B", released)
	}

	e2A := rail.source.abortSourcePolicyWindow(e1A)
	e2B := other.abortSourcePolicyWindow(e1B)
	before := len(recoveryWrittenUpdates(t, rail.conn.written()))
	rfc8212AbortRestoreWrite(t, rail.destination, ownershipRailBody(family.IPv4Unicast, false, p, 1), installedA.revision, e2A)
	restoredA, held := adjOutTestEntry(rail.destination)
	if !held || restoredA.owner.source != rail.source || restoredA.owner.policyEpoch != e2A {
		t.Fatalf("A's abort restore = held %v source %p epoch %d, want A %p under %d",
			held, restoredA.owner.source, restoredA.owner.policyEpoch, rail.source, e2A)
	}
	if written := len(recoveryWrittenUpdates(t, rail.conn.written())); written != before+1 {
		t.Fatalf("A's abort restore wrote %d UPDATEs, want its announcement", written-before)
	}

	before = len(recoveryWrittenUpdates(t, rail.conn.written()))
	rfc8212AbortRestoreWrite(t, rail.destination, ownershipRailBody(family.IPv4Unicast, false, p, 1), installedB.revision, e2B)
	if written := len(recoveryWrittenUpdates(t, rail.conn.written())); written != before {
		t.Fatalf("B's abort restore wrote %d UPDATEs over A's entry, want none before the election", written-before)
	}
	request := rail.request(t)
	if len(request.Keys) != 1 || !bytes.Equal(request.Keys[0].NLRI, p) {
		t.Fatalf("the election asked for %v, want P", request.Keys)
	}
	rail.release <- owedSurvivorMessage(other.Settings().Address, e2B, p, installedB.revision)
	lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the round elected B")
	written := recoveryWrittenUpdates(t, rail.conn.written())[before:]
	if len(written) != 1 || len(written[0].NLRI) == 0 {
		t.Fatalf("the election wrote %d UPDATEs, want B's announcement", len(written))
	}
	elected, held := adjOutTestEntry(rail.destination)
	if !held || elected.owner.source != other || elected.owner.policyEpoch != e2B {
		t.Fatalf("elected entry = held %v source %p epoch %d, want B %p under %d",
			held, elected.owner.source, elected.owner.policyEpoch, other, e2B)
	}
}

// TestRFC8212ReleasedReceiptDepartureDropped proves a source whose session
// departs while its invalidation is open loses its released receipts at every
// destination, and that the invalidation's abort restore then brings nothing
// back. Method: a live router with two Established peers; the destination
// holds a released receipt of the source, stamped with the source's open
// window; the source's TCP connection closes, and its down (sessionDown) is
// awaited; then the window aborts and the owner's restore of the receipt is
// written at the destination. RFC 8212 Section 3.
func TestRFC8212ReleasedReceiptDepartureDropped(t *testing.T) {
	peers := lowLiveRouter(t,
		lowLiveSettings("10.0.0.1", 65000, 65002),
		lowLiveSettings("10.0.0.2", 65000, 65003))
	source, destination := peers[0].peer, peers[1].peer
	observer := &downOrderObserver{downAdded: make(chan struct{}, 1)}
	source.reactor.addPeerObserver(observer)

	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	var path fwdPathKey
	if err := fwdPathKeyFor(&path, family.IPv4Unicast, 0, syncOrderPrefixWire, false, scratch[:]); err != nil {
		t.Fatal(err)
	}
	key := adjOutKey{path: path}
	const revision = 77
	window := source.openSourcePolicyWindow()
	owner := adjOutPath{source: source, generation: source.forwardGeneration.Load(),
		revocation: source.forwardRevocation.Load(), policyEpoch: window - 1}
	destination.adjOut.mu.Lock()
	if destination.adjOut.released == nil {
		destination.adjOut.released = make(map[adjOutKey][]adjOutReleased)
	}
	destination.adjOut.released[key] = []adjOutReleased{{owner: owner, revision: revision, window: window}}
	destination.adjOut.mu.Unlock()

	_ = peers[0].remote.Close() // The source's TCP connection ends.
	lowEventually(t, func() bool { return len(observer.records()) != 0 }, "the source's down")
	if released := rfc8212ReleasedCount(destination); released != 0 {
		t.Fatalf("the source's departure left %d released receipts at the destination", released)
	}

	restore := source.abortSourcePolicyWindow(window)
	before := rfc8212LiveUpdates(peers[1])
	rfc8212AbortRestoreWrite(t, destination, ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 1), revision, restore)
	destination.adjOut.mu.Lock()
	entry := destination.adjOut.routes[key]
	destination.adjOut.mu.Unlock()
	if entry != nil {
		t.Fatalf("the abort restore re-installed the departed source's receipt: owner %p", entry.owner.source)
	}
	if after := rfc8212LiveUpdates(peers[1]); after != before {
		t.Fatalf("the abort restore of a departed source's receipt wrote %d UPDATEs", after-before)
	}
}

// rfc8212LiveUpdates counts the UPDATE frames the remote end of v has read.
// RFC 4271 Section 4.1: the message type is the octet after the 16-octet
// marker and the 2-octet length.
func rfc8212LiveUpdates(v *lowLivePeer) int {
	v.mu.Lock()
	defer v.mu.Unlock()
	updates := 0
	for _, frame := range v.frames {
		if len(frame) > 18 && frame[18] == 2 {
			updates++
		}
	}
	return updates
}
