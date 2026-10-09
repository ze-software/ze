// Design: docs/architecture/api/architecture.md -- ordinary route injection and commits.
package reactor

import (
	"bufio"
	"bytes"
	"errors"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/rib"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

type ordinaryJointRail uint8

const (
	ordinaryJointRailUnspecified ordinaryJointRail = iota
	ordinaryJointBatch
	ordinaryJointQueued
	ordinaryJointCommit
)

type ordinaryJointSource uint8

const (
	ordinaryJointSourceUnspecified ordinaryJointSource = iota
	ordinaryJointOwnGlobal
	ordinaryJointPolicyPair
)

// TestOrdinaryIPv6OwnGlobalJointSubnet requires the available speaker-owned LL
// when speaker, effective global next hop and recipient share one subnet. The
// off-link control must still deliver the same NLRI with the global address only.
// These are ordinary API producers, not the already-normalized forwarding rail.
//
// RFC requirement: RFC2545-3-3 positive -- batch, queued drain and named commit
// include the configured speaker-owned LL after the global address when speaker,
// next-hop entity and recipient share one subnet; exact wire and delivery are asserted.
// RFC requirement: RFC2545-3-4 negative -- those common-subnet advertisements keep
// the full 32-octet pair rather than taking the otherwise-global-only branch.
func TestOrdinaryIPv6OwnGlobalJointSubnet(t *testing.T) {
	testOrdinaryIPv6JointSubnet(t, ordinaryJointOwnGlobal)
}

// TestOrdinaryIPv6PolicyPairJointSubnet requires normalization of the effective
// post-policy pair, not the pre-policy next hop. A valid off-link pair is trimmed,
// never rejected; the same pair on the common subnet must remain intact. Existing
// invalid-field admission tests separately require malformed address roles to fail.
//
// RFC requirement: RFC2545-3-3 negative -- a valid effective export-policy pair
// loses its LL when the recipient is off-link on batch, queued drain and commit.
// RFC requirement: RFC2545-3-4 positive -- each off-link policy replacement emits
// the exact 16-octet global field and unchanged NLRI, with successful delivery.
func TestOrdinaryIPv6PolicyPairJointSubnet(t *testing.T) {
	testOrdinaryIPv6JointSubnet(t, ordinaryJointPolicyPair)
}

// TestOrdinaryIPv6NormalizationPreservesAdmission ensures off-link trimming
// cannot conceal an invalid pair or the recipient's own address in its LL slot.
// Capability 77 licenses a standalone LL, never invalid roles within a pair.
func TestOrdinaryIPv6NormalizationPreservesAdmission(t *testing.T) {
	for _, negotiation := range []struct {
		name string
		caps []capability.Capability
	}{
		{name: "without-cap77"},
		{name: "with-cap77", caps: []capability.Capability{&capability.LinkLocalNextHop{}}},
	} {
		t.Run(negotiation.name, func(t *testing.T) {
			offLink := netip.MustParseAddr("2001:db8:b::2")
			for _, tc := range []struct {
				name      string
				global    netip.Addr
				linkLocal netip.Addr
				recipient netip.Addr
				wantErr   error
			}{
				{name: "link-local-first", global: netip.MustParseAddr("fe80::1"), linkLocal: netip.MustParseAddr("fe80::9"), recipient: offLink, wantErr: message.ErrUnicastNextHopUnusable},
				{name: "global-second", global: netip.MustParseAddr("2001:db8:a::1"), linkLocal: netip.MustParseAddr("2001:db8:a::9"), recipient: offLink, wantErr: message.ErrUnicastNextHopUnusable},
				{name: "unspecified-second", global: netip.MustParseAddr("2001:db8:a::1"), linkLocal: netip.IPv6Unspecified(), recipient: offLink, wantErr: message.ErrUnicastNextHopUnusable},
				{name: "multicast-second", global: netip.MustParseAddr("2001:db8:a::1"), linkLocal: netip.MustParseAddr("ff02::1"), recipient: offLink, wantErr: message.ErrUnicastNextHopUnusable},
				{name: "peer-owned-second", global: netip.MustParseAddr("2001:db8:a::1"), linkLocal: netip.MustParseAddr("fe80::9"), recipient: netip.MustParseAddr("fe80::9")},
			} {
				t.Run(tc.name, func(t *testing.T) {
					r, peer, conn := newOrdinaryJointPeer(t, tc.recipient, ordinaryJointPolicyPair)
					peer.session.negotiated = capability.Negotiate(negotiation.caps, negotiation.caps, capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001})
					// Literal field construction bypasses builder admission so the
					// actual policy-to-writer boundary has to judge both slots.
					mp := []byte{0, 2, 1, 32}
					mp = append(mp, tc.global.AsSlice()...)
					mp = append(mp, tc.linkLocal.AsSlice()...)
					mp = append(mp, mustHex(t, "004020010db800070000")...)
					attrs := mustHex(t, "4001010040020602010000fde8")
					attrs = append(attrs, 0x80, 0x0e, byte(len(mp)))
					attrs = append(attrs, mp...)
					body := append([]byte{0, 0, byte(len(attrs) >> 8), byte(len(attrs))}, attrs...)
					r.api = &pluginserver.Server{}
					policyCalls := 0
					r.policyFilterSeam = func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
						policyCalls++
						return PolicyResponse{Action: PolicyModify, Raw: body}
					}
					batch := bgptypes.NLRIBatch{
						Family:  family.IPv6Unicast,
						NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv6Unicast, netip.MustParsePrefix("2001:db8:7::/64"), 0)},
						NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8:c::1")),
					}
					api := &reactorAPIAdapter{r: r}
					if err := api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()); !errors.Is(err, tc.wantErr) {
						t.Errorf("ordinary refusal = %v, want %v", err, tc.wantErr)
					}
					if got := conn.written(); len(got) != 0 {
						t.Errorf("inadmissible policy pair reached wire: %x", got)
					}
					if stats := peer.Stats(); stats.UpdatesSent != 0 || stats.EORSent != 0 || peer.hasAdvertised() {
						t.Errorf("refused pair earned accounting: updates/EOR=%d/%d advertised=%v", stats.UpdatesSent, stats.EORSent, peer.hasAdvertised())
					}
					if policyCalls != 1 {
						t.Errorf("export policy calls = %d, want 1", policyCalls)
					}
				})
			}
		})
	}
}

func testOrdinaryIPv6JointSubnet(t *testing.T, source ordinaryJointSource) {
	t.Helper()
	// Independent complete-frame oracles: MP_REACH length, both address roles,
	// reserved octet, unchanged /64 NLRI, ORIGIN and exactly one local-AS prepend.
	globalWire := mustHex(t, "ffffffffffffffffffffffffffffffff0045020000002e4001010040020602010000fde8800e1e0002011020010db8000a00000000000000000001004020010db800070000")
	pairWire := mustHex(t, "ffffffffffffffffffffffffffffffff0055020000003e4001010040020602010000fde8800e2e0002012020010db8000a00000000000000000001fe800000000000000000000000000001004020010db800070000")
	for _, rail := range []struct {
		name string
		kind ordinaryJointRail
	}{
		{name: "batch", kind: ordinaryJointBatch},
		{name: "queued-initial-drain", kind: ordinaryJointQueued},
		{name: "named-commit", kind: ordinaryJointCommit},
	} {
		t.Run(rail.name, func(t *testing.T) {
			for _, tc := range []struct {
				name      string
				recipient netip.Addr
				wantWire  []byte
			}{
				{name: "common-subnet", recipient: netip.MustParseAddr("2001:db8:a::2"), wantWire: pairWire},
				{name: "recipient-off-link", recipient: netip.MustParseAddr("2001:db8:b::2"), wantWire: globalWire},
			} {
				t.Run(tc.name, func(t *testing.T) {
					r, peer, conn := newOrdinaryJointPeer(t, tc.recipient, source)
					api := &reactorAPIAdapter{r: r}
					nextHop := peer.settings.LocalAddress
					policyCalls := 0
					switch source {
					case ordinaryJointOwnGlobal:
						// No policy is configured, but the real ordinary egress callback
						// is still attached by newOrdinaryJointPeer, as in Peer.runOnce.
					case ordinaryJointPolicyPair:
						// The input global is deliberately outside the common subnet.
						// Only the policy's effective global (S) establishes the pair.
						nextHop = netip.MustParseAddr("2001:db8:c::1")
						r.api = &pluginserver.Server{}
						r.policyFilterSeam = func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
							policyCalls++
							return PolicyResponse{Action: PolicyModify, Raw: bytes.Clone(pairWire[19:])}
						}
					case ordinaryJointSourceUnspecified:
						t.Fatal("ordinary joint source must be specified")
					default:
						t.Fatalf("unknown ordinary joint source %d", source)
					}
					batch := bgptypes.NLRIBatch{
						Family:  family.IPv6Unicast,
						NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv6Unicast, netip.MustParsePrefix("2001:db8:7::/64"), 0)},
						NextHop: bgptypes.NewNextHopExplicit(nextHop),
					}
					var eors uint32
					switch rail.kind {
					case ordinaryJointBatch:
						if err := api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()); err != nil {
							t.Errorf("valid batch announcement refused: %v", err)
						}
					case ordinaryJointQueued:
						peer.sendingInitialRoutes.Store(1)
						peer.initialSyncEOROwed.Store(true)
						if err := api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()); err != nil {
							t.Fatalf("queue admission refused: %v", err)
						}
						if len(peer.opQueue) != 1 {
							t.Fatalf("queued operations = %d, want 1", len(peer.opQueue))
						}
						if len(conn.written()) != 0 {
							t.Fatal("announcement or EOR escaped before initial synchronization")
						}
						if stats := peer.Stats(); stats.UpdatesSent != 0 || stats.EORSent != 0 || peer.hasAdvertised() || policyCalls != 0 {
							t.Fatalf("queued route ran early: updates/EOR=%d/%d advertised=%v policy=%d",
								stats.UpdatesSent, stats.EORSent, peer.hasAdvertised(), policyCalls)
						}
						peer.sendInitialRoutes()
						if len(peer.opQueue) != 0 || peer.sendingInitialRoutes.Load() != 0 || peer.initialSyncEOROwed.Load() {
							t.Errorf("initial drain incomplete: queued=%d gate=%d EOR-owed=%v",
								len(peer.opQueue), peer.sendingInitialRoutes.Load(), peer.initialSyncEOROwed.Load())
						}
						eors = 1
					case ordinaryJointCommit:
						routes := []*rib.Route{rib.NewRouteWithASPath(batch.NLRIs[0], nextHop, nil, nil)}
						result, err := api.SendRoutes(selector.All(), routes, nil, true, plugin.OperatorSender())
						if err != nil {
							t.Fatalf("named commit refused: %v", err)
						}
						if result.RoutesQueued != 1 || result.RoutesAnnounced != 1 || result.RoutesWithdrawn != 0 || result.UpdatesSent != 2 || result.EORSent != 1 {
							t.Errorf("commit result = %+v, want one announcement, no withdrawal, two updates and one EOR", result)
						}
						if len(result.Peers) != 1 {
							t.Fatalf("commit peer rows = %d, want 1", len(result.Peers))
						}
						row := result.Peers[0]
						if row.Address != tc.recipient.String() || row.RoutesAnnounced != 1 || row.RoutesWithdrawn != 0 || row.UpdatesSent != 2 || row.EORSent != 1 || len(row.Reasons) != 0 {
							t.Errorf("commit recipient row = %+v, want successful one-route delivery to %s", row, tc.recipient)
						}
						eors = 1
					case ordinaryJointRailUnspecified:
						t.Fatal("ordinary joint rail must be specified")
					default:
						t.Fatalf("unknown ordinary joint rail %d", rail.kind)
					}
					want := bytes.Clone(tc.wantWire)
					if eors != 0 {
						want = append(want, eorWire(family.IPv6Unicast)...)
					}
					if got := conn.written(); !bytes.Equal(got, want) {
						t.Errorf("recipient wire = %x, want %x", got, want)
					}
					if stats := peer.Stats(); stats.UpdatesSent != 1+eors || stats.EORSent != eors {
						t.Errorf("production updates/EOR = %d/%d, want %d/%d", stats.UpdatesSent, stats.EORSent, 1+eors, eors)
					}
					if !peer.hasAdvertised() {
						t.Error("successful reachable advertisement was not recorded")
					}
					wantPolicyCalls := 0
					if source == ordinaryJointPolicyPair {
						wantPolicyCalls = 1
					}
					if policyCalls != wantPolicyCalls {
						t.Errorf("export policy calls = %d, want %d", policyCalls, wantPolicyCalls)
					}
				})
			}
		})
	}
}

// newOrdinaryJointPeer follows newInitialSyncPeer's Established recording Session
// pattern, but registers the FINAL topology before constructing the Session. In
// particular Session.settings and Peer.settings must name the same recipient.
// The connected-prefix snapshot is explicit: host interfaces cannot change the
// verdict, and no third-party link-local discovery is supplied or claimed.
func newOrdinaryJointPeer(t *testing.T, recipient netip.Addr, source ordinaryJointSource) (*Reactor, *Peer, *recordingConn) {
	t.Helper()
	settings := &PeerSettings{
		Connection:   ConnectionBoth,
		Address:      recipient,
		LocalAddress: netip.MustParseAddr("2001:db8:a::1"),
		LinkLocal:    netip.MustParseAddr("fe80::1"),
		LocalAS:      65000,
		PeerAS:       65001,
		RouterID:     0x01020301,
	}
	if source == ordinaryJointPolicyPair {
		settings.ExportFilters = []filterapi.FilterRef{{Name: "ordinary-joint:replace-next-hop"}}
	}
	r := New(&Config{ListenAddr: "127.0.0.1:0", LocalAS: 65000})
	if err := r.AddPeer(settings); err != nil {
		t.Fatal(err)
	}
	r.mu.RLock()
	peer, found := r.findPeerByAddr(recipient)
	r.mu.RUnlock()
	if !found {
		t.Fatal("recipient was not registered")
	}
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{families: map[family.Family]bool{family.IPv6Unicast: true}})
	peer.sendCtx.Store(bgpctx.NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001},
		&capability.EncodingCaps{ASN4: true}, bgpctx.DirectionSend))
	session := NewSession(peer.settings)
	for _, event := range []fsm.Event{
		fsm.EventManualStart, fsm.EventTCPConnectionConfirmed, fsm.EventBGPOpen, fsm.EventKeepaliveMsg,
	} {
		if err := session.fsm.Event(event); err != nil {
			t.Fatal(err)
		}
	}
	if session.fsm.State() != fsm.StateEstablished {
		t.Fatal("recording Session did not reach Established")
	}
	session.negotiated = capability.Negotiate(nil, nil, capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001})
	conn := &recordingConn{}
	session.conn = conn
	session.bufWriter = bufio.NewWriterSize(conn, 4096)
	peer.session = session
	if session.settings.Address != peer.settings.Address || session.settings.LocalAddress != peer.settings.LocalAddress || session.settings.LinkLocal != peer.settings.LinkLocal {
		t.Fatal("Session and registered Peer topology disagree")
	}
	// AddPeer installed production sent-message accounting. Peer.runOnce also
	// attaches this ordinary egress callback for EVERY reactor-backed Session,
	// including peers with no export policy. Never replace it with normalization
	// in the harness: the actual producer-to-consumer path is under test.
	session.SetMessageCallback(peer.messageCallback)
	session.egressRouteFilter = func(body []byte) (bool, []byte) {
		return r.exportFilterForBody(peer, body)
	}
	peer.refreshLinkScopeFrom([]netip.Prefix{netip.MustParsePrefix("2001:db8:a::1/64")})
	peer.fwdFacts.Store(peer.buildForwardFacts())
	return r, peer, conn
}
