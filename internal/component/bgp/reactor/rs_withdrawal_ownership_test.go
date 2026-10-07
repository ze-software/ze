// Design: docs/guide/route-reflection.md -- forward-all route-server withdrawal ownership.
// Related: rfc8654_rib_recovery_test.go -- registered plugins and real TCP peers.
// RFC 4271 Section 9 -- see rfc/short/rfc4271.md.
package reactor

import (
	"bytes"
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// TestRSReturnedWithdrawalPreservesOtherSource sends an AS-loop announcement
// through actual ingress, then its sender's attribute-free withdrawal. Ordered
// recipient TCP history must retain the original source until that source withdraws.
func TestRSReturnedWithdrawalPreservesOtherSource(t *testing.T) {
	peers := extendedRecoveryPeers(t, false, true)
	a, b, c := peers[0], peers[1], peers[2]
	p := netip.MustParsePrefix("203.0.114.0/24")
	q := netip.MustParsePrefix("203.0.115.0/24")
	initial := netip.MustParsePrefix("203.0.120.0/24")
	rejected := netip.MustParsePrefix("203.0.121.0/24")
	returned := netip.MustParsePrefix("203.0.122.0/24")
	legitimate := netip.MustParsePrefix("203.0.123.0/24")
	attrsA, attrsB := firstASAttrs(4, 65002), firstASAttrs(4, 65003) // RFC 6793 Section 3.

	ownershipLiveSend(t, a, nil, attrsA, p, q)    // RFC 4271 Section 4.3.
	ownershipLiveSend(t, a, nil, attrsA, initial) // RFC 4271 Section 4.3.
	view := ownershipLiveThrough(t, c, initial, attrsA)
	ownershipLiveRoute(t, view, p, attrsA)
	ownershipLiveRoute(t, view, q, attrsA)

	// The first AS matches B, so first-AS validation cannot substitute for the
	// real LoopIngress rejection of local AS 65000 (RFC 4271 Section 9.1.2).
	ownershipLiveSend(t, b, nil, firstASAttrs(4, 65003, 65000, 65002), p) // RFC 4271 Section 4.3; RFC 6793 Section 3.
	ownershipLiveSend(t, b, nil, attrsB, rejected)                        // RFC 4271 Section 4.3.
	view = ownershipLiveThrough(t, c, rejected, attrsB)
	ownershipLiveRoute(t, view, p, attrsA)
	ownershipLiveAnnouncements(t, view, p, attrsA)
	ownershipLiveWithdrawals(t, view, p, 0)

	// RFC 4271 Section 9: this new UPDATE has no attributes and did not
	// originate at A. Its source cannot remove A's advertisement at C.
	ownershipLiveSend(t, b, []netip.Prefix{p}, nil) // RFC 4271 Section 4.3.
	ownershipLiveSend(t, b, nil, attrsB, returned)  // RFC 4271 Section 4.3.
	view = ownershipLiveThrough(t, c, returned, attrsB)
	ownershipLiveRoute(t, view, p, attrsA)
	ownershipLiveRoute(t, view, q, attrsA)
	ownershipLiveAnnouncements(t, view, p, attrsA)
	ownershipLiveWithdrawals(t, view, p, 0)
	ownershipLiveWithdrawals(t, view, q, 0)

	// A later ordinary UPDATE from the actual owner MUST still withdraw P;
	// dropping all attribute-free withdrawals cannot satisfy this control.
	ownershipLiveSend(t, a, []netip.Prefix{p}, nil)  // RFC 4271 Section 4.3.
	ownershipLiveSend(t, a, nil, attrsA, legitimate) // RFC 4271 Section 4.3.
	view = ownershipLiveThrough(t, c, legitimate, attrsA)
	ownershipLiveAbsent(t, view, p)
	ownershipLiveWithdrawals(t, view, p, 1)
	ownershipLiveRoute(t, view, q, attrsA)
	ownershipLiveWithdrawals(t, view, q, 0)
	ownershipLiveEstablished(t, peers)
}

// TestRSWithdrawalKeepsCurrentDestinationOwner observes A then B at C before
// either source withdraws. Both distinct and identical attributes must transfer
// destination ownership; remembered source membership alone is insufficient.
func TestRSWithdrawalKeepsCurrentDestinationOwner(t *testing.T) {
	for _, identical := range []bool{false, true} {
		name := "distinct-attributes"
		if identical {
			name = "identical-attributes"
		}
		t.Run(name, func(t *testing.T) {
			var peers []*lowLivePeer
			if identical {
				peers = ownershipLiveSameASPeers(t) // RFC 4271 Section 4.2; RFC 6793 Section 3.
			} else {
				peers = extendedRecoveryPeers(t, false, true)
			}
			a, b, c := peers[0], peers[1], peers[2]
			p := netip.MustParsePrefix("203.0.114.0/24")
			first := netip.MustParsePrefix("203.0.120.0/24")
			second := netip.MustParsePrefix("203.0.121.0/24")
			older := netip.MustParsePrefix("203.0.122.0/24")
			current := netip.MustParsePrefix("203.0.123.0/24")
			attrsA := firstASAttrs(4, 65002) // RFC 6793 Section 3.
			attrsB := firstASAttrs(4, 65003) // RFC 6793 Section 3.
			if identical {
				attrsB = firstASAttrs(4, 65002) // RFC 6793 Section 3.
			}

			ownershipLiveSend(t, a, nil, attrsA, p)     // RFC 4271 Section 4.3.
			ownershipLiveSend(t, a, nil, attrsA, first) // RFC 4271 Section 4.3.
			view := ownershipLiveThrough(t, c, first, attrsA)
			ownershipLiveRoute(t, view, p, attrsA)
			ownershipLiveAnnouncements(t, view, p, attrsA)

			ownershipLiveSend(t, b, nil, attrsB, p)      // RFC 4271 Section 4.3.
			ownershipLiveSend(t, b, nil, attrsB, second) // RFC 4271 Section 4.3.
			view = ownershipLiveThrough(t, c, second, attrsB)
			ownershipLiveRoute(t, view, p, attrsB)
			// In the identical case, the second actual P announcement (not
			// merely its later marker) proves B reached the recipient.
			ownershipLiveAnnouncements(t, view, p, attrsA, attrsB)

			ownershipLiveSend(t, a, []netip.Prefix{p}, nil) // RFC 4271 Section 4.3.
			ownershipLiveSend(t, a, nil, attrsA, older)     // RFC 4271 Section 4.3.
			view = ownershipLiveThrough(t, c, older, attrsA)
			ownershipLiveRoute(t, view, p, attrsB)
			ownershipLiveWithdrawals(t, view, p, 0)
			ownershipLiveAnnouncements(t, view, p, attrsA, attrsB)

			ownershipLiveSend(t, b, []netip.Prefix{p}, nil) // RFC 4271 Section 4.3.
			ownershipLiveSend(t, b, nil, attrsB, current)   // RFC 4271 Section 4.3.
			view = ownershipLiveThrough(t, c, current, attrsB)
			ownershipLiveAbsent(t, view, p)
			ownershipLiveWithdrawals(t, view, p, 1)
			ownershipLiveEstablished(t, peers)
		})
	}
}

// TestRSMixedWithdrawalPreservesAnnouncementSibling decodes both sections of
// one real UPDATE: an unknown P withdrawal must not hide the owned Q withdrawal
// or the feasible R sibling, and must leave A's exact P advertisement intact.
func TestRSMixedWithdrawalPreservesAnnouncementSibling(t *testing.T) {
	peers := extendedRecoveryPeers(t, false, true)
	a, b, c := peers[0], peers[1], peers[2]
	p := netip.MustParsePrefix("203.0.114.0/24")
	q := netip.MustParsePrefix("203.0.115.0/24")
	r := netip.MustParsePrefix("203.0.116.0/24")
	first := netip.MustParsePrefix("203.0.120.0/24")
	second := netip.MustParsePrefix("203.0.121.0/24")
	mixed := netip.MustParsePrefix("203.0.122.0/24")
	attrsA, attrsB := firstASAttrs(4, 65002), firstASAttrs(4, 65003) // RFC 6793 Section 3.

	ownershipLiveSend(t, a, nil, attrsA, p)     // RFC 4271 Section 4.3.
	ownershipLiveSend(t, a, nil, attrsA, first) // RFC 4271 Section 4.3.
	view := ownershipLiveThrough(t, c, first, attrsA)
	ownershipLiveRoute(t, view, p, attrsA)
	ownershipLiveSend(t, b, nil, attrsB, q)      // RFC 4271 Section 4.3.
	ownershipLiveSend(t, b, nil, attrsB, second) // RFC 4271 Section 4.3.
	view = ownershipLiveThrough(t, c, second, attrsB)
	ownershipLiveRoute(t, view, p, attrsA)
	ownershipLiveRoute(t, view, q, attrsB)

	// RFC 4271 Section 9 processes withdrawn routes before feasible NLRI;
	// filtering an unowned withdrawal must preserve both other sections.
	ownershipLiveSend(t, b, []netip.Prefix{p, q}, attrsB, r) // RFC 4271 Section 4.3.
	ownershipLiveSend(t, b, nil, attrsB, mixed)              // RFC 4271 Section 4.3.
	view = ownershipLiveThrough(t, c, mixed, attrsB)
	ownershipLiveRoute(t, view, p, attrsA)
	ownershipLiveAnnouncements(t, view, p, attrsA)
	ownershipLiveWithdrawals(t, view, p, 0)
	ownershipLiveAbsent(t, view, q)
	ownershipLiveAnnouncements(t, view, q, attrsB)
	ownershipLiveWithdrawals(t, view, q, 1)
	ownershipLiveRoute(t, view, r, attrsB)
	ownershipLiveAnnouncements(t, view, r, attrsB)
	ownershipLiveWithdrawals(t, view, r, 0)
	ownershipLiveEstablished(t, peers)
}

// ownershipLiveProjection is an observer-only replay of at most 64 captured
// frames. Its maps are initialized by ownershipLiveRead; the zero value is empty
// and is never used to apply an UPDATE. It is not shared between goroutines.
type ownershipLiveProjection struct {
	routes        map[netip.Prefix][]byte
	announcements map[netip.Prefix][][]byte
	withdrawals   map[netip.Prefix]int
}

// ownershipLiveThrough uses the source's next valid announcement as a causal
// fence. Unlike a historical announcement lookup, each snapshot applies every
// preceding UPDATE in TCP order, including withdrawals and replacements.
func ownershipLiveThrough(t *testing.T, peer *lowLivePeer, marker netip.Prefix, attrs []byte) ownershipLiveProjection {
	t.Helper()
	var view ownershipLiveProjection
	lowEventually(t, func() bool {
		view = ownershipLiveRead(t, peer) // RFC 4271 Section 9.
		return bytes.Equal(view.routes[marker], attrs)
	}, "recipient TCP causal marker "+marker.String())
	return view
}

func ownershipLiveRead(t *testing.T, peer *lowLivePeer) ownershipLiveProjection {
	t.Helper()
	peer.mu.Lock()
	defer peer.mu.Unlock()
	view := ownershipLiveProjection{
		routes:        make(map[netip.Prefix][]byte),
		announcements: make(map[netip.Prefix][][]byte),
		withdrawals:   make(map[netip.Prefix]int),
	}
	for _, frame := range peer.frames {
		if frame[18] == byte(msgtype.TypeNOTIFICATION) {
			t.Fatalf("recipient received NOTIFICATION: %x", frame[message.HeaderLen:])
		}
		if frame[18] != byte(msgtype.TypeUPDATE) {
			continue
		}
		update, err := message.UnpackUpdate(frame[message.HeaderLen:]) // RFC 4271 Section 4.3.
		if err != nil {
			t.Fatalf("decode recipient UPDATE: %v", err)
		}
		// RFC 4271 Section 9: "If the UPDATE message contains a non-empty
		// WITHDRAWN ROUTES field, the previously advertised routes, whose
		// destinations (expressed as IP prefixes) are contained in this field,
		// SHALL be removed from the Adj-RIB-In."
		for _, prefix := range ownershipLivePrefixes(t, update.WithdrawnRoutes) { // RFC 4271 Section 4.3.
			delete(view.routes, prefix)
			view.withdrawals[prefix]++
		}
		// RFC 4271 Section 9: "If the UPDATE message contains a feasible route,
		// the Adj-RIB-In will be updated with this route as follows: if the
		// NLRI of the new route is identical to the one the route currently
		// has stored in the Adj-RIB-In, then the new route SHALL replace the
		// older route in the Adj-RIB-In, thus implicitly withdrawing the
		// older route from service."
		// "Otherwise, if the Adj-RIB-In has no route with NLRI identical to
		// the new route, the new route SHALL be placed in the Adj-RIB-In."
		for _, prefix := range ownershipLivePrefixes(t, update.NLRI) { // RFC 4271 Section 4.3.
			view.routes[prefix] = update.PathAttributes
			view.announcements[prefix] = append(view.announcements[prefix], update.PathAttributes)
		}
	}
	return view
}

func ownershipLiveRoute(t *testing.T, view ownershipLiveProjection, prefix netip.Prefix, attrs []byte) {
	t.Helper()
	got, present := view.routes[prefix]
	if !present {
		t.Errorf("recipient lost %s; want current attributes %x", prefix, attrs)
		return
	}
	if !bytes.Equal(got, attrs) {
		t.Errorf("recipient current %s attributes = %x, want %x", prefix, got, attrs)
	}
}

func ownershipLiveAbsent(t *testing.T, view ownershipLiveProjection, prefix netip.Prefix) {
	t.Helper()
	if attrs, present := view.routes[prefix]; present {
		t.Errorf("recipient retained withdrawn %s with attributes %x", prefix, attrs)
	}
}

func ownershipLiveWithdrawals(t *testing.T, view ownershipLiveProjection, prefix netip.Prefix, want int) {
	t.Helper()
	if got := view.withdrawals[prefix]; got != want {
		t.Errorf("recipient withdrawal history for %s = %d, want %d", prefix, got, want)
	}
}

func ownershipLiveAnnouncements(t *testing.T, view ownershipLiveProjection, prefix netip.Prefix, want ...[]byte) {
	t.Helper()
	got := view.announcements[prefix]
	if len(got) != len(want) {
		t.Errorf("recipient announcement history for %s = %x, want %x", prefix, got, want)
		return
	}
	for i, attrs := range want {
		if !bytes.Equal(got[i], attrs) {
			t.Errorf("recipient %s announcement %d attributes = %x, want %x", prefix, i, got[i], attrs)
		}
	}
}

func ownershipLiveEstablished(t *testing.T, peers []*lowLivePeer) {
	t.Helper()
	for _, peer := range peers {
		if got := peer.peer.SessionState(); got != fsm.StateEstablished {
			t.Errorf("peer %s session = %s, want Established", peer.peer.Settings().Address, got)
		}
	}
}

// ownershipLivePrefixes decodes RFC 4271 Section 4.3's bounded IPv4 NLRI
// tuples. This fixture negotiates IPv4 unicast only, without ADD-PATH.
// RFC 4271 Section 4.3: "The Length field indicates the length in bits of the
// IP address prefix." "The Prefix field contains an IP address prefix, followed
// by enough trailing bits to make the end of the field fall on an octet boundary."
//
//	Byte offset: | 0: Length (1 octet) | 1..: Prefix (ceil(Length/8) octets) |
func ownershipLivePrefixes(t *testing.T, field []byte) []netip.Prefix {
	t.Helper()
	var prefixes []netip.Prefix
	for len(field) > 0 {
		bits := int(field[0])
		if bits > 32 {
			t.Fatalf("recipient IPv4 prefix length = %d", bits)
		}
		octets := (bits + 7) / 8
		if len(field) < 1+octets {
			t.Fatalf("truncated recipient IPv4 NLRI: %x", field)
		}
		var address [4]byte
		copy(address[:], field[1:1+octets])
		prefixes = append(prefixes, netip.PrefixFrom(netip.AddrFrom4(address), bits).Masked())
		field = field[1+octets:]
	}
	return prefixes
}

// ownershipLiveSend encodes RFC 4271 Section 4.3's independent withdrawn and
// feasible IPv4 fields. Each invocation sends a new UPDATE on the source TCP
// connection; no cached message ID or ownership state is supplied by the test.
// RFC 4271 Section 4.3: "An UPDATE message MAY simultaneously advertise a
// feasible route and withdraw multiple unfeasible routes from service."
func ownershipLiveSend(t *testing.T, peer *lowLivePeer, withdrawn []netip.Prefix, attrs []byte, announced ...netip.Prefix) {
	t.Helper()
	update := &message.Update{
		WithdrawnRoutes: ownershipLiveNLRI(withdrawn), // RFC 4271 Section 4.3.
		PathAttributes:  attrs,
		NLRI:            ownershipLiveNLRI(announced), // RFC 4271 Section 4.3.
	}
	peer.send(t, message.PackTo(update, nil)) // RFC 4271 Section 4.3.
}

// ownershipLiveNLRI writes the fixture's IPv4 prefixes as bounded wire tuples.
// RFC 4271 Section 4.3: "The Length field indicates the length in bits of the
// IP address prefix." "The Prefix field contains an IP address prefix, followed
// by enough trailing bits to make the end of the field fall on an octet boundary."
//
//	Byte offset: | 0: Length (1 octet) | 1..: Prefix (ceil(Length/8) octets) |
func ownershipLiveNLRI(prefixes []netip.Prefix) []byte {
	field := make([]byte, 5*len(prefixes))
	offset := 0
	for _, prefix := range prefixes {
		address := prefix.Addr().As4()
		field[offset] = byte(prefix.Bits())
		offset++
		octets := (prefix.Bits() + 7) / 8
		offset += copy(field[offset:], address[:octets])
	}
	return field[:offset]
}

// ownershipLiveSameASPeers supplies the one variation extendedRecoveryPeers
// cannot express: two distinct TCP neighbors in the same AS. Their identical
// AS_PATHs then pass real first-AS validation without an ingress exception.
func ownershipLiveSameASPeers(t *testing.T) []*lowLivePeer {
	t.Helper()
	r := New(&Config{ListenAddr: "127.0.0.1:0"})
	settings := []*PeerSettings{
		lowLiveSettings("192.0.2.1", 65000, 65002),
		lowLiveSettings("192.0.2.2", 65000, 65002),
		lowLiveSettings("192.0.2.3", 65000, 65004),
	}
	for _, s := range settings {
		s.RSClient = true
		for _, binding := range []struct{ name, receive string }{
			{"bgp-rib", "update state refresh"},
			{"bgp-rs", "update-received state open-received refresh"},
			{"bgp-adj-rib-in", "update-received state"},
		} {
			if err := EnsureProcessBinding(s, binding.name, binding.receive, "update"); err != nil {
				t.Fatal(err)
			}
		}
		if err := r.AddPeer(s); err != nil {
			t.Fatal(err)
		}
	}
	srv := flowForwardPluginServer(t, r)
	r.SetPluginServer(srv)
	if err := r.StartWithContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopAndWait(t, r) })
	startBorrowedPeers(t, r, srv)
	peers := make([]*lowLivePeer, 0, len(settings))
	for _, s := range settings {
		peer := r.peers[s.PeerKey()]
		lowEventually(t, func() bool {
			if peer.currentSession() == nil {
				return false
			}
			return peer.SessionState() == fsm.StateActive
		}, "same-AS recovery peer active")
		listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			//nolint:errcheck // The listener is normally closed after Accept.
			listener.Close()
		})
		remote, err := (&net.Dialer{}).DialContext(t.Context(), "tcp4", listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			//nolint:errcheck // Covers an Accept failure before reader ownership.
			remote.Close()
		})
		server, err := listener.Accept()
		if err != nil {
			t.Fatal(err)
		}
		capture := &lowLivePeer{peer: peer, remote: remote, stopped: make(chan struct{})}
		go capture.readFrames()
		t.Cleanup(func() {
			// MUST close both endpoints before joining the owned reader.
			//nolint:errcheck // Cleanup may encounter an already closed endpoint.
			remote.Close()
			//nolint:errcheck // Reactor teardown may have closed this endpoint.
			server.Close()
			<-capture.stopped
		})
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
		if err := peer.acceptConnection(server); err != nil {
			t.Fatal(err)
		}
		// RFC 4271 Section 4.2: "This 2-octet unsigned integer indicates the
		// Autonomous System number of the sender."
		// RFC 6793 Section 3: "The capability that is used by a BGP speaker
		// to convey to its BGP peer the four-octet Autonomous System number
		// capability also carries the AS number (encoded as a four-octet
		// entity) of the speaker in the Capability Value field of the capability."
		// "The Capability Length field of the capability is set to 4."
		//
		// Optional parameter octets: | 0: Type=2 | 1: Length=6 |
		// | 2: Capability=65 | 3: Length=4 | 4..7: ASN |
		// | 8: Type=2 | 9: Length=6 | 10: Capability=1 | 11: Length=4 |
		// | 12..13: AFI=1 | 14: Reserved=0 | 15: SAFI=1 |
		open := &message.Open{
			Version: 4, MyAS: uint16(s.PeerAS), HoldTime: 90,
			BGPIdentifier:  0x0a000001 + uint32(len(peers)),
			OptionalParams: []byte{2, 6, 65, 4, 0, 0, byte(s.PeerAS >> 8), byte(s.PeerAS), 2, 6, 1, 4, 0, 1, 0, 1},
		}
		capture.send(t, message.PackTo(open, nil)) // RFC 4271 Section 4.2.
		lowEventually(t, func() bool {
			return peer.SessionState() == fsm.StateOpenConfirm
		}, "same-AS recovery OPEN accepted")
		capture.send(t, message.PackTo(message.NewKeepalive(), nil)) // RFC 4271 Section 4.4.
		lowEventually(t, func() bool {
			if peer.State() != PeerStateEstablished {
				return false
			}
			return !peer.pendingSync()
		}, "same-AS recovery peer initial sync")
		peers = append(peers, capture)
	}
	return peers
}
