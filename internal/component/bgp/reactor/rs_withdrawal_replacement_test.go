// Design: docs/guide/route-reflection.md -- route-server replacement on an owner withdrawal.
// Related: rs_withdrawal_ownership_test.go -- the recipient TCP projection these tests read.
// Related: rfc7705_live_behavior_test.go -- lowLiveRouter, real sessions over net.Pipe.
// RFC 4271 Section 9.2 -- see rfc/short/rfc4271.md.
package reactor

import (
	"bytes"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
)

// replacementRail names which route-server rail carries the owner's withdrawal.
type replacementRail uint8

const (
	replacementRailUnspecified replacementRail = iota
	// replacementRailCached is bgp-rs processForward -> ForwardCached.
	replacementRailCached
	// replacementRailFastPath is the reactor's reactorForwardRS.
	replacementRailFastPath
)

// TestRouteServerWithdrawalKeepsTheReplacementRoute is the Evidence-shape probe
// of spec-bgp-rs-replacement-on-withdrawal, recreated: sources A (10.0.0.1) and
// B (10.0.0.2) announce 10.9.0.0/24 in the order B then A, so A owns the prefix
// at client C (10.0.0.3, no ADD-PATH); A then withdraws. RFC 4271 Section 9.2
// owes C B's route as the replacement, never a bare withdrawal. The order A, B,
// A-withdraw is already green at HEAD (the final-writer ownership fence) and is
// carried by TestRSWithdrawalKeepsCurrentDestinationOwner.
//
// VALIDATES: an owner withdrawal at a non-ADD-PATH client is answered with the
// surviving source's route on every route-server rail.
// PREVENTS: a client losing a prefix that another source still offers.
func TestRouteServerWithdrawalKeepsTheReplacementRoute(t *testing.T) {
	run := replacementScenario(t, replacementRailCached, "10.0.0.1", "10.0.0.2", "10.0.0.3")
	run.expectReplacement(t)
}

// TestRSOwnerWithdrawalRecoversOnCachedRail proves AC-1 on bgp-rs's cached rail:
// the withdrawal must not traverse the reactor fast path, and C's wire must end
// with exactly one new announcement carrying B's received attributes unchanged.
func TestRSOwnerWithdrawalRecoversOnCachedRail(t *testing.T) {
	run := replacementScenario(t, replacementRailCached, "192.0.2.1", "192.0.2.2", "192.0.2.3")
	run.expectReplacement(t)
}

// TestRSOwnerWithdrawalRecoversOnFastPath proves AC-1 on reactorForwardRS. The
// withdrawal must be handled by the fast path, and C must receive B's route
// once: a second copy from the cached rail would show as a fourth announcement.
func TestRSOwnerWithdrawalRecoversOnFastPath(t *testing.T) {
	run := replacementScenario(t, replacementRailFastPath, "192.0.2.1", "192.0.2.2", "192.0.2.3")
	run.expectReplacement(t)
}

// replacementRun holds one scenario after A's withdrawal has passed C's writer.
type replacementRun struct {
	client       *lowLivePeer
	prefix       netip.Prefix
	attrsA       []byte
	attrsB       []byte
	rail         replacementRail
	observedRail replacementRail
}

// replacementScenario runs B announce, A announce, A withdraw over real
// sessions. A's marker after its withdrawal is the causal fence: once C holds
// the marker, the withdrawal has passed C's final writer on whichever rail.
func replacementScenario(t *testing.T, rail replacementRail, addressA, addressB, addressC string) replacementRun {
	t.Helper()
	settings := []*PeerSettings{
		lowLiveSettings(addressA, 65000, 65002),
		lowLiveSettings(addressB, 65000, 65003),
		lowLiveSettings(addressC, 65000, 65004),
	}
	for _, setting := range settings {
		setting.RSClient = true
		setting.RSFastPath = rail == replacementRailFastPath
	}
	peers := lowLiveRouter(t, settings...)
	a, b, c := peers[0], peers[1], peers[2]
	r := a.peer.reactor
	run := replacementRun{client: c, prefix: netip.MustParsePrefix("10.9.0.0/24"),
		attrsA: firstASAttrs(4, 65002), attrsB: firstASAttrs(4, 65003), rail: rail} // RFC 6793 Section 3.

	r.mu.RLock()
	prior := r.messageReceiver
	r.mu.RUnlock()
	observer := &replacementRailObserver{MessageReceiver: prior, source: a.peer.Settings().Address}
	r.setMessageReceiver(observer)
	t.Cleanup(func() { r.setMessageReceiver(prior) })

	ownershipLiveSend(t, b, nil, run.attrsB, run.prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, run.prefix, run.attrsB)
	ownershipLiveSend(t, a, nil, run.attrsA, run.prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, run.prefix, run.attrsA)

	marker := netip.MustParsePrefix("10.9.255.0/24")
	ownershipLiveSend(t, a, []netip.Prefix{run.prefix}, nil) // RFC 4271 Section 4.3.
	ownershipLiveSend(t, a, nil, run.attrsA, marker)         // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, marker, run.attrsA)
	run.observedRail = observer.rail()
	ownershipLiveEstablished(t, peers)
	return run
}

// expectReplacement waits for C to converge on B's route, then asserts the
// whole history: B, A, then B again, and no withdrawal of the prefix.
func (run replacementRun) expectReplacement(t *testing.T) {
	t.Helper()
	if run.observedRail != run.rail {
		t.Fatalf("A's withdrawal took rail %d, want %d", run.observedRail, run.rail)
	}
	deadline := time.Now().Add(5 * time.Second)
	view := ownershipLiveRead(t, run.client)
	for !bytes.Equal(view.routes[run.prefix], run.attrsB) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
		view = ownershipLiveRead(t, run.client)
	}
	// RFC 4271 Section 9.2: "All newly installed routes and all newly
	// unfeasible routes for which there is no replacement route SHALL be
	// advertised to its peers by means of an UPDATE message."
	ownershipLiveRoute(t, view, run.prefix, run.attrsB)
	ownershipLiveWithdrawals(t, view, run.prefix, 0)
	ownershipLiveAnnouncements(t, view, run.prefix, run.attrsB, run.attrsA, run.attrsB)
}

// replacementRailObserver records which rail carried the source's withdrawal
// of 10.9.0.0/24. It wraps the reactor's receiver and changes nothing; the
// reactor's delivery worker calls it, so the rail is published atomically.
type replacementRailObserver struct {
	MessageReceiver
	source   netip.Addr
	fastPath atomic.Bool
	seen     atomic.Bool
}

func (observer *replacementRailObserver) OnMessageBatchReceived(peer *plugin.PeerInfo, messages []bgptypes.RawMessage) []int {
	if peer.Address == observer.source {
		for i := range messages {
			// RFC 4271 Section 4.3: Withdrawn Routes Length 4, 10.9.0.0/24,
			// Total Path Attribute Length 0.
			if bytes.Equal(messages[i].RawBytes, []byte{0, 4, 24, 10, 9, 0, 0, 0}) {
				observer.fastPath.Store(messages[i].ReactorForwarded)
				observer.seen.Store(true)
			}
		}
	}
	return observer.MessageReceiver.OnMessageBatchReceived(peer, messages)
}

func (observer *replacementRailObserver) rail() replacementRail {
	if !observer.seen.Load() {
		return replacementRailUnspecified
	}
	if observer.fastPath.Load() {
		return replacementRailFastPath
	}
	return replacementRailCached
}
