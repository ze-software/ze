// Design: docs/architecture/update-building.md -- RFC 7313 Section 4 Graceful Restart EoR gating and Cap-70 receive precondition
// Related: selected_refresh_eor.go -- checkBoRRAllowedHeld, checkRawBoRRAllowedHeld, normalizeReceivedRouteRefresh, routeRefreshBoRRBeforeEOR
// Related: reactor_api_forward.go -- sendRouteRefreshContext
// Related: selected_refresh.go -- SelectRefresh
// Related: destination_marker.go -- sendBoundMarker
// Related: session_write.go -- SendRawMessage
// Related: source_session.go -- messageReceiverForPeer
// Related: session_read.go -- recvEOR tracking on received End-of-RIB
// RFC 7313 Section 4 -- see rfc/short/rfc7313.md

package reactor

import (
	"bufio"
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	bgpformat "github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC7313TxBoRRBlockedBeforeEoRWhenGRSupported proves RFC 7313 Section 4:
// a speaker supporting BGP Graceful Restart MUST NOT send a BoRR for an
// <AFI, SAFI> to a neighbor before it sends the EoR for that <AFI, SAFI>,
// across ALL BoRR transmission paths: unbound SendBoRR (+ atomic SendRawMessage
// gate under writeMu), ordinary SelectRefresh, and bound legacy sendBoundMarker.
//
// RFC requirement: RFC7313-4-6 positive -- when localGR is true, unbound SendBoRR, atomic SendRawMessage, ordinary SelectRefresh, and bound sendBoundMarker all refuse IPv4 unicast BoRR before IPv4 EoR reaches the wire (even after IPv6 EoR has been sent) and write zero BoRR frames to the wire.
// RFC requirement: RFC7313-4-6 negative -- after IPv4 EoR reaches the wire (or on a session without Graceful Restart), unbound SendBoRR, ordinary SelectRefresh, and bound sendBoundMarker succeed and write the BoRR frame to the wire.
func TestRFC7313TxBoRRBlockedBeforeEoRWhenGRSupported(t *testing.T) {
	setupPeer := func(t *testing.T, withGR bool) (*Peer, *recordingConn, *reactorAPIAdapter) {
		t.Helper()
		peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast, family.IPv6Unicast)
		writer := &observedBGPWriter{session: peer.session, writer: conn}
		peer.session.wireWriter = writer
		peer.session.bufWriter = bufio.NewWriter(writer)

		localCaps := []capability.Capability{
			&capability.Multiprotocol{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast},
			&capability.Multiprotocol{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast},
			&capability.RouteRefresh{},
			&capability.EnhancedRouteRefresh{},
		}
		if withGR {
			localCaps = append(localCaps, &capability.GracefulRestart{RestartTime: 120})
		}
		peerCaps := []capability.Capability{
			&capability.Multiprotocol{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast},
			&capability.Multiprotocol{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast},
			&capability.RouteRefresh{},
			&capability.EnhancedRouteRefresh{},
		}
		peer.session.proposal = preparedOpenForTest(peer.session, &message.Open{Version: 4, MyAS: 65000, HoldTime: 90, BGPIdentifier: 1})
		peer.session.peerOpen = &message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 2}
		peer.session.negotiateWith(localCaps, peerCaps)
		require.NotNil(t, peer.session.negotiated)
		require.Equal(t, withGR, peer.session.localGR)
		peer.negotiated.Store(NewNegotiatedCapabilities(peer.session.negotiated))
		// Clear initial sync queueing so SendBoRR writes directly onto the socket.
		peer.session.sendingInitialRoutes.Store(0)

		return peer, conn, newSendPermissionReactor(peer)
	}

	// Path 1: Unbound SendBoRR AND atomic SendRawMessage gate (closes TOCTOU on session replacement/drain).
	t.Run("unbound_and_raw", func(t *testing.T) {
		peerGR, connGR, apiGR := setupPeer(t, true)
		err := apiGR.SendBoRR(selector.All(), uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.OperatorSender())
		require.ErrorIs(t, err, errBoRRBeforeEOR)
		require.Empty(t, connGR.written(), "no BoRR may reach the wire before EoR when GR is supported")

		// Direct SendRawMessage (used by sendQueuedRefreshMarker during opQueue drain)
		// MUST ALSO enforce checkBoRRAllowedHeld atomically under writeMu!
		borrFrame := refreshMarkerWire(message.RouteRefreshBoRR)
		require.ErrorIs(t, peerGR.session.SendRawMessage(0, borrFrame), errBoRRBeforeEOR)
		require.Empty(t, connGR.written())

		// Send IPv6 EoR only: IPv4 BoRR MUST STILL be refused (per-<AFI, SAFI> gate!).
		require.NoError(t, peerGR.session.SendUpdate(message.BuildEOR(family.IPv6Unicast)))
		require.Len(t, splitWireMessages(t, connGR.written()), 1)

		err = apiGR.SendBoRR(selector.All(), uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.OperatorSender())
		require.ErrorIs(t, err, errBoRRBeforeEOR)
		require.ErrorIs(t, peerGR.session.SendRawMessage(0, borrFrame), errBoRRBeforeEOR)
		require.Len(t, splitWireMessages(t, connGR.written()), 1, "IPv6 EoR must not unlock IPv4 BoRR")

		// Negative 1: once IPv4 EoR reaches the wire, IPv4 BoRR IS sent!
		require.NoError(t, peerGR.session.SendUpdate(message.BuildEOR(family.IPv4Unicast)))
		err = apiGR.SendBoRR(selector.All(), uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.OperatorSender())
		require.NoError(t, err)
		afterFrames := splitWireMessages(t, connGR.written())
		require.Len(t, afterFrames, 3)
		require.Equal(t, borrFrame, afterFrames[2])

		// Negative 2: speaker without GR support (localGR=false) sends BoRR before EoR cleanly.
		_, connNoGR, apiNoGR := setupPeer(t, false)
		err = apiNoGR.SendBoRR(selector.All(), uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.OperatorSender())
		require.NoError(t, err)
		noGRFrames := splitWireMessages(t, connNoGR.written())
		require.Len(t, noGRFrames, 1)
		require.Equal(t, borrFrame, noGRFrames[0])
	})

	// Path 2: Ordinary SelectRefresh (selected_refresh.go -> checkBoRRAllowedHeld).
	t.Run("select_refresh_with_gr", func(t *testing.T) {
		fixture, input := selectedRefreshFixture(t)
		selectedRefreshNegotiateGR(t, fixture, true, false)
		fixture.peer.session.initialSyncEOROwed.Store(false)
		request, err := plugin.NewSelectedRefresh(input)
		require.NoError(t, err)

		// Positive: blocked before IPv4 EoR.
		_, err = fixture.api.SelectRefresh(t.Context(), request, fixture.owner)
		require.ErrorIs(t, err, errBoRRBeforeEOR)
		require.Empty(t, fixture.conn.written())

		// Negative: unlocked after IPv4 EoR reaches the wire.
		require.NoError(t, fixture.peer.session.SendUpdate(message.BuildEOR(family.IPv4Unicast)))
		out, err := fixture.api.SelectRefresh(t.Context(), request, fixture.owner)
		require.NoError(t, err)
		require.Equal(t, rpc.SelectedRefreshWritten, out.Outcome)
		require.Equal(t, []string{"eor", "borr"}, refreshWireLabels(t, fixture.conn.written()))
	})

	t.Run("select_refresh_without_gr", func(t *testing.T) {
		// Negative 2: non-GR speaker sends BoRR before EoR via SelectRefresh.
		fixtureNoGR, inputNoGR := selectedRefreshFixture(t)
		selectedRefreshNegotiateGR(t, fixtureNoGR, false, false)
		fixtureNoGR.peer.session.initialSyncEOROwed.Store(false)
		reqNoGR, err := plugin.NewSelectedRefresh(inputNoGR)
		require.NoError(t, err)
		outNoGR, err := fixtureNoGR.api.SelectRefresh(t.Context(), reqNoGR, fixtureNoGR.owner)
		require.NoError(t, err)
		require.Equal(t, rpc.SelectedRefreshWritten, outNoGR.Outcome)
		require.Equal(t, []string{"borr"}, refreshWireLabels(t, fixtureNoGR.conn.written()))
	})

	// Path 3: Bound legacy sendBoundMarker (destination_marker.go -> checkBoRRAllowedHeld).
	t.Run("bound_marker", func(t *testing.T) {
		fixture, input := selectedRefreshFixture(t)
		selectedRefreshNegotiateGR(t, fixture, true, false)
		fixture.peer.establishedSession.Store(fixture.peer.session)
		fixture.peer.session.initialSyncEOROwed.Store(false)
		marker := &message.RouteRefresh{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast, Subtype: message.RouteRefreshBoRR}

		// Positive: blocked before IPv4 EoR.
		err := fixture.peer.session.sendBoundMarker(t.Context(), marker, input.DestinationSession, fixture.owner, fixture.peer)
		require.ErrorIs(t, err, errBoRRBeforeEOR)
		require.Empty(t, fixture.conn.written())

		// Negative: unlocked after IPv4 EoR reaches the wire.
		require.NoError(t, fixture.peer.session.SendUpdate(message.BuildEOR(family.IPv4Unicast)))
		err = fixture.peer.session.sendBoundMarker(t.Context(), marker, input.DestinationSession, fixture.owner, fixture.peer)
		require.NoError(t, err)
		require.Equal(t, []string{"eor", "borr"}, refreshWireLabels(t, fixture.conn.written()))
	})
}

// TestRFC7313RxNonEnhancedPeerReservedOctetNotBoRR proves the RFC 7313 Section 4
// Capability-70 precondition: when a peer did NOT advertise Enhanced Route Refresh
// (RFC 2918 only), octet 2 of a 4-byte ROUTE-REFRESH is Reserved ("ignored by the
// receiver") and MUST be normalized to subtype 0 ("refresh") before plugin/JSON
// delivery so downstream consumers never treat it as a BoRR/EoRR; when Cap 70 IS
// advertised by the peer, octet 2 == 1 arrives as BoRR ("borr").
//
// RFC requirement: RFC7313-4-4 negative -- for an RFC-2918-only peer that did not advertise Enhanced Route Refresh Capability (code 70), receiving a 4-byte ROUTE-REFRESH with Reserved octet 0x01 delivers subtype 0 ("refresh" in JSON and RawBytes[2]==0 in DirectBridge) rather than BoRR; for a Cap-70 peer, octet 0x01 delivers BoRR ("borr" and RawBytes[2]==1).
func TestRFC7313RxNonEnhancedPeerReservedOctetNotBoRR(t *testing.T) {
	runCase := func(t *testing.T, peerSentCap70 bool) (bgptypes.RawMessage, string) {
		t.Helper()
		peerAddr := netip.MustParseAddr("192.0.2.88")
		settings := NewPeerSettings(peerAddr, 65000, 65001, 0x01020301)
		r := New(&Config{ListenAddr: "127.0.0.1:0"})
		require.NoError(t, r.AddPeer(settings))
		peer, found := r.findPeerByAddr(peerAddr)
		require.True(t, found)

		var gotPeer plugin.PeerInfo
		var gotMsg bgptypes.RawMessage
		gotCount := 0
		r.setMessageReceiver(&testDeliveryReceiver{
			consumerCount: 1,
			onReceived: func(pi plugin.PeerInfo, msg bgptypes.RawMessage) {
				if msg.Type == msgtype.TypeROUTEREFRESH {
					gotPeer = pi
					gotMsg = msg
					gotCount++
				}
			},
		})

		s := NewSession(settings)
		conn := &recordingConn{}
		s.conn = conn
		s.bufWriter = bufio.NewWriter(conn)
		require.NoError(t, s.fsm.Event(fsm.EventManualStart))
		require.NoError(t, s.fsm.Event(fsm.EventTCPConnectionConfirmed))
		require.NoError(t, s.fsm.Event(fsm.EventBGPOpen))
		require.NoError(t, s.fsm.Event(fsm.EventKeepaliveMsg))

		localCaps := []capability.Capability{
			&capability.Multiprotocol{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast},
			&capability.RouteRefresh{},
			&capability.EnhancedRouteRefresh{},
		}
		peerCaps := []capability.Capability{
			&capability.Multiprotocol{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast},
			&capability.RouteRefresh{},
		}
		if peerSentCap70 {
			peerCaps = append(peerCaps, &capability.EnhancedRouteRefresh{})
		}
		s.proposal = preparedOpenForTest(s, &message.Open{Version: 4, MyAS: 65000, HoldTime: 90, BGPIdentifier: 1})
		s.peerOpen = &message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 2}
		s.negotiateWith(localCaps, peerCaps)
		require.NotNil(t, s.negotiated)
		s.onMessageReceived = r.messageReceiverForPeer(peer)

		// Feed wire ROUTE-REFRESH with octet 2 == 0x01 (BoRR under RFC 7313, Reserved under RFC 2918).
		frame := refreshMarkerWire(message.RouteRefreshBoRR)
		s.bufReader = bufio.NewReader(bytes.NewReader(frame))
		require.NoError(t, s.ReadAndProcess())
		require.Equal(t, 1, gotCount)

		jsonEvent := bgpformat.NewJSONEncoder("test-host").Message(&gotPeer, gotMsg, plugin.FormatParsed)
		return gotMsg, jsonEvent
	}

	// RFC-2918-only peer (no Cap 70): octet 2 == 0x01 is Reserved -> normalized to subtype 0 ("refresh"), never "borr".
	msg2918, json2918 := runCase(t, false)
	require.Equal(t, byte(0), msg2918.RawBytes[2], "RFC-2918-only peer must deliver subtype 0 to plugins, never BoRR")
	require.Contains(t, json2918, `"type":"refresh"`)
	require.NotContains(t, json2918, `"type":"borr"`)

	// Cap-70 peer: octet 2 == 0x01 arrives as BoRR ("borr").
	msg7313, json7313 := runCase(t, true)
	require.Equal(t, byte(1), msg7313.RawBytes[2], "Cap-70 peer must preserve BoRR subtype 1")
	require.Contains(t, json7313, `"type":"borr"`)
}

// TestRFC7313RxBoRRIgnoredBeforeEoRFromGRPeer proves RFC 7313 Section 4:
// a BGP speaker that has received the Graceful Restart Capability from its
// neighbor MUST ignore any BoRR for an <AFI, SAFI> before it receives the EoR
// for that <AFI, SAFI> from the neighbor, while keeping the session Established.
//
// RFC requirement: RFC7313-4-7 positive -- from a peer that advertised Graceful Restart and Enhanced Route Refresh, a received BoRR for IPv4 unicast before IPv4 EoR (even after IPv6 EoR) is ignored before plugin delivery, sends no NOTIFICATION, and leaves the session Established.
// RFC requirement: RFC7313-4-7 negative -- after the peer sends IPv4 EoR, a subsequent IPv4 BoRR IS delivered (not ignored); from a peer that advertised Enhanced Route Refresh without Graceful Restart, an IPv4 BoRR before EoR IS delivered.
func TestRFC7313RxBoRRIgnoredBeforeEoRFromGRPeer(t *testing.T) {
	setupSession := func(t *testing.T, peerSentGR bool) (*Session, *recordingConn, *int) {
		t.Helper()
		settings := &PeerSettings{
			Address: netip.MustParseAddr("192.0.2.77"),
			LocalAS: 65000,
			PeerAS:  65001,
		}
		s := NewSession(settings)
		conn := &recordingConn{}
		s.conn = conn
		s.bufWriter = bufio.NewWriter(conn)

		require.NoError(t, s.fsm.Event(fsm.EventManualStart))
		require.NoError(t, s.fsm.Event(fsm.EventTCPConnectionConfirmed))
		require.NoError(t, s.fsm.Event(fsm.EventBGPOpen))
		require.NoError(t, s.fsm.Event(fsm.EventKeepaliveMsg))
		require.Equal(t, fsm.StateEstablished, s.fsm.State())

		localCaps := []capability.Capability{
			&capability.Multiprotocol{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast},
			&capability.Multiprotocol{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast},
			&capability.RouteRefresh{},
			&capability.EnhancedRouteRefresh{},
		}
		peerCaps := []capability.Capability{
			&capability.Multiprotocol{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast},
			&capability.Multiprotocol{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast},
			&capability.RouteRefresh{},
			&capability.EnhancedRouteRefresh{},
		}
		if peerSentGR {
			peerCaps = append(peerCaps, &capability.GracefulRestart{RestartTime: 120})
		}
		s.proposal = preparedOpenForTest(s, &message.Open{Version: 4, MyAS: 65000, HoldTime: 90, BGPIdentifier: 1})
		s.peerOpen = &message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 2}
		s.negotiateWith(localCaps, peerCaps)
		require.NotNil(t, s.negotiated)

		deliveredBoRR := 0
		s.onMessageReceived = func(_ *Session, _ netip.Addr, typ msgtype.MessageType, body []byte, _ *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection, _ BufHandle, _ map[string]any, _ string, _ uint64) bool {
			if typ == msgtype.TypeROUTEREFRESH && len(body) == 4 && message.RouteRefreshSubtype(body[2]) == message.RouteRefreshBoRR {
				deliveredBoRR++
			}
			return false
		}
		return s, conn, &deliveredBoRR
	}

	feedFrames := func(t *testing.T, s *Session, frames ...[]byte) {
		t.Helper()
		var stream bytes.Buffer
		for _, f := range frames {
			stream.Write(f)
		}
		s.bufReader = bufio.NewReader(&stream)
		for range frames {
			require.NoError(t, s.ReadAndProcess())
		}
	}

	borrV4 := refreshMarkerWire(message.RouteRefreshBoRR)
	eorV6 := message.PackTo(message.BuildEOR(family.IPv6Unicast), nil)
	eorV4 := message.PackTo(message.BuildEOR(family.IPv4Unicast), nil)

	// Positive: peer sent GR + Enhanced RR -> IPv4 BoRR before IPv4 EoR ignored,
	// even after IPv6 EoR arrives; no NOTIFICATION sent; session remains Established.
	sGR, connGR, deliveredGR := setupSession(t, true)
	feedFrames(t, sGR, borrV4, eorV6, borrV4)
	require.Equal(t, 0, *deliveredGR, "BoRR before IPv4 EoR from GR peer must be ignored (even after IPv6 EoR)")
	require.Empty(t, connGR.written(), "ignoring early BoRR must not send NOTIFICATION")
	require.Equal(t, fsm.StateEstablished, sGR.fsm.State())

	// Negative 1: once IPv4 EoR arrives, subsequent IPv4 BoRR IS delivered!
	feedFrames(t, sGR, eorV4, borrV4)
	require.Equal(t, 1, *deliveredGR, "BoRR after IPv4 EoR from GR peer must be accepted and delivered")
	require.Equal(t, fsm.StateEstablished, sGR.fsm.State())

	// Negative 2: peer without GR capability -> IPv4 BoRR before EoR IS delivered immediately!
	sNoGR, connNoGR, deliveredNoGR := setupSession(t, false)
	feedFrames(t, sNoGR, borrV4)
	require.Equal(t, 1, *deliveredNoGR, "BoRR from non-GR peer must not be gated on EoR")
	require.Empty(t, connNoGR.written())
	require.Equal(t, fsm.StateEstablished, sNoGR.fsm.State())
}
