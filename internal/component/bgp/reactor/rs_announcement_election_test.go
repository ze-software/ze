// Design: docs/architecture/bgp/structural-forwarding.md -- announcement election at the final writer
// Related: session_ownership.go -- adjOutWrite.oweAnnouncement
// Related: rs_withdrawal_replacement_test.go -- the live route-server fixtures
package reactor

import (
	"bytes"
	"cmp"
	"context"
	"encoding/binary"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// announcementLive is one live route-server run: sources A, B and E, and
// client C with no ADD-PATH, over real sessions, with the real bgp-rs and
// bgp-rib.
type announcementLive struct {
	a, b, e, c *lowLivePeer
	peers      []*lowLivePeer
	markers    int
	client     *Peer // C, which announcementLiveLate leaves down
}

func announcementLiveRouter(t *testing.T, fastPath bool) *announcementLive {
	t.Helper()
	peers := lowLiveRouter(t, announcementSettings(fastPath, true)...)
	return &announcementLive{a: peers[0], b: peers[1], e: peers[2], c: peers[3], peers: peers, client: peers[3].peer}
}

// announcementLiveLate is announcementLiveRouter with C left down: A, B and
// E are up, and C comes up when the scenario calls connectClient. Without
// rs, bgp-rs is not loaded and bgp-adj-rib-in replays to C itself.
func announcementLiveLate(t *testing.T, fastPath, rs bool, api func(*reactorAPIAdapter) plugin.ReactorLifecycle) *announcementLive {
	t.Helper()
	peers := lowLiveStart(t, lowLiveOptions{api: api, withoutRS: !rs}, announcementSettings(fastPath, rs)...)
	run := &announcementLive{client: peers[3]}
	for i, peer := range peers[:3] {
		run.peers = append(run.peers, lowLiveConnect(t, peer, 0x0a000001+uint32(i)))
	}
	run.a, run.b, run.e = run.peers[0], run.peers[1], run.peers[2]
	return run
}

// connectClient brings C up on a new session, its first or a reconnect.
func (run *announcementLive) connectClient(t *testing.T) {
	t.Helper()
	run.c = lowLiveConnect(t, run.client, 0x0a000004)
}

// The addresses of A, B and C in announcementSettings.
const (
	announcementAddrA = "10.0.0.1"
	announcementAddrB = "10.0.0.2"
	announcementAddrC = "10.0.0.3"
)

// announcementFirstAddr is the address of the source relayed first.
func announcementFirstAddr(betterFirst bool) string {
	if betterFirst {
		return announcementAddrA
	}
	return announcementAddrB
}

// requireRelayedFirst checks the relay carried both sources to C, first one first.
func requireRelayedFirst(t *testing.T, race *replayRaceAdapter, first string) {
	t.Helper()
	order := race.relayOrder()
	require.Contains(t, order, announcementAddrA, "A was relayed to C")
	require.Contains(t, order, announcementAddrB, "B was relayed to C")
	require.Equal(t, first, order[0], "the relay carried %s first", first)
}

// announcementSettings is A, B, E and C, each a route-server client, so the
// egress keeps the received attributes (RFC 7947 Section 2.2) whether or not
// bgp-rs is loaded; the fast path needs bgp-rs.
func announcementSettings(fastPath, rs bool) []*PeerSettings {
	settings := []*PeerSettings{
		lowLiveSettings("10.0.0.1", 65000, 65002),
		lowLiveSettings("10.0.0.2", 65000, 65003),
		lowLiveSettings("10.0.0.4", 65000, 65005),
		lowLiveSettings("10.0.0.3", 65000, 65004),
	}
	for _, setting := range settings {
		setting.RSClient = true
		setting.RSFastPath = fastPath && rs
	}
	return settings
}

// settle returns once every UPDATE source sent before it has passed C's
// writer and C owes nothing: source's marker reaches C, C's owed set
// empties, and a second marker written after any decision reaches C.
func (run *announcementLive) settle(t *testing.T, source *lowLivePeer, attrs []byte) {
	t.Helper()
	for range 2 {
		run.markers++
		marker := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 9, byte(100 + run.markers), 0}), 24)
		ownershipLiveSend(t, source, nil, attrs, marker) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, run.c, marker, attrs)
		lowEventually(t, func() bool { return adjOutOwedCount(run.c.peer) == 0 }, "C owes nothing")
	}
}

// announcementElects runs AC-22 (a) to (d) on one rail. A (AS_PATH 65002)
// is better than B and E (one AS more), so the RIB elects A.
func announcementElects(t *testing.T, fastPath bool) {
	t.Helper()
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	attrsA := firstASAttrs(4, 65002)
	attrsB := firstASAttrs(4, 65003, 65100)
	attrsE := firstASAttrs(4, 65005, 65100)
	worseA := firstASAttrs(4, 65002, 65100, 65101)
	t.Run("a-worse-later-route-writes-nothing", func(t *testing.T) {
		run := announcementLiveRouter(t, fastPath)
		ownershipLiveSend(t, run.a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, run.c, prefix, attrsA)
		ownershipLiveSend(t, run.b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
		run.settle(t, run.b, attrsB)
		run.settle(t, run.a, attrsA)
		view := ownershipLiveRead(t, run.c)
		ownershipLiveRoute(t, view, prefix, attrsA)
		ownershipLiveAnnouncements(t, view, prefix, attrsA)
		ownershipLiveWithdrawals(t, view, prefix, 0)
		ownershipLiveEstablished(t, run.peers)
	})
	t.Run("b-better-later-route-written", func(t *testing.T) {
		run := announcementLiveRouter(t, fastPath)
		ownershipLiveSend(t, run.b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, run.c, prefix, attrsB)
		ownershipLiveSend(t, run.a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
		run.settle(t, run.a, attrsA)
		view := ownershipLiveRead(t, run.c)
		ownershipLiveRoute(t, view, prefix, attrsA)
		ownershipLiveAnnouncements(t, view, prefix, attrsB, attrsA)
		ownershipLiveWithdrawals(t, view, prefix, 0)
		ownershipLiveEstablished(t, run.peers)
	})
	t.Run("c-owner-worsens-other-elected", func(t *testing.T) {
		run := announcementLiveRouter(t, fastPath)
		ownershipLiveSend(t, run.a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, run.c, prefix, attrsA)
		ownershipLiveSend(t, run.b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
		run.settle(t, run.b, attrsB)
		ownershipLiveSend(t, run.a, nil, worseA, prefix) // RFC 4271 Section 4.3.
		run.settle(t, run.a, attrsA)
		view := ownershipLiveRead(t, run.c)
		ownershipLiveRoute(t, view, prefix, attrsB)
		ownershipLiveAnnouncements(t, view, prefix, attrsA, attrsB)
		ownershipLiveWithdrawals(t, view, prefix, 0)
		ownershipLiveEstablished(t, run.peers)
	})
	t.Run("d-third-source-in-and-out-writes-nothing", func(t *testing.T) {
		run := announcementLiveRouter(t, fastPath)
		ownershipLiveSend(t, run.a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, run.c, prefix, attrsA)
		ownershipLiveSend(t, run.b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
		run.settle(t, run.b, attrsB)
		ownershipLiveSend(t, run.e, nil, attrsE, prefix) // RFC 4271 Section 4.3.
		run.settle(t, run.e, attrsE)
		ownershipLiveSend(t, run.e, []netip.Prefix{prefix}, nil) // RFC 4271 Section 4.3.
		run.settle(t, run.e, attrsE)
		view := ownershipLiveRead(t, run.c)
		ownershipLiveRoute(t, view, prefix, attrsA)
		ownershipLiveAnnouncements(t, view, prefix, attrsA)
		ownershipLiveWithdrawals(t, view, prefix, 0)
		// RFC 4271 Section 9.2: the route C holds is bgp-rib's best route.
		best := announcementRIBCandidates(t, run.c.peer, prefix)
		require.NotEmpty(t, best, "bgp-rib's Loc-RIB holds the prefix")
		require.Equal(t, run.a.peer.Settings().Address, best[0].Source, "bgp-rib's best route is A's, the route C holds")
		require.Equal(t, attrsA, best[0].Attributes, "bgp-rib's best route carries A's attributes")
		ownershipLiveEstablished(t, run.peers)
	})
}

// announcementRIBCandidates asks bgp-rib, the recovery owner, for prefix's
// candidates at destination with no source excluded: its Loc-RIB election
// for destination, best first.
func announcementRIBCandidates(t *testing.T, destination *Peer, prefix netip.Prefix) []ribevents.RecoveryRoute {
	t.Helper()
	owner := ribevents.RecoveryProvider()
	require.NotNil(t, owner, "bgp-rib published no recovery owner")
	outcomes, err := owner.Lookup(ribevents.RecoveryRequest{Destination: destination.Settings().Address, Family: family.IPv4Unicast,
		Keys: []ribevents.RecoveryKey{{NLRI: ownershipLiveNLRI([]netip.Prefix{prefix})}}})
	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	return outcomes[0].Survivors
}

// announcementOrder is the two sources in the order they announce: A's route
// (better) first, or B's (worse) first.
func announcementOrder(run *announcementLive, betterFirst bool, attrsA, attrsB []byte) (first, second *lowLivePeer, firstAttrs, secondAttrs []byte) {
	if betterFirst {
		return run.a, run.b, attrsA, attrsB
	}
	return run.b, run.a, attrsB, attrsA
}

// announcementOrderName names a sub-test for its rail and its order.
func announcementOrderName(fastPath, betterFirst bool) string {
	name := "cached-rail"
	if fastPath {
		name = "fast-path"
	}
	if betterFirst {
		return name + "/better-first"
	}
	return name + "/worse-first"
}

// announcementBothStored waits until bgp-rib holds both A's and B's route for
// prefix, so a client coming up afterwards is relayed both.
func announcementBothStored(t *testing.T, run *announcementLive, prefix netip.Prefix) {
	t.Helper()
	lowEventually(t, func() bool { return len(announcementRIBCandidates(t, run.e.peer, prefix)) == 2 }, "bgp-rib holds both routes")
}

// announcementWant is C's announcement history for prefix: the elected route
// A alone when it arrived first, and B's then A's when B arrived first.
func announcementWant(betterFirst bool, attrsA, attrsB []byte) [][]byte {
	if betterFirst {
		return [][]byte{attrsA}
	}
	return [][]byte{attrsB, attrsA}
}

// TestRSAnnouncementFirstArrivalConverges proves A-14. Method: on both rails
// and in both arrival orders, A (better) and B (worse) announce the prefix
// while C is up, and separately before C's session comes up, so C is relayed
// both on the forwarded rail. The first route to reach C's writer for the key
// is written directly; the second meets the entry and is owed, so C always
// ends on A, with B written first only when B arrived first.
//
// VALIDATES: A-14, the first arrival written directly still ends on the
// election (Data Flow 3a).
// PREVENTS: C left on whichever route arrived last.
func TestRSAnnouncementFirstArrivalConverges(t *testing.T) {
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	attrsA := firstASAttrs(4, 65002)
	attrsB := firstASAttrs(4, 65003, 65100)
	for _, fastPath := range []bool{false, true} {
		for _, betterFirst := range []bool{true, false} {
			t.Run(announcementOrderName(fastPath, betterFirst)+"/client-up", func(t *testing.T) {
				run := announcementLiveRouter(t, fastPath)
				first, second, firstAttrs, secondAttrs := announcementOrder(run, betterFirst, attrsA, attrsB)
				ownershipLiveSend(t, first, nil, firstAttrs, prefix) // RFC 4271 Section 4.3.
				ownershipLiveThrough(t, run.c, prefix, firstAttrs)
				ownershipLiveSend(t, second, nil, secondAttrs, prefix) // RFC 4271 Section 4.3.
				run.settle(t, second, secondAttrs)
				run.settle(t, first, firstAttrs)
				view := ownershipLiveRead(t, run.c)
				ownershipLiveRoute(t, view, prefix, attrsA)
				ownershipLiveAnnouncements(t, view, prefix, announcementWant(betterFirst, attrsA, attrsB)...)
				ownershipLiveWithdrawals(t, view, prefix, 0)
				ownershipLiveEstablished(t, run.peers)
			})
			t.Run(announcementOrderName(fastPath, betterFirst)+"/client-up-later", func(t *testing.T) {
				race, api := orderedRelay(announcementFirstAddr(betterFirst))
				run := announcementLiveLate(t, fastPath, true, api)
				first, second, firstAttrs, secondAttrs := announcementOrder(run, betterFirst, attrsA, attrsB)
				ownershipLiveSend(t, first, nil, firstAttrs, prefix)   // RFC 4271 Section 4.3.
				ownershipLiveSend(t, second, nil, secondAttrs, prefix) // RFC 4271 Section 4.3.
				announcementBothStored(t, run, prefix)
				run.connectClient(t)
				run.settle(t, run.a, attrsA)
				requireRelayedFirst(t, race, announcementFirstAddr(betterFirst))
				view := ownershipLiveRead(t, run.c)
				ownershipLiveRoute(t, view, prefix, attrsA)
				ownershipLiveAnnouncements(t, view, prefix, announcementWant(betterFirst, attrsA, attrsB)...)
				ownershipLiveWithdrawals(t, view, prefix, 0)
				ownershipLiveEstablished(t, append(run.peers, run.c))
			})
		}
	}
}

// announcementFrames returns the indexes of C's frames that are IPv4 unicast
// End-of-RIB markers, and of those that announce prefix with attrs.
// RFC 4724 Section 2: the IPv4 unicast End-of-RIB is an UPDATE with no
// withdrawn routes, no path attributes and no NLRI, a 23-octet message.
func announcementFrames(t *testing.T, client *lowLivePeer, prefix netip.Prefix, attrs []byte) (eor, announced []int) {
	t.Helper()
	want := ownershipLiveNLRI([]netip.Prefix{prefix})
	client.mu.Lock()
	defer client.mu.Unlock()
	for i, frame := range client.frames {
		if frame[18] != byte(msgtype.TypeUPDATE) {
			continue
		}
		if len(frame) == message.HeaderLen+4 {
			eor = append(eor, i)
			continue
		}
		update, err := message.UnpackUpdate(frame[message.HeaderLen:]) // RFC 4271 Section 4.3.
		require.NoError(t, err)
		if bytes.Contains(update.NLRI, want) && bytes.Equal(update.PathAttributes, attrs) {
			announced = append(announced, i)
		}
	}
	return eor, announced
}

// TestRSPeerUpReplayElectsBest proves AC-24's first-session half. Method: A
// (better) and B (worse) announce the prefix before C's session comes up, in
// both orders, so the relay replays them to C in that order on the forwarded
// rail: through bgp-rs replayForPeer, and separately through bgp-adj-rib-in's
// own relayRoutes with bgp-rs not loaded. C ends on A; A relayed first leaves
// B's relay writing nothing, and B relayed first is written then replaced by
// A. C receives one IPv4 unicast End-of-RIB. On the bgp-rs relay, bgp-rib's
// lookup for C is held until C has its End-of-RIB, so the election's write of
// A follows the End-of-RIB.
//
// VALIDATES: AC-24, peer-up relays of a second candidate are elected, and
// End-of-RIB is sent once, before a recovery write still owed.
// PREVENTS: the relay order deciding the route a client holds.
func TestRSPeerUpReplayElectsBest(t *testing.T) {
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	attrsA := firstASAttrs(4, 65002)
	attrsB := firstASAttrs(4, 65003, 65100)
	for _, rs := range []bool{true, false} {
		for _, betterFirst := range []bool{true, false} {
			name := "bgp-rs-replay"
			if !rs {
				name = "adj-rib-in-relay"
			}
			name += "/" + announcementOrderName(false, betterFirst)
			t.Run(name, func(t *testing.T) {
				race, api := orderedRelay(announcementFirstAddr(betterFirst))
				run := announcementLiveLate(t, false, rs, api)
				first, second, firstAttrs, secondAttrs := announcementOrder(run, betterFirst, attrsA, attrsB)
				ownershipLiveSend(t, first, nil, firstAttrs, prefix)   // RFC 4271 Section 4.3.
				ownershipLiveSend(t, second, nil, secondAttrs, prefix) // RFC 4271 Section 4.3.
				announcementBothStored(t, run, prefix)
				release := make(chan struct{})
				var held atomic.Bool
				clientAddr := run.client.Settings().Address
				if rs {
					publishLookupTap(t, func(request *ribevents.RecoveryRequest, outcomes []ribevents.RecoveryOutcome, err error) ([]ribevents.RecoveryOutcome, error) {
						if request.Destination == clientAddr {
							held.Store(true)
							select {
							case <-release:
							case <-time.After(5 * time.Second):
							}
						}
						return outcomes, err
					})
				}
				run.connectClient(t)
				lowEventually(t, func() bool {
					eor, _ := announcementFrames(t, run.c, prefix, attrsA)
					return len(eor) != 0
				}, "C receives End-of-RIB")
				close(release)
				lowEventually(t, func() bool {
					_, announced := announcementFrames(t, run.c, prefix, attrsA)
					return len(announced) != 0 && adjOutOwedCount(run.c.peer) == 0
				}, "C holds A and owes nothing")
				if rs {
					run.settle(t, run.b, attrsB)
				}
				requireRelayedFirst(t, race, announcementFirstAddr(betterFirst))
				view := ownershipLiveRead(t, run.c)
				ownershipLiveRoute(t, view, prefix, attrsA)
				ownershipLiveAnnouncements(t, view, prefix, announcementWant(betterFirst, attrsA, attrsB)...)
				ownershipLiveWithdrawals(t, view, prefix, 0)
				eor, announced := announcementFrames(t, run.c, prefix, attrsA)
				require.Len(t, eor, 1, "End-of-RIB is sent once for IPv4 unicast")
				if rs && !betterFirst {
					require.True(t, held.Load(), "B's relay owed the key to a recovery round")
					require.Less(t, eor[0], announced[0], "End-of-RIB precedes the recovery write of A")
				}
				ownershipLiveEstablished(t, append(run.peers, run.c))
			})
		}
	}
}

// replayRaceAdapter stands between the plugins and the reactor so a test
// decides which of C's two reconnect replays reaches the reactor first:
// bgp-rib's ribOut replay (an initial-replay AnnounceNLRIBatch) or the
// forwarded-rail relay (RelayStoredRoute to C). The armed producer waits
// until ready answers true, or 5 seconds; safe for concurrent use.
type replayRaceAdapter struct {
	*reactorAPIAdapter
	client      netip.Addr
	mu          sync.Mutex
	armed       bool
	holdInitial bool        // hold the ribOut replay; else hold the relay
	ready       func() bool // releases the held producer
	held        atomic.Int32
	timeouts    atomic.Int32
	// relayFirst, when set, is the source whose stored routes a relay page to
	// the client carries first: the relay's producers walk per-source maps,
	// so either order is one they emit. relayed records, in order, the source
	// of every route relayed to the client.
	relayFirst string
	relayed    []string
}

func (a *replayRaceAdapter) RelayStoredRoute(destination netip.Addr, routes []rpc.StoredRoute, sender plugin.Sender) error {
	if destination != a.client {
		return a.reactorAPIAdapter.RelayStoredRoute(destination, routes, sender)
	}
	a.hold(false)
	a.mu.Lock()
	if a.relayFirst != "" {
		routes = slices.Clone(routes)
		slices.SortStableFunc(routes, func(x, y rpc.StoredRoute) int {
			return cmp.Compare(boolRank(x.SourcePeer != a.relayFirst), boolRank(y.SourcePeer != a.relayFirst))
		})
	}
	for i := range routes {
		a.relayed = append(a.relayed, routes[i].SourcePeer)
	}
	a.mu.Unlock()
	return a.reactorAPIAdapter.RelayStoredRoute(destination, routes, sender)
}

// boolRank orders false before true.
func boolRank(b bool) int {
	if b {
		return 1
	}
	return 0
}

// relayOrder returns the sources relayed to the client, in order.
func (a *replayRaceAdapter) relayOrder() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.relayed)
}

// orderedRelay is a replayRaceAdapter seam that only orders the relay: first
// relays first, nothing is held.
func orderedRelay(first string) (*replayRaceAdapter, func(*reactorAPIAdapter) plugin.ReactorLifecycle) {
	race := &replayRaceAdapter{client: netip.MustParseAddr(announcementAddrC), relayFirst: first}
	return race, func(adapter *reactorAPIAdapter) plugin.ReactorLifecycle {
		race.reactorAPIAdapter = adapter
		return race
	}
}

func (a *replayRaceAdapter) AnnounceNLRIBatch(ctx context.Context, sel *selector.Selector, batch bgptypes.NLRIBatch, sender plugin.Sender) error {
	if batch.InitialReplay != 0 {
		a.hold(true)
	}
	return a.reactorAPIAdapter.AnnounceNLRIBatch(ctx, sel, batch, sender)
}

func (a *replayRaceAdapter) hold(initial bool) {
	a.mu.Lock()
	armed := a.armed && a.holdInitial == initial
	ready := a.ready
	a.mu.Unlock()
	if !armed {
		return
	}
	a.held.Add(1)
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for !ready() {
		select {
		case <-tick.C:
		case <-deadline.C:
			a.timeouts.Add(1)
			return
		}
	}
}

// adjOutSessionHolds reports whether destination's current session holds
// prefix from source.
func adjOutSessionHolds(t *testing.T, destination *Peer, prefix netip.Prefix, source *Peer) bool {
	t.Helper()
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	var key adjOutKey
	if err := fwdPathKeyFor(&key.path, family.IPv4Unicast, 0, ownershipLiveNLRI([]netip.Prefix{prefix}), false, scratch[:]); err != nil {
		t.Errorf("key for %s: %v", prefix, err)
		return false
	}
	session := destination.currentSession()
	table := &destination.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()
	if table.session != session {
		return false
	}
	entry := table.routes[key]
	return entry != nil && entry.owner.source == source
}

// TestRSReconnectRibOutReplayElects proves AC-24's reconnect half through the
// real producers. Method: A (better) then B (worse) announce the prefix, so C
// holds A; C's session ends and C reconnects, so bgp-rib replays ribOut[C]
// (A, initial replay with its received message ID) while bgp-rs relays A and
// B on the forwarded rail. A seam between the plugins and the reactor holds
// the relay until the ribOut replay has installed A on C's new session, and
// orders the relay A then B. A's relay repeats A once (the forwarded rail
// admits its own uncontested owner), and B's relay is owed and elects the
// revision C already holds, which writes nothing. C never receives B, nor a
// withdrawal.
//
// The other order, the relay written before the ribOut replay, is not one
// the reactor admits: Peer.forwardOrderHold holds every forward to C, relay
// included, while C's initial sync (which carries the ribOut replay) is in
// progress, so a seam holding the ribOut replay until the relay is written
// stalls C's initial sync instead.
//
// VALIDATES: AC-24 reconnect, the relay of a second candidate after the
// ribOut replay, elected against the replayed entry.
// PREVENTS: the reconnect leaving C on B, or re-sending A per election.
func TestRSReconnectRibOutReplayElects(t *testing.T) {
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	attrsA := firstASAttrs(4, 65002)
	attrsB := firstASAttrs(4, 65003, 65100)
	race, api := orderedRelay(announcementAddrA)
	run := announcementLiveLate(t, false, true, api)
	run.connectClient(t)
	ownershipLiveSend(t, run.a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, run.c, prefix, attrsA)
	ownershipLiveSend(t, run.b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
	run.settle(t, run.b, attrsB)

	lowLiveDepart(t, run.c)
	race.mu.Lock()
	race.relayed = nil
	race.armed, race.holdInitial = true, false
	race.ready = func() bool { return adjOutSessionHolds(t, run.client, prefix, run.a.peer) }
	race.mu.Unlock()
	run.connectClient(t)
	run.settle(t, run.b, attrsB)
	require.NotZero(t, race.held.Load(), "the seam held the relay")
	require.Zero(t, race.timeouts.Load(), "the ribOut replay's write released the relay")
	requireRelayedFirst(t, race, announcementAddrA)
	view := ownershipLiveRead(t, run.c)
	ownershipLiveRoute(t, view, prefix, attrsA)
	ownershipLiveAnnouncements(t, view, prefix, attrsA, attrsA)
	ownershipLiveWithdrawals(t, view, prefix, 0)
	ownershipLiveEstablished(t, append(run.peers, run.c))
}

// TestRSAnnouncementElectsOnCachedRail proves AC-22 (a) to (d) on bgp-rs's
// cached rail. Method: real sessions over the live router fixture, C's wire
// parsed into its announcement and withdrawal history per prefix. A worse
// later route writes nothing, a better one is written, an owner that
// worsens is replaced by the elected route, and a third source that comes
// and goes writes nothing.
//
// VALIDATES: Data Flow 3a end to end on the cached rail (owner, Q-3).
// PREVENTS: forward-all, where C's wire carries the newest arrival.
func TestRSAnnouncementElectsOnCachedRail(t *testing.T) {
	announcementElects(t, false)
}

// TestRSAnnouncementElectsOnFastPath proves AC-22 (a) to (d) on
// reactorForwardRS. C's history counts every announcement, so a second copy
// from the cached rail after the fast path handled an owed UPDATE shows as an
// extra entry.
//
// VALIDATES: Data Flow 3a end to end on the fast path (owner, Q-3).
// PREVENTS: forward-all on the fast path, and a duplicate cached-rail write.
func TestRSAnnouncementElectsOnFastPath(t *testing.T) {
	announcementElects(t, true)
}

// addPathClientPaths reads C's received UPDATEs as ADD-PATH IPv4 unicast and
// returns, in arrival order, the path identifier and attributes of each
// announcement of prefix.
func addPathClientPaths(t *testing.T, client *lowLivePeer, prefix netip.Prefix) (ids []uint32, attrs [][]byte) {
	t.Helper()
	want := ownershipLiveNLRI([]netip.Prefix{prefix})
	client.mu.Lock()
	defer client.mu.Unlock()
	for _, frame := range client.frames {
		if frame[18] != byte(msgtype.TypeUPDATE) {
			continue
		}
		update, err := message.UnpackUpdate(frame[message.HeaderLen:]) // RFC 4271 Section 4.3.
		require.NoError(t, err)
		// RFC 7911 Section 3: each NLRI is a four-octet Path Identifier,
		// then the RFC 4271 Section 4.3 prefix tuple.
		for field := update.NLRI; len(field) >= 5; {
			size := 5 + (int(field[4])+7)/8
			require.LessOrEqual(t, size, len(field), "an ADD-PATH NLRI overruns its field")
			if string(field[4:size]) == string(want) {
				ids = append(ids, binary.BigEndian.Uint32(field))
				attrs = append(attrs, update.PathAttributes)
			}
			field = field[size:]
		}
	}
	return ids, attrs
}

// TestRSAnnouncementAddPathClientGetsEveryPath proves AC-23. Method: client C
// negotiated ADD-PATH send for IPv4 unicast with Ze; A and B announce the
// prefix in both orders, B's route worse. C receives both paths, each with
// its own identifier, in arrival order, and C owes nothing, so no election
// runs.
//
// VALIDATES: oweAnnouncement leaves an ADD-PATH destination to forwarding
// (RFC 7947 Section 2.3.2.2).
// PREVENTS: an ADD-PATH client given only the elected path.
func TestRSAnnouncementAddPathClientGetsEveryPath(t *testing.T) {
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	attrsA := firstASAttrs(4, 65002)
	attrsB := firstASAttrs(4, 65003, 65100)
	for _, betterFirst := range []bool{true, false} {
		name := "worse-route-second"
		if !betterFirst {
			name = "worse-route-first"
		}
		t.Run(name, func(t *testing.T) {
			settings := []*PeerSettings{
				lowLiveSettings("192.0.2.1", 65000, 65002),
				lowLiveSettings("192.0.2.2", 65000, 65003),
				lowLiveSettings("192.0.2.3", 65000, 65004),
			}
			for _, setting := range settings {
				setting.RSClient = true
			}
			// RFC 7911 Section 4: Ze is able to send multiple paths to C.
			settings[2].Capabilities = append(settings[2].Capabilities, &capability.AddPath{
				Families: []capability.AddPathFamily{{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast, Mode: capability.AddPathSend}},
			})
			peers := addPathLiveRouter(t, settings...)
			a, b, c := peers[0], peers[1], peers[2]
			order := []struct {
				peer  *lowLivePeer
				attrs []byte
			}{{a, attrsA}, {b, attrsB}}
			if !betterFirst {
				order[0], order[1] = order[1], order[0]
			}
			for i, step := range order {
				ownershipLiveSend(t, step.peer, nil, step.attrs, prefix) // RFC 4271 Section 4.3.
				lowEventually(t, func() bool {
					ids, _ := addPathClientPaths(t, c, prefix)
					return len(ids) == i+1
				}, "C receives the path")
			}
			require.Zero(t, adjOutOwedCount(c.peer), "an ADD-PATH client owes nothing")
			ids, attrs := addPathClientPaths(t, c, prefix)
			require.Len(t, ids, 2)
			require.NotEqual(t, ids[0], ids[1], "each path has its own identifier")
			require.Equal(t, [][]byte{order[0].attrs, order[1].attrs}, attrs, "the paths arrive in arrival order")
			ownershipLiveEstablished(t, peers)
		})
	}
}

// lowLiveOptions varies lowLiveStart's plugin set and its reactor seam.
type lowLiveOptions struct {
	// api wraps the adapter the plugins reach the reactor through; nil keeps
	// the adapter itself.
	api func(*reactorAPIAdapter) plugin.ReactorLifecycle
	// withoutRS leaves bgp-rs out, so bgp-adj-rib-in drives peer-up replay
	// itself (relayRoutes) and nothing forwards a live UPDATE.
	withoutRS bool
}

// lowLiveStart starts lowLiveRouter's reactor and plugins and returns its
// peers, each Active and none connected: the caller brings each one up with
// lowLiveConnect, when the scenario wants it up.
func lowLiveStart(t *testing.T, options lowLiveOptions, settings ...*PeerSettings) []*Peer {
	t.Helper()
	r := New(&Config{ListenAddr: "127.0.0.1:0"})
	for _, s := range settings {
		if err := EnsureProcessBinding(s, "bgp-rib", "update state refresh", "update"); err != nil {
			t.Fatal(err)
		}
		if !options.withoutRS {
			if err := EnsureProcessBinding(s, "bgp-rs", "update-received state open-received refresh", "update"); err != nil {
				t.Fatal(err)
			}
		}
		if err := EnsureProcessBinding(s, "bgp-adj-rib-in", "update-received state", "update"); err != nil {
			t.Fatal(err)
		}
		if err := r.AddPeer(s); err != nil {
			t.Fatal(err)
		}
	}
	adapter := &reactorAPIAdapter{r: r}
	var lifecycle plugin.ReactorLifecycle = adapter
	if options.api != nil {
		lifecycle = options.api(adapter)
	}
	names := []string{"bgp-rib", "bgp-adj-rib-in", "bgp-rs"}
	if options.withoutRS {
		names = names[:2]
	}
	srv := newBorrowedPluginServerOver(t, lifecycle, names...)
	r.SetPluginServer(srv)
	if err := r.StartWithContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopAndWait(t, r) })
	startBorrowedPeers(t, r, srv)
	peers := make([]*Peer, 0, len(settings))
	for _, s := range settings {
		peers = append(peers, r.peers[s.PeerKey()])
	}
	return peers
}
