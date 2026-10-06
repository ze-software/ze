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
	var recoveryMu sync.Mutex
	var recovered uint8
	var retained [2]locrib.ForwardHandle
	// Register before peer cleanup so shared UPDATE handles remain retained
	// until all producing handlers stop, not merely until one prefix dispatch.
	t.Cleanup(func() {
		recoveryMu.Lock()
		defer recoveryMu.Unlock()
		for _, handle := range retained {
			if handle != nil {
				handle.Release()
			}
		}
	})
	peers := extendedRecoveryPeers(t, true, false)
	source, recipient := peers[0], peers[1]
	session := source.peer.currentSession()
	update := fatalLengthAnnouncement()
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

	// Adj-RIB-In insertion and independent RS forwarding do not fence Loc-RIB
	// mirroring. Observe the recovered UPDATE at the actual publication boundary.
	unsubscribe := locrib.Default().OnChange(func(change locrib.Change) {
		if change.Family != family.IPv4Unicast ||
			(change.Kind != locrib.ChangeAdd && change.Kind != locrib.ChangeUpdate) {
			return
		}
		prefixes := fatalLengthPrefixes()
		i := slices.Index(prefixes[:], change.Prefix)
		if i < 0 {
			return
		}
		recoveryMu.Lock()
		defer recoveryMu.Unlock()
		if retained[i] != nil {
			return
		}
		wire, ok := change.Forward.(locrib.ForwardBytes)
		if !ok {
			t.Errorf("recovered Loc-RIB prefix %s has no UPDATE bytes", change.Prefix)
			return
		}
		change.Forward.AddRef()
		retained[i] = change.Forward
		// RFC 8654 Section 3; RFC 4271 Section 4.3.
		update, err := message.UnpackUpdate(wire.Bytes())
		if err != nil {
			t.Errorf("recovered Loc-RIB prefix %s: %v", change.Prefix, err)
			return
		}
		_, _, origin, found := attribute.AttrFind(update.PathAttributes, attribute.AttrOrigin)
		if !found || !bytes.Equal(origin, []byte{1}) {
			t.Errorf("recovered Loc-RIB prefix %s ORIGIN = %x, want 01", change.Prefix, origin)
			return
		}
		recovered |= 1 << i
	})
	t.Cleanup(unsubscribe)

	// A different valid ORIGIN is a causal post-error barrier, not a sleep or
	// a stale match against the original announcement.
	valid[message.HeaderLen+4+3] = 1
	source.send(t, valid)
	lowEventually(t, func() bool {
		attrs := lowInstalledAttributes([]byte{24, 203, 0, 114})
		_, _, origin, found := attribute.AttrFind(attrs, attribute.AttrOrigin)
		return found && bytes.Equal(origin, []byte{1})
	}, "valid extended announcement reinstalls routes after error")
	lowEventually(t, func() bool {
		return extendedRecoveryRecipientAttribute(t, recipient, 0, attribute.AttrOrigin, []byte{1}) == 3
	}, "both post-error routes reach the recipient")
	lowEventually(t, func() bool {
		recoveryMu.Lock()
		defer recoveryMu.Unlock()
		return recovered == 3
	}, "both recovered ORIGIN=1 routes published in Loc-RIB")
	_, routes := fatalLengthRIBSnapshot()
	require.Equal(t, uint8(3), routes)
	for _, prefix := range fatalLengthPrefixes() {
		_, found := locrib.Default().Lookup(family.IPv4Unicast, prefix)
		require.True(t, found)
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
// Only the source's local OPEN optionally advertises capability 6; remote OPENs
// omit it, so the extended test also exercises independent receive permission.
func extendedRecoveryPeers(t *testing.T, extended, alternate bool) []*lowLivePeer {
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
