// Design: docs/architecture/bgp/interface-event-reactions.md -- watched next-hop changes.
// Related: rfc2545_neighbor_late_discovery_test.go -- live IPv6 peers and retained-route checks.
// Related: rfc4271_selected_boundaries_test.go -- shared recipient TCP capture fence.
// Related: plan/release/fixes/spec-rfc2545-readvertise-on-condition-flip.md -- AC-1, AC-4 and B1.
// RFC naming: untagged -- callback-triggered transition probes have unverified baselines and earn no conformance credit.
package reactor

import (
	"bytes"
	"context"
	"encoding/json"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// transitionNeighborLive retains the address-table handle so a test can signal
// the actual address watch. beforeStart may wrap the forward handler before any
// worker exists; it MUST NOT replace the handler after startup.
func transitionNeighborLive(t *testing.T, fast bool, table *lateNeighborTable, beforeStart func(*Reactor)) (*fakeAddressTable, *lowLivePeer, *lowLivePeer) {
	t.Helper()
	addresses := &fakeAddressTable{prefixes: []netip.Prefix{netip.MustParsePrefix("2001:db8:1::1/64")}}
	addresses.install(t)
	table.install(t)
	loc := locrib.Default()
	require.NotNil(t, loc)
	connected := netip.MustParsePrefix("2001:db8:1::/64")
	protocol := redistevents.RegisterProtocol("rfc2545-neighbor-transition-test")
	t.Cleanup(func() { loc.Remove(family.IPv6Unicast, connected, protocol, 0) })
	settings := []*PeerSettings{
		lowLiveSettings("2001:db8:1::3", 65000, 65001),
		lowLiveSettings("2001:db8:1::2", 65000, 65002),
	}
	for _, setting := range settings {
		setting.LocalAddress = netip.MustParseAddr("2001:db8:1::1")
		setting.RSClient, setting.RSFastPath = true, fast
		setting.Capabilities = append(setting.Capabilities, &capability.Multiprotocol{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast})
	}
	options := lowLiveOptions{api: func(adapter *reactorAPIAdapter) plugin.ReactorLifecycle {
		adapter.r.SetListenerFactory(lateNeighborListenerFactory{})
		if beforeStart != nil {
			beforeStart(adapter.r)
		}
		return adapter
	}}
	peers := lowLiveStart(t, options, acceptingPolicies(t, settings...)...)
	require.Same(t, loc, locrib.Default())
	loc.Insert(family.IPv6Unicast, connected, locrib.Path{Source: protocol})
	require.True(t, igpcost.Resolve(loc, netip.MustParseAddr(thirdPartyGlobal)).Resolved,
		"the selecting RIB must resolve the next hop before ingress")
	source := lateNeighborConnect(t, peers[0], 0x0a000003)
	destination := lateNeighborConnect(t, peers[1], 0x0a000002)
	r := source.peer.reactor
	lowEventually(t, func() bool { return r.neighborWatchState().status == neighborWatchRunning }, "neighbor watch running")
	lowEventually(t, func() bool {
		addresses.mu.Lock()
		defer addresses.mu.Unlock()
		return len(addresses.watches) != 0
	}, "address watch running")
	// Establishment reads host interfaces. Publish the modeled initial topology
	// once; subsequent transitions come only from the registered watch workers.
	table.latest().changes <- struct{}{}
	lowEventually(t, func() bool {
		scope := destination.peer.llScope.Load()
		if scope == nil {
			return false
		}
		return len(scope.connected) == 1 && scope.connected[0] == connected &&
			thirdPartyAnswer(destination.peer) == netip.MustParseAddr(thirdPartyLinkLocal)
	}, "initial watched third-party identity and common subnet")
	return addresses, source, destination
}

// transitionNeighborInitial proves useful output and both real stores before a
// callback changes the topology. A dropped initial route cannot pass a probe.
func transitionNeighborInitial(t *testing.T, source, destination *lowLivePeer) []byte {
	t.Helper()
	source.send(t, message.PackTo(&message.Update{PathAttributes: mixedAttrs(mixedReach(1,
		netip.MustParseAddr(thirdPartyGlobal).AsSlice(), mustHex(t, "4020010db857090000")))}, nil))
	want := append(netip.MustParseAddr(thirdPartyGlobal).AsSlice(), netip.MustParseAddr(thirdPartyLinkLocal).AsSlice()...)
	lowEventually(t, func() bool { return len(lateNeighborFields(t, destination)) == 1 }, "initial paired next hop on recipient socket")
	require.Equal(t, [][]byte{want}, lateNeighborFields(t, destination))
	lateNeighborStored(t, source.peer)
	return want
}

// TestRFC2545KnownNeighborReplacementReadvertisesRetainedRoute starts with a
// usable advertised pair, then changes that router's known Link-Local through
// the real neighbor watch. The retained route must reach the socket again
// with the replacement identity, without another announcement from the source.
// VALIDATES: spec-rfc2545-readvertise-on-condition-flip AC-4's moved-identity trigger.
func TestRFC2545KnownNeighborReplacementReadvertisesRetainedRoute(t *testing.T) {
	for _, fast := range []bool{false, true} {
		t.Run(map[bool]string{false: "general", true: "native-rs"}[fast], func(t *testing.T) {
			table := &lateNeighborTable{fakeNeighborTable: fakeNeighborTable{entries: lateNeighborEntries(thirdPartyLinkLocal)}}
			source, destination := lateNeighborLive(t, fast, table)
			initial := transitionNeighborInitial(t, source, destination)
			table.replace(lateNeighborEntries("fe80::88"))
			table.latest().changes <- struct{}{}
			lowEventually(t, func() bool { return thirdPartyAnswer(destination.peer) == netip.MustParseAddr("fe80::88") }, "neighbor callback replaces an already-known identity")
			require.Equal(t, []netip.Prefix{netip.MustParsePrefix("2001:db8:1::/64")}, destination.peer.llScope.Load().connected)
			want := append(netip.MustParseAddr(thirdPartyGlobal).AsSlice(), netip.MustParseAddr("fe80::88").AsSlice()...)
			lowEventually(t, func() bool { return len(lateNeighborFields(t, destination)) >= 2 }, "known neighbor replacement re-advertises the retained route")
			require.Equal(t, [][]byte{initial, want}, lateNeighborFields(t, destination))
		})
	}
}

// TestRFC2545SharedSubnetLossReadvertisesGlobalOnly changes only the local
// address prefix length through the real address watch. The established peer
// and retained, resolved path survive; the new advertisement must omit the
// Link-Local after the joint-subnet condition becomes false.
// VALIDATES: spec-rfc2545-readvertise-on-condition-flip AC-1; the owner supplies the trigger.
func TestRFC2545SharedSubnetLossReadvertisesGlobalOnly(t *testing.T) {
	for _, fast := range []bool{false, true} {
		t.Run(map[bool]string{false: "general", true: "native-rs"}[fast], func(t *testing.T) {
			table := &lateNeighborTable{fakeNeighborTable: fakeNeighborTable{entries: lateNeighborEntries(thirdPartyLinkLocal)}}
			addresses, source, destination := transitionNeighborLive(t, fast, table, nil)
			initial := transitionNeighborInitial(t, source, destination)
			addresses.set([]netip.Prefix{netip.MustParsePrefix("2001:db8:1::1/128")})
			addresses.signal()
			lowEventually(t, func() bool {
				return !destination.peer.llScope.Load().holdsCommonSubnet(netip.MustParseAddr(thirdPartyGlobal), linkLocalConditionNextHopAndPeer)
			}, "address callback removes the joint-subnet condition")
			require.Equal(t, []netip.Prefix{netip.MustParsePrefix("2001:db8:1::1/128")}, destination.peer.llScope.Load().connected)
			require.Equal(t, netip.MustParseAddr(thirdPartyLinkLocal), thirdPartyAnswer(destination.peer), "the neighbor identity remains known")
			require.True(t, igpcost.Resolve(locrib.Default(), netip.MustParseAddr(thirdPartyGlobal)).Resolved, "this transition changes export scope, not route admission")
			lateNeighborStored(t, source.peer)
			lowEventually(t, func() bool { return len(lateNeighborFields(t, destination)) >= 2 }, "shared-subnet loss re-advertises without the link-local address")
			require.Equal(t, [][]byte{initial, netip.MustParseAddr(thirdPartyGlobal).AsSlice()}, lateNeighborFields(t, destination))
		})
	}
}

// TestRFC2545WatchReplayConcurrentWithdrawalEndsWithdrawn holds a replay that
// the neighbor callback itself scheduled, then receives a real source withdrawal
// while that replay is queued. Both real stores must remove the route before
// release, and the last socket UPDATE must remain its single withdrawal.
// VALIDATES: spec-rfc2545-readvertise-on-condition-flip B1 at the actual watch trigger.
func TestRFC2545WatchReplayConcurrentWithdrawalEndsWithdrawn(t *testing.T) {
	for _, fast := range []bool{false, true} {
		t.Run(map[bool]string{false: "general", true: "native-rs"}[fast], func(t *testing.T) {
			var hold atomic.Bool
			var replayPeer netip.AddrPort
			entered, release := make(chan struct{}), make(chan struct{})
			table := &lateNeighborTable{fakeNeighborTable: fakeNeighborTable{entries: lateNeighborEntries(thirdPartyLinkLocal)}}
			_, source, destination := transitionNeighborLive(t, fast, table, func(r *Reactor) {
				handler := r.fwdPool.handler
				r.fwdPool.handler = func(key fwdKey, items []fwdItem) {
					// replayPeer is written before hold.Store(true), which
					// publishes it to the worker that observes hold.Load().
					if hold.Load() {
						if key.peerAddr == replayPeer {
							if transitionNeighborSubjectReplay(items) {
								if hold.Swap(false) {
									close(entered)
									<-release
								}
							}
						}
					}
					handler(key, items)
				}
			})
			t.Cleanup(func() {
				select {
				case <-release:
				default:
					close(release)
				}
			})
			transitionNeighborInitial(t, source, destination)
			replayPeer = destination.peer.Settings().PeerKey()
			hold.Store(true)
			table.replace(lateNeighborEntries("fe80::88"))
			table.latest().changes <- struct{}{}
			lowEventually(t, func() bool { return thirdPartyAnswer(destination.peer) == netip.MustParseAddr("fe80::88") }, "watch publishes the replacement before replay")
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("neighbor watch did not dispatch its retained-route replay to the destination worker")
			}
			source.send(t, message.PackTo(&message.Update{PathAttributes: mixedAttr(15, mixedUnreachValue(2, 1, mustHex(t, "4020010db857090000")))}, nil))
			transitionNeighborSourceWithdrawn(t, source.peer)
			close(release)
			forwardSocketBarrier(t, source.peer.reactor)
			// Source withdrawal joined both stores above, and the writer
			// barrier joined the released replay. Capture its later KEEPALIVE
			// before taking the final history, so an unread resurrection
			// cannot hide behind an already captured withdrawal.
			ordinaryBoundaryCapture(t, destination)
			lowEventually(t, func() bool {
				withdrawals, _ := transitionNeighborWireState(t, destination)
				return withdrawals != 0
			}, "source withdrawal reaches the recipient socket")
			withdrawals, lastWithdraw := transitionNeighborWireState(t, destination)
			require.Equal(t, 1, withdrawals, "the callback replay must not replace or duplicate the later source withdrawal")
			require.True(t, lastWithdraw, "the subject route's final socket UPDATE must be its withdrawal")
			ownershipLiveEstablished(t, []*lowLivePeer{source, destination})
		})
	}
}

// transitionNeighborSubjectReplay recognizes only an announcement of the one
// retained native IPv6 subject. The callback can enqueue other work, including
// markers or other peers' routes; none of it may satisfy the hold barrier.
func transitionNeighborSubjectReplay(items []fwdItem) bool {
	for _, item := range items {
		for _, body := range item.rawBodies {
			update, err := message.UnpackUpdate(body)
			if err != nil {
				continue
			}
			if transitionNeighborSubjectReach(update.PathAttributes) {
				return true
			}
		}
		for _, update := range item.updates {
			if update == nil {
				continue
			}
			if transitionNeighborSubjectReach(update.PathAttributes) {
				return true
			}
		}
	}
	return false
}

// transitionNeighborSubjectReach checks the MP_REACH family and native NLRI,
// independent of whether the final writer recomputes the queued next hop.
func transitionNeighborSubjectReach(attrs []byte) bool {
	_, _, reach, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI)
	if !found {
		return false
	}
	if len(reach) < 5 {
		return false
	}
	if !bytes.Equal(reach[:3], []byte{0, 2, 1}) {
		return false
	}
	n := int(reach[3])
	if len(reach) < 5+n {
		return false
	}
	return bytes.Equal(reach[5+n:], []byte{0x40, 0x20, 0x01, 0x0d, 0xb8, 0x57, 0x09, 0, 0})
}

func transitionNeighborSourceWithdrawn(t *testing.T, source *Peer) {
	t.Helper()
	lowEventually(t, func() bool {
		owner := ribevents.RecoveryProvider()
		require.NotNil(t, owner)
		answers, err := owner.Lookup(ribevents.RecoveryRequest{Destination: netip.MustParseAddr("2001:db8:1::2"), Family: family.IPv6Unicast,
			Keys: []ribevents.RecoveryKey{{NLRI: mustHex(t, "4020010db857090000")}}})
		require.NoError(t, err)
		require.Len(t, answers, 1)
		return len(answers[0].Survivors) == 0
	}, "the selecting RIB removes the withdrawn native IPv6 route")
	proc := source.reactor.api.ProcessManager().GetProcess("bgp-adj-rib-in")
	require.NotNil(t, proc)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, source.receiveCut().wait(ctx))
	require.NoError(t, proc.DrainEventsApplied(ctx))
	var page ribevents.ReplayPage
	require.NoError(t, json.Unmarshal(llgrWriterCommand(t, proc, ribevents.ReplayRoutesCommand, "2001:db8:1::2", "0"), &page))
	require.False(t, page.More)
	for _, route := range page.Routes {
		if route.SourcePeer == source.Settings().Address.String() {
			require.NotEqual(t, "4020010db857090000", route.NLRIHex, "Adj-RIB-In must remove the source's route before queued replay resumes")
		}
	}
}

func transitionNeighborWireState(t *testing.T, destination *lowLivePeer) (int, bool) {
	t.Helper()
	destination.mu.Lock()
	defer destination.mu.Unlock()
	require.Zero(t, destination.dropped, "the recipient socket history must be complete")
	removed := mixedUnreachValue(2, 1, mustHex(t, "4020010db857090000"))
	withdrawals := 0
	lastWithdraw := false
	for _, frame := range destination.frames {
		require.NotEqual(t, byte(msgtype.TypeNOTIFICATION), frame[18], "a session reset cannot satisfy the withdrawal probe")
		if frame[18] != byte(msgtype.TypeUPDATE) {
			continue
		}
		update, err := message.UnpackUpdate(frame[message.HeaderLen:])
		require.NoError(t, err)
		_, _, value, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
		if found && bytes.Equal(value, removed) {
			withdrawals++
			lastWithdraw = true
			continue
		}
		_, _, reach, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
		if found {
			require.GreaterOrEqual(t, len(reach), 5)
			n := int(reach[3])
			require.GreaterOrEqual(t, len(reach), 5+n)
			if bytes.Equal(reach[5+n:], removed[3:]) {
				lastWithdraw = false
			}
		}
	}
	return withdrawals, lastWithdraw
}
