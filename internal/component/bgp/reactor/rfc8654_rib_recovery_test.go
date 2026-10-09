// Design: docs/architecture/edge-cases/extended-message.md -- extended UPDATE error handling.
// Related: rfc8654_fatal_length_cleanup_test.go -- fatal cleanup controls.
package reactor

import (
	"bytes"
	"context"
	"net"
	"net/netip"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgprib "github.com/ze-software/ze/internal/component/bgp/plugins/rib"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// TestRFC8654TreatAsWithdrawRemovesInstalledRoutes sends valid and malformed
// extended announcements through TCP and observes registered RIB storage, the
// Loc-RIB and recipient TCP. No observer changes the routing state.
// MUTATION: Suppress the RFC7606ActionTreatAsWithdraw dispatch in processMessage:
// the installed routes remain and the actual storage-removal assertion fails.
// RFC requirement: RFC8654-3-1 positive -- an otherwise valid 4097-octet UPDATE installs both routes, and a later valid extended UPDATE reinstalls them on the same live session.
// RFC requirement: RFC8654-3-1 negative -- changing only ORIGIN to an invalid value in that extended UPDATE removes both already installed routes from actual Adj-RIB-In and Loc-RIB storage and withdraws them downstream without resetting the session.
func TestRFC8654TreatAsWithdrawRemovesInstalledRoutes(t *testing.T) {
	const recoveredMED = 17
	var recoveryMu sync.Mutex
	var recovered uint8
	peers := extendedRecoveryPeers(t, true, false)
	source, recipient := peers[0], peers[1]
	session := source.peer.currentSession()
	update := fatalLengthAnnouncement()
	// RFC 4271 Section 5.1.4: "The value of the MULTI_EXIT_DISC attribute is a
	// four-octet unsigned number, called a metric."
	// The RIB mirrors MED as the Loc-RIB path's Metric.
	update.PathAttributes = append(update.PathAttributes, 0x80, 4, 4, 0, 0, 0, 0)
	medLastOctet := message.HeaderLen + 4 + len(update.PathAttributes) - 1
	padding := 4097 - message.HeaderLen - 4 - len(update.NLRI) - len(update.PathAttributes) - 4
	update.PathAttributes = append(update.PathAttributes, 0xd0, 99, byte(padding>>8), byte(padding))
	update.PathAttributes = append(update.PathAttributes, bytes.Repeat([]byte{0x5a}, padding)...)
	valid := message.PackTo(update, nil)
	require.Len(t, valid, 4097)
	source.send(t, valid)
	lowEventually(t, func() bool {
		_, routes := fatalLengthRIBSnapshot()
		return routes == 3
	}, "extended routes installed in actual Adj-RIB-In")
	for _, prefix := range fatalLengthPrefixes() {
		lowEventually(t, func() bool {
			_, found := locrib.Default().Lookup(family.IPv4Unicast, prefix)
			return found
		}, "extended route installed in Loc-RIB")
	}
	lowEventually(t, func() bool {
		announced, _ := fatalLengthRecipientRoutes(t, recipient)
		return announced == 3
	}, "extended routes announced downstream")

	// RFC 7606 Section 7.1: ORIGIN value 3 alone is invalid. Framing, AS_PATH,
	// NEXT_HOP and the optional filler are identical to the installed control.
	malformed := bytes.Clone(valid)
	malformed[message.HeaderLen+4+3] = 3
	source.send(t, malformed)
	lowEventually(t, func() bool {
		present, routes := fatalLengthRIBSnapshot()
		return present && routes == 0
	}, "treat-as-withdraw removes routes but retains the live peer's storage")
	for _, prefix := range fatalLengthPrefixes() {
		lowEventually(t, func() bool {
			_, found := locrib.Default().Lookup(family.IPv4Unicast, prefix)
			return !found
		}, "treat-as-withdraw removes actual Loc-RIB route")
	}
	lowEventually(t, func() bool {
		_, withdrawn := fatalLengthRecipientRoutes(t, recipient)
		return withdrawn == 3
	}, "treat-as-withdraw reaches recipient TCP")

	// An AIGP reselection can publish an inserted route before the receive
	// handler reaches its per-prefix election. That publication legitimately
	// has no ForwardBytes; the later identical Path insert emits no event.
	// Observe the recovered MED at the selected-path publication boundary,
	// and check the actual ORIGIN and MED separately in storage and on TCP.
	unsubscribe := locrib.Default().OnChange(func(change locrib.Change) {
		if change.Family != family.IPv4Unicast {
			return
		}
		if change.Kind != locrib.ChangeAdd && change.Kind != locrib.ChangeUpdate {
			return
		}
		prefixes := fatalLengthPrefixes()
		i := slices.Index(prefixes[:], change.Prefix)
		if i < 0 {
			return
		}
		if change.Best.Metric != recoveredMED {
			t.Errorf("recovered Loc-RIB prefix %s metric = %d, want %d",
				change.Prefix, change.Best.Metric, recoveredMED)
			return
		}
		recoveryMu.Lock()
		defer recoveryMu.Unlock()
		recovered |= 1 << i
	})
	t.Cleanup(unsubscribe)

	// Changed ORIGIN and MED identify this post-error UPDATE. MED also names
	// its selected Loc-RIB path, which does not store ORIGIN.
	valid[message.HeaderLen+4+3] = 1
	valid[medLastOctet] = recoveredMED
	source.send(t, valid)
	for _, prefix := range fatalLengthPrefixes() {
		address := prefix.Addr().As4()
		lowEventually(t, func() bool {
			attrs := lowInstalledAttributes([]byte{24, address[0], address[1], address[2]})
			_, _, origin, found := attribute.AttrFind(attrs, attribute.AttrOrigin)
			if !found {
				return false
			}
			if !bytes.Equal(origin, []byte{1}) {
				return false
			}
			_, _, med, found := attribute.AttrFind(attrs, attribute.AttrMED)
			return found && bytes.Equal(med, []byte{0, 0, 0, recoveredMED})
		}, "each recovered route has ORIGIN=1 and MED=17 in actual Adj-RIB-In")
	}
	lowEventually(t, func() bool {
		return extendedRecoveryRecipientAttribute(t, recipient, 0, attribute.AttrOrigin, []byte{1}) == 3
	}, "both post-error routes reach the recipient")
	lowEventually(t, func() bool {
		return extendedRecoveryRecipientAttribute(t, recipient, 0, attribute.AttrMED, []byte{0, 0, 0, recoveredMED}) == 3
	}, "both post-error routes carry MED=17 to the recipient")
	lowEventually(t, func() bool {
		recoveryMu.Lock()
		defer recoveryMu.Unlock()
		return recovered == 3
	}, "both recovered MED=17 paths published in Loc-RIB")
	_, routes := fatalLengthRIBSnapshot()
	require.Equal(t, uint8(3), routes)
	for _, prefix := range fatalLengthPrefixes() {
		best, found := locrib.Default().Best(family.IPv4Unicast, prefix)
		require.True(t, found)
		require.Equal(t, uint32(recoveredMED), best.Metric, "the installed path must be the recovered route")
	}
	require.Same(t, session, source.peer.currentSession())
	require.Equal(t, fsm.StateEstablished, session.State())
	source.mu.Lock()
	defer source.mu.Unlock()
	for _, frame := range source.frames {
		require.NotEqual(t, byte(msgtype.TypeNOTIFICATION), frame[18])
	}
}

// TestRFC8654FatalLengthReelectsAlternateBest leaves one prefix with an alternate
// path and one without. The production election must replace the former in one
// Loc-RIB change and advertise its own AS_PATH/NEXT_HOP, while withdrawing only
// the latter. Actual TCP delivery is not required to precede unrelated storage
// cleanup; the synchronous Loc-RIB consumer sees the selection transition.
// MUTATION: Skip re-election in emitPurgedWithdraws and withdraw every purged
// route: the replacement attribute and ChangeUpdate assertions fail.
// RFC requirement: RFC8654-5-4 positive -- fatal extended-length rejection re-elects an installed alternate, emits no transient Loc-RIB removal or downstream withdrawal for it, and advertises the alternate's exact AS_PATH and NEXT_HOP while withdrawing the route with no survivor.
func TestRFC8654FatalLengthReelectsAlternateBest(t *testing.T) {
	peers := extendedRecoveryPeers(t, false, true)
	source, recipient, alternate := peers[0], peers[1], peers[2]
	source.send(t, message.PackTo(fatalLengthAnnouncement(), nil))
	lowEventually(t, func() bool {
		announced, _ := fatalLengthRecipientRoutes(t, recipient)
		return announced == 3
	}, "original best advertised before alternate")
	alternatePath := []byte{2, 2, 0, 0, 0xfd, 0xec, 0, 0, 0xfd, 0xec}
	backup := &message.Update{
		PathAttributes: []byte{0x40, 1, 1, 0, 0x40, 2, 10},
		NLRI:           []byte{24, 203, 0, 114},
	}
	backup.PathAttributes = append(backup.PathAttributes, alternatePath...)
	backup.PathAttributes = append(backup.PathAttributes, 0x40, 3, 4, 192, 0, 2, 3)
	alternate.send(t, message.PackTo(backup, nil))
	lowEventually(t, func() bool {
		return bytes.Equal(extendedRecoveryStoredAttributes("192.0.2.3", []byte{203, 0, 114}), backup.PathAttributes)
	}, "alternate path installed in actual Adj-RIB-In")
	lowEventually(t, func() bool {
		return extendedRecoveryRecipientAttribute(t, recipient, 0, attribute.AttrASPath, alternatePath) == 1
	}, "alternate's initial advertisement drained before the fatal event")
	recipient.mu.Lock()
	beforeFatal := len(recipient.frames)
	recipient.mu.Unlock()
	prefix := fatalLengthPrefixes()[0]
	loc := locrib.Default()
	best, found := loc.Best(family.IPv4Unicast, prefix)
	require.True(t, found)
	require.Equal(t, netip.MustParseAddr("192.0.2.1"), best.NextHop)

	var mu sync.Mutex
	var changes []locrib.ChangeKind
	unsubscribe := loc.OnChange(func(change locrib.Change) {
		if change.Family == family.IPv4Unicast && change.Prefix == prefix {
			mu.Lock()
			changes = append(changes, change.Kind)
			mu.Unlock()
		}
	})
	t.Cleanup(unsubscribe)
	session := source.peer.currentSession()
	events := &peerDownRecorder{established: make(chan *Peer, 4), closed: make(chan *Peer, 4)}
	source.peer.reactor.addPeerObserver(events)
	header := append(bytes.Repeat([]byte{0xff}, message.MarkerLen), 0x10, 0x01, byte(msgtype.TypeUPDATE))
	source.send(t, header)
	requireEstablishedReleased(t, &mpLinkNeighbor{peer: source.peer, events: events}, session)
	lowEventually(t, func() bool {
		present, routes := fatalLengthRIBSnapshot()
		return !present && routes == 0
	}, "failed peer storage removed with alternate retained")
	lowEventually(t, func() bool {
		path, installed := loc.Best(family.IPv4Unicast, prefix)
		return installed && path.NextHop == netip.MustParseAddr("192.0.2.3")
	}, "alternate becomes the actual Loc-RIB best")
	lowEventually(t, func() bool {
		_, installed := loc.Lookup(family.IPv4Unicast, fatalLengthPrefixes()[1])
		return !installed
	}, "route without alternate removed")
	lowEventually(t, func() bool {
		return extendedRecoveryRecipientAttribute(t, recipient, beforeFatal, attribute.AttrASPath, alternatePath) == 1 &&
			extendedRecoveryRecipientAttribute(t, recipient, beforeFatal, attribute.AttrNextHop, []byte{192, 0, 2, 3}) == 1
	}, "replacement best's own attributes received on TCP")
	lowEventually(t, func() bool {
		_, withdrawn := fatalLengthRecipientRoutes(t, recipient)
		return withdrawn&2 != 0
	}, "route without alternate withdrawn downstream")
	_, withdrawn := fatalLengthRecipientRoutes(t, recipient)
	require.Equal(t, uint8(2), withdrawn, "the surviving prefix must never be withdrawn")
	mu.Lock()
	gotChanges := append([]locrib.ChangeKind(nil), changes...)
	mu.Unlock()
	require.Equal(t, []locrib.ChangeKind{locrib.ChangeUpdate}, gotChanges, "one replacement, never Remove then Add")
	require.Equal(t, backup.PathAttributes, extendedRecoveryStoredAttributes("192.0.2.3", []byte{203, 0, 114}))
	require.Equal(t, PeerStateEstablished, alternate.peer.State())
	require.Equal(t, PeerStateEstablished, recipient.peer.State())
}

// extendedRecoveryPeers uses the registered RIB/route-server and actual TCP.
// Only the source's local OPEN optionally advertises capability 6. Remote OPENs
// omit it unless extra MP families request an extended-message recovery recipient.
func extendedRecoveryPeers(t *testing.T, extended, alternate bool, families ...capability.Family) []*lowLivePeer {
	t.Helper()
	r := New(&Config{ListenAddr: "127.0.0.1:0"})
	settings := []*PeerSettings{
		lowLiveSettings("192.0.2.1", 65000, 65002),
		lowLiveSettings("192.0.2.2", 65000, 65003),
	}
	if extended {
		settings[0].Capabilities = append(settings[0].Capabilities, &capability.ExtendedMessage{})
	}
	if alternate {
		settings = append(settings, lowLiveSettings("192.0.2.3", 65000, 65004))
	}
	for _, s := range settings {
		for _, fam := range families {
			s.Capabilities = append(s.Capabilities, &capability.Multiprotocol{AFI: fam.AFI, SAFI: fam.SAFI})
		}
		s.RSClient = true
		for _, binding := range []struct{ name, receive string }{
			{"bgp-rib", "update state refresh"},
			{"bgp-rs", "update-received state open-received refresh"},
			{"bgp-adj-rib-in", "update-received state"},
		} {
			require.NoError(t, EnsureProcessBinding(s, binding.name, binding.receive, "update"))
		}
		require.NoError(t, r.AddPeer(s))
	}
	srv := flowForwardPluginServer(t, r)
	r.SetPluginServer(srv)
	require.NoError(t, r.StartWithContext(context.Background()))
	t.Cleanup(func() { stopAndWait(t, r) })
	startBorrowedPeers(t, r, srv)
	peers := make([]*lowLivePeer, 0, len(settings))
	for _, s := range settings {
		peer := r.peers[s.PeerKey()]
		lowEventually(t, func() bool {
			return peer.currentSession() != nil && peer.SessionState() == fsm.StateActive
		}, "recovery peer active")
		listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp4", "127.0.0.1:0")
		require.NoError(t, err)
		t.Cleanup(func() { _ = listener.Close() })
		remote, err := (&net.Dialer{}).DialContext(t.Context(), "tcp4", listener.Addr().String())
		require.NoError(t, err)
		t.Cleanup(func() { _ = remote.Close() })
		server, err := listener.Accept()
		require.NoError(t, err)
		require.NoError(t, listener.Close())
		capture := &lowLivePeer{peer: peer, remote: remote, stopped: make(chan struct{})}
		go capture.readFrames()
		t.Cleanup(func() {
			// MUST close both endpoints before joining the owned reader.
			_ = remote.Close()
			_ = server.Close()
			<-capture.stopped
		})
		require.NoError(t, peer.acceptConnection(server))
		open := &message.Open{Version: 4, MyAS: uint16(s.PeerAS), HoldTime: 90,
			BGPIdentifier:  0x0a000001 + uint32(len(peers)),
			OptionalParams: []byte{2, 6, 65, 4, 0, 0, byte(s.PeerAS >> 8), byte(s.PeerAS), 2, 6, 1, 4, 0, 1, 0, 1}}
		for _, fam := range families {
			open.OptionalParams = append(open.OptionalParams, 2, 6, 1, 4,
				byte(fam.AFI>>8), byte(fam.AFI), 0, byte(fam.SAFI))
		}
		if len(families) > 0 {
			// The MP recovery recipient must accept the valid extended control.
			open.OptionalParams = append(open.OptionalParams, 2, 2, 6, 0)
		}
		capture.send(t, message.PackTo(open, nil))
		lowEventually(t, func() bool { return peer.SessionState() == fsm.StateOpenConfirm }, "recovery OPEN accepted")
		capture.send(t, message.PackTo(message.NewKeepalive(), nil))
		lowEventually(t, func() bool {
			return peer.State() == PeerStateEstablished && !peer.pendingSync()
		}, "recovery peer initial sync")
		peers = append(peers, capture)
	}
	return peers
}

func extendedRecoveryStoredAttributes(address string, prefix []byte) []byte {
	var attrs []byte
	bgprib.RIBDumpBridge.DumpRIB(registry.RIBDumpVisitor{
		OnPeer: func(peer string, _ uint32, _ [4]byte, _ bool) uint16 {
			if peer == address {
				return 1
			}
			return 0
		},
		OnRoute: func(peer, afi, safi uint16, bits uint8, nlri, attributes []byte) {
			if peer == 1 && afi == 1 && safi == 1 && bits == 24 && bytes.Equal(nlri, prefix) {
				attrs = bytes.Clone(attributes)
			}
		},
	})
	return attrs
}

func extendedRecoveryRecipientAttribute(t *testing.T, peer *lowLivePeer, start int, code attribute.AttributeCode, value []byte) uint8 {
	t.Helper()
	peer.mu.Lock()
	defer peer.mu.Unlock()
	var routes uint8
	for _, frame := range peer.frames[start:] {
		if frame[18] != byte(msgtype.TypeUPDATE) {
			continue
		}
		update, err := message.UnpackUpdate(frame[message.HeaderLen:])
		require.NoError(t, err)
		_, _, got, found := attribute.AttrFind(update.PathAttributes, code)
		if found && bytes.Equal(got, value) {
			routes |= fatalLengthNLRIMask(t, update.NLRI)
		}
	}
	return routes
}

// TestRFC8654ExtendedTwoFamilyWithdrawalRecovery sends the same two-MP-family
// extended UPDATE with valid and invalid ORIGIN through actual storage and TCP.
// RFC 7606 Section 2: "In this approach, the UPDATE message containing the path
// attribute in question MUST be treated as though all contained routes had been
// withdrawn just as if they had been listed in the WITHDRAWN ROUTES field (or in
// the MP_UNREACH_NLRI attribute if appropriate) of the UPDATE message, thus
// causing them to be removed from the Adj-RIB-In according to the procedures of
// [RFC4271]."
// MUTATION: Skip processMessage's bodies[1:] callback: the previously installed
// IPv4 route remains in the RIB and its recipient withdrawal never arrives.
// MUTATION: Dispatch the original MP_REACH instead of its synthesized withdrawal:
// the IPv6 route remains; resetting the session instead also removes the survivor.
// RFC requirement: RFC8654-3-1 positive -- a valid extended mixed-family UPDATE preserves its IPv6 announcement and delivers its IPv4 withdrawal; both families recover with new ORIGIN on the identical live session after the error.
// RFC requirement: RFC8654-3-1 negative -- changing only ORIGIN withdraws both contained MP families from actual RIB and Loc-RIB and recipient TCP, leaving an unrelated IPv6 route intact without NOTIFICATION.
func TestRFC8654ExtendedTwoFamilyWithdrawalRecovery(t *testing.T) {
	peers := extendedRecoveryPeers(t, true, false,
		capability.Family{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast})
	source, recipient := peers[0], peers[1]
	session := source.peer.currentSession()
	routes := []struct {
		fam    family.Family
		raw    []byte
		hop    []byte
		prefix netip.Prefix
	}{
		{family.IPv4Unicast, []byte{24, 203, 0, 114}, []byte{192, 0, 2, 1},
			netip.MustParsePrefix("203.0.114.0/24")},
		{family.IPv6Unicast, []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 2},
			netip.MustParseAddr("2001:db8::1").AsSlice(), netip.MustParsePrefix("2001:db8:2::/48")},
		{family.IPv6Unicast, []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 3},
			netip.MustParseAddr("2001:db8::1").AsSlice(), netip.MustParsePrefix("2001:db8:3::/48")},
	}
	announcements := make([][]byte, len(routes))
	for i, route := range routes {
		var mp [64]byte
		n := writeMPReach(mp[:], 0, route.fam, route.hop, route.raw)
		attrs := append(fatalLengthAnnouncement().PathAttributes, mp[:n]...)
		announcements[i] = receivedUpdateBody(attrs, nil)
		source.send(t, buildUpdateMsg(announcements[i]))
	}
	for _, route := range routes {
		lowEventually(t, func() bool {
			return extendedRecoveryHasMPRoute(route.fam, route.raw, 0)
		}, "initial route reaches actual source RIB")
		lowEventually(t, func() bool {
			_, found := locrib.Default().Lookup(route.fam, route.prefix)
			return found
		}, "initial route reaches Loc-RIB")
		lowEventually(t, func() bool {
			return extendedRecoveryRecipientMP(t, recipient, 0, route.fam, route.raw, true, 0)
		}, "initial MP route reaches recipient")
	}

	// The valid control already carries the IPv4 withdrawal. Reinstall that
	// route before the ORIGIN-only mutation so suppressing the extra callback
	// cannot pass by observing a route that was absent before the error.
	control, err := message.UnpackUpdate(announcements[1])
	require.NoError(t, err)
	control.PathAttributes = append(bytes.Clone(control.PathAttributes),
		0x80, 15, 7, 0, 1, 1, 24, 203, 0, 114)
	control.PathAttributes = append(control.PathAttributes, 0xd0, 99, 0x10, 0)
	control.PathAttributes = append(control.PathAttributes, bytes.Repeat([]byte{0x5a}, 4096)...)
	valid := message.PackTo(control, nil)
	require.Greater(t, len(valid), message.MaxMsgLen)
	source.send(t, valid)
	lowEventually(t, func() bool {
		return !extendedRecoveryHasMPRoute(routes[0].fam, routes[0].raw, -1)
	}, "valid control applies its explicit IPv4 withdrawal")
	lowEventually(t, func() bool {
		return extendedRecoveryRecipientMP(t, recipient, 0, routes[0].fam, routes[0].raw, false, 0)
	}, "valid control forwards explicit withdrawal")
	require.True(t, extendedRecoveryHasMPRoute(routes[1].fam, routes[1].raw, 0))
	announcements[0][7] = 1
	source.send(t, buildUpdateMsg(announcements[0]))
	lowEventually(t, func() bool {
		return extendedRecoveryHasMPRoute(routes[0].fam, routes[0].raw, 1)
	}, "IPv4 route reinstalled before malformed input")
	lowEventually(t, func() bool {
		return extendedRecoveryRecipientMP(t, recipient, 0, routes[0].fam, routes[0].raw, true, 1)
	}, "IPv4 reinstall reaches recipient before malformed input")
	recipient.mu.Lock()
	start := len(recipient.frames)
	recipient.mu.Unlock()
	malformed := bytes.Clone(valid)
	malformed[message.HeaderLen+7] = 3
	source.send(t, malformed)
	for _, route := range routes[:2] {
		lowEventually(t, func() bool {
			return !extendedRecoveryHasMPRoute(route.fam, route.raw, -1)
		}, "both primary and extra families removed from source RIB")
		lowEventually(t, func() bool {
			_, found := locrib.Default().Lookup(route.fam, route.prefix)
			return !found
		}, "both families removed from Loc-RIB")
		lowEventually(t, func() bool {
			return extendedRecoveryRecipientMP(t, recipient, start, route.fam, route.raw, false, 0)
		}, "both family withdrawals reach actual recipient TCP")
	}
	require.True(t, extendedRecoveryHasMPRoute(routes[2].fam, routes[2].raw, 0))
	_, found := locrib.Default().Lookup(routes[2].fam, routes[2].prefix)
	require.True(t, found, "unrelated route survives treat-as-withdraw")

	for i, route := range routes[:2] {
		announcements[i][7] = 2
		source.send(t, buildUpdateMsg(announcements[i]))
		lowEventually(t, func() bool {
			return extendedRecoveryHasMPRoute(route.fam, route.raw, 2)
		}, "post-error ORIGIN reaches source RIB")
		lowEventually(t, func() bool {
			_, present := locrib.Default().Lookup(route.fam, route.prefix)
			return present
		}, "post-error route reaches Loc-RIB")
		lowEventually(t, func() bool {
			return extendedRecoveryRecipientMP(t, recipient, start, route.fam, route.raw, true, 2)
		}, "post-error ORIGIN reaches recipient on same session")
	}
	require.False(t, extendedRecoveryRecipientMP(t, recipient, start,
		routes[2].fam, routes[2].raw, false, 0), "surviving route never withdrawn")
	require.Same(t, session, source.peer.currentSession())
	require.Equal(t, fsm.StateEstablished, session.State())
	source.mu.Lock()
	defer source.mu.Unlock()
	for _, frame := range source.frames {
		require.NotEqual(t, byte(msgtype.TypeNOTIFICATION), frame[18])
	}
}

// extendedRecoveryHasMPRoute reads registered source storage, not a test mirror.
// Origin -1 checks presence regardless of attributes, for absence assertions.
func extendedRecoveryHasMPRoute(fam family.Family, raw []byte, origin int) bool {
	found := false
	bgprib.RIBDumpBridge.DumpRIB(registry.RIBDumpVisitor{
		OnPeer: func(peer string, _ uint32, _ [4]byte, _ bool) uint16 {
			if peer == "192.0.2.1" {
				return 1
			}
			return 0
		},
		OnRoute: func(peer, afi, safi uint16, bits uint8, nlri, attrs []byte) {
			if peer == 1 && afi == uint16(fam.AFI) && safi == uint16(fam.SAFI) &&
				bits == raw[0] && bytes.Equal(nlri, raw[1:]) {
				_, _, value, present := attribute.AttrFind(attrs, attribute.AttrOrigin)
				found = origin == -1 || present && bytes.Equal(value, []byte{byte(origin)})
			}
		},
	})
	return found
}

// extendedRecoveryRecipientMP accepts legacy IPv4 or MP encoding of the same
// route, but checks complete NLRI records rather than a byte substring.
func extendedRecoveryRecipientMP(t *testing.T, peer *lowLivePeer, start int,
	fam family.Family, raw []byte, announce bool, origin byte) bool {
	t.Helper()
	peer.mu.Lock()
	defer peer.mu.Unlock()
	for _, frame := range peer.frames[start:] {
		if frame[18] != byte(msgtype.TypeUPDATE) {
			continue
		}
		update, err := message.UnpackUpdate(frame[message.HeaderLen:])
		require.NoError(t, err)
		code := attribute.AttrMPUnreachNLRI
		nlri := update.WithdrawnRoutes
		if announce {
			code = attribute.AttrMPReachNLRI
			nlri = update.NLRI
			_, _, value, present := attribute.AttrFind(update.PathAttributes, attribute.AttrOrigin)
			if !present || !bytes.Equal(value, []byte{origin}) {
				continue
			}
		}
		_, _, mp, present := attribute.AttrFind(update.PathAttributes, code)
		if present {
			require.GreaterOrEqual(t, len(mp), 3)
			if uint16(mp[0])<<8|uint16(mp[1]) != uint16(fam.AFI) || mp[2] != byte(fam.SAFI) {
				continue
			}
			offset := 3
			if announce {
				require.GreaterOrEqual(t, len(mp), 5)
				offset = 5 + int(mp[3])
				require.GreaterOrEqual(t, len(mp), offset)
			}
			nlri = mp[offset:]
		} else if fam != family.IPv4Unicast {
			continue
		}
		for len(nlri) > 0 {
			octets := 1 + (int(nlri[0])+7)/8
			require.GreaterOrEqual(t, len(nlri), octets)
			if bytes.Equal(nlri[:octets], raw) {
				return true
			}
			nlri = nlri[octets:]
		}
	}
	return false
}
