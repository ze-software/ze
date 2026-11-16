// Design: docs/research/l2tpv2-ze-integration.md -- PPP admission refusal and L2TP lifetime
// Related: reactor_kernel.go -- handlePPPEvent

package l2tp

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// A refused duplicate cannot revoke the session that already owns the key.
// The actual reactor consumer must preserve both its session and reliable
// sender, so no CDN is queued for the still-admitted transport.
func TestL2TPReactorPPPRejectedExistingSessionPreservesTransport(t *testing.T) {
	r := newReactorForSnapshot(t)
	peer := newPPPRejectionPeer(t, r)
	address, ok := peer.LocalAddr().(*net.UDPAddr)
	require.True(t, ok)
	session := &L2TPSession{
		localSID: 41, remoteSID: 42, state: L2TPSessionEstablished,
		pppInterface: "ppp7", negotiatedMRU: 1492,
	}
	insertEstablishedTunnel(t, r, 100, 200, address.AddrPort(), session)
	tunnel := r.tunnelsByLocalID[100]
	before := tunnel.engine.Stats()
	t.Cleanup(func() { tunnel.engine.Close(r.params.Clock()) })

	r.handlePPPEvent(ppp.EventSessionRejected{
		TunnelID: 100, SessionID: 41, ExistingSession: true,
		Reason: "duplicate admission for an active key",
	})

	require.Same(t, session, tunnel.lookupSession(41), "refusal must retain the admitted transport")
	require.Equal(t, L2TPSessionEstablished, session.state)
	require.Equal(t, "ppp7", session.pppInterface)
	require.Equal(t, uint16(1492), session.negotiatedMRU)
	require.Equal(t, before, tunnel.engine.Stats(), "refusal must not enqueue a CDN")
}

// A newly refused call still needs a CDN. Drive the actual consumer and parse
// the datagram at the peer, while an unrelated admitted call remains usable.
func TestL2TPReactorPPPRejectedNewSessionSendsCDN(t *testing.T) {
	r := newReactorForSnapshot(t)
	peer := newPPPRejectionPeer(t, r)
	address, ok := peer.LocalAddr().(*net.UDPAddr)
	require.True(t, ok)
	admitted := &L2TPSession{localSID: 41, remoteSID: 42, state: L2TPSessionEstablished}
	refused := &L2TPSession{localSID: 43, remoteSID: 44, state: L2TPSessionEstablished}
	insertEstablishedTunnel(t, r, 100, 200, address.AddrPort(), admitted, refused)
	tunnel := r.tunnelsByLocalID[100]
	t.Cleanup(func() { tunnel.engine.Close(r.params.Clock()) })

	r.handlePPPEvent(ppp.EventSessionRejected{
		TunnelID: 100, SessionID: 43, ExistingSession: false,
		Reason: "owner is already admitted under a different key",
	})

	require.Nil(t, tunnel.lookupSession(43), "a call PPP refused must leave the transport table")
	require.Same(t, admitted, tunnel.lookupSession(41), "another key's admitted call must survive")
	require.NoError(t, peer.SetReadDeadline(time.Now().Add(2*time.Second)))
	var buffer [4096]byte
	n, _, err := peer.ReadFromUDP(buffer[:])
	require.NoError(t, err, "the peer must receive the refused call's CDN")
	header, err := ParseMessageHeader(buffer[:n])
	require.NoError(t, err)
	require.True(t, header.IsControl)
	require.Equal(t, uint16(44), header.SessionID, "CDN must address the refused peer call")
	iterator := NewAVPIterator(buffer[header.PayloadOff:int(header.Length)])
	vendor, attribute, _, value, present := iterator.Next()
	require.True(t, present)
	require.NoError(t, iterator.Err())
	require.Equal(t, uint16(0), vendor)
	require.Equal(t, AVPMessageType, attribute)
	messageType, err := readAVPUint16(value)
	require.NoError(t, err)
	require.Equal(t, MsgCDN, MessageType(messageType))
}

// newPPPRejectionPeer gives the consumer a real sender without starting its
// reactor goroutine; each test owns the map and drives one event synchronously.
func newPPPRejectionPeer(t *testing.T, reactor *l2tpReactor) *net.UDPConn {
	t.Helper()
	listener := newUDPListener(netip.MustParseAddrPort("127.0.0.1:0"), reactor.logger)
	require.NoError(t, listener.Start(context.Background()))
	t.Cleanup(func() { require.NoError(t, listener.Stop()) })
	reactor.listener = listener
	peer, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, peer.Close()) })
	return peer
}
