package reactor

import (
	"bytes"
	"errors"
	"io"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// TestQueuedDefaultNextHopDrains reaches both drains while they own p.mu.
// Resolving through the direct default chooser from either critical section
// would re-enter p.mu and prevent this positive announcement from completing.
//
// RFC requirement: RFC4271-5.1.3-1 positive -- on both actual queue drains, an
// Auto internal-peer gateway equal to that peer is replaced by the session's
// local NEXT_HOP, with the useful IPv4 prefix still advertised.
// RFC requirement: RFC4271-5.1.3-1 negative -- an Auto internal-peer gateway
// naming another router is retained with its useful IPv4 prefix on both drains.
//
// MUTATION: after writeUpdateGated's final peer-owned-address guard, rewrite
// the internal session's serialized NEXT_HOP to the peer's own address. Target
// session-self output for the positive claim and the other gateway for the
// negative control; each exact wire assertion must reject that advertisement.
func TestQueuedDefaultNextHopDrains(t *testing.T) {
	for _, rail := range []string{"initial-drain", "closing-drain"} {
		t.Run(rail, func(t *testing.T) {
			for _, tc := range []struct {
				name      string
				internal  bool
				candidate string
				mode      uint8
				explicit  string
				want      string
			}{
				{name: "external", candidate: "10.1.0.2", want: "192.0.2.1"},
				{name: "external-ipv6-candidate", candidate: "2001:db8::9", want: "192.0.2.1"},
				{name: "internal", internal: true, candidate: "10.1.0.2", want: "10.1.0.2"},
				{name: "internal-peer-is-gateway", internal: true, candidate: "10.0.0.2", want: "192.0.2.1"},
				{name: "configured-self", internal: true, candidate: "10.1.0.2", mode: NextHopSelf, want: "192.0.2.1"},
				{name: "configured-explicit", candidate: "10.1.0.2", mode: NextHopExplicit, explicit: "192.0.2.9", want: "192.0.2.9"},
				{name: "configured-unchanged", candidate: "10.1.0.2", mode: NextHopUnchanged, want: "10.1.0.2"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					peer, conn := newDefaultNextHopControlPeer(t, rail, "192.0.2.1")
					if tc.internal {
						peer.settings.PeerAS = peer.settings.LocalAS
					}
					peer.settings.NextHopMode = tc.mode
					if tc.explicit != "" {
						peer.settings.NextHopAddress = netip.MustParseAddr(tc.explicit)
					}
					batch := defaultNextHopControlBatch("10.30.0.0/24", bgptypes.NewNextHopDefault(netip.MustParseAddr(tc.candidate)))
					if err := defaultNextHopControlAdapter(peer).AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()); err != nil {
						t.Fatal(err)
					}
					assertDefaultNextHopQueued(t, peer, tc.candidate)
					if len(conn.written()) != 0 {
						t.Fatal("queued route reached the wire before the drain")
					}
					drainDefaultNextHopControl(t, peer, rail)
					bodies := defaultNextHopControlBodies(t, conn.written())
					if len(bodies) != 1 {
						t.Fatalf("announcements = %d, want exactly one useful route", len(bodies))
					}
					assertDefaultNextHopWire(t, bodies[0], netip.MustParseAddr(tc.want), []byte{24, 10, 30, 0})
				})
			}
		})
	}
}

// TestDefaultNextHopUsesResolvedAddressFamily checks that an IPv6 candidate
// replaced by the external session's IPv4 endpoint retains the classic IPv4
// wire form, whether or not Extended Next Hop Encoding was negotiated.
func TestDefaultNextHopUsesResolvedAddressFamily(t *testing.T) {
	for _, rail := range []string{"direct", "initial-drain", "closing-drain"} {
		t.Run(rail, func(t *testing.T) {
			for _, extended := range []bool{false, true} {
				name := "without-extended-next-hop"
				if extended {
					name = "with-extended-next-hop"
				}
				t.Run(name, func(t *testing.T) {
					peer, conn := newDefaultNextHopControlPeer(t, rail, "192.0.2.1")
					caps := capability.EncodingCaps{}
					if extended {
						caps.ExtendedNextHop = map[capability.Family]capability.AFI{
							{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}: capability.AFIIPv6,
						}
					}
					peer.sendCtx.Store(bgpctx.NewEncodingContext(nil, &caps, bgpctx.DirectionSend))
					batch := defaultNextHopControlBatch("10.30.0.0/24", bgptypes.NewNextHopDefault(netip.MustParseAddr("2001:db8::9")))
					if err := defaultNextHopControlAdapter(peer).AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()); err != nil {
						t.Fatal(err)
					}
					drainDefaultNextHopControl(t, peer, rail)
					bodies := defaultNextHopControlBodies(t, conn.written())
					if len(bodies) != 1 {
						t.Fatalf("announcements = %d, want one IPv4 route", len(bodies))
					}
					assertDefaultNextHopWire(t, bodies[0], netip.MustParseAddr("192.0.2.1"), []byte{24, 10, 30, 0})
				})
			}
		})
	}
}

// TestDefaultNextHopIPv6RequiresExactExtendedPair pairs each refusal with an
// explicit IPv4 announcement that must survive. The explicit IPv6 control
// keeps its requested next hop even when the configured default is self.
//
// RFC requirement: RFC8950-4-1 positive -- the exact IPv4/unicast-to-IPv6 pair
// permits a useful IPv4 prefix with the selected IPv6 next hop in MP_REACH,
// without classic NEXT_HOP or inline NLRI, on direct and both queue drains.
// RFC requirement: RFC8950-4-1 negative -- no pair or a VPN-only pair refuses
// that IPv6 next hop for IPv4 unicast on direct and both queue drains, while
// an explicit IPv4 announcement still reaches the wire.
//
// MUTATION: before Negotiate publishes its composite facts, remove the exact
// pair to break the useful licensed output, or inject it without advertisement
// to emit the forbidden IPv6 announcement. A resolver-only bypass can change
// the direct error class while the final writer still refuses; that is not a
// substitute for observing the forbidden wire announcement.
func TestDefaultNextHopIPv6RequiresExactExtendedPair(t *testing.T) {
	for _, rail := range []string{"direct", "initial-drain", "closing-drain"} {
		t.Run(rail, func(t *testing.T) {
			for _, policy := range []string{"internal-gateway", "external-session", "explicit-api"} {
				t.Run(policy, func(t *testing.T) {
					for _, pair := range []string{"none", "vpn-only", "unicast"} {
						t.Run(pair, func(t *testing.T) {
							peer, conn := newDefaultNextHopControlPeer(t, rail, "192.0.2.1")
							hop := bgptypes.NewNextHopDefault(netip.MustParseAddr("2001:db8::9"))
							switch policy {
							case "internal-gateway":
								peer.settings.PeerAS = peer.settings.LocalAS
							case "external-session":
								peer.session.transport.Store(&sessionTransport{local: netip.MustParseAddr("2001:db8::9")})
								hop = bgptypes.NewNextHopDefault(netip.MustParseAddr("10.1.0.2"))
							case "explicit-api":
								peer.settings.NextHopMode = NextHopSelf
								hop = bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8::9"))
							default:
								t.Fatal("unknown test policy")
							}
							caps := []capability.Capability{
								&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
							}
							switch pair {
							case "none":
							case "vpn-only":
								caps = append(caps, &capability.ExtendedNextHop{Families: []capability.ExtendedNextHopFamily{{
									NLRIAFI: capability.AFIIPv4, NLRISAFI: capability.SAFIVPN, NextHopAFI: capability.AFIIPv6,
								}}})
							case "unicast":
								caps = append(caps, &capability.ExtendedNextHop{Families: []capability.ExtendedNextHopFamily{{
									NLRIAFI: capability.AFIIPv4, NLRISAFI: capability.SAFIUnicast, NextHopAFI: capability.AFIIPv6,
								}}})
							default:
								t.Fatal("unknown test pair")
							}
							neg := capability.Negotiate(caps, caps, capability.PeerIdentity{
								LocalASN: peer.settings.LocalAS,
								PeerASN:  peer.settings.PeerAS,
								Internal: peer.settings.IsIBGP(),
							})
							peer.session.negotiated = neg
							peer.negotiated.Store(NewNegotiatedCapabilities(neg))
							peer.setEncodingContexts(neg)
							adapter := defaultNextHopControlAdapter(peer)
							err := adapter.AnnounceNLRIBatch(t.Context(), selector.All(), defaultNextHopControlBatch("10.30.0.0/24", hop), plugin.OperatorSender())
							if rail == "direct" && pair != "unicast" {
								if !errors.Is(err, ErrNextHopIncompatible) {
									t.Fatalf("unlicensed IPv6 next hop: got %v, want ErrNextHopIncompatible", err)
								}
							} else if err != nil {
								t.Fatal(err)
							}
							canary := defaultNextHopControlBatch("10.31.0.0/24", bgptypes.NewNextHopExplicit(netip.MustParseAddr("198.51.100.1")))
							if err := adapter.AnnounceNLRIBatch(t.Context(), selector.All(), canary, plugin.OperatorSender()); err != nil {
								t.Fatal(err)
							}
							drainDefaultNextHopControl(t, peer, rail)
							bodies := defaultNextHopControlBodies(t, conn.written())
							want := 1
							if pair == "unicast" {
								want = 2
							}
							if len(bodies) != want {
								t.Fatalf("announcements = %d, want %d with the IPv4 survivor", len(bodies), want)
							}
							if pair == "unicast" {
								assertDefaultNextHopWire(t, bodies[0], netip.MustParseAddr("2001:db8::9"), []byte{24, 10, 30, 0})
							}
							assertDefaultNextHopWire(t, bodies[len(bodies)-1], netip.MustParseAddr("198.51.100.1"), []byte{24, 10, 31, 0})
						})
					}
				})
			}
		})
	}
}

// TestDefaultNextHopRetainsMigrationInternalGateway observes the next-hop
// decision on the wire for the two internal AS numbers of a migration session.
// The same legacy peer without migration remains external and receives self.
func TestDefaultNextHopRetainsMigrationInternalGateway(t *testing.T) {
	for _, rail := range []string{"direct", "initial-drain", "closing-drain"} {
		t.Run(rail, func(t *testing.T) {
			for _, tc := range []struct {
				name      string
				peerAS    uint32
				migration uint32
				want      string
			}{
				{"retained-as", 65000, 65010, "10.1.0.2"},
				{"legacy-as", 65010, 65010, "10.1.0.2"},
				{"legacy-without-migration", 65010, 0, "192.0.2.1"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					peer, conn := newDefaultNextHopControlPeer(t, rail, "192.0.2.1")
					peer.settings.PeerAS = tc.peerAS
					if err := setMigrationAS(peer.settings, tc.migration); err != nil {
						t.Fatal(err)
					}
					batch := defaultNextHopControlBatch("10.30.0.0/24", bgptypes.NewNextHopDefault(netip.MustParseAddr("10.1.0.2")))
					if err := defaultNextHopControlAdapter(peer).AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()); err != nil {
						t.Fatal(err)
					}
					drainDefaultNextHopControl(t, peer, rail)
					bodies := defaultNextHopControlBodies(t, conn.written())
					if len(bodies) != 1 {
						t.Fatalf("announcements = %d, want one route", len(bodies))
					}
					assertDefaultNextHopWire(t, bodies[0], netip.MustParseAddr(tc.want), []byte{24, 10, 30, 0})
				})
			}
		})
	}
}

// TestQueuedDefaultNextHopResolvesReplacementConnection queues two routes on
// a real connection, retires it, then drains on a different connected endpoint.
// The wire must retain both prefixes and use only the replacement endpoint.
func TestQueuedDefaultNextHopResolvesReplacementConnection(t *testing.T) {
	for _, rail := range []string{"initial-drain", "closing-drain"} {
		t.Run(rail, func(t *testing.T) {
			peer, _ := newDefaultNextHopControlPeer(t, rail, "192.0.2.1")
			old, oldClient := newConnectedLocalSession(t, peer.settings, "127.0.0.1")
			peer.session = old
			// The initial-sync gate and End-of-RIB mark live on the session,
			// so each installed session carries the closed gate the fixture
			// set (newInitialSyncPeer) on the session it installed.
			peer.setTestInitialSync(1, true)
			adapter := defaultNextHopControlAdapter(peer)
			for _, prefix := range []string{"10.30.0.0/24", "10.31.0.0/24"} {
				batch := defaultNextHopControlBatch(prefix, bgptypes.NewNextHopDefault(netip.MustParseAddr("10.1.0.2")))
				if err := adapter.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()); err != nil {
					t.Fatal(err)
				}
			}
			if len(peer.opQueue) != 2 {
				t.Fatalf("queued routes = %d, want two", len(peer.opQueue))
			}
			for _, op := range peer.opQueue {
				if op.NextHopPolicy != bgptypes.NextHopDefault {
					t.Fatalf("queued policy = %v, want Default", op.NextHopPolicy)
				}
				if op.Route.NextHop() != netip.MustParseAddr("10.1.0.2") {
					t.Fatalf("queued gateway = %v, want the unresolved candidate", op.Route.NextHop())
				}
			}
			old.closeConn()
			if _, err := core4271ReadMessage(oldClient); !errors.Is(err, io.EOF) {
				t.Fatalf("old connection received a queued route: %v", err)
			}
			replacement, client := newConnectedLocalSession(t, peer.settings, "127.0.0.2")
			if err := replacement.fsm.Event(fsm.EventBGPOpen); err != nil {
				t.Fatal(err)
			}
			if err := replacement.fsm.Event(fsm.EventKeepaliveMsg); err != nil {
				t.Fatal(err)
			}
			peer.session = replacement
			peer.setTestInitialSync(1, true)
			drainDefaultNextHopControl(t, peer, rail)
			// Closing makes EOF a deterministic end of the complete wire capture.
			replacement.closeConn()
			var bodies [][]byte
			for {
				packet, err := core4271ReadMessage(client)
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				if packet[18] != byte(msgtype.TypeUPDATE) {
					t.Fatalf("message type = %d, want UPDATE", packet[18])
				}
				bodies = append(bodies, defaultNextHopControlBodies(t, packet)...)
			}
			if len(bodies) != 2 {
				t.Fatalf("replacement announcements = %d, want both queued prefixes", len(bodies))
			}
			assertDefaultNextHopWire(t, bodies[0], netip.MustParseAddr("127.0.0.2"), []byte{24, 10, 30, 0})
			assertDefaultNextHopWire(t, bodies[1], netip.MustParseAddr("127.0.0.2"), []byte{24, 10, 31, 0})
		})
	}
}

func newDefaultNextHopControlPeer(t *testing.T, rail, local string) (*Peer, *recordingConn) {
	t.Helper()
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	// A configured bind address must not replace the connection's identity.
	peer.settings.LocalAddress = netip.MustParseAddr("198.51.100.99")
	peer.session.transport.Store(&sessionTransport{local: netip.MustParseAddr(local)})
	if rail == "direct" {
		peer.testInitialSync().sendingInitialRoutes.Store(0)
		peer.testInitialSync().initialSyncEOROwed.Store(false)
	}
	return peer, conn
}

func defaultNextHopControlAdapter(peer *Peer) *reactorAPIAdapter {
	return &reactorAPIAdapter{r: &Reactor{
		config:          &Config{LocalAS: peer.settings.LocalAS},
		attrModHandlers: attrModHandlersWithDefaults(),
		peers:           map[netip.AddrPort]*Peer{peer.settings.PeerKey(): peer},
	}}
}

func defaultNextHopControlBatch(prefix string, hop bgptypes.RouteNextHop) bgptypes.NLRIBatch {
	return bgptypes.NLRIBatch{
		Family:  family.IPv4Unicast,
		NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix(prefix), 0, false)},
		NextHop: hop,
	}
}

func assertDefaultNextHopQueued(t *testing.T, peer *Peer, candidate string) {
	t.Helper()
	if len(peer.opQueue) != 1 {
		t.Fatalf("queued routes = %d, want one", len(peer.opQueue))
	}
	op := peer.opQueue[0]
	if op.NextHopPolicy != bgptypes.NextHopDefault {
		t.Fatalf("queued policy = %v, want Default", op.NextHopPolicy)
	}
	if op.Route.NextHop() != netip.MustParseAddr(candidate) {
		t.Fatalf("queued gateway = %v, want %s", op.Route.NextHop(), candidate)
	}
}

func drainDefaultNextHopControl(t *testing.T, peer *Peer, rail string) {
	t.Helper()
	switch rail {
	case "direct":
	case "initial-drain":
		peer.runInitialSyncNow()
	case "closing-drain":
		if peer.drainAndCloseQueueGate(peer.testInitialSync(), peer.addrString, message.MaxMsgLen) {
			t.Fatal("announcement drain unexpectedly tore down the session")
		}
	default:
		t.Fatalf("unknown test rail %s", rail)
	}
	if len(peer.opQueue) != 0 {
		t.Fatalf("drain left %d queued operations", len(peer.opQueue))
	}
	if peer.testInitialSync().sendingInitialRoutes.Load() != 0 {
		t.Fatal("drain left the queue gate closed")
	}
}

func defaultNextHopControlBodies(t *testing.T, frames []byte) [][]byte {
	t.Helper()
	var bodies [][]byte
	for _, body := range updateBodies(t, frames) {
		if bytes.Equal(body, []byte{0, 0, 0, 0}) {
			continue // The initial drain also sends the IPv4 End-of-RIB.
		}
		bodies = append(bodies, body)
	}
	return bodies
}

func assertDefaultNextHopWire(t *testing.T, body []byte, hop netip.Addr, prefix []byte) {
	t.Helper()
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(sections.Withdrawn(body)) != 0 {
		t.Fatal("announcement unexpectedly withdraws a route")
	}
	attrs := sections.Attrs(body)
	_, _, classic, hasClassic := attribute.AttrFind(attrs, attribute.AttrNextHop)
	_, _, reach, hasReach := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI)
	if hop.Is4() {
		if !hasClassic {
			t.Fatal("IPv4 next hop is missing NEXT_HOP")
		}
		if hasReach {
			t.Fatal("IPv4 session endpoint unexpectedly became MP_REACH_NLRI")
		}
		if !bytes.Equal(classic, hop.AsSlice()) {
			t.Fatalf("NEXT_HOP = %x, want %x", classic, hop.AsSlice())
		}
		if !bytes.Equal(sections.NLRI(body), prefix) {
			t.Fatalf("inline NLRI = %x, want %x", sections.NLRI(body), prefix)
		}
		return
	}
	if hasClassic {
		t.Fatal("IPv6 next hop unexpectedly carries classic NEXT_HOP")
	}
	if !hasReach {
		t.Fatal("IPv6 next hop is missing MP_REACH_NLRI")
	}
	if len(sections.NLRI(body)) != 0 {
		t.Fatalf("IPv6 next hop leaked inline NLRI: %x", sections.NLRI(body))
	}
	want := append([]byte{0, 1, 1, 16}, hop.AsSlice()...)
	want = append(want, 0)
	want = append(want, prefix...)
	if !bytes.Equal(reach, want) {
		t.Fatalf("MP_REACH_NLRI = %x, want %x", reach, want)
	}
}
