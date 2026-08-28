// Design: docs/architecture/behavior/peer-lifecycle.md — prefix teardown and reconnect
// Overview: peer_run.go — Peer.run, the reconnect loop this test drives
// Related: session_prefix.go — applyPrefixCheck, the teardown producer
// Related: session_prefix_family_test.go — the per-family decision unit tests

package reactor

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/report"
)

// overflowUpdateBytes builds one iBGP UPDATE announcing two IPv4 /24 prefixes.
//
// Two is one over the maximum the test configures, so the second prefix is what
// stops the session. The attributes are the iBGP mandatory set (RFC 4271
// Section 5.1): ORIGIN, an empty AS_PATH, NEXT_HOP and LOCAL_PREF. An
// attribute error would stop the session with a different NOTIFICATION and the
// test would prove nothing about prefix limits.
func overflowUpdateBytes() []byte {
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN, IGP
		0x40, 0x02, 0x00, // AS_PATH, empty (legal on an iBGP session)
		0x40, 0x03, 0x04, 10, 0, 0, 1, // NEXT_HOP 10.0.0.1
		0x40, 0x05, 0x04, 0, 0, 0, 100, // LOCAL_PREF 100
	}
	nlri := []byte{24, 10, 0, 0, 24, 10, 0, 1}

	body := make([]byte, 0, 4+len(attrs)+len(nlri))
	body = append(body, 0, 0) // withdrawn routes length
	body = binary.BigEndian.AppendUint16(body, uint16(len(attrs)))
	body = append(body, attrs...)
	body = append(body, nlri...)

	msg := make([]byte, message.HeaderLen, message.HeaderLen+len(body))
	for i := range 16 {
		msg[i] = 0xFF
	}
	binary.BigEndian.PutUint16(msg[16:18], uint16(message.HeaderLen+len(body)))
	msg[18] = 2 // UPDATE
	return append(msg, body...)
}

// servePrefixOverflow listens on a loopback port, speaks enough BGP to bring ze
// to Established, then announces two prefixes on a session whose maximum is one.
//
// The returned counter holds the number of TCP connections accepted, which is
// how the test sees whether ze dialed again after the teardown.
func servePrefixOverflow(t *testing.T, peerAS uint16) (port uint16, conns *atomic.Int32, stop func()) {
	t.Helper()

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	conns = &atomic.Int32{}
	// Accept loop and per-connection speaker: test helpers, stopped by the
	// returned closer (ai/rules/goroutine-lifecycle.md permits both).
	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			conns.Add(1)
			go serveOverflowConn(conn, peerAS)
		}
	}()

	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok, "loopback listener must carry a TCP address")
	return uint16(addr.Port), conns, func() { _ = ln.Close() } //nolint:gosec // a TCP port is a uint16 by definition
}

// serveOverflowConn drives one connection through OPEN, KEEPALIVE and the
// over-limit UPDATE, then reads until ze closes the socket.
func serveOverflowConn(conn net.Conn, peerAS uint16) {
	defer conn.Close() //nolint:errcheck // test helper

	buf := make([]byte, 4096)
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return
	}
	if _, err := conn.Read(buf); err != nil { // ze's OPEN
		return
	}
	open := &message.Open{Version: 4, MyAS: peerAS, HoldTime: 90, BGPIdentifier: 0x02020202}
	if _, err := conn.Write(message.PackTo(open, nil)); err != nil {
		return
	}
	if _, err := conn.Write(message.PackTo(message.NewKeepalive(), nil)); err != nil {
		return
	}
	if _, err := conn.Read(buf); err != nil { // ze's KEEPALIVE: it saw the OPEN
		return
	}
	if _, err := conn.Write(overflowUpdateBytes()); err != nil {
		return
	}
	for {
		if _, err := conn.Read(buf); err != nil { // ze's NOTIFICATION, then EOF
			return
		}
	}
}

// TestPeerHeldDownAfterPrefixTeardown is the wiring test for the reconnect
// contract of `prefix { idle-timeout 0; }`, which is the YANG default.
//
// VALIDATES: a peer whose family exceeded its prefix maximum stays DOWN. It
// dials exactly once, reaches Established, is torn down by the limit, and then
// waits for an operator instead of reconnecting.
// PREVENTS: the flap loop the old code produced. `prefixReconnectDecision`
// declined the prefix backoff for a zero idle-timeout, and Peer.run then fell
// through to its NORMAL backoff, so the peer came straight back, re-exceeded
// the maximum, and cycled forever. The YANG description said "0 = no
// reconnect" the whole time.
//
// The connection count alone would be a vacuous absence assertion: it also
// stays at one when the peer never starts (ai/rules/interop-and-goal-validation.md).
// The positive assertions are that the peer DID connect once and that it names
// its own state as held-down, which no broken peer reaches.
func TestPeerHeldDownAfterPrefixTeardown(t *testing.T) {
	port, conns, stopServer := servePrefixOverflow(t, 65000)
	defer stopServer()

	settings := NewPeerSettings(mustParseAddr("127.0.0.1"), 65000, 65000, 0x01010101)
	settings.Port = port
	settings.Connection = ConnectionActive // dial only; the test runs no listener for ze
	settings.PrefixMaximum = map[string]uint32{"ipv4/unicast": 1}
	settings.PrefixWarning = map[string]uint32{"ipv4/unicast": 1}
	// PrefixIdleTimeout is deliberately left unset. That is the YANG default of
	// 0, and 0 means the peer stays down.

	peer := NewPeer(settings)
	// A backoff far shorter than the observation window below, so a peer that
	// reconnects gets many chances to prove it.
	peer.setReconnectDelay(20*time.Millisecond, 40*time.Millisecond)
	stopPeer := startAndStop(t, peer)
	defer stopPeer()

	require.Eventually(t, func() bool {
		return peer.State().String() == "idle-hold"
	}, 5*time.Second, 20*time.Millisecond,
		"the peer must name itself held-down after a prefix-limit teardown")

	assert.Equal(t, int32(1), conns.Load(), "the peer must have connected exactly once")
	assert.Never(t, func() bool {
		return conns.Load() > 1
	}, 500*time.Millisecond, 25*time.Millisecond,
		"a held peer must not dial again, and 500ms is 25 backoff windows")

	held := false
	for _, w := range report.Warnings() {
		if w.Source == reportSourceBGP && w.Code == reportCodePrefixHold && w.Subject == "127.0.0.1" {
			held = true
			assert.Contains(t, w.Message, "ipv4/unicast", "the warning must name the family that held the peer")
		}
	}
	assert.True(t, held, "an operator running `ze show warnings` must see the hold")
}

// TestHoldDownRefusesInboundAndEndsOnStop drives the hold loop directly.
//
// VALIDATES: a held peer refuses the inbound connections a passive peer would
// otherwise accept, and the loop ends when the peer is stopped. Refusing
// matters because a remote that keeps retrying would otherwise walk straight
// back into a session the operator asked to stay down.
// PREVENTS: a busy wait (the loop blocks on channels, never on a timer) and a
// peer goroutine that outlives its context.
func TestHoldDownRefusesInboundAndEndsOnStop(t *testing.T) {
	report.ResetForTest()

	settings := NewPeerSettings(mustParseAddr("192.0.2.7"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)
	ctx, cancel := context.WithCancel(context.Background())
	peer.ctx = ctx

	done := make(chan struct{})
	go func() {
		defer close(done)
		peer.holdDownAfterPrefixTeardown("ipv6/unicast")
	}()

	require.Eventually(t, func() bool {
		return peer.State() == PeerStateIdleHold
	}, 2*time.Second, 10*time.Millisecond, "the hold must name itself in the peer state")

	client, server := net.Pipe()
	defer client.Close() //nolint:errcheck // test cleanup
	peer.SetInboundConnection(server)

	// A refused connection is a CLOSED connection: the read below returns an
	// error once the hold loop has dropped it.
	// the polled form is REPLACED, not removed, and by a stricter
	// check -- two hard assertions (an error, and not a deadline) where there was
	// one polled boolean. Coverage is identical: a held peer must close the
	// inbound connection.
	//
	// ONE blocking read with a generous deadline. The read IS the wait: net.Pipe
	// unblocks it with io.EOF the moment the hold loop closes the far end, so it
	// returns as soon as the behavior happens and burns the deadline only when it
	// never does.
	//
	// The polled form was flaky at 4/20, and worse the harder the settle: with a
	// 200ms sleep before it, it failed 12/12 while a direct read on the same
	// connection returned EOF 6/6. Eventually re-arms a 50ms read deadline on the
	// same pipe every tick, so the deadline it sets and the read it then makes
	// belong to different moments. The production path was never at fault -- an
	// instrumented run showed the loop woke, took the connection and closed it in
	// every case, flaking or not.
	require.NoError(t, client.SetReadDeadline(time.Now().Add(2*time.Second)),
		"the test pipe must accept a read deadline")
	_, err := client.Read(make([]byte, 1))
	require.Error(t, err, "a held peer must close the inbound connection")
	require.False(t, isTimeout(err),
		"the read must end because the peer closed the connection, not because the deadline expired")

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the hold must end when the peer context is canceled")
	}
}

// isTimeout reports whether err is a deadline expiry rather than a real close.
func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
