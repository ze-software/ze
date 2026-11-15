// Design: docs/architecture/behavior/fsm-open-sent.md -- OPEN refusal closes TCP
// Related: rfc7607_session_open_as_test.go -- existing notification and FSM assertions
// Related: session_open_as.go -- rejectOpenPeerAS owns notification and socket closure
// RFC 7607 Section 2 -- see rfc/short/rfc7607.md

package reactor

import (
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// TestRFC7607ZeroASOpenAbortsTCP reads the required NOTIFICATION followed by
// remote EOF for both OPEN consumers and both AS fields. The session runs no
// outer Run cleanup, so the OPEN producer itself must close the socket.
// RFC 7607 Section 2: "If a BGP speaker receives zero as the peer AS in an OPEN
// message, it MUST abort the connection and send a NOTIFICATION with Error Code
// "OPEN Message Error" and subcode "Bad Peer AS" (see Section 6 of [RFC4271])."
// MUTATION: omit closeConn in rejectOpenPeerAS; the notification still arrives,
// but the following read times out instead of observing EOF.
// RFC requirement: RFC7607-2-4 positive -- on both normal and collision-winner OPEN rails, My AS zero or a zero Four-octet AS capability produces NOTIFICATION 2/2 followed by remote TCP EOF.
func TestRFC7607ZeroASOpenAbortsTCP(t *testing.T) {
	for _, collision := range []bool{false, true} {
		for _, tc := range rfc7607OpenTCPCases() {
			t.Run(rfc7607OpenTCPRail(collision)+"/"+tc.name, func(t *testing.T) {
				session, far, _, err := rfc7607OpenTCPExchange(t, tc.peerAS, tc.zero, collision)
				require.ErrorIs(t, err, ErrBadPeerAS)
				require.Equal(t, []byte{2, 2}, rfc7607OpenTCPRead(t, far, msgtype.TypeNOTIFICATION))
				require.NoError(t, far.SetReadDeadline(time.Now().Add(time.Second)))
				var trailing [1]byte
				n, err := far.Read(trailing[:])
				require.Zero(t, n, "nothing may follow the refusal notification")
				require.ErrorIs(t, err, io.EOF, "the OPEN consumer must close TCP, not merely reject the message")
				require.Equal(t, fsm.StateIdle, session.State())
			})
		}
	}
}

// TestRFC7607RealASOpenRetainsTCP pairs each forbidden OPEN with its real-AS
// twin. Both consumers must finish the handshake and permit another outbound
// KEEPALIVE on the same TCP socket, so unconditional notification or closure
// cannot satisfy the zero-AS test.
// MUTATION: force the advertised-AS zero branch true; the real-AS handshake
// receives NOTIFICATION instead of KEEPALIVE and cannot reach Established.
// RFC requirement: RFC7607-2-4 negative -- matching real-AS OPENs on both rails complete OPEN/KEEPALIVE to Established and transmit a further KEEPALIVE on the same TCP connection.
func TestRFC7607RealASOpenRetainsTCP(t *testing.T) {
	for _, collision := range []bool{false, true} {
		for _, tc := range rfc7607OpenTCPCases() {
			t.Run(rfc7607OpenTCPRail(collision)+"/"+tc.name, func(t *testing.T) {
				session, far, near, err := rfc7607OpenTCPExchange(t, tc.peerAS, tc.real, collision)
				require.NoError(t, err)
				require.Empty(t, rfc7607OpenTCPRead(t, far, msgtype.TypeKEEPALIVE))
				require.Equal(t, fsm.StateOpenConfirm, session.State())
				keepalive := message.PackTo(message.NewKeepalive(), nil)
				n, err := far.Write(keepalive)
				require.NoError(t, err)
				require.Len(t, keepalive, n)
				require.NoError(t, session.ReadAndProcess())
				require.Equal(t, fsm.StateEstablished, session.State())
				require.NoError(t, session.sendKeepalive(near))
				require.Empty(t, rfc7607OpenTCPRead(t, far, msgtype.TypeKEEPALIVE),
					"the established connection must remain able to send")
				require.Equal(t, fsm.StateEstablished, session.State())
			})
		}
	}
}

type rfc7607OpenTCPCase struct {
	name   string
	peerAS uint32
	zero   []byte
	real   []byte
}

func rfc7607OpenTCPCases() []rfc7607OpenTCPCase {
	return []rfc7607OpenTCPCase{
		{"my-as", 65002, openBodyWithIdentifier(0, 0x0a000001), openBodyWithIdentifier(65002, 0x0a000001)},
		{"four-octet-capability", 196608, openBodyWithASN4Capability(0), openBodyWithASN4Capability(196608)},
	}
}

func rfc7607OpenTCPRail(collision bool) string {
	if collision {
		return "processOpen"
	}
	return "handleOpen"
}

// rfc7607OpenTCPExchange uses real TCP but deliberately does not start Run:
// its final cleanup would otherwise hide a missing close in the OPEN producer.
// The collision consumer receives the exact OPEN frame read before handoff,
// as acceptWithOpen does after collision resolution in the peer run loop.
func rfc7607OpenTCPExchange(t *testing.T, peerAS uint32, body []byte, collision bool) (*Session, net.Conn, net.Conn, error) {
	t.Helper()
	far, near := collisionProofTCPPair(t)
	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65001, peerAS, 0x0a000002)
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = 90 * time.Second
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: settings.LocalAS},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}
	session := NewSession(settings)
	t.Cleanup(session.timers.StopAll)
	require.NoError(t, session.Start())
	if !collision {
		require.NoError(t, session.Accept(near))
		rfc7607OpenTCPRead(t, far, msgtype.TypeOPEN)
	}
	open, err := message.UnpackOpen(body)
	require.NoError(t, err)
	packet := message.PackTo(open, nil)
	n, err := far.Write(packet)
	require.NoError(t, err)
	require.Len(t, packet, n)
	if !collision {
		err = session.ReadAndProcess()
		return session, far, near, err
	}
	received, err := core4271ReadMessage(near)
	require.NoError(t, err)
	require.Equal(t, packet, received, "collision handoff must preserve the original OPEN frame")
	open, err = message.UnpackOpen(received[message.HeaderLen:])
	require.NoError(t, err)
	err = session.acceptWithOpen(near, open, received)
	rfc7607OpenTCPRead(t, far, msgtype.TypeOPEN)
	return session, far, near, err
}

func rfc7607OpenTCPRead(t *testing.T, conn net.Conn, expected msgtype.MessageType) []byte {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	packet, err := core4271ReadMessage(conn)
	require.NoError(t, err)
	require.Equal(t, byte(expected), packet[message.HeaderLen-1])
	return packet[message.HeaderLen:]
}
