// Design: docs/architecture/bgp/structural-forwarding.md -- third-party IPv6 next hops.
// Related: reactor_iface.go -- neighbor watch refresh and the missing re-advertisement.
// Related: plan/release/fixes/spec-rfc2545-readvertise-on-condition-flip.md -- AC-4.
// RFC naming: untagged -- late-neighbor re-advertisement defect probes have failing baselines and earn no conformance credit.
package reactor

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// lateNeighborTable extends the existing watched table with a failed-read state.
// The mutex also protects the error so the real watch worker can read it.
type lateNeighborTable struct {
	fakeNeighborTable
	readErr error
}

func (n *lateNeighborTable) install(t *testing.T) {
	t.Helper()
	n.fakeNeighborTable.install(t)
	readNeighborTable = n.read
}

func (n *lateNeighborTable) read() ([]network.Neighbor, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]network.Neighbor(nil), n.entries...), n.readErr
}

func (n *lateNeighborTable) replace(entries []network.Neighbor) {
	n.mu.Lock()
	n.entries, n.readErr = entries, nil
	n.mu.Unlock()
}

func lateNeighborEntries(linkLocal string) []network.Neighbor {
	return []network.Neighbor{
		{Address: netip.MustParseAddr(thirdPartyGlobal), LinkAddress: thirdPartyMAC, Device: thirdPartyDevice, Binding: network.NeighborBindingValid},
		{Address: netip.MustParseAddr(linkLocal), LinkAddress: thirdPartyMAC, Device: thirdPartyDevice, Binding: network.NeighborBindingValid},
	}
}

// lateNeighborListenerFactory keeps the modeled session endpoint immutable
// from construction while binding its otherwise-unused listener to loopback.
// The live protocol sessions use net.Pipe, as before; the reactor still owns
// and closes every real listener this factory returns.
type lateNeighborListenerFactory struct{}

func (lateNeighborListenerFactory) Listen(ctx context.Context, transport, address string) (net.Listener, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if host == "2001:db8:1::1" {
		return (network.RealListenerFactory{}).Listen(ctx, "tcp", "127.0.0.1:0")
	}
	return (network.RealListenerFactory{}).Listen(ctx, transport, address)
}

// TestRFC2545NeighborWatchUsesAddressSnapshot checks the prerequisite independently:
// the registered neighbor callback must refresh against the same address-table
// read seam as the address watch. It cannot silently substitute this test host's
// interfaces for the modeled common subnet.
// VALIDATES: RFC 2545 Section 3 common-subnet input refresh, through the real neighbor worker.
// This is deliberately untagged: no conformance or discrimination claim is made.
func TestRFC2545NeighborWatchUsesAddressSnapshot(t *testing.T) {
	addresses := &fakeAddressTable{prefixes: []netip.Prefix{netip.MustParsePrefix("2001:db8:1::1/64")}}
	addresses.install(t)
	table := &lateNeighborTable{entries: lateNeighborEntries(thirdPartyLinkLocal)}
	table.install(t)
	r := newIfaceTestReactor(t)
	peer := addLivePeer(t, r)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go runNeighborWatchWorker(ctx, r, done)
	t.Cleanup(func() { cancel(); <-done })
	require.Eventually(t, func() bool {
		return peer.llScope.Load().neighbors.watch.status == neighborWatchRunning
	}, 5*time.Second, time.Millisecond)
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("2001:db8:1::/64")}, peer.llScope.Load().connected,
		"neighbor refresh must consume the address-table seam before wire tests can model the common subnet")
}

// lateNeighborConnect negotiates native IPv6 on lowLiveStart's real passive
// session. lowLiveConnect's remote OPEN offers IPv4 only, so it cannot carry
// this test's native IPv6 MP_REACH.
func lateNeighborConnect(t *testing.T, peer *Peer, identifier uint32) *lowLivePeer {
	t.Helper()
	lowEventually(t, func() bool { return peer.SessionState() == fsm.StateActive }, "IPv6 peer Active")
	server, remote := net.Pipe()
	live := &lowLivePeer{peer: peer, remote: remote, stopped: make(chan struct{})}
	live.startCapture(t)
	t.Cleanup(func() {
		require.NoError(t, remote.Close())
		require.NoError(t, server.Close())
		<-live.stopped
	})
	require.NoError(t, peer.acceptConnection(server))
	peerAS := peer.Settings().PeerAS
	open := &message.Open{Version: 4, MyAS: uint16(peerAS), HoldTime: 90, BGPIdentifier: identifier,
		OptionalParams: []byte{2, 6, 65, 4, 0, 0, byte(peerAS >> 8), byte(peerAS), 2, 6, 1, 4, 0, 2, 0, 1}}
	live.send(t, message.PackTo(open, nil))
	lowEventually(t, func() bool { return peer.SessionState() == fsm.StateOpenConfirm }, "IPv6 OPEN accepted")
	live.send(t, message.PackTo(message.NewKeepalive(), nil))
	lowEventually(t, func() bool { return peer.State() == PeerStateEstablished && !peer.pendingSync() }, "IPv6 initial replay complete")
	return live
}

func lateNeighborLive(t *testing.T, fast bool, table *lateNeighborTable) (*lowLivePeer, *lowLivePeer) {
	t.Helper()
	addresses := &fakeAddressTable{prefixes: []netip.Prefix{netip.MustParsePrefix("2001:db8:1::1/64")}}
	addresses.install(t)
	table.install(t)
	// The interface view and the routing table describe the same connected
	// link. Recovery correctly excludes an unresolvable next hop even though
	// route-server forwarding can already preserve and send that next hop.
	loc := locrib.Default()
	require.NotNil(t, loc)
	connected := netip.MustParsePrefix("2001:db8:1::/64")
	protocol := redistevents.RegisterProtocol("rfc2545-late-neighbor-test")
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
		return adapter
	}}
	peers := lowLiveStart(t, options, acceptingPolicies(t, settings...)...)
	// The real in-process bgp-rib Run wires locrib.Default with SetLocRIB;
	// Default is process-wide and startup must leave this instance intact.
	require.Same(t, loc, locrib.Default())
	loc.Insert(family.IPv6Unicast, connected, locrib.Path{Source: protocol})
	require.True(t, igpcost.Resolve(loc, netip.MustParseAddr(thirdPartyGlobal)).Resolved,
		"the actual selecting RIB must resolve the third-party next hop before ingress")
	source := lateNeighborConnect(t, peers[0], 0x0a000003)
	destination := lateNeighborConnect(t, peers[1], 0x0a000002)
	r := source.peer.reactor
	lowEventually(t, func() bool { return r.neighborWatchState().status == neighborWatchRunning }, "neighbor watch running")
	// Session establishment reads the real host interfaces. Install the modeled
	// initial topology explicitly, then let only the actual watch callback
	// update it. The separate prerequisite test checks that callback's input.
	refreshPeerLinkScopesMu.Lock()
	for _, peer := range peers {
		peer.refreshForwardFactsIfLiveFrom([]netip.Prefix{netip.MustParsePrefix("2001:db8:1::/64")}, readNeighborView(r.neighborWatchState()))
	}
	refreshPeerLinkScopesMu.Unlock()
	return source, destination
}

func lateNeighborBody(t *testing.T, withdraw bool) []byte {
	t.Helper()
	raw := mustHex(t, "4020010db857090000")
	if withdraw {
		return makeUpdateBody(nil, mixedAttr(15, mixedUnreachValue(2, 1, raw)), nil)
	}
	return buildUpdatePayload(mixedAttrs(mixedReach(1, netip.MustParseAddr(thirdPartyGlobal).AsSlice(), raw)), nil)
}

// lateNeighborFields reads the complete remote socket history, including the
// next-hop length and the subject NLRI; unrelated UPDATEs cannot satisfy it.
func lateNeighborFields(t *testing.T, peer *lowLivePeer) [][]byte {
	t.Helper()
	peer.mu.Lock()
	defer peer.mu.Unlock()
	var fields [][]byte
	raw := mustHex(t, "4020010db857090000")
	for _, frame := range peer.frames {
		if frame[18] != 2 {
			continue
		}
		update, err := message.UnpackUpdate(frame[message.HeaderLen:])
		require.NoError(t, err)
		_, _, reach, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
		if !found {
			continue
		}
		require.GreaterOrEqual(t, len(reach), 5)
		n := int(reach[3])
		require.GreaterOrEqual(t, len(reach), 5+n)
		if !bytes.Equal(reach[5+n:], raw) {
			continue
		}
		require.Equal(t, []byte{0, 2, 1}, reach[:3])
		require.Zero(t, reach[4+n])
		fields = append(fields, bytes.Clone(reach[4:4+n]))
	}
	return fields
}

func lateNeighborStored(t *testing.T, source *Peer) {
	t.Helper()
	destPeer, ok := source.reactor.lookupPeer("2001:db8:1::2")
	require.True(t, ok)
	lowEventually(t, func() bool {
		owner := ribevents.RecoveryProvider()
		require.NotNil(t, owner)
		answers, err := owner.Lookup(ribevents.RecoveryRequest{Destination: netip.MustParseAddr("2001:db8:1::2"), DestinationOwner: destPeer.sourceOwner, Family: family.IPv6Unicast,
			Keys: []ribevents.RecoveryKey{{NLRI: mustHex(t, "4020010db857090000")}}})
		require.NoError(t, err)
		require.Len(t, answers, 1)
		for _, route := range answers[0].Survivors {
			if route.Source == source.Settings().Address {
				return true
			}
		}
		return false
	}, "the actual RIB retains the source's native IPv6 route")
	// Check the actual replay store as well as the selecting RIB. A selecting
	// RIB survivor alone does not prove the source bytes can be replayed.
	proc := source.reactor.api.ProcessManager().GetProcess("bgp-adj-rib-in")
	require.NotNil(t, proc)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, source.receiveCut().wait(ctx))
	require.NoError(t, proc.DrainEventsApplied(ctx))
	var page ribevents.ReplayPage
	require.NoError(t, json.Unmarshal(llgrWriterCommand(t, proc, ribevents.ReplayRoutesCommand, "2001:db8:1::2", "0"), &page))
	require.False(t, page.More, "one retained route must fit a replay page")
	for _, route := range page.Routes {
		if route.SourcePeer != source.Settings().Address.String() {
			continue
		}
		if route.NLRIHex != "4020010db857090000" {
			continue
		}
		require.Equal(t, family.IPv6Unicast.String(), route.Family)
		require.Equal(t, hex.EncodeToString(netip.MustParseAddr(thirdPartyGlobal).AsSlice()), route.NextHopHex)
		require.False(t, route.Withdraw)
		require.NotZero(t, route.MsgID)
		require.Equal(t, source.sourcePolicyEpoch.Load(), route.SourcePolicyEpoch)
		return
	}
	t.Fatal("the actual Adj-RIB-In replay store did not retain the native IPv6 source path")
}

// TestRFC2545NeighborKnownAtReceiptReachesSocket is the positive control: a
// usable neighbor pair already known when a real peer sends the UPDATE reaches
// the recipient as global plus the third party's own Link-Local on both rails.
// VALIDATES: RFC 2545 Section 3, the conditional 32-octet field on actual sockets.
// This is deliberately untagged until a recorded producing-code break is owed.
func TestRFC2545NeighborKnownAtReceiptReachesSocket(t *testing.T) {
	for _, fast := range []bool{false, true} {
		t.Run(map[bool]string{false: "general", true: "native-rs"}[fast], func(t *testing.T) {
			table := &lateNeighborTable{entries: lateNeighborEntries(thirdPartyLinkLocal)}
			source, destination := lateNeighborLive(t, fast, table)
			source.send(t, message.PackTo(&message.Update{PathAttributes: mixedAttrs(mixedReach(1, netip.MustParseAddr(thirdPartyGlobal).AsSlice(), mustHex(t, "4020010db857090000")))}, nil))
			want := append(netip.MustParseAddr(thirdPartyGlobal).AsSlice(), netip.MustParseAddr(thirdPartyLinkLocal).AsSlice()...)
			lowEventually(t, func() bool { return len(lateNeighborFields(t, destination)) != 0 }, "known neighbor route on recipient socket")
			require.Equal(t, [][]byte{want}, lateNeighborFields(t, destination))
			lateNeighborStored(t, source.peer)
		})
	}
}

// TestRFC2545LateNeighborDiscoveryReadvertisesRetainedRoute sends one UPDATE
// from a real peer, waits for real RIB retention, then changes only the watched
// neighbor table. The event must produce a new advertisement without a new
// source UPDATE or an explicit test call to RelayStoredRoute.
// VALIDATES: spec-rfc2545-readvertise-on-condition-flip AC-4; RFC 2545 Section 3
// supplies the conditional field, while the owner decision supplies the trigger.
// This is deliberately untagged: the missing trigger is an expected behavior red.
func TestRFC2545LateNeighborDiscoveryReadvertisesRetainedRoute(t *testing.T) {
	for _, fast := range []bool{false, true} {
		for _, state := range []string{"unknown", "unresolved", "ambiguous", "unreadable"} {
			t.Run(map[bool]string{false: "general", true: "native-rs"}[fast]+"/"+state, func(t *testing.T) {
				entries := lateNeighborEntries(thirdPartyLinkLocal)
				table := &lateNeighborTable{}
				switch state {
				case "unknown":
					table.entries = entries[:1]
				case "unresolved":
					entries[1].Binding = network.NeighborBindingInvalid
					table.entries = entries
				case "ambiguous":
					entries = append(entries, network.Neighbor{Address: netip.MustParseAddr("fe80::99"), LinkAddress: thirdPartyMAC, Device: thirdPartyDevice, Binding: network.NeighborBindingValid})
					table.entries = entries
				case "unreadable":
					table.readErr = errors.New("synthetic neighbor table read failure")
				}
				source, destination := lateNeighborLive(t, fast, table)
				source.send(t, message.PackTo(&message.Update{PathAttributes: mixedAttrs(mixedReach(1, netip.MustParseAddr(thirdPartyGlobal).AsSlice(), mustHex(t, "4020010db857090000")))}, nil))
				lowEventually(t, func() bool { return len(lateNeighborFields(t, destination)) == 1 }, "initial global-only advertisement")
				require.Equal(t, [][]byte{netip.MustParseAddr(thirdPartyGlobal).AsSlice()}, lateNeighborFields(t, destination))
				lateNeighborStored(t, source.peer)
				table.replace(lateNeighborEntries(thirdPartyLinkLocal))
				table.latest().changes <- struct{}{}
				lowEventually(t, func() bool { return thirdPartyAnswer(destination.peer) == netip.MustParseAddr(thirdPartyLinkLocal) }, "neighbor callback publishes the new third-party identity")
				require.Equal(t, []netip.Prefix{netip.MustParsePrefix("2001:db8:1::/64")}, destination.peer.llScope.Load().connected,
					"prerequisite: callback retained the modeled common subnet")
				want := append(netip.MustParseAddr(thirdPartyGlobal).AsSlice(), netip.MustParseAddr(thirdPartyLinkLocal).AsSlice()...)
				lowEventually(t, func() bool { return len(lateNeighborFields(t, destination)) >= 2 }, "neighbor change re-advertises the retained route without new ingress")
				require.Equal(t, [][]byte{netip.MustParseAddr(thirdPartyGlobal).AsSlice(), want}, lateNeighborFields(t, destination))
			})
		}
	}
}

// lateNeighborReplay is a retained route at the reactor's actual relay boundary.
// The received attributes and next hop remain unchanged when facts are rebuilt.
func lateNeighborReplay(t *testing.T, f *aigpReplayFixture) rpc.StoredRoute {
	t.Helper()
	sections, err := message.UnpackUpdate(lateNeighborBody(t, false))
	require.NoError(t, err)
	return rpc.StoredRoute{SourcePeer: f.source.Settings().Address.String(), SourcePolicyEpoch: f.source.sourcePolicyEpoch.Load(),
		Family: family.IPv6Unicast.String(), AttrHex: hex.EncodeToString(sections.PathAttributes),
		NLRIHex: "4020010db857090000", NextHopHex: hex.EncodeToString(netip.MustParseAddr(thirdPartyGlobal).AsSlice()),
		MsgID: nextMsgID(), NLRIFraming: rpc.NLRIFramingPrefixOnly}
}

// TestRFC2545RefreshedFactsReachStoredReplaySocket separates the replay writer
// from the absent watch trigger: a caller-requested stored replay recomputes the
// third-party pair from fresh facts and does not reuse its old 16-octet output.
// VALIDATES: RFC 2545 Section 3 at RelayStoredRoute's final socket boundary.
// This is deliberately untagged pending baseline and discrimination proof.
func TestRFC2545RefreshedFactsReachStoredReplaySocket(t *testing.T) {
	f := rfc2545ReceiveFixture(t, false, true, true)
	api := &reactorAPIAdapter{r: f.r}
	route := lateNeighborReplay(t, f)
	f.destination.refreshForwardFactsIfLiveFrom([]netip.Prefix{netip.MustParsePrefix("2001:db8:1::/64")}, neighborView{links: network.NewNeighborLinkLocals(nil)})
	require.NoError(t, api.RelayStoredRoute(f.destination.Settings().Address, []rpc.StoredRoute{route}, plugin.OperatorSender()))
	forwardSocketBarrier(t, f.r)
	initial := aigpSocketBodies(t, f.conn)
	require.Len(t, initial, 1)
	first, err := message.UnpackUpdate(initial[0])
	require.NoError(t, err)
	rfc2545AssertNative(t, first, netip.MustParseAddr(thirdPartyGlobal).AsSlice(), mustHex(t, route.NLRIHex), true)
	f.destination.refreshForwardFactsIfLiveFrom([]netip.Prefix{netip.MustParsePrefix("2001:db8:1::/64")}, thirdPartyNeighborView(network.NeighborBindingValid, network.NeighborBindingValid))
	require.NoError(t, api.RelayStoredRoute(f.destination.Settings().Address, []rpc.StoredRoute{route}, plugin.OperatorSender()))
	forwardSocketBarrier(t, f.r)
	bodies := aigpSocketBodies(t, f.conn)
	require.Len(t, bodies, 2)
	last, err := message.UnpackUpdate(bodies[1])
	require.NoError(t, err)
	want := append(netip.MustParseAddr(thirdPartyGlobal).AsSlice(), netip.MustParseAddr(thirdPartyLinkLocal).AsSlice()...)
	rfc2545AssertNative(t, last, want, mustHex(t, route.NLRIHex), true)
}

// TestRFC2545QueuedNeighborReplayEndsWithdrawn holds an already dispatched
// replay before its actual destination writer, then withdraws the source on
// each live rail. Release must leave the route withdrawn, including when the
// native RS rail can otherwise overtake the queued replay. This isolates the
// ordering safeguard the re-advertisement owner owes from its missing trigger.
// VALIDATES: spec-rfc2545-readvertise-on-condition-flip B1; RFC 4271 Section 9
// requires the source's later withdrawal to remain the final routing state.
// This is deliberately untagged until the baseline and producing-code proof run.
func TestRFC2545QueuedNeighborReplayEndsWithdrawn(t *testing.T) {
	for _, rail := range []string{"cached", "rs"} {
		t.Run(rail, func(t *testing.T) {
			f := rfc2545ReceiveFixture(t, false, true, true)
			var hold atomic.Bool
			entered, release := make(chan struct{}), make(chan struct{})
			handler := f.r.fwdPool.handler
			// No worker exists yet: this replaces only the fixture's wrapper,
			// and always invokes the real fwdBatchHandler through that wrapper.
			f.r.fwdPool.handler = func(key fwdKey, items []fwdItem) {
				if hold.Swap(false) {
					close(entered)
					<-release
				}
				handler(key, items)
			}
			t.Cleanup(func() {
				select {
				case <-release:
				default:
					close(release)
				}
			})
			api := &reactorAPIAdapter{r: f.r}
			route := lateNeighborReplay(t, f)
			require.NoError(t, api.RelayStoredRoute(f.destination.Settings().Address, []rpc.StoredRoute{route}, plugin.OperatorSender()))
			forwardSocketBarrier(t, f.r)
			require.Len(t, aigpSocketBodies(t, f.conn), 1, "positive control: this retained route reached the socket")
			f.destination.refreshForwardFactsIfLiveFrom([]netip.Prefix{netip.MustParsePrefix("2001:db8:1::/64")}, thirdPartyNeighborView(network.NeighborBindingValid, network.NeighborBindingValid))
			hold.Store(true)
			require.NoError(t, api.RelayStoredRoute(f.destination.Settings().Address, []rpc.StoredRoute{route}, plugin.OperatorSender()))
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("refreshed stored replay did not enter the actual destination worker")
			}
			rfc2545DispatchReceived(t, f, rail, f.receive(t, lateNeighborBody(t, true)))
			close(release)
			forwardSocketBarrier(t, f.r)
			bodies := aigpSocketBodies(t, f.conn)
			require.GreaterOrEqual(t, len(bodies), 2)
			last, err := message.UnpackUpdate(bodies[len(bodies)-1])
			require.NoError(t, err)
			rfc2545AssertNative(t, last, nil, mustHex(t, route.NLRIHex), false)
			withdrawals := 0
			for _, body := range bodies {
				update, err := message.UnpackUpdate(body)
				require.NoError(t, err)
				_, _, removed, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
				if found && bytes.Equal(removed, mixedUnreachValue(2, 1, mustHex(t, route.NLRIHex))) {
					withdrawals++
				}
			}
			require.Equal(t, 1, withdrawals, "a later replay cannot replace or duplicate the source's withdrawal")
		})
	}
}
