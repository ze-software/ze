package reactor

import (
	"bytes"
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/network"
)

// The third router N on the shared subnet 2001:db8:1::/64: its global address,
// its Link-Local, and the link-layer address the kernel neighbor table binds
// both of them to on one device.
const (
	thirdPartyGlobal    = "2001:db8:1::9"
	thirdPartyLinkLocal = "fe80::9"
	thirdPartyMAC       = "02:00:00:00:00:09"
	thirdPartyDevice    = "eth0"
)

// thirdPartyNeighborView is a neighbor table read in which N's global and
// Link-Local entries carry state, one MAC on one device.
func thirdPartyNeighborView(globalBinding, linkLocalBinding network.NeighborBinding) neighborView {
	return neighborView{links: network.NewNeighborLinkLocals([]network.Neighbor{
		{Address: netip.MustParseAddr(thirdPartyGlobal), LinkAddress: thirdPartyMAC, Device: thirdPartyDevice, Binding: globalBinding},
		{Address: netip.MustParseAddr(thirdPartyLinkLocal), LinkAddress: thirdPartyMAC, Device: thirdPartyDevice, Binding: linkLocalBinding},
	})}
}

// TestRFC2545ThirdPartyLinkLocalAnnounce drives RFC 2545 Section 3 for a next
// hop that is ANOTHER router on the subnet the speaker shares with the peer,
// through the single-route announce rail (Peer.SendAnnounce -> WriteAnnounceUpdate)
// from the entry point to the socket.
//
// RFC 2545 Section 3: "The link-local address shall be included in the Next Hop
// field if and only if the BGP speaker shares a common subnet with the entity
// identified by the global IPv6 address carried in the Network Address of Next
// Hop field and the peer the route is being advertised to."
//
// RFC requirement: RFC2545-3-3 positive -- the entity named by the global (2001:db8:1::9, another router) and the peer (2001:db8:1::2) share one connected subnet with the speaker, so the field carries that router's own link-local (fe80::9, from the neighbor table) after its global, never the speaker's.
// RFC requirement: RFC2545-3-6 positive -- the include half for another router on the common subnet: its link-local is found by its link-layer address in the neighbor table and written second, with the length octet 32.
// RFC requirement: RFC2545-3-3 negative -- the same router's neighbor entries do not make the field 32 octets when the peer shares no subnet with the speaker.
// RFC requirement: RFC2545-3-6 negative -- a peer off the common subnet receives the third-party global alone even though the neighbor table names its link-local.
//
// MUTATION: return the zero address from linkScope.thirdPartyLinkLocal (the
// positive goes red), or write the speaker's configured link-local after a
// third-party global (the positive goes red). The peer-off-link negative is
// decided before the common-subnet check; TestRFC2545ThirdPartyLinkLocalForward
// covers that check.
func TestRFC2545ThirdPartyLinkLocalAnnounce(t *testing.T) {
	nlriBytes := []byte{0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01, 0x00, 0x00}
	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("2001:db8:1::/64"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr(thirdPartyGlobal)),
	}
	connected := []netip.Prefix{netip.MustParsePrefix("2001:db8:1::/64")}
	view := thirdPartyNeighborView(network.NeighborBindingValid, network.NeighborBindingValid)

	t.Run("common-subnet", func(t *testing.T) {
		peer, conn := newAnnouncePeer(t, "2001:db8:1::2")
		peer.settings.LocalAddress = netip.MustParseAddr("2001:db8:1::1")
		peer.refreshLinkScopeWith(connected, view)

		require.NoError(t, peer.SendAnnounce(route, 65000))

		written := string(conn.written())
		assert.Contains(t, written, string(mpReachIPv6Attr(t, nlriBytes, thirdPartyGlobal, thirdPartyLinkLocal)),
			"RFC 2545 Section 3: the third router's global, then its own link-local, length octet 0x20")
		assert.NotContains(t, written, string(mpReachIPv6Attr(t, nlriBytes, thirdPartyGlobal, "fe80::1")),
			"the speaker's own link-local never follows another router's global")
	})

	t.Run("peer-off-link", func(t *testing.T) {
		peer, conn := newAnnouncePeer(t, "2001:db8:dead:beef::2")
		peer.settings.LocalAddress = netip.MustParseAddr("2001:db8:1::1")
		peer.refreshLinkScopeWith(connected, view)

		require.NoError(t, peer.SendAnnounce(route, 65000))

		written := string(conn.written())
		assert.Contains(t, written, string(mpReachIPv6Attr(t, nlriBytes, thirdPartyGlobal)),
			"RFC 2545 Section 3: the global alone, length octet 0x10")
		assert.NotContains(t, written, string(mpReachIPv6Attr(t, nlriBytes, thirdPartyGlobal, thirdPartyLinkLocal)),
			"no link-local may follow when the peer shares no subnet with the speaker")
	})
}

// TestRFC2545ThirdPartyLinkLocalForward drives the same sentence through the
// forwarding rail: a route received with a 16-octet next hop naming another
// router on the recipient's subnet leaves with that router's Link-Local
// appended (applyEgressNextHopScope -> egressLinkLocal), and a recipient whose
// subnet does not hold that router gets the global alone.
//
// RFC requirement: RFC2545-3-3 positive -- a forwarded single-global next hop naming another router on the subnet the speaker shares with the recipient leaves as that router's global then its own link-local.
// RFC requirement: RFC2545-3-6 positive -- on the forwarding rail the include half for another router is met from the neighbor table rather than left at 16 octets.
// RFC requirement: RFC2545-3-3 negative -- when the router's global is off the recipient's subnet the forwarded field stays the 16-octet global alone despite its neighbor entries.
// RFC requirement: RFC2545-3-6 negative -- the neighbor table's link-local is not appended for a recipient that does not share the router's subnet.
//
// MUTATION: return the zero address from linkScope.thirdPartyLinkLocal (the
// positive goes red), or drop the joint-subnet check for a third party (the
// negative goes red).
func TestRFC2545ThirdPartyLinkLocalForward(t *testing.T) {
	for _, tc := range []struct {
		name         string
		globalOnLink bool
		wantPair     bool
	}{
		{name: "common-subnet", globalOnLink: true, wantPair: true},
		{name: "global-off-recipient-subnet", globalOnLink: false, wantPair: false},
	} {
		for _, rail := range []string{"cached", "rs"} {
			t.Run(tc.name+"/"+rail, func(t *testing.T) {
				f := rfc2545ReceiveFixture(t, false, true, tc.globalOnLink)
				scope := f.destination.llScope.Load()
				require.NotNil(t, scope)
				f.destination.refreshLinkScopeWith(scope.connected,
					thirdPartyNeighborView(network.NeighborBindingValid, network.NeighborBindingValid))
				f.destination.fwdFacts.Store(f.destination.buildForwardFacts())

				global := netip.MustParseAddr(thirdPartyGlobal).AsSlice()
				raw := mustHex(t, "4020010db800070000")
				reach := append([]byte{0, 2, 1, byte(len(global))}, global...)
				reach = append(reach, 0)
				reach = append(reach, raw...)
				body := buildUpdatePayload(mixedAttrs(mixedAttr(14, reach)), nil)

				updates := rfc2545ReceiveForward(t, f, rail, body)
				require.Len(t, updates, 1)

				hop := global
				if tc.wantPair {
					hop = append(bytes.Clone(global), netip.MustParseAddr(thirdPartyLinkLocal).AsSlice()...)
				}
				want := append([]byte{0, 2, 1, byte(len(hop))}, hop...)
				want = append(want, 0)
				want = append(want, raw...)
				_, _, got, found := attribute.AttrFind(updates[0].PathAttributes, attribute.AttrMPReachNLRI)
				if !found || !bytes.Equal(got, want) {
					t.Errorf("recipient MP_REACH=%x present=%v, want %x", got, found, want)
				}
			})
		}
	}
}

// TestRFC2545NeighbourStateClasses proves which neighbor states bind: N's
// global and Link-Local entries in REACHABLE, STALE, DELAY, PROBE or PERMANENT
// (the reader's NeighborBindingValid) yield the Link-Local; INCOMPLETE or
// FAILED (NeighborBindingInvalid) on either entry yield the global alone. The
// state names are classified by the reader (iface.neighborBinding); this test
// pins what the reactor does with each class.
func TestRFC2545NeighbourStateClasses(t *testing.T) {
	global := netip.MustParseAddr(thirdPartyGlobal)
	connected := []netip.Prefix{netip.MustParsePrefix("2001:db8:1::/64")}
	for _, tc := range []struct {
		name             string
		globalBinding    network.NeighborBinding
		linkLocalBinding network.NeighborBinding
		want             netip.Addr
	}{
		{name: "both-valid", globalBinding: network.NeighborBindingValid, linkLocalBinding: network.NeighborBindingValid, want: netip.MustParseAddr(thirdPartyLinkLocal)},
		{name: "global-invalid", globalBinding: network.NeighborBindingInvalid, linkLocalBinding: network.NeighborBindingValid},
		{name: "link-local-invalid", globalBinding: network.NeighborBindingValid, linkLocalBinding: network.NeighborBindingInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scope := newLinkScopeWith(connected, thirdPartyNeighborView(tc.globalBinding, tc.linkLocalBinding), netip.MustParseAddr("2001:db8:1::2"))
			got := scope.linkLocalNextHop(netip.MustParseAddr("fe80::1"), global, nextHopRouterThirdParty, linkLocalConditionNextHopAndPeer)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestRFC2545ThirdPartyLinkLocalMisses proves each way the neighbor table can
// fail to name the router's Link-Local leaves the field at 16 octets, and that
// none of them falls back to the speaker's own Link-Local: no entry for the
// global, no Link-Local with its MAC, two Link-Locals with its MAC, an
// unreadable table (no interface backend, or no reader in this build) and a
// snapshot that never read the table.
func TestRFC2545ThirdPartyLinkLocalMisses(t *testing.T) {
	global := netip.MustParseAddr(thirdPartyGlobal)
	connected := []netip.Prefix{netip.MustParsePrefix("2001:db8:1::/64")}
	ambiguous := neighborView{links: network.NewNeighborLinkLocals([]network.Neighbor{
		{Address: global, LinkAddress: thirdPartyMAC, Device: thirdPartyDevice, Binding: network.NeighborBindingValid},
		{Address: netip.MustParseAddr("fe80::9"), LinkAddress: thirdPartyMAC, Device: thirdPartyDevice, Binding: network.NeighborBindingValid},
		{Address: netip.MustParseAddr("fe80::99"), LinkAddress: thirdPartyMAC, Device: thirdPartyDevice, Binding: network.NeighborBindingValid},
	})}
	noCandidate := neighborView{links: network.NewNeighborLinkLocals([]network.Neighbor{
		{Address: global, LinkAddress: thirdPartyMAC, Device: thirdPartyDevice, Binding: network.NeighborBindingValid},
		{Address: netip.MustParseAddr("fe80::9"), LinkAddress: thirdPartyMAC, Device: "eth1", Binding: network.NeighborBindingValid},
	})}
	for _, tc := range []struct {
		name string
		view neighborView
	}{
		{name: "no-entry", view: neighborView{links: network.NewNeighborLinkLocals(nil)}},
		{name: "no-candidate-on-device", view: noCandidate},
		{name: "ambiguous", view: ambiguous},
		{name: "no-backend", view: neighborView{err: errors.New("no interface backend is loaded")}},
		{name: "no-reader", view: neighborView{err: network.ErrNeighborReaderNotRegistered}},
		{name: "not-read", view: neighborView{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scope := newLinkScopeWith(connected, tc.view, netip.MustParseAddr("2001:db8:1::2"))
			got := scope.linkLocalNextHop(netip.MustParseAddr("fe80::1"), global, nextHopRouterThirdParty, linkLocalConditionNextHopAndPeer)
			assert.False(t, got.IsValid(), "got %v, want the global alone", got)
		})
	}
}

// TestRFC2545ThirdPartyLinkLocalScope proves the neighbor table is consulted
// only where RFC 2545's own sentence names the next-hop entity: the speaker's
// global keeps the configured Link-Local whatever the table says, and the
// VPN-IPv6 condition (RFC 4659 Section 3.2.1.1) never appends a third party's.
func TestRFC2545ThirdPartyLinkLocalScope(t *testing.T) {
	connected := []netip.Prefix{netip.MustParsePrefix("2001:db8:1::/64")}
	view := neighborView{links: network.NewNeighborLinkLocals([]network.Neighbor{
		{Address: netip.MustParseAddr("2001:db8:1::1"), LinkAddress: thirdPartyMAC, Device: thirdPartyDevice, Binding: network.NeighborBindingValid},
		{Address: netip.MustParseAddr(thirdPartyGlobal), LinkAddress: thirdPartyMAC, Device: thirdPartyDevice, Binding: network.NeighborBindingValid},
		{Address: netip.MustParseAddr(thirdPartyLinkLocal), LinkAddress: thirdPartyMAC, Device: thirdPartyDevice, Binding: network.NeighborBindingValid},
	})}
	scope := newLinkScopeWith(connected, view, netip.MustParseAddr("2001:db8:1::2"))
	configured := netip.MustParseAddr("fe80::1")

	own := scope.linkLocalNextHop(configured, netip.MustParseAddr("2001:db8:1::1"), nextHopRouterSpeaker, linkLocalConditionNextHopAndPeer)
	assert.Equal(t, configured, own, "the speaker's global takes the configured link-local")

	vpn := scope.linkLocalNextHop(configured, netip.MustParseAddr(thirdPartyGlobal), nextHopRouterThirdParty, linkLocalConditionPeer)
	assert.False(t, vpn.IsValid(), "VPN-IPv6 keeps RFC 4659's own contract, got %v", vpn)

	assert.Equal(t, linkLocalConditionPeer, linkLocalConditionFor(family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}))
}

// TestRFC2545NeighbourChangeRefreshes proves a neighbor-changed signal re-reads
// the neighbor table and moves a live peer's answer for a third-party next hop,
// and that a signal over an unchanged table keeps the peer's snapshot. The
// event payload is not read: the pairing spans two entries, so the worker
// re-reads the table whole (neighborChangeWorker).
//
// MUTATION: drop the neighbor comparison from the refresh skip in
// refreshPeerLinkScopes (the learned Link-Local never reaches the peer).
func TestRFC2545NeighbourChangeRefreshes(t *testing.T) {
	table := []network.Neighbor{
		{Address: netip.MustParseAddr(thirdPartyGlobal), LinkAddress: thirdPartyMAC, Device: thirdPartyDevice, Binding: network.NeighborBindingValid},
	}
	saved := readNeighborTable
	readNeighborTable = func() ([]network.Neighbor, error) { return table, nil }
	t.Cleanup(func() { readNeighborTable = saved })

	r := newIfaceTestReactor(t)
	s := NewPeerSettings(netip.MustParseAddr("2001:db8:1::2"), 65000, 65001, 0x01010101)
	s.LocalAddress = netip.MustParseAddr("2001:db8:1::1")
	peer := NewPeer(s)
	r.peers[s.PeerKey()] = peer
	// The peer's scope holds exactly the interface table refreshPeerLinkScopes
	// reads, so only the neighbor comparison can make the refresh rebuild.
	addresses, err := network.InterfacePrefixes()
	require.NoError(t, err)
	hostTable := make([]netip.Prefix, len(addresses))
	for i, address := range addresses {
		hostTable[i] = address.Masked()
	}
	peer.refreshLinkScopeWith(hostTable, readNeighborView())
	peer.fwdFacts.Store(peer.buildForwardFacts())

	// The answer is probed against 2001:db8:1::/64, which the host does not
	// hold, with the neighbor half the peer's scope carries.
	connected := []netip.Prefix{netip.MustParsePrefix("2001:db8:1::/64")}
	answer := func() netip.Addr {
		scope := peer.llScope.Load()
		probe := newLinkScopeWith(connected, scope.neighbors, s.Address)
		return probe.linkLocalNextHop(netip.Addr{}, netip.MustParseAddr(thirdPartyGlobal), nextHopRouterThirdParty, linkLocalConditionNextHopAndPeer)
	}
	require.False(t, answer().IsValid(), "before the event the table names no Link-Local")

	// The worker runs on its own goroutine; each signal is driven through it
	// and waited for by the refresh it causes, never by a sleep.
	ctx, cancel := context.WithCancel(context.Background())
	pending := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.neighborChangeWorker(ctx, pending)
	}()
	// Registered after the table restore, so it runs first: the worker has
	// exited before readNeighborTable is put back.
	t.Cleanup(func() {
		cancel()
		<-done
	})

	table = append(table, network.Neighbor{Address: netip.MustParseAddr(thirdPartyLinkLocal), LinkAddress: thirdPartyMAC, Device: thirdPartyDevice, Binding: network.NeighborBindingValid})
	signalNeighborChange(pending)
	require.Eventually(t, func() bool { return answer().IsValid() }, 5*time.Second, time.Millisecond, "the signal re-read the table")
	assert.Equal(t, netip.MustParseAddr(thirdPartyLinkLocal), answer())

	// An unchanged table: drive the refresh directly so the assertion does
	// not race the worker, and prove it keeps the snapshot.
	before := peer.forwardFacts()
	r.refreshPeerLinkScopes()
	assert.Same(t, before, peer.forwardFacts(), "an unchanged table does not rebuild the peer")
}

// TestSignalNeighborChangeNeverBlocks proves the bus-side handler coalesces:
// a burst of events while one signal is pending leaves exactly one pending
// and never blocks the netlink monitor's read loop that delivers them.
func TestSignalNeighborChangeNeverBlocks(t *testing.T) {
	pending := make(chan struct{}, 1)
	for range 100 {
		signalNeighborChange(pending)
	}
	assert.Len(t, pending, 1)
}
