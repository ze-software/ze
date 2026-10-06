// Design: docs/architecture/behavior/fsm-established.md -- fatal header errors release the established connection.
// Related: docs/architecture/behavior/peer-lifecycle.md -- peer-down drives the real RIB and downstream withdrawals.
// Related: flowspec_rs_wire_test.go -- real TCP, reactor and registered RIB/route-server plugin fixture.
// RFC 8654 Sections 5 and 6 -- see rfc/short/rfc8654.md.
// RFC 4271 Sections 6 and 6.1 -- see rfc/short/rfc4271.md.
package reactor

import (
	"bytes"
	"encoding/binary"
	"io"
	"net/netip"
	"testing"
	"time"

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

// TestRFC8654FatalLengthReleasesInstalledRoutes sends only an over-length header
// after two routes have entered the real Adj-RIB-In, Loc-RIB and recipient TCP.
// The header alone must trigger cleanup; waiting for 4078 body octets is a defect.
//
// RFC 8654 Section 5: "Similarly, any speaker that treats an improper BGP Extended
// Message as a fatal error MUST follow the error-handling procedures of [RFC4271]."
// RFC 4271 Section 6: "The phrase \"the BGP connection is closed\" means the TCP
// connection has been closed, the associated Adj-RIB-In has been cleared, and all
// resources for that BGP connection have been deallocated."
// RFC 4271 Section 6: "Before the invalid routes are deleted from the system, it
// advertises, to its peers, either withdraws for the routes marked as invalid, or
// the new best routes before the invalid routes are deleted from the system."
//
// This test observes completed cleanup, not queue-admission order. In production,
// RS handleStateDown transfers its compact withdrawal inventory to an owned
// asynchronous sender after draining source workers; a socket write need not
// precede the separate RIB plugin's Adj-RIB-In release. Neither this test nor a
// timestamp comparison between those plugins proves the lifetime of the last
// advertisement owner. The alternate-best test separately checks re-election.
//
// MUTATION: Bypass notifyPeerClosed or the RIB peer-down purge; the preinstalled
// routes or their downstream advertisements then survive the fatal Length error.
// RFC requirement: RFC8654-5-4 positive -- an actual Established peer without local capability 6 rejects UPDATE Length 4097 with exact NOTIFICATION 1/2 Data 1001 and EOF, clears its real Adj-RIB-In and Loc-RIB routes, withdraws both previously advertised routes on recipient TCP, and releases its session, timers and encoding contexts.
func TestRFC8654FatalLengthReleasesInstalledRoutes(t *testing.T) {
	// RFC 8654 Sections 5/6 and RFC 4271 Section 6: first prove installed state.
	peers := fatalLengthInstalledPeers(t)
	source, recipient := peers[0], peers[1]
	session := source.peer.currentSession()
	require.NotNil(t, session)
	events := &peerDownRecorder{established: make(chan *Peer, 4), closed: make(chan *Peer, 4)}
	source.peer.reactor.addPeerObserver(events)

	// RFC 8654 Section 6 leaves the non-advertiser's limit at 4096.
	// RFC 4271 Section 6.1: marker [0:16], Length [16:18], Type [18].
	header := append(bytes.Repeat([]byte{0xff}, message.MarkerLen), 0x10, 0x01, byte(msgtype.TypeUPDATE))
	source.send(t, header)

	// RFC 4271 Section 6: the peer loop, not a test-authored DOWN, releases state.
	requireEstablishedReleased(t, &mpLinkNeighbor{peer: source.peer, events: events}, session)
	lowEventually(t, func() bool { return session.State() == fsm.StateIdle }, "fatal session Idle")
	require.Nil(t, session.Conn(), "the failed session releases its connection")
	require.Zero(t, session.sendHoldDeadline.Load(), "the SendHoldTimer is released")
	require.Nil(t, source.peer.recvContext(), "receive encoding context released")
	require.Nil(t, source.peer.sendContext(), "send encoding context released")
	require.Zero(t, source.peer.recvContextID(), "receive context handle released")
	require.Zero(t, source.peer.sendContextID(), "send context handle released")
	require.Nil(t, source.peer.negotiated.Load(), "negotiated session capabilities released")

	// RFC 4271 Sections 6/6.1: exact NOTIFICATION bytes precede actual EOF.
	select {
	case <-source.stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("the fatal-length connection never closed")
	}
	require.NoError(t, source.remote.SetReadDeadline(time.Now().Add(time.Second)))
	var octet [1]byte
	n, err := source.remote.Read(octet[:])
	require.Zero(t, n)
	require.ErrorIs(t, err, io.EOF, "EOF, not a read deadline or connection reset")
	want := append(bytes.Repeat([]byte{0xff}, message.MarkerLen), 0, 23, 3, 1, 2, 0x10, 0x01)
	source.mu.Lock()
	var notifications [][]byte
	for _, frame := range source.frames {
		if frame[18] == byte(msgtype.TypeNOTIFICATION) {
			notifications = append(notifications, frame)
		}
	}
	source.mu.Unlock()
	require.Equal(t, [][]byte{want}, notifications, "one exact Bad Message Length NOTIFICATION")

	// RFC 4271 Section 6: observe real storage, never fake route deletion in an observer.
	lowEventually(t, func() bool {
		peerPresent, routes := fatalLengthRIBSnapshot()
		return !peerPresent && routes == 0
	}, "offending peer's Adj-RIB-In storage removed")
	loc := locrib.Default()
	for _, prefix := range fatalLengthPrefixes() {
		lowEventually(t, func() bool {
			_, installed := loc.Lookup(family.IPv4Unicast, prefix)
			return !installed
		}, "failed peer's route removed from Loc-RIB")
	}
	// RFC 4271 Section 6: both downstream advertisements must be withdrawn.
	lowEventually(t, func() bool {
		_, withdrawn := fatalLengthRecipientRoutes(t, recipient)
		return withdrawn == 3
	}, "both downstream withdrawals on recipient TCP")
	require.Equal(t, PeerStateEstablished, recipient.peer.State(), "the recipient session survives")
}

// TestRFC8654ValidLengthRetainsInstalledRoutes exercises the adjacent accepted
// length, then waits for its changed MED in real storage and on recipient TCP.
// That causal barrier replaces a sleep and makes rejection of every UPDATE fail.
//
// RFC 8654 Section 6: "For all messages except for OPEN and KEEPALIVE messages,
// if the receiver has advertised the BGP Extended Message Capability, this
// document raises that limit to 65,535."
// MUTATION: Apply fatal cleanup to Length 4096 as well as 4097; the changed MED
// never reaches the RIB/recipient and the established resources are lost.
// RFC requirement: RFC8654-5-4 negative -- a valid 4096-octet UPDATE without local capability 6 retains both installed Adj-RIB-In and Loc-RIB routes and the Established session, timers and encoding contexts, reaches recipient TCP with its changed MED, and causes neither NOTIFICATION nor withdrawal.
func TestRFC8654ValidLengthRetainsInstalledRoutes(t *testing.T) {
	// RFC 8654 Sections 5/6 and RFC 4271 Section 6: same installed starting state.
	peers := fatalLengthInstalledPeers(t)
	source, recipient := peers[0], peers[1]
	session := source.peer.currentSession()
	require.NotNil(t, session)
	recvContext, sendContext := source.peer.recvContext(), source.peer.sendContext()

	// RFC 4271 Section 4.3 and RFC 8654 Section 6: valid UPDATE at the old maximum.
	update := fatalLengthAnnouncement()
	update.PathAttributes = append(update.PathAttributes, 0x80, 4, 4, 0, 0, 0, 7)
	padding := 4096 - message.HeaderLen - 4 - len(update.NLRI) - len(update.PathAttributes) - 4
	// Unknown optional transitive attribute: flags, type, two-octet value length.
	update.PathAttributes = append(update.PathAttributes, 0xd0, 99, byte(padding>>8), byte(padding))
	update.PathAttributes = append(update.PathAttributes, bytes.Repeat([]byte{0x5a}, padding)...)
	wire := message.PackTo(update, nil)
	require.Len(t, wire, 4096)
	source.send(t, wire)

	// RFC 4271 Section 4.3: the changed MED proves this UPDATE was processed.
	lowEventually(t, func() bool {
		attrs := lowInstalledAttributes([]byte{24, 203, 0, 114})
		_, _, med, found := attribute.AttrFind(attrs, attribute.AttrMED)
		return found && bytes.Equal(med, []byte{0, 0, 0, 7})
	}, "4096-octet UPDATE installed by the RIB")
	lowEventually(t, func() bool { return fatalLengthRecipientMED(t, recipient) == 3 },
		"4096-octet UPDATE's MED received for both downstream routes")

	// RFC 4271 Section 6 does not tear down a valid message's connection or routes.
	peerPresent, routes := fatalLengthRIBSnapshot()
	require.True(t, peerPresent)
	require.Equal(t, uint8(3), routes)
	for _, prefix := range fatalLengthPrefixes() {
		_, installed := locrib.Default().Lookup(family.IPv4Unicast, prefix)
		require.True(t, installed, "valid UPDATE retains %s in Loc-RIB", prefix)
	}
	require.Equal(t, PeerStateEstablished, source.peer.State())
	require.Same(t, session, source.peer.currentSession())
	require.NotNil(t, session.Conn())
	require.True(t, session.timers.IsHoldTimerRunning())
	require.True(t, session.timers.IsKeepaliveTimerRunning())
	require.False(t, session.timers.IsConnectRetryTimerRunning())
	require.NotZero(t, session.sendHoldDeadline.Load())
	require.Same(t, recvContext, source.peer.recvContext())
	require.Same(t, sendContext, source.peer.sendContext())
	require.Equal(t, uint32(0), source.peer.ConnectRetryCounter())
	_, withdrawn := fatalLengthRecipientRoutes(t, recipient)
	require.Zero(t, withdrawn, "valid-length routes were never withdrawn")
	var notified bool
	source.mu.Lock()
	for _, frame := range source.frames {
		if frame[18] == byte(msgtype.TypeNOTIFICATION) {
			notified = true
		}
	}
	source.mu.Unlock()
	require.False(t, notified, "valid-length UPDATE causes no NOTIFICATION")
	select {
	case <-source.stopped:
		t.Fatal("the valid-length session closed")
	default:
	}
}

// fatalLengthInstalledPeers starts the real route-server fixture and installs two
// distinct routes through TCP. Snapshot callbacks only observe production storage.
func fatalLengthInstalledPeers(t *testing.T) []*lowLivePeer {
	t.Helper()
	// RFC 8654 Sections 5/6: this receiver has not advertised capability 6.
	peers := flowForwardLiveRouter(t, "unchanged")
	for _, cap := range peers[0].peer.Settings().Capabilities {
		_, extended := cap.(*capability.ExtendedMessage)
		require.False(t, extended, "fixture must not advertise Extended Message")
	}
	require.NotNil(t, peers[0].peer.recvContext())
	require.NotNil(t, peers[0].peer.sendContext())
	// RFC 4271 Section 4.3: received announcements seed the cleanup obligation.
	peers[0].send(t, message.PackTo(fatalLengthAnnouncement(), nil))
	lowEventually(t, func() bool {
		_, routes := fatalLengthRIBSnapshot()
		return routes == 3
	}, "both announcements installed in the actual Adj-RIB-In")
	loc := locrib.Default()
	require.NotNil(t, loc)
	for _, prefix := range fatalLengthPrefixes() {
		lowEventually(t, func() bool {
			_, installed := loc.Lookup(family.IPv4Unicast, prefix)
			return installed
		}, "received route installed in Loc-RIB")
	}
	// RFC 4271 Section 9.2: prove downstream reachability before testing withdrawal.
	lowEventually(t, func() bool {
		announced, _ := fatalLengthRecipientRoutes(t, peers[1])
		return announced == 3
	}, "both original advertisements on recipient TCP")
	return peers
}

// fatalLengthAnnouncement encodes two /24s with the source's four-octet AS path.
// RFC 4271 Section 4.3: "A variable-length sequence of path attributes is present
// in every UPDATE message, except for an UPDATE message that carries only the
// withdrawn routes.".
func fatalLengthAnnouncement() *message.Update {
	return &message.Update{
		PathAttributes: []byte{
			0x40, 1, 1, 0, // ORIGIN: IGP.
			0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xea, // AS_SEQUENCE: AS 65002.
			0x40, 3, 4, 192, 0, 2, 1, // NEXT_HOP: source neighbor.
		},
		NLRI: []byte{24, 203, 0, 114, 24, 203, 0, 115},
	}
}

func fatalLengthPrefixes() [2]netip.Prefix {
	return [2]netip.Prefix{netip.MustParsePrefix("203.0.114.0/24"), netip.MustParsePrefix("203.0.115.0/24")}
}

// fatalLengthRIBSnapshot reports peer storage and a bit per expected route.
func fatalLengthRIBSnapshot() (bool, uint8) {
	var peerPresent bool
	var routes uint8
	bgprib.RIBDumpBridge.DumpRIB(registry.RIBDumpVisitor{
		OnPeer: func(address string, _ uint32, _ [4]byte, _ bool) uint16 {
			if address == "192.0.2.1" {
				peerPresent = true
				return 1
			}
			return 0
		},
		OnRoute: func(peer, afi, safi uint16, bits uint8, nlri, _ []byte) {
			if peer != 1 {
				return
			}
			if afi != 1 {
				return
			}
			if safi != 1 {
				return
			}
			if bits != 24 {
				return
			}
			if bytes.Equal(nlri, []byte{203, 0, 114}) {
				routes |= 1
			}
			if bytes.Equal(nlri, []byte{203, 0, 115}) {
				routes |= 2
			}
		},
	})
	return peerPresent, routes
}

// fatalLengthRecipientRoutes distinguishes announcements from withdrawals in
// decoded UPDATE sections; the same bytes in an attribute cannot satisfy it.
func fatalLengthRecipientRoutes(t *testing.T, peer *lowLivePeer) (uint8, uint8) {
	t.Helper()
	peer.mu.Lock()
	defer peer.mu.Unlock()
	var announced, withdrawn uint8
	for _, frame := range peer.frames {
		if frame[18] != byte(msgtype.TypeUPDATE) {
			continue
		}
		// RFC 4271 Section 4.3: distinguish withdrawn routes from trailing NLRI.
		update, err := message.UnpackUpdate(frame[message.HeaderLen:])
		require.NoError(t, err)
		// RFC 4271 Section 4.3: decode each bounded NLRI tuple.
		announced |= fatalLengthNLRIMask(t, update.NLRI)
		withdrawn |= fatalLengthNLRIMask(t, update.WithdrawnRoutes)
	}
	return announced, withdrawn
}

func fatalLengthRecipientMED(t *testing.T, peer *lowLivePeer) uint8 {
	t.Helper()
	peer.mu.Lock()
	defer peer.mu.Unlock()
	var routes uint8
	for _, frame := range peer.frames {
		if frame[18] != byte(msgtype.TypeUPDATE) {
			continue
		}
		// RFC 4271 Sections 4.3/5.1.4: observe the accepted UPDATE's MED.
		update, err := message.UnpackUpdate(frame[message.HeaderLen:])
		require.NoError(t, err)
		_, _, med, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMED)
		if !found {
			continue
		}
		require.Len(t, med, 4)
		if binary.BigEndian.Uint32(med) == 7 {
			// RFC 4271 Section 4.3: decode each bounded NLRI tuple.
			routes |= fatalLengthNLRIMask(t, update.NLRI)
		}
	}
	return routes
}

// fatalLengthNLRIMask parses the bounded IPv4 NLRI list, not a substring search.
// RFC 4271 Section 4.3: "Reachability information is encoded as one or more
// 2-tuples of the form <length, prefix>, whose fields are described below:".
func fatalLengthNLRIMask(t *testing.T, nlri []byte) uint8 {
	t.Helper()
	var routes uint8
	for len(nlri) > 0 {
		require.LessOrEqual(t, nlri[0], byte(32))
		octets := (int(nlri[0]) + 7) / 8
		require.GreaterOrEqual(t, len(nlri), 1+octets)
		if bytes.Equal(nlri[:1+octets], []byte{24, 203, 0, 114}) {
			routes |= 1
		}
		if bytes.Equal(nlri[:1+octets], []byte{24, 203, 0, 115}) {
			routes |= 2
		}
		nlri = nlri[1+octets:]
	}
	return routes
}
