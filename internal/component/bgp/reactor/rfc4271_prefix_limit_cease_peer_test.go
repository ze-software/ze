// Design: docs/architecture/behavior/fsm.md — the peer run loop owns the connection a prefix-limit Cease ends
// Related: session_prefix_test.go — checkPrefixLimits on a bare Session
// Related: rfc4271_opensent_error_peer_test.go — the dial-only harness this file configures
// RFC: rfc/short/rfc4271.md — Section 6.7, prefix upper bound

package reactor

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// startLimitedEstablishedNeighbor runs a dial-only Peer (AS 65000) whose
// settings configure applies before the start, answers its OPEN and KEEPALIVE,
// and returns once the session reports Established. The reconnect delay is a
// minute, so the Peer does not redial inside the test.
func startLimitedEstablishedNeighbor(t *testing.T, configure func(*PeerSettings)) *openSentNeighbor {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, ln.Close()) })
	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok, "listener address must be TCP")

	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
	settings.Port = uint16(addr.Port) //nolint:gosec // a listener port fits 16 bits
	settings.Connection = ConnectionActive
	configure(settings)
	peer := NewPeer(settings)
	peer.setReconnectDelay(time.Minute, time.Minute)
	t.Cleanup(startAndStop(t, peer))

	tcpListener, isTCP := ln.(*net.TCPListener)
	require.True(t, isTCP, "Listen(\"tcp\") returns a TCPListener")
	require.NoError(t, tcpListener.SetDeadline(time.Now().Add(5*time.Second)))
	conn, err := ln.Accept()
	require.NoError(t, err, "the peer never dialed")
	t.Cleanup(func() { assert.NoError(t, conn.Close()) })

	buf := make([]byte, message.MaxMsgLen)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	read, err := conn.Read(buf)
	require.NoError(t, err)
	require.GreaterOrEqual(t, read, message.HeaderLen)
	require.Equal(t, byte(1), buf[18], "the peer's first message is its OPEN")

	_, err = conn.Write(wellFormedOpen())
	require.NoError(t, err)
	_, err = conn.Write(message.PackTo(message.NewKeepalive(), nil))
	require.NoError(t, err)

	var session *Session
	require.Eventually(t, func() bool {
		session = peer.currentSession()
		return session != nil && session.State() == fsm.StateEstablished
	}, 5*time.Second, time.Millisecond, "the peer's session never reported Established")
	n := &openSentNeighbor{peer: peer, session: session, conn: conn}
	n.readUntil(t, 200*time.Millisecond) // the Peer's KEEPALIVE
	return n
}

// threePrefixUpdate announces 10.0.0.0/24, 10.0.1.0/24 and 10.0.2.0/24 from AS
// 65001, one prefix over an upper bound of two.
func threePrefixUpdate() []byte {
	nlri := []byte{24, 10, 0, 0, 24, 10, 0, 1, 24, 10, 0, 2}
	return buildUpdateMsg(makeUpdateBody(nil, firstASAttrs(4, 65001), nlri))
}

// TestRFC4271PrefixLimitTeardownSendsCease runs a Peer to Established with an
// IPv4 unicast upper bound of two prefixes and the default teardown, then the far
// end announces three.
//
// VALIDATES: the Peer writes exactly one NOTIFICATION, Error Code 6 (Cease) with
// subcode 1 (Maximum Number of Prefixes Reached), and then drops the TCP
// connection, read off the socket as the neighbor receives it.
// PREVENTS: a prefix-limit termination that closes without telling the neighbor,
// or that tells it with another Error Code.
//
// RFC requirement: RFC4271-6.7-4 positive -- a running Peer in Established with an IPv4 unicast upper bound of 2 and teardown enabled that receives an UPDATE announcing 3 prefixes writes exactly one NOTIFICATION with Error Code Cease (6/1) on the wire and then closes the TCP connection.
// RFC requirement: RFC4271-6.7-1 positive -- a prefix-limit teardown, where no fatal error of Section 6 exists, is the case Cease is for: a running Established Peer writes exactly one NOTIFICATION 6/1 and closes the TCP connection.
func TestRFC4271PrefixLimitTeardownSendsCease(t *testing.T) {
	n := startLimitedEstablishedNeighbor(t, func(settings *PeerSettings) {
		settings.PrefixMaximum = map[string]uint32{"ipv4/unicast": 2}
	})

	_, err := n.conn.Write(threePrefixUpdate())
	require.NoError(t, err)
	wire, closed := n.readUntil(t, 5*time.Second)

	assert.Equal(t, [][2]byte{{6, message.NotifyCeaseMaxPrefixes}}, notifications(t, wire),
		"exactly one Cease, Maximum Number of Prefixes Reached")
	assert.True(t, closed, "the speaker terminates the connection")
}

// TestRFC4271PrefixLimitWithoutTeardownSendsNoCease is the same Peer and the
// same three prefixes with teardown disabled for IPv4 unicast, so the speaker
// does not decide to terminate.
//
// VALIDATES: no NOTIFICATION within 500 ms, the connection kept, and the session
// still Established.
// PREVENTS: a Cease bound to the limit being crossed rather than to the decision
// to terminate the connection.
//
// RFC requirement: RFC4271-6.7-4 negative -- a running Peer in Established with an IPv4 unicast upper bound of 2 and teardown disabled that receives an UPDATE announcing 3 prefixes writes no NOTIFICATION within 500 ms, keeps the TCP connection and stays Established.
func TestRFC4271PrefixLimitWithoutTeardownSendsNoCease(t *testing.T) {
	n := startLimitedEstablishedNeighbor(t, func(settings *PeerSettings) {
		settings.PrefixMaximum = map[string]uint32{"ipv4/unicast": 2}
		settings.PrefixTeardown = map[string]bool{"ipv4/unicast": false}
	})

	_, err := n.conn.Write(threePrefixUpdate())
	require.NoError(t, err)
	wire, closed := n.readUntil(t, 500*time.Millisecond)

	assert.Empty(t, notifications(t, wire), "no NOTIFICATION")
	assert.False(t, closed, "the connection stays up")
	assert.Equal(t, fsm.StateEstablished, n.session.State(), "the session stays Established")
}
