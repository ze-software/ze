// Design: docs/guide/route-reflection.md -- what a client is sent when a reflected source withdraws or departs.
// Related: rs_withdrawal_replacement_test.go -- the bgp-rs scenarios these mirror.
// Related: rs_withdrawal_ownership_test.go -- the recipient TCP projection these tests read.
// Related: recovery_departure_labels_test.go -- the multi-label withdrawal framing.
// RFC 4456 Section 8 -- see rfc/short/rfc4456.md.
package reactor

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rr"
	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/family"
)

// rrLiveClusterID is the cluster identifier every reflected route carries:
// the fixture's local BGP Identifier (lowLiveSettings), since no cluster-id
// is configured.
const rrLiveClusterID = 0x0a0000fe

// Identifiers of rrLiveRouter's peers by position: peer i opens with
// 0x0a000001+i.
const (
	rrIDA uint32 = 0x0a000001
	rrIDB uint32 = 0x0a000002
	rrIDC uint32 = 0x0a000003
	rrIDE uint32 = 0x0a000004
)

// rrLivePeerSpec is one peer of an rrLiveRouter: its address and whether it
// is a route-reflector client.
type rrLivePeerSpec struct {
	address string
	client  bool
}

// rrLiveRouter is lowLiveRouter with bgp-rr in place of bgp-rs: every peer is
// iBGP in AS 65000, and a spec's client flag sets RouteReflectorClient. With
// vpn set, every peer also negotiates ipv4/mpls-vpn. The read-pool budget is
// rsLiveRouter's, for the same reason. The returned peers keep specs' order.
func rrLiveRouter(t *testing.T, vpn bool, specs ...rrLivePeerSpec) []*lowLivePeer {
	t.Helper()
	previous := env.Get("ze.fwd.pool.maxbytes")
	if err := env.Set("ze.fwd.pool.maxbytes", strconv.FormatInt(rsLiveReadBudget, 10)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := env.Set("ze.fwd.pool.maxbytes", previous); err != nil {
			t.Error(err)
		}
	})
	r := New(&Config{ListenAddr: "127.0.0.1:0"})
	settings := make([]*PeerSettings, 0, len(specs))
	for _, spec := range specs {
		s := lowLiveSettings(spec.address, 65000, 65000)
		s.RouteReflectorClient = spec.client
		if vpn {
			s.Capabilities = append(s.Capabilities,
				&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIMPLS})
		}
		for _, binding := range []struct{ name, receive string }{
			{"bgp-rib", "update state refresh"},
			{"bgp-rr", "update-received state open-received refresh"},
			{"bgp-adj-rib-in", "update-received state"},
		} {
			if err := EnsureProcessBinding(s, binding.name, binding.receive, "update"); err != nil {
				t.Fatal(err)
			}
		}
		if err := r.AddPeer(s); err != nil {
			t.Fatal(err)
		}
		settings = append(settings, s)
	}
	srv := newBorrowedPluginServer(t, r, "bgp-rib", "bgp-adj-rib-in", "bgp-rr")
	r.SetPluginServer(srv)
	if err := r.StartWithContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopAndWait(t, r) })
	startBorrowedPeers(t, r, srv)
	out := make([]*lowLivePeer, 0, len(settings))
	for i, s := range settings {
		out = append(out, rrLiveConnect(t, r.peers[s.PeerKey()], vpn, rrIDA+uint32(i)))
	}
	return out
}

// rrLiveConnect waits for p to be Active, opens one session of p over a pipe
// with the given BGP Identifier, and waits for its initial sync. A peer that
// dropped its session reconnects through it too.
func rrLiveConnect(t *testing.T, p *Peer, vpn bool, identifier uint32) *lowLivePeer {
	t.Helper()
	lowEventually(t, func() bool { return p.currentSession() != nil && p.SessionState() == fsm.StateActive }, "peer active")
	server, remote := net.Pipe()
	v := &lowLivePeer{peer: p, remote: remote, stopped: make(chan struct{})}
	v.startCapture(t)
	t.Cleanup(func() {
		// MUST close before joining: the reader may be waiting for a header.
		_ = remote.Close()
		_ = server.Close()
		<-v.stopped
	})
	if err := p.acceptConnection(server); err != nil {
		t.Fatal(err)
	}
	// RFC 5492 Section 4: capability 65 (four-octet AS, AS 65000), then
	// capability 1 (Multiprotocol) for IPv4 unicast and, with vpn, IPv4
	// MPLS-labeled VPN (SAFI 128).
	params := []byte{2, 6, 65, 4, 0, 0, 0xfd, 0xe8, 2, 6, 1, 4, 0, 1, 0, 1}
	if vpn {
		params = append(params, 2, 6, 1, 4, 0, 1, 0, byte(capability.SAFIMPLS))
	}
	open := &message.Open{Version: 4, MyAS: 65000, HoldTime: 90, BGPIdentifier: identifier, OptionalParams: params}
	v.send(t, message.PackTo(open, nil))
	lowEventually(t, func() bool { return p.SessionState() == fsm.StateOpenConfirm }, "OPEN accepted")
	v.send(t, message.PackTo(message.NewKeepalive(), nil))
	lowEventually(t, func() bool { return p.State() == PeerStateEstablished && !p.pendingSync() }, "peer initial sync")
	// bgp-rr relays its peer-up replay itself and holds the session's live
	// forwards behind the replay fence until it reports session ready. A
	// route a test sends before the fence is lowered reaches this peer twice,
	// once live and once replayed, so the scenario starts after it.
	lowEventually(t, func() bool { return !p.initialUpdateOwed.Load() }, "peer replay fence lowered")
	return v
}

// rrBaseAttrs is ORIGIN IGP, an AS_PATH of asns and LOCAL_PREF 100. Every
// source carries the same LOCAL_PREF, so RFC 4271 Section 9.1.2.2, step a,
// the shorter AS_PATH, decides the election.
func rrBaseAttrs(asns ...uint32) []byte {
	attrs := collapseAttr(0x40, byte(attribute.AttrOrigin), []byte{0})
	attrs = append(attrs, collapseAttr(0x40, byte(attribute.AttrASPath), collapseASPathValue(4, asns...))...)
	return append(attrs, collapseAttr(0x40, byte(attribute.AttrLocalPref), []byte{0, 0, 0, 100})...)
}

// rrAttrs is an iBGP source's IPv4 unicast path attributes: rrBaseAttrs with
// NEXT_HOP 192.0.2.254 in type-code order.
func rrAttrs(asns ...uint32) []byte {
	return append(firstASAttrs(4, asns...), collapseAttr(0x40, byte(attribute.AttrLocalPref), []byte{0, 0, 0, 100})...)
}

// rrReflected is attrs as a client receives them from the reflector, for a
// route learned from the source with BGP Identifier originator.
//
// RFC 4456 Section 8: "This attribute is 4 bytes long and it will be
// created by an RR in reflecting a route." (ORIGINATOR_ID) "When an RR
// reflects a route, it MUST prepend the local CLUSTER_ID to the
// CLUSTER_LIST."
func rrReflected(attrs []byte, originator uint32) []byte {
	out := slices.Clone(attrs)
	out = append(out, collapseAttr(0x80, byte(attribute.AttrOriginatorID), binary.BigEndian.AppendUint32(nil, originator))...)
	return append(out, collapseAttr(0x80, byte(attribute.AttrClusterList), binary.BigEndian.AppendUint32(nil, rrLiveClusterID))...)
}

// rrSettle waits until destination owes nothing, then reads it through a
// fresh marker from fence, which destination's forward worker writes behind
// any write the owed set made.
func rrSettle(t *testing.T, fence, destination *lowLivePeer, marker netip.Prefix, attrs []byte, originator uint32) ownershipLiveProjection {
	t.Helper()
	lowEventually(t, func() bool { return adjOutOwedCount(destination.peer) == 0 }, "the destination owes nothing")
	ownershipLiveSend(t, fence, nil, attrs, marker) // RFC 4271 Section 4.3.
	return ownershipLiveThrough(t, destination, marker, rrReflected(attrs, originator))
}

// rrConverge waits up to five seconds for destination to hold attrs for
// prefix, or, with attrs nil, to hold no route for it. The caller asserts
// on what it reads afterwards, so a timeout here is not itself a failure.
func rrConverge(t *testing.T, destination *lowLivePeer, prefix netip.Prefix, attrs []byte) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		route, present := ownershipLiveRead(t, destination).routes[prefix]
		if attrs == nil && !present {
			return
		}
		if attrs != nil && bytes.Equal(route, attrs) {
			return
		}
		time.Sleep(time.Millisecond)
	}
}

// TestRROwnerWithdrawalRecovers proves AC-10 (a) on bgp-rr's cached rail:
// non-client sources A and B announce 10.9.0.0/24 in the order B then A, A's
// route is better and owns it at client C, which negotiated no ADD-PATH. A
// withdraws. C receives B's route with B's received attributes plus the
// reflector's ORIGINATOR_ID and CLUSTER_LIST, never a withdrawal.
//
// VALIDATES: AC-10 (a), an owner withdrawal on bgp-rr's cached rail is
// answered with the surviving source's reflected route.
// PREVENTS: an RR client losing a prefix another source still offers.
func TestRROwnerWithdrawalRecovers(t *testing.T) {
	peers := rrLiveRouter(t, false,
		rrLivePeerSpec{"192.0.2.1", false}, rrLivePeerSpec{"192.0.2.2", false}, rrLivePeerSpec{"192.0.2.3", true})
	a, b, c := peers[0], peers[1], peers[2]
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	attrsA, attrsB := rrAttrs(65100), rrAttrs(65200, 65200)
	reflectedA, reflectedB := rrReflected(attrsA, rrIDA), rrReflected(attrsB, rrIDB)

	ownershipLiveSend(t, b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefix, reflectedB)
	ownershipLiveSend(t, a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefix, reflectedA)

	ownershipLiveSend(t, a, []netip.Prefix{prefix}, nil) // RFC 4271 Section 4.3.
	rrConverge(t, c, prefix, reflectedB)
	view := rrSettle(t, b, c, netip.MustParsePrefix("10.9.255.0/24"), attrsB, rrIDB)
	// RFC 4271 Section 9.2: "All newly installed routes and all newly
	// unfeasible routes for which there is no replacement route SHALL be
	// advertised to its peers by means of an UPDATE message."
	ownershipLiveRoute(t, view, prefix, reflectedB)
	ownershipLiveWithdrawals(t, view, prefix, 0)
	ownershipLiveAnnouncements(t, view, prefix, reflectedB, reflectedA, reflectedB)
	ownershipLiveEstablished(t, peers)
}

// TestRRDepartureRecoversThroughOwedSet proves AC-10 (b) for bgp-rr: the
// departure bgp-rr sends (handleStateDown, with recovery-source meta) decides
// a key at client C only where the departed source owns C's route. A also
// announces Q, which only A offers: Q is the no-survivor key, and C's single
// withdrawal of it proves C's share of A's departure was decided.
//
//   - "owner-departs": B then A announce P, A owns C's route; A departs and
//     C receives B's reflected route, never a withdrawal.
//   - "owner-stays": A then B announce P, B's route is better and owns C's
//     route; A departs and nothing is written to C for P.
//   - "owner-stays-preferred-third": E, A, then B announce P; E's route is
//     the best and owns C's route; A departs and nothing is written to C.
//
// VALIDATES: AC-10 (b), bgp-rr's departure goes through the owed set.
// PREVENTS: a departure withdrawing a route another source owns at the
// client, or withdrawing the departed owner's route while a survivor exists.
func TestRRDepartureRecoversThroughOwedSet(t *testing.T) {
	t.Run("owner-departs", func(t *testing.T) { rrDepartureDecides(t, true, false) })
	t.Run("owner-stays", func(t *testing.T) { rrDepartureDecides(t, false, false) })
	t.Run("owner-stays-preferred-third", func(t *testing.T) { rrDepartureDecides(t, false, true) })
}

func rrDepartureDecides(t *testing.T, sourceOwns, withE bool) {
	specs := []rrLivePeerSpec{{"192.0.2.1", false}, {"192.0.2.2", false}, {"192.0.2.3", true}}
	if withE {
		specs = append(specs, rrLivePeerSpec{"192.0.2.5", false})
	}
	peers := rrLiveRouter(t, false, specs...)
	a, b, c := peers[0], peers[1], peers[2]
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	only := netip.MustParsePrefix("10.9.1.0/24")
	// RFC 4271 Section 9.1.2.2, step a: the shorter AS_PATH is preferred.
	attrsA, attrsB := rrAttrs(65100, 65100, 65100), rrAttrs(65200, 65200)
	first, second := a, b
	if sourceOwns {
		attrsA, attrsB = rrAttrs(65100, 65100), rrAttrs(65200, 65200, 65200)
		first, second = b, a
	}
	attrsE := rrAttrs(65300)
	attrsOf := map[*lowLivePeer][]byte{a: attrsA, b: attrsB}
	idOf := map[*lowLivePeer]uint32{a: rrIDA, b: rrIDB}

	if withE {
		ownershipLiveSend(t, peers[3], nil, attrsE, prefix) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, c, prefix, rrReflected(attrsE, rrIDE))
	}
	for i, source := range []*lowLivePeer{first, second} {
		ownershipLiveSend(t, source, nil, attrsOf[source], prefix) // RFC 4271 Section 4.3.
		// Each source's own marker fences its announcement of P through C's
		// writer, whether the election writes it or not.
		marker := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 9, byte(2 + i), 0}), 24)
		ownershipLiveSend(t, source, nil, attrsOf[source], marker) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, c, marker, rrReflected(attrsOf[source], idOf[source]))
	}
	ownershipLiveSend(t, a, nil, attrsA, only) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, only, rrReflected(attrsA, rrIDA))
	before := rrSettle(t, b, c, netip.MustParsePrefix("10.9.4.0/24"), attrsB, rrIDB)
	owner := rrReflected(attrsB, rrIDB)
	if sourceOwns {
		owner = rrReflected(attrsA, rrIDA)
	}
	if withE {
		owner = rrReflected(attrsE, rrIDE)
	}
	ownershipLiveRoute(t, before, prefix, owner)

	_ = a.remote.Close() //nolint:errcheck // A departure: the session ends with no NOTIFICATION.

	ownershipLiveDecided(t, c, only)
	if sourceOwns {
		rrConverge(t, c, prefix, rrReflected(attrsB, rrIDB))
	}
	settled := rrSettle(t, b, c, netip.MustParsePrefix("10.9.255.0/24"), attrsB, rrIDB)
	ownershipLiveWithdrawals(t, settled, only, 1)
	ownershipLiveWithdrawals(t, settled, prefix, 0)
	if sourceOwns {
		// RFC 4271 Section 9.2: "All newly installed routes and all newly
		// unfeasible routes for which there is no replacement route SHALL
		// be advertised to its peers by means of an UPDATE message."
		ownershipLiveRoute(t, settled, prefix, rrReflected(attrsB, rrIDB))
		want := append(slices.Clone(before.announcements[prefix]), rrReflected(attrsB, rrIDB))
		ownershipLiveAnnouncements(t, settled, prefix, want...)
	} else {
		// Nothing for P since before A's departure: the owner is not A.
		ownershipLiveRoute(t, settled, prefix, owner)
		ownershipLiveAnnouncements(t, settled, prefix, before.announcements[prefix]...)
	}
	ownershipLiveEstablished(t, []*lowLivePeer{b, c})
}

// rrVPNUpdate is an UPDATE announcing one ipv4/mpls-vpn route carrying three
// labels (departureMultiLabelNLRI) with rrBaseAttrs(asns...).
//
// RFC 4760 Section 3: MP_REACH_NLRI is [AFI:2][SAFI:1][Next Hop Length:1]
// [Next Hop][Reserved:1][NLRI]. RFC 4364 Section 4.3.2: the VPN next hop is
// an RD of zero followed by the IPv4 address, 192.0.2.254 here.
func rrVPNUpdate(asns ...uint32) []byte {
	value := []byte{0, 1, byte(capability.SAFIMPLS), 12, 0, 0, 0, 0, 0, 0, 0, 0, 192, 0, 2, 254, 0}
	value = append(value, departureMultiLabelNLRI(family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN})...)
	path := append(rrBaseAttrs(asns...), collapseAttr(0x80, byte(attribute.AttrMPReachNLRI), value)...)
	return message.PackTo(&message.Update{PathAttributes: path}, nil)
}

// rrVPNHistory answers, in TCP order, the MP_REACH_NLRI and MP_UNREACH_NLRI
// values of every ipv4/mpls-vpn UPDATE destination received.
func rrVPNHistory(t *testing.T, destination *lowLivePeer) (reached, unreached [][]byte) {
	t.Helper()
	destination.mu.Lock()
	frames := slices.Clone(destination.frames)
	destination.mu.Unlock()
	for _, frame := range frames {
		if frame[18] != byte(msgtype.TypeUPDATE) {
			continue
		}
		update, err := message.UnpackUpdate(frame[message.HeaderLen:]) // RFC 4271 Section 4.3.
		if err != nil {
			t.Fatalf("decode recipient UPDATE: %v", err)
		}
		if value := rrAttrValue(t, update.PathAttributes, attribute.AttrMPReachNLRI); len(value) > 2 && value[2] == byte(capability.SAFIMPLS) {
			reached = append(reached, value)
		}
		// RFC 4724 Section 2: an MP_UNREACH_NLRI with no withdrawn routes is
		// the End-of-RIB marker, not a withdrawal.
		if value := rrAttrValue(t, update.PathAttributes, attribute.AttrMPUnreachNLRI); len(value) > 3 && value[2] == byte(capability.SAFIMPLS) {
			unreached = append(unreached, value)
		}
	}
	return reached, unreached
}

// rrAttrValue answers the value of attribute code in attrs, or nil when attrs
// holds none. RFC 4271 Section 4.3: [Flags:1][Type:1][Length:1, or 2 with
// Extended Length][Value].
func rrAttrValue(t *testing.T, attrs []byte, code attribute.AttributeCode) []byte {
	t.Helper()
	for len(attrs) != 0 {
		if len(attrs) < 3 {
			t.Fatalf("truncated attribute header %x", attrs)
		}
		header, length := 3, int(attrs[2])
		if attrs[0]&byte(attribute.FlagExtLength) != 0 {
			if len(attrs) < 4 {
				t.Fatalf("truncated extended attribute header %x", attrs)
			}
			header, length = 4, int(binary.BigEndian.Uint16(attrs[2:4]))
		}
		if len(attrs) < header+length {
			t.Fatalf("attribute overruns the field: %x", attrs)
		}
		if attrs[1] == byte(code) {
			return attrs[header : header+length]
		}
		attrs = attrs[header+length:]
	}
	return nil
}

// TestRRDepartureHonoursRetention proves AC-10 (c) and Q-5 for bgp-rr, with
// the round 9 NOTE's three-label departure withdrawal. A owns 10.9.0.0/24 at
// client C and alone announces a three-label ipv4/mpls-vpn route; bgp-gr
// retains A's ipv4/unicast and not its VPN family (retain-routes, then
// retention.Publish, as bgp-gr does before the session drops). A's session
// ends:
//
//   - the retained unicast route is not withdrawn from C: nothing is written
//     to C for it, and C keeps A's reflected route;
//   - the VPN route, not retained, goes through departure recovery, has no
//     survivor, and is withdrawn from C once, with one Compatibility field
//     0x800000 in place of the three-label stack.
//
// VALIDATES: AC-10 (c), Q-5, and that bgp-rr's departure of a multi-label
// route reaches the client as a withdrawal the client can match.
// PREVENTS: an RR client losing a GR-retained route the moment its source
// drops (RFC 4724 Section 4.2), and a departure withdrawal naming a route the
// client never held.
func TestRRDepartureHonoursRetention(t *testing.T) {
	peers := rrLiveRouter(t, true,
		rrLivePeerSpec{"192.0.2.1", false}, rrLivePeerSpec{"192.0.2.2", false}, rrLivePeerSpec{"192.0.2.3", true})
	a, b, c := peers[0], peers[1], peers[2]
	r := a.peer.reactor
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	attrsA, attrsB := rrAttrs(65100), rrAttrs(65200)
	reflectedA := rrReflected(attrsA, rrIDA)

	ownershipLiveSend(t, a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefix, reflectedA)
	a.send(t, rrVPNUpdate(65100)) // RFC 4760 Section 3.
	lowEventually(t, func() bool {
		reached, _ := rrVPNHistory(t, c)
		return len(reached) == 1
	}, "C holds A's VPN route")

	source := a.peer.Settings().Address.String()
	// RFC 4724 Section 4.2: "it MUST retain the routes received from the
	// peer for all the address families that were previously received in
	// the Graceful Restart Capability and MUST mark them as stale routing
	// information."
	purgeRIBCommand(t, r, "request bgp rib retain-routes", source, "ipv4/unicast")
	retained := retention.Publish(func(peer string, fam family.Family) bool {
		return peer == source && fam == family.IPv4Unicast
	})
	t.Cleanup(retained.Close)
	_ = a.remote.Close() //nolint:errcheck // A departure: the session ends with no NOTIFICATION.
	lowEventually(t, func() bool { return a.peer.SessionState() != fsm.StateEstablished }, "A's session ended")
	purgeRIBCommand(t, r, "request bgp rib mark-stale", source, "0", "1", "ipv4/unicast")

	lowEventually(t, func() bool {
		_, unreached := rrVPNHistory(t, c)
		return len(unreached) != 0
	}, "C is sent the VPN withdrawal")
	// bgp-rr sends one departure command per family group, in map order, so
	// the VPN withdrawal fences nothing about the unicast group: a departure
	// that wrongly sent P would issue that command after the VPN one, at
	// once. Watch C for one second after the VPN withdrawal; a withdrawal of
	// P inside it is the defect.
	for watch := time.Now().Add(time.Second); time.Now().Before(watch); time.Sleep(5 * time.Millisecond) {
		if withdrawn := ownershipLiveRead(t, c).withdrawals[prefix]; withdrawn != 0 {
			t.Fatalf("A's retained %s was withdrawn from C at departure %d time(s)", prefix, withdrawn)
		}
	}
	view := rrSettle(t, b, c, netip.MustParsePrefix("10.9.255.0/24"), attrsB, rrIDB)
	// RFC 4724 Section 4.2: "The router MUST NOT differentiate between stale
	// and other routing information during forwarding."
	ownershipLiveRoute(t, view, prefix, reflectedA)
	ownershipLiveWithdrawals(t, view, prefix, 0)
	ownershipLiveAnnouncements(t, view, prefix, reflectedA)

	_, unreached := rrVPNHistory(t, c)
	if len(unreached) != 1 {
		t.Fatalf("C was sent %d VPN withdrawals, want 1: %x", len(unreached), unreached)
	}
	vpn := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}
	// RFC 8277 Section 2.4: "Upon transmission, the Compatibility field
	// SHOULD be set to 0x800000." RFC 4760 Section 4: [AFI:2][SAFI:1]
	// [Withdrawn Routes].
	want := append([]byte{0, 1, byte(capability.SAFIMPLS)}, departureWithdrawalNLRI(vpn)...)
	if !bytes.Equal(unreached[0], want) {
		t.Fatalf("C's VPN withdrawal MP_UNREACH_NLRI = %x, want %x (Compatibility 0x800000 in place of the three-label stack)",
			unreached[0], want)
	}
	ownershipLiveEstablished(t, []*lowLivePeer{b, c})
}

// TestRRGRPurgeRecoversSurvivor proves AC-19 for bgp-rr against the real
// bgp-rib. A owns 10.9.0.0/24 at client C and B announces it too. A's
// ipv4/unicast is retained; A's session ends, bgp-rr leaves the retained
// family to the RIB (Q-5), and C keeps A's route. The retained route is then
// purged by purge-stale (End-of-RIB), by release-routes, or by the restart
// time running out. C receives B's reflected route and no withdrawal; with no
// B, C receives the withdrawal exactly once.
//
// VALIDATES: AC-19 for bgp-rr, A-10: bgp-rib records bgp-rr's forwards with
// the reflected source, so a purge reaches an RR client as an owner
// withdrawal.
// PREVENTS: an RR client sent a bare withdrawal when a retained route is
// purged while a survivor exists.
func TestRRGRPurgeRecoversSurvivor(t *testing.T) {
	for _, purge := range []struct {
		name string
		how  grPurge
	}{{"end-of-rib", grPurgeEndOfRIB}, {"release", grPurgeRelease}, {"expiry", grPurgeExpiry}} {
		t.Run(purge.name+"/survivor", func(t *testing.T) { rrGRPurgeRecovers(t, purge.how, true) })
		t.Run(purge.name+"/no-survivor", func(t *testing.T) { rrGRPurgeRecovers(t, purge.how, false) })
	}
}

func rrGRPurgeRecovers(t *testing.T, how grPurge, withB bool) {
	peers := rrLiveRouter(t, false,
		rrLivePeerSpec{"192.0.2.1", false}, rrLivePeerSpec{"192.0.2.2", false}, rrLivePeerSpec{"192.0.2.3", true})
	a, b, c := peers[0], peers[1], peers[2]
	r := a.peer.reactor
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	attrsA, attrsB := rrAttrs(65100), rrAttrs(65200, 65200)
	reflectedA, reflectedB := rrReflected(attrsA, rrIDA), rrReflected(attrsB, rrIDB)
	if withB {
		ownershipLiveSend(t, b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, c, prefix, reflectedB)
	}
	ownershipLiveSend(t, a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefix, reflectedA)

	source := a.peer.Settings().Address.String()
	// RFC 4724 Section 4.2: "it MUST retain the routes received from the
	// peer for all the address families that were previously received in
	// the Graceful Restart Capability and MUST mark them as stale routing
	// information."
	purgeRIBCommand(t, r, "request bgp rib retain-routes", source, "ipv4/unicast")
	retained := retention.Publish(func(peer string, fam family.Family) bool {
		return peer == source && fam == family.IPv4Unicast
	})
	t.Cleanup(retained.Close)
	_ = a.remote.Close() //nolint:errcheck // A departure: the session ends with no NOTIFICATION.
	lowEventually(t, func() bool { return a.peer.SessionState() != fsm.StateEstablished }, "A's session ended")
	restart := "0"
	if how == grPurgeExpiry {
		restart = "1"
	}
	purgeRIBCommand(t, r, "request bgp rib mark-stale", source, restart, "1", "ipv4/unicast")
	// The retained route stays at C until it is purged.
	held := rrSettle(t, b, c, netip.MustParsePrefix("10.9.253.0/24"), attrsB, rrIDB)
	ownershipLiveRoute(t, held, prefix, reflectedA)
	before := held.withdrawals[prefix]

	switch how {
	case grPurgeEndOfRIB:
		purged, err := purgeRIBStale(r, a.peer.Settings().Address)
		if err != nil {
			t.Fatal(err)
		}
		if purged != 1 {
			t.Fatalf("purge-stale purged %d of A's routes, want 1", purged)
		}
	case grPurgeRelease:
		purgeRIBCommand(t, r, "request bgp rib release-routes", source)
	case grPurgeExpiry:
		// The restart time of one second runs out inside bgp-rib.
	case grPurgeUnspecified:
		t.Fatal("BUG: a purge with no kind")
	}
	// bgp-rib's expiry fires after the restart time plus its 5 s margin
	// (grTimerMargin), so the wait allows 20 s.
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		route, present := ownershipLiveRead(t, c).routes[prefix]
		if withB && bytes.Equal(route, reflectedB) {
			break
		}
		if !withB && !present {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	view := rrSettle(t, b, c, netip.MustParsePrefix("10.9.254.0/24"), attrsB, rrIDB)
	// RFC 4271 Section 9.2: "All newly installed routes and all newly
	// unfeasible routes for which there is no replacement route SHALL be
	// advertised to its peers by means of an UPDATE message."
	if withB {
		ownershipLiveRoute(t, view, prefix, reflectedB)
		ownershipLiveWithdrawals(t, view, prefix, before)
	} else {
		ownershipLiveAbsent(t, view, prefix)
		ownershipLiveWithdrawals(t, view, prefix, before+1)
	}
	ownershipLiveEstablished(t, []*lowLivePeer{b, c})
}

// TestRRAnnouncementElects proves AC-25 on bgp-rr's cached rail:
//
//   - "worse-later": non-clients A then B announce P, B's route worse; client
//     C (no ADD-PATH) keeps A's route and is never sent B's.
//   - "better-later": B then A; C ends on A's route.
//   - "non-client-stepped-past": client K1 announces P, non-client A the
//     best route, then client K2 a route better than K1's. At non-client D,
//     RFC 4456 forbids reflecting A's route, so the election steps past it:
//     D ends on K2's route and is never sent A's.
//
// VALIDATES: AC-25, a contested announcement at bgp-rr's destination is
// decided by the RIB's election under the RFC 4456 reflection rules.
// PREVENTS: forward-all overriding the reflector's choice, or a forbidden
// candidate being written to a non-client.
func TestRRAnnouncementElects(t *testing.T) {
	t.Run("worse-later", func(t *testing.T) { rrAnnouncementElects(t, false) })
	t.Run("better-later", func(t *testing.T) { rrAnnouncementElects(t, true) })
	t.Run("non-client-stepped-past", rrAnnouncementStepsPast)
}

func rrAnnouncementElects(t *testing.T, betterLater bool) {
	peers := rrLiveRouter(t, false,
		rrLivePeerSpec{"192.0.2.1", false}, rrLivePeerSpec{"192.0.2.2", false}, rrLivePeerSpec{"192.0.2.3", true})
	a, b, c := peers[0], peers[1], peers[2]
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	attrsA, attrsB := rrAttrs(65100), rrAttrs(65200, 65200)
	reflectedA, reflectedB := rrReflected(attrsA, rrIDA), rrReflected(attrsB, rrIDB)
	if betterLater {
		ownershipLiveSend(t, b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, c, prefix, reflectedB)
		ownershipLiveSend(t, a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, c, prefix, reflectedA)
	} else {
		ownershipLiveSend(t, a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, c, prefix, reflectedA)
		ownershipLiveSend(t, b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
	}
	view := rrSettle(t, b, c, netip.MustParsePrefix("10.9.255.0/24"), attrsB, rrIDB)
	// RFC 4456 Section 11: "the RR summarizes routing information and only
	// reflects its best path."
	ownershipLiveRoute(t, view, prefix, reflectedA)
	ownershipLiveWithdrawals(t, view, prefix, 0)
	if betterLater {
		ownershipLiveAnnouncements(t, view, prefix, reflectedB, reflectedA)
	} else {
		ownershipLiveAnnouncements(t, view, prefix, reflectedA)
	}
	ownershipLiveEstablished(t, peers)
}

func rrAnnouncementStepsPast(t *testing.T) {
	peers := rrLiveRouter(t, false,
		rrLivePeerSpec{"192.0.2.1", false}, rrLivePeerSpec{"192.0.2.6", true}, rrLivePeerSpec{"192.0.2.7", true},
		rrLivePeerSpec{"192.0.2.4", false})
	a, k1, k2, d := peers[0], peers[1], peers[2], peers[3]
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	attrsA, attrsK1, attrsK2 := rrAttrs(65100), rrAttrs(65400, 65400, 65400), rrAttrs(65500, 65500)
	reflectedK1, reflectedK2 := rrReflected(attrsK1, rrIDB), rrReflected(attrsK2, rrIDC)

	ownershipLiveSend(t, k1, nil, attrsK1, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, d, prefix, reflectedK1)
	ownershipLiveSend(t, a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
	// A's own marker, reflected to client K1, fences A's announcement of P
	// through the forward path.
	marker := netip.MustParsePrefix("10.9.2.0/24")
	ownershipLiveSend(t, a, nil, attrsA, marker) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, k1, marker, rrReflected(attrsA, rrIDA))
	ownershipLiveSend(t, k2, nil, attrsK2, prefix) // RFC 4271 Section 4.3.
	rrConverge(t, d, prefix, reflectedK2)
	view := rrSettle(t, k2, d, netip.MustParsePrefix("10.9.255.0/24"), attrsK2, rrIDC)
	// RFC 4456 Section 6: a route from a non-client peer is reflected "to
	// all the Clients", never to another non-client, so A's route is never
	// written to D.
	ownershipLiveRoute(t, view, prefix, reflectedK2)
	ownershipLiveWithdrawals(t, view, prefix, 0)
	ownershipLiveAnnouncements(t, view, prefix, reflectedK1, reflectedK2)
	ownershipLiveAbsent(t, view, marker)
	ownershipLiveEstablished(t, peers)
}

// TestRRPeerUpReplayElects proves AC-25's replay case (round 11, N-8): non-
// clients A and B announce P, A's route better, while client C is up. C's
// session drops and C reconnects; bgp-rr's replayForPeer relays both routes
// to C's new session on the forwarded rail. C's new session ends on A's
// reflected route, with no withdrawal of P, whichever order the relay used.
//
// VALIDATES: AC-25 replay, the relay of a second candidate at a client
// without ADD-PATH is decided by the election.
// PREVENTS: the replay order deciding which route a client holds.
func TestRRPeerUpReplayElects(t *testing.T) {
	peers := rrLiveRouter(t, false,
		rrLivePeerSpec{"192.0.2.1", false}, rrLivePeerSpec{"192.0.2.2", false}, rrLivePeerSpec{"192.0.2.3", true})
	a, b, c := peers[0], peers[1], peers[2]
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	attrsA, attrsB := rrAttrs(65100), rrAttrs(65200, 65200)
	reflectedA, reflectedB := rrReflected(attrsA, rrIDA), rrReflected(attrsB, rrIDB)
	ownershipLiveSend(t, b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefix, reflectedB)
	ownershipLiveSend(t, a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefix, reflectedA)

	_ = c.remote.Close() //nolint:errcheck // C's session ends; C reconnects below.
	again := rrLiveConnect(t, c.peer, false, rrIDC)
	rrConverge(t, again, prefix, reflectedA)
	view := rrSettle(t, b, again, netip.MustParsePrefix("10.9.255.0/24"), attrsB, rrIDB)
	ownershipLiveRoute(t, view, prefix, reflectedA)
	ownershipLiveWithdrawals(t, view, prefix, 0)
	for _, attrs := range view.announcements[prefix] {
		if !bytes.Equal(attrs, reflectedA) && !bytes.Equal(attrs, reflectedB) {
			t.Errorf("C's new session was sent %x for %s, neither A's nor B's route", attrs, prefix)
		}
	}
	ownershipLiveEstablished(t, []*lowLivePeer{a, b})
}
