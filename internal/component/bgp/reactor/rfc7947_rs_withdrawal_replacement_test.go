// Design: docs/guide/route-reflection.md -- route-server replacement on an owner withdrawal.
// Related: rs_withdrawal_ownership_test.go -- the recipient TCP projection these tests read.
// Related: rfc7705_live_behavior_test.go -- lowLiveRouter, real sessions over net.Pipe.
// RFC 4271 Section 9.2 -- see rfc/short/rfc4271.md.
package reactor

import (
	"bytes"
	"errors"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
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
// B (10.0.0.2) announce 10.9.0.0/24 in the order B then A; the election
// prefers A, so A owns the prefix at client C (10.0.0.3, no ADD-PATH); A then
// withdraws. RFC 4271 Section 9.2 owes C B's route as the replacement, never a
// bare withdrawal. The order A then B, where B's route is owed and not
// elected, is carried by TestRSWithdrawalKeepsCurrentDestinationOwner.
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
	observer     *replacementRailObserver
}

// replacementScenario runs B announce, A announce, A withdraw over real
// sessions. A's announcement contests B's at C and the election writes it,
// so the test waits for C to hold A's route. A's marker after its withdrawal
// is the causal fence: once C holds the marker, the withdrawal has passed C's
// final writer on whichever rail.
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
	observer := &replacementRailObserver{MessageReceiver: prior, source: a.peer.Settings().Address,
		client: c.peer.Settings().Address}
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
	run.observer = observer
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
	// R-12: only the recovery write's sent event carries recovery-write, so
	// bgp-rib checks that projection against a purge and no forwarded one.
	// B's first route is the key's first arrival and is forwarded. A's route
	// contests it, is owed, and is written by the election (owner, Q-3): RFC
	// 4271 Section 9.1.2.2, step f, "Remove from consideration all routes
	// other than the route that was advertised by the BGP speaker with the
	// lowest BGP Identifier value." A's identifier is the fixture's lowest.
	// B's replacement is the recovery of A's withdrawal. The sent event
	// follows the write, so the third mark may trail C's view.
	marks := run.observer.recoveryMarks()
	for len(marks) < 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
		marks = run.observer.recoveryMarks()
	}
	if !slices.Equal(marks, []bool{false, true, true}) {
		t.Fatalf("recovery-write on C's sent announcements of %s = %v, want [false true true]", run.prefix, marks)
	}
}

// replacementRailObserver records which rail carried the source's withdrawal
// of 10.9.0.0/24. It wraps the reactor's receiver and changes nothing; the
// reactor's delivery worker calls it, so the rail is published atomically.
type replacementRailObserver struct {
	MessageReceiver
	source   netip.Addr
	fastPath atomic.Bool
	seen     atomic.Bool
	// client and marks record, per sent announcement of 10.9.0.0/24 to the
	// client, whether its sent event carried the recovery-write meta key.
	client netip.Addr
	mu     sync.Mutex
	marks  []bool
}

// OnMessageSent records the recovery-write mark of each sent announcement of
// 10.9.0.0/24 to the client, then passes the event on unchanged.
func (observer *replacementRailObserver) OnMessageSent(peer *plugin.PeerInfo, msg bgptypes.RawMessage) {
	// RFC 4271 Section 4.3: Withdrawn Routes Length 0, and the NLRI field
	// closes the body: 10.9.0.0/24 is its last four octets here.
	if peer.Address == observer.client && len(msg.RawBytes) > 2 && msg.RawBytes[0] == 0 && msg.RawBytes[1] == 0 &&
		bytes.HasSuffix(msg.RawBytes, []byte{24, 10, 9, 0}) {
		_, marked := msg.Meta[bgptypes.RecoveryWriteMeta]
		observer.mu.Lock()
		observer.marks = append(observer.marks, marked)
		observer.mu.Unlock()
	}
	observer.MessageReceiver.OnMessageSent(peer, msg)
}

func (observer *replacementRailObserver) recoveryMarks() []bool {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	return slices.Clone(observer.marks)
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

// TestRSOwnerWithdrawalSkipsDestinationOwnPath proves AC-13 on C's wire. C
// itself announces 10.9.0.0/24 with the shortest AS_PATH, so its own path is
// the RIB's best overall; sources B then A announce it too, A with the shorter
// path, so A owns it at C. A withdraws: the RIB excludes C's own candidates
// before electing, so C receives B's route, never a withdrawal and never its
// own route reflected back.
//
// VALIDATES: AC-13, the destination's own path is never its replacement.
// PREVENTS: a client left with no route, or sent its own path, when another
// source still offers the prefix.
func TestRSOwnerWithdrawalSkipsDestinationOwnPath(t *testing.T) {
	settings := []*PeerSettings{
		lowLiveSettings("192.0.2.1", 65000, 65002),
		lowLiveSettings("192.0.2.2", 65000, 65003),
		lowLiveSettings("192.0.2.3", 65000, 65004),
	}
	for _, setting := range settings {
		setting.RSClient = true
	}
	peers := lowLiveRouter(t, settings...)
	a, b, c := peers[0], peers[1], peers[2]
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	// RFC 4271 Section 9.1.2.2, step a: C's path is shortest, then A's, then B's.
	attrsC := firstASAttrs(4, 65004)
	attrsA := firstASAttrs(4, 65002, 65002)
	attrsB := firstASAttrs(4, 65003, 65003, 65003)

	ownershipLiveSend(t, c, nil, attrsC, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, a, prefix, attrsC)
	ownershipLiveSend(t, b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefix, attrsB)
	ownershipLiveSend(t, a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefix, attrsA)

	marker := netip.MustParsePrefix("10.9.255.0/24")
	ownershipLiveSend(t, a, []netip.Prefix{prefix}, nil) // RFC 4271 Section 4.3.
	ownershipLiveSend(t, a, nil, attrsA, marker)         // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, marker, attrsA)

	deadline := time.Now().Add(5 * time.Second)
	view := ownershipLiveRead(t, c)
	for len(view.announcements[prefix]) < 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
		view = ownershipLiveRead(t, c)
	}
	// RFC 4271 Section 9.2: "All newly installed routes and all newly
	// unfeasible routes for which there is no replacement route SHALL be
	// advertised to its peers by means of an UPDATE message."
	ownershipLiveRoute(t, view, prefix, attrsB)
	ownershipLiveWithdrawals(t, view, prefix, 0)
	ownershipLiveAnnouncements(t, view, prefix, attrsB, attrsA, attrsB)
	ownershipLiveEstablished(t, peers)
}

// TestRSOwnerWithdrawalLookupFailureRetiresDestination proves AC-11 for a
// lookup error: A owns 10.9.0.0/24 at C and withdraws it, and C's recovery
// lookup fails. The key cannot be decided, so C's session is retired with
// Cease / Out of Resources, and C is never left holding A's withdrawn route
// on a session that stays up. Sources A and B, whose own lookups succeed,
// stay Established.
//
// VALIDATES: AC-11, a failed lookup retires the destination.
// PREVENTS: a client silently keeping a route its source withdrew.
func TestRSOwnerWithdrawalLookupFailureRetiresDestination(t *testing.T) {
	settings := []*PeerSettings{
		lowLiveSettings("192.0.2.1", 65000, 65002),
		lowLiveSettings("192.0.2.2", 65000, 65003),
		lowLiveSettings("192.0.2.3", 65000, 65004),
	}
	for _, setting := range settings {
		setting.RSClient = true
	}
	peers := lowLiveRouter(t, settings...)
	a, b, c := peers[0], peers[1], peers[2]
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	attrsA := firstASAttrs(4, 65002)
	attrsB := firstASAttrs(4, 65003, 65003)

	ownershipLiveSend(t, b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefix, attrsB)
	ownershipLiveSend(t, a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefix, attrsA)

	destination := c.peer.Settings().Address
	var failed atomic.Int32
	publishLookupTap(t, func(request *ribevents.RecoveryRequest, outcomes []ribevents.RecoveryOutcome, err error) ([]ribevents.RecoveryOutcome, error) {
		if request.Destination != destination {
			return outcomes, err
		}
		failed.Add(1)
		return nil, errors.New("injected recovery lookup failure")
	})

	ownershipLiveSend(t, a, []netip.Prefix{prefix}, nil) // RFC 4271 Section 4.3.

	// RFC 4486 Section 4: Cease (6) with subcode Out of Resources (8).
	lowEventually(t, func() bool { return ceaseSent(c, message.NotifyCeaseOutOfResources) }, "Cease / Out of Resources to C")
	lowEventually(t, func() bool { return c.peer.State() != PeerStateEstablished }, "C left Established")
	if failed.Load() == 0 {
		t.Fatal("C's recovery lookup was never asked")
	}
	for _, source := range []*lowLivePeer{a, b} {
		if got := source.peer.SessionState(); got != fsm.StateEstablished {
			t.Errorf("source %s session = %s, want Established", source.peer.Settings().Address, got)
		}
	}
}

// TestRSDepartureWritesOnlyWhereSourceOwns proves round-2 B-1 for bgp-rs: a
// source's departure decides a key at a client only where that source owns
// the client's route. A also announces Q, which only A offers, so C's
// withdrawal of Q proves C's share of A's departure was decided; P's history
// at C is then read against its state before the departure.
//
//   - "owner-stays": A then B announce P, B owns C's route; A departs and
//     nothing is written to C for P.
//   - "owner-stays-preferred-third": E, A, then B announce P. E's route is
//     the RIB's best, so A's and B's contest it, are owed to the election
//     (owner, Q-3) and are never written: E owns C's route. A departs and
//     nothing is written to C for P either.
//   - "owner-departs": B then A announce P, A owns C's route; A departs and
//     C receives B's route, never a withdrawal.
//
// VALIDATES: round-2 B-1, AC-10 for bgp-rs, a departure owes only its own
// session's keys.
// PREVENTS: a departure re-electing, or withdrawing, a route another source
// still owns at the client.
func TestRSDepartureWritesOnlyWhereSourceOwns(t *testing.T) {
	t.Run("owner-stays", func(t *testing.T) { departureWritesOnlyWhereSourceOwns(t, false, false) })
	t.Run("owner-stays-preferred-third", func(t *testing.T) { departureWritesOnlyWhereSourceOwns(t, false, true) })
	t.Run("owner-departs", func(t *testing.T) { departureWritesOnlyWhereSourceOwns(t, true, false) })
}

func departureWritesOnlyWhereSourceOwns(t *testing.T, sourceOwns, withE bool) {
	settings := []*PeerSettings{
		lowLiveSettings("192.0.2.1", 65000, 65002),
		lowLiveSettings("192.0.2.2", 65000, 65003),
		lowLiveSettings("192.0.2.3", 65000, 65004),
	}
	if withE {
		settings = append(settings, lowLiveSettings("192.0.2.5", 65000, 65006))
	}
	for _, setting := range settings {
		setting.RSClient = true
	}
	peers := rsLiveRouter(t, settings...)
	a, b, c := peers[0], peers[1], peers[2]
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	only := netip.MustParsePrefix("10.9.1.0/24")
	// RFC 4271 Section 9.1.2.2, step a: the shorter AS_PATH is preferred.
	// The owner of C's route has the shorter of A's and B's paths, and E,
	// when present, the shortest of all.
	attrsA := firstASAttrs(4, 65002, 65002, 65002)
	attrsB := firstASAttrs(4, 65003, 65003)
	if sourceOwns {
		attrsA = firstASAttrs(4, 65002, 65002)
		attrsB = firstASAttrs(4, 65003, 65003, 65003)
	}
	attrsE := firstASAttrs(4, 65006)

	first, second := a, b
	firstAttrs, secondAttrs := attrsA, attrsB
	if sourceOwns {
		first, second = b, a
		firstAttrs, secondAttrs = attrsB, attrsA
	}
	if withE {
		ownershipLiveSend(t, peers[3], nil, attrsE, prefix) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, c, prefix, attrsE)
	}
	ownershipLiveSend(t, first, nil, firstAttrs, prefix) // RFC 4271 Section 4.3.
	if withE {
		// E stays elected, so each source's own marker is the fence that its
		// announcement of P passed C's writer.
		ownershipLiveSend(t, first, nil, firstAttrs, netip.MustParsePrefix("10.9.2.0/24")) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, c, netip.MustParsePrefix("10.9.2.0/24"), firstAttrs)
	} else {
		ownershipLiveThrough(t, c, prefix, firstAttrs)
	}
	ownershipLiveSend(t, second, nil, secondAttrs, prefix) // RFC 4271 Section 4.3.
	if withE {
		ownershipLiveSend(t, second, nil, secondAttrs, netip.MustParsePrefix("10.9.3.0/24")) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, c, netip.MustParsePrefix("10.9.3.0/24"), secondAttrs)
	} else {
		ownershipLiveThrough(t, c, prefix, secondAttrs)
	}
	ownershipLiveSend(t, a, nil, attrsA, only) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, only, attrsA)
	lowEventually(t, func() bool { return adjOutOwedCount(c.peer) == 0 }, "C's election of P settled")
	// An empty owed set is reached when the election's write is flushed,
	// before C's reader appends the frame, so C is read through B's fresh
	// fence prefix, which C's forward worker writes behind any write the
	// election made. B stays up through the departure below.
	fence := netip.MustParsePrefix("10.9.4.0/24")
	ownershipLiveSend(t, b, nil, attrsB, fence) // RFC 4271 Section 4.3.
	before := ownershipLiveThrough(t, c, fence, attrsB)
	if withE {
		// RFC 4271 Section 9.1.2.2, step a: E's one-AS path is the
		// shortest, so the election keeps E and C is sent neither A's nor
		// B's route.
		ownershipLiveRoute(t, before, prefix, attrsE)
		ownershipLiveAnnouncements(t, before, prefix, attrsE)
	}

	_ = a.remote.Close() // A's session ends with no NOTIFICATION: a departure.

	view := ownershipLiveDecided(t, c, only)
	if sourceOwns {
		deadline := time.Now().Add(5 * time.Second)
		for !bytes.Equal(view.routes[prefix], attrsB) && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
			view = ownershipLiveRead(t, c)
		}
		// RFC 4271 Section 9.2: "All newly installed routes and all newly
		// unfeasible routes for which there is no replacement route SHALL be
		// advertised to its peers by means of an UPDATE message."
		ownershipLiveRoute(t, view, prefix, attrsB)
		ownershipLiveWithdrawals(t, view, prefix, 0)
		ownershipLiveAnnouncements(t, view, prefix, attrsB, attrsA, attrsB)
	}

	// A later marker from B passes C's writer after every decision for P.
	marker := netip.MustParsePrefix("10.9.255.0/24")
	ownershipLiveSend(t, b, nil, attrsB, marker) // RFC 4271 Section 4.3.
	settled := ownershipLiveThrough(t, c, marker, attrsB)
	if sourceOwns {
		ownershipLiveAnnouncements(t, settled, prefix, attrsB, attrsA, attrsB)
	} else {
		// Nothing for P since before A's departure: B still owns C's route.
		ownershipLiveRoute(t, settled, prefix, before.routes[prefix])
		ownershipLiveAnnouncements(t, settled, prefix, before.announcements[prefix]...)
	}
	ownershipLiveWithdrawals(t, settled, prefix, 0)
	ownershipLiveEstablished(t, []*lowLivePeer{b, c})
}

// policyDenyASNs installs a policy seam that rejects, at destination's export
// chain "test:deny", every UPDATE whose text names one of asns, and accepts
// every other UPDATE. The seam is set before any route is sent; the reactor
// reads it on goroutines that each first read a later socket write.
func policyDenyASNs(r *Reactor, destination netip.Addr, asns ...string) {
	r.policyFilterSeam = func(_, filterName, _, peer string, _ uint32, updateText string) PolicyResponse {
		if filterName != "deny" || peer != destination.String() {
			return PolicyResponse{Action: PolicyAccept}
		}
		for _, asn := range asns {
			if strings.Contains(updateText, asn) {
				return PolicyResponse{Action: PolicyReject}
			}
		}
		return PolicyResponse{Action: PolicyAccept}
	}
}

// TestRSOwnerWithdrawalPolicyDeniedReplacement proves AC-5's no-survivor half.
// A owns 10.9.0.0/24 at client C; B's route, the only survivor, is denied by
// C's export policy, so C never held it. A withdraws: the election's only
// candidate cannot be sent to C, so C receives the withdrawal exactly once,
// never B's denied route and never A's stale one.
//
// VALIDATES: AC-5, an egress-denied survivor is not a replacement.
// PREVENTS: a client sent a route its policy refuses, or kept on a withdrawn
// one.
func TestRSOwnerWithdrawalPolicyDeniedReplacement(t *testing.T) {
	policyDeniedSurvivors(t, false, true)
}

// TestRSOwnerWithdrawalPolicyDeniedSkipsToNextSurvivor proves AC-5 with three
// sources. B, E then A announce 10.9.0.0/24; A's AS_PATH is the shortest, then
// B's, then E's, so A owns C's route and the election orders the survivors B,
// E. C's export policy denies B. A withdraws: B is stepped past and C
// receives E's route with E's received attributes, no withdrawal. With E
// denied too, C receives the withdrawal exactly once.
//
// VALIDATES: AC-5, the egress check steps through the election order.
// PREVENTS: a client withdrawn while a permitted survivor exists.
//
// RFC requirement: RFC7947-x-5 positive -- when the source of the route a route-server client holds withdraws it and that client's export policy denies the next survivor in election order, the client's wire receives the following permitted survivor's received attributes with no withdrawal and never the denied route; with every survivor denied, it receives the withdrawal exactly once.
//
// RFC 7947 Section 2.1: "The route server SHOULD perform one or more BGP
// Decision Processes to select routes for subsequent advertisement to its
// clients, taking into account possible configuration to provide multiple
// Network Layer Reachability Information (NLRI) paths to a particular client
// as described in Section 2.3.2.2 or multiple Loc-RIBs as described in
// Section 2.3.2.1."
//
// MUTATION: make recoveryStagedAnnouncement (relay_recovery.go) answer true
// for a candidate staged only as its synthesized withdrawal: queueRecoveryOutcome
// takes the denied B as C's replacement, and C is withdrawn instead of
// receiving E's route.
func TestRSOwnerWithdrawalPolicyDeniedSkipsToNextSurvivor(t *testing.T) {
	t.Run("next-permitted", func(t *testing.T) { policyDeniedSurvivors(t, true, false) })
	t.Run("all-denied", func(t *testing.T) { policyDeniedSurvivors(t, true, true) })
}

func policyDeniedSurvivors(t *testing.T, withE, denyE bool) {
	settings := []*PeerSettings{
		lowLiveSettings("192.0.2.1", 65000, 65002),
		lowLiveSettings("192.0.2.2", 65000, 65003),
		lowLiveSettings("192.0.2.3", 65000, 65004),
	}
	if withE {
		settings = append(settings, lowLiveSettings("192.0.2.5", 65000, 65006))
	}
	for _, setting := range settings {
		setting.RSClient = true
	}
	settings[2].ExportPolicy = mustPolicy(t, []routepolicy.Ref{{Name: "test:deny"}}, routepolicy.FinalAccept)
	peers := rsLiveRouter(t, settings...)
	a, b, c := peers[0], peers[1], peers[2]
	denied := []string{"65003"}
	if denyE {
		denied = append(denied, "65006")
	}
	policyDenyASNs(a.peer.reactor, c.peer.Settings().Address, denied...)
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	// RFC 4271 Section 9.1.2.2, step a: A, then B, then E.
	attrsA := firstASAttrs(4, 65002)
	attrsB := firstASAttrs(4, 65003, 65003)
	attrsE := firstASAttrs(4, 65006, 65006, 65006)
	marker := netip.MustParsePrefix("10.9.255.0/24")

	// Each source's UPDATE arrives on its own session, so the next source
	// sends only once A, a client with no export policy, holds a marker sent
	// after it: C's forward queue then keeps the order in which the sources'
	// UPDATEs were processed. Without the wait, E's UPDATE could be processed
	// after A's. A holds B's route, the election's best of B and E, so E's
	// own marker is the fence.
	ownershipLiveSend(t, b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, a, prefix, attrsB)
	if withE {
		fence := netip.MustParsePrefix("10.9.250.0/24")
		ownershipLiveSend(t, peers[3], nil, attrsE, prefix, fence) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, a, fence, attrsE)
	}
	ownershipLiveSend(t, a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
	// A denied announcement reaches C as a withdrawal of a route C never
	// held; only the withdrawals after A's route count below.
	before := ownershipLiveThrough(t, c, prefix, attrsA).withdrawals[prefix]
	ownershipLiveSend(t, a, []netip.Prefix{prefix}, nil) // RFC 4271 Section 4.3.
	ownershipLiveSend(t, a, nil, attrsA, marker)         // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, marker, attrsA)

	permitted := withE && !denyE
	deadline := time.Now().Add(5 * time.Second)
	view := ownershipLiveRead(t, c)
	for time.Now().Before(deadline) {
		if permitted && bytes.Equal(view.routes[prefix], attrsE) {
			break
		}
		if _, held := view.routes[prefix]; !permitted && !held {
			break
		}
		time.Sleep(time.Millisecond)
		view = ownershipLiveRead(t, c)
	}
	// Let any late write for the prefix land before the history is judged.
	later := netip.MustParsePrefix("10.9.254.0/24")
	ownershipLiveSend(t, a, nil, attrsA, later) // RFC 4271 Section 4.3.
	view = ownershipLiveThrough(t, c, later, attrsA)
	for _, attrs := range view.announcements[prefix] {
		if bytes.Equal(attrs, attrsB) {
			t.Errorf("C was sent B's route, which its export policy denies")
		}
	}
	// RFC 4271 Section 9.2: "All newly installed routes and all newly
	// unfeasible routes for which there is no replacement route SHALL be
	// advertised to its peers by means of an UPDATE message."
	if permitted {
		ownershipLiveRoute(t, view, prefix, attrsE)
		ownershipLiveWithdrawals(t, view, prefix, before)
	} else {
		ownershipLiveAbsent(t, view, prefix)
		ownershipLiveWithdrawals(t, view, prefix, before+1)
	}
	ownershipLiveEstablished(t, peers)
}

// TestRSOwnerWithdrawalDeniedCandidateReannounces proves the fence over a
// stepped-past candidate. A owns 10.9.0.0/24 at C; C's export policy denies
// B's two-AS route, and E's four-AS route is permitted. A withdraws and C's
// lookup is held after bgp-rib answered B then E. Meanwhile B re-announces a
// three-AS route C permits, which reaches C's writer before the admission:
// the key is owed, so the announcement is not written but merged into the
// record (owner, Q-3), and C still holds A's route. The held answer steps
// past the denied B and decides E; the record carries B's announcement, so
// settle re-owes the key and the next round elects B's newer route, which C
// ends on.
//
// VALIDATES: AC-5 with AC-26 (b): a candidate stepped past for policy that
// rewrites the key since the snapshot makes the decision stale.
// PREVENTS: a client moved off a better permitted route onto a worse one.
func TestRSOwnerWithdrawalDeniedCandidateReannounces(t *testing.T) {
	settings := []*PeerSettings{
		lowLiveSettings("192.0.2.1", 65000, 65002),
		lowLiveSettings("192.0.2.2", 65000, 65003),
		lowLiveSettings("192.0.2.3", 65000, 65004),
		lowLiveSettings("192.0.2.5", 65000, 65006),
	}
	for _, setting := range settings {
		setting.RSClient = true
	}
	settings[2].ExportPolicy = mustPolicy(t, []routepolicy.Ref{{Name: "test:deny"}}, routepolicy.FinalAccept)
	peers := rsLiveRouter(t, settings...)
	a, b, c, e := peers[0], peers[1], peers[2], peers[3]
	policyDenyASNs(a.peer.reactor, c.peer.Settings().Address, "[65003 65003]")
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	// RFC 4271 Section 9.1.2.2, step a: A, then B's two-AS and three-AS
	// routes, then E.
	attrsA := firstASAttrs(4, 65002)
	attrsB := firstASAttrs(4, 65003, 65003)
	attrsBNewer := firstASAttrs(4, 65003, 65003, 65003)
	attrsE := firstASAttrs(4, 65006, 65006, 65006, 65006)

	// B's UPDATE is processed before E's sends, A holding it, so the newest
	// write at C is E's (policyDeniedSurvivors).
	ownershipLiveSend(t, b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, a, prefix, attrsB)
	ownershipLiveSend(t, e, nil, attrsE, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefix, attrsE)
	ownershipLiveSend(t, a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefix, attrsA)

	tap := &keyLookups{destination: c.peer.Settings().Address, key: ownershipLiveNLRI([]netip.Prefix{prefix}),
		held: make(chan struct{}), hold: make(chan struct{})}
	publishLookupTap(t, tap.after)
	t.Cleanup(func() {
		select {
		case <-tap.hold:
		default:
			close(tap.hold)
		}
	})
	ownershipLiveSend(t, a, []netip.Prefix{prefix}, nil) // RFC 4271 Section 4.3.
	select {
	case <-tap.held:
	case <-time.After(5 * time.Second):
		t.Fatal("C's recovery lookup for the owed key never ran")
	}
	fence := netip.MustParsePrefix("10.9.253.0/24")
	ownershipLiveSend(t, b, nil, attrsBNewer, prefix, fence) // RFC 4271 Section 4.3.
	merged := ownershipLiveThrough(t, c, fence, attrsBNewer)
	// The owed key's election is held, so B's newer route is not written.
	ownershipLiveRoute(t, merged, prefix, attrsA)
	close(tap.hold)

	lowEventually(t, func() bool { return adjOutOwedCount(c.peer) == 0 }, "C owes nothing")
	later := netip.MustParsePrefix("10.9.254.0/24")
	ownershipLiveSend(t, a, nil, attrsA, later) // RFC 4271 Section 4.3.
	view := ownershipLiveThrough(t, c, later, attrsA)
	ownershipLiveRoute(t, view, prefix, attrsBNewer)
	if got := tap.count(); got < 2 {
		t.Errorf("the owed key reached bgp-rib in %d lookups, want a second round after B's write", got)
	}
	ownershipLiveEstablished(t, peers)
}

// TestRSOwnerSynthesizedWithdrawalRecovers proves AC-14 on both rails. B,
// then A, announce 10.9.0.0/24, so A owns C's route. A then re-announces the
// prefix in a form C may not be sent, which the egress turns into a
// withdrawal of A's route at C (a synthesized withdrawal):
//
//   - "policy": the new AS_PATH carries 65099, which C's export policy denies;
//   - "next-hop": the new NEXT_HOP is C's own address, which Ze refuses to
//     send to C (egressNextHopIsPeerOwn, after RFC 4271 Section 5.1.3).
//
// A's new path is in the RIB and may be elected first; the egress check
// steps past it, so C receives B's route with B's received attributes and no
// withdrawal of the prefix, never A's new path.
//
// VALIDATES: AC-14, a synthesized owner withdrawal is decided like a received
// one, on the cached rail and the fast path.
// PREVENTS: a client withdrawn because its owner re-announced a route it may
// not receive while a permitted survivor exists.
func TestRSOwnerSynthesizedWithdrawalRecovers(t *testing.T) {
	for _, rail := range []struct {
		name     string
		fastPath bool
	}{{"cached-rail", false}, {"fast-path", true}} {
		t.Run(rail.name+"/policy", func(t *testing.T) { synthesizedWithdrawalRecovers(t, rail.fastPath, false) })
		t.Run(rail.name+"/next-hop", func(t *testing.T) { synthesizedWithdrawalRecovers(t, rail.fastPath, true) })
	}
}

func synthesizedWithdrawalRecovers(t *testing.T, fastPath, nextHop bool) {
	settings := []*PeerSettings{
		lowLiveSettings("192.0.2.1", 65000, 65002),
		lowLiveSettings("192.0.2.2", 65000, 65003),
		lowLiveSettings("192.0.2.3", 65000, 65004),
	}
	for _, setting := range settings {
		setting.RSClient = true
		setting.RSFastPath = fastPath
	}
	if !nextHop {
		settings[2].ExportPolicy = mustPolicy(t, []routepolicy.Ref{{Name: "test:deny"}}, routepolicy.FinalAccept)
	}
	peers := rsLiveRouter(t, settings...)
	a, b, c := peers[0], peers[1], peers[2]
	if !nextHop {
		policyDenyASNs(a.peer.reactor, c.peer.Settings().Address, "65099")
	}
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	attrsA := firstASAttrs(4, 65002)
	// B's AS_PATH is longer than A's refused one, so the election puts A's
	// refused path first and the step-past is what the test exercises.
	attrsB := firstASAttrs(4, 65003, 65003, 65003)
	refused := firstASAttrs(4, 65002, 65099)
	if nextHop {
		// RFC 4271 Section 5.1.3: "A route originated by a BGP speaker SHALL
		// NOT be advertised to a peer using an address of that peer as
		// NEXT_HOP." Ze applies the same refusal to a relayed route
		// (egressNextHopIsPeerOwn, forward_next_hop.go).
		refused = synthesizedNextHopAttrs(c.peer.Settings().Address, 65002)
	}

	ownershipLiveSend(t, b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefix, attrsB)
	ownershipLiveSend(t, a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
	before := ownershipLiveThrough(t, c, prefix, attrsA).withdrawals[prefix]

	ownershipLiveSend(t, a, nil, refused, prefix) // RFC 4271 Section 4.3.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if bytes.Equal(ownershipLiveRead(t, c).routes[prefix], attrsB) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	lowEventually(t, func() bool { return adjOutOwedCount(c.peer) == 0 }, "C owes nothing")
	// Let any late write for the prefix land before the history is judged.
	later := netip.MustParsePrefix("10.9.254.0/24")
	ownershipLiveSend(t, b, nil, attrsB, later) // RFC 4271 Section 4.3.
	view := ownershipLiveThrough(t, c, later, attrsB)
	for _, attrs := range view.announcements[prefix] {
		if bytes.Equal(attrs, refused) {
			t.Errorf("C was sent A's re-announcement %x, which it may not receive", refused)
		}
	}
	// RFC 4271 Section 9.2: "All newly installed routes and all newly
	// unfeasible routes for which there is no replacement route SHALL be
	// advertised to its peers by means of an UPDATE message."
	ownershipLiveRoute(t, view, prefix, attrsB)
	ownershipLiveWithdrawals(t, view, prefix, before)
	ownershipLiveEstablished(t, peers)
}

// synthesizedNextHopAttrs is firstASAttrs with the NEXT_HOP set to nextHop.
func synthesizedNextHopAttrs(nextHop netip.Addr, asns ...uint32) []byte {
	attrs := collapseAttr(0x40, byte(attribute.AttrOrigin), []byte{0})
	attrs = append(attrs, collapseAttr(0x40, byte(attribute.AttrASPath), collapseASPathValue(4, asns...))...)
	address := nextHop.As4()
	return append(attrs, collapseAttr(0x40, byte(attribute.AttrNextHop), address[:])...)
}

// TestRSOwnerWithdrawalLaterAnnouncementElected is AC-6 (owner, Q-3). Method:
// the real bgp-rs and bgp-rib over live sessions. A (the best) and B announce
// P, so C holds A's route. C's recovery lookups are parked after bgp-rib
// answers; A withdraws P, so C owes it and the parked round has elected B. F
// then announces P: its write at C merges into the owed record as an ANNOUNCE
// mover. Once the lookup is released, the admission writes the round's
// choice and re-owes P, and the next round elects with F. C ends on the RIB's
// best of B and F, run with F better and with F worse than B, never on A's
// route and never with a withdrawal of P.
//
// VALIDATES: AC-6.
// PREVENTS: the newest arrival, or a round's stale choice, ending on C.
func TestRSOwnerWithdrawalLaterAnnouncementElected(t *testing.T) {
	for _, fBetter := range []bool{true, false} {
		name := "f-worse-than-b"
		if fBetter {
			name = "f-better-than-b"
		}
		t.Run(name, func(t *testing.T) {
			settings := []*PeerSettings{
				lowLiveSettings("192.0.2.10", 65000, 65001),
				lowLiveSettings("192.0.2.11", 65000, 65002),
				lowLiveSettings("192.0.2.12", 65000, 65003),
				lowLiveSettings("192.0.2.3", 65000, 65100),
			}
			for _, setting := range settings {
				setting.RSClient = true
			}
			peers := rsLiveRouter(t, settings...)
			a, b, f, c := peers[0], peers[1], peers[2], peers[3]
			gate := &lookupGate{destination: c.peer.Settings().Address}
			publishLookupTap(t, gate.after)
			p := netip.MustParsePrefix("10.6.0.0/24")
			// RFC 4271 Section 9.1.2.2, step a: the shorter AS_PATH wins.
			attrsA := firstASAttrs(4, 65001)                                // RFC 6793 Section 3.
			attrsB := firstASAttrs(4, slices.Repeat([]uint32{65002}, 3)...) // RFC 6793 Section 3.
			attrsF := firstASAttrs(4, 65003, 65003)                         // RFC 6793 Section 3.
			want := attrsF
			if !fBetter {
				attrsB = firstASAttrs(4, 65002, 65002)                         // RFC 6793 Section 3.
				attrsF = firstASAttrs(4, slices.Repeat([]uint32{65003}, 3)...) // RFC 6793 Section 3.
				want = attrsB
			}

			ownershipLiveSend(t, a, nil, attrsA, p) // RFC 4271 Section 4.3.
			ownershipLiveThrough(t, c, p, attrsA)
			ownershipLiveSend(t, b, nil, attrsB, p) // RFC 4271 Section 4.3.
			lowEventually(t, func() bool {
				_, owed := adjOutTestRecord(c.peer)
				return !owed && adjOutPrefixOwner(t, c.peer, p) == a.peer
			}, "C's election of B's arrival kept A")
			before := ownershipLiveRead(t, c).withdrawals[p]

			gate.close()
			lookups := gate.count()
			ownershipLiveSend(t, a, []netip.Prefix{p}, nil) // RFC 4271 Section 4.3.
			lowEventually(t, func() bool { return gate.count() > lookups }, "C's round asked the RIB about P")
			ownershipLiveSend(t, f, nil, attrsF, p) // RFC 4271 Section 4.3.
			lowEventually(t, func() bool {
				record, owed := adjOutTestRecord(c.peer)
				return owed && record.announced
			}, "F's announcement merged into C's owed record")
			gate.open()

			lowEventually(t, func() bool {
				_, owed := adjOutTestRecord(c.peer)
				return !owed && adjOutPrefixOwner(t, c.peer, p) != a.peer
			}, "C decided P")
			view := ownershipLiveRead(t, c)
			ownershipLiveRoute(t, view, p, want)
			ownershipLiveWithdrawals(t, view, p, before)
			ownershipLiveEstablished(t, peers)
		})
	}
}
