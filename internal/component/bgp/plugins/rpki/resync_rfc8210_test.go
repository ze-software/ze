package rpki

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RFC 8210 Section 8.1: a router that holds no serial for a cache cannot ask for a
// delta, so "a router that lacks the data needed for fast resynchronization MUST fall
// back to a Reset Query". connectAndSync decides that on the serial ze holds.

// openingPDU accepts one connection from a session prepared by prepare and returns the
// first want octets the router writes on it.
func openingPDU(t *testing.T, prepare func(*RTRSession), want int) []byte {
	t.Helper()

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok, "listener address is a TCP address")
	port := tcpAddr.Port

	stopCh := make(chan struct{})
	defer close(stopCh)

	s := newRTRSession("127.0.0.1", uint16(port), 100, "", newROACache(), newASPACache(), stopCh) //nolint:gosec // listener port fits uint16
	prepare(s)

	done := make(chan error, 1)
	go func() { done <- s.connectAndSync() }()

	conn, err := ln.Accept()
	require.NoError(t, err)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))

	buf := make([]byte, want)
	_, err = io.ReadFull(conn, buf)
	require.NoError(t, err, "the router must speak first on a new connection")

	_ = conn.Close() // ends readLoop so connectAndSync returns
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("connectAndSync did not return after the cache closed the connection")
	}
	return buf
}

// TestRFC8210ResetQueryFallback pins the query connectAndSync opens with.
//
// VALIDATES: RFC8210-8.1-3, a session without a serial opens with a Reset Query, and one
// that holds a serial opens with a Serial Query instead.
// PREVENTS: asking a cache for a delta from a serial ze never held, or discarding a
// usable serial and reloading the whole set on every reconnect.
func TestRFC8210ResetQueryFallback(t *testing.T) {
	t.Run("no serial after a cache reset falls back to a reset query", func(t *testing.T) {
		// RFC requirement: RFC8210-8.1-3 positive -- a session whose serial a Cache Reset cleared
		// lacks the data for a Serial Query and opens its next connection with a Reset Query.
		buf := openingPDU(t, func(s *RTRSession) {
			s.serial = 42
			s.sessionID = 0x1234
			_, err := s.handlePDU(rTRHeader{Version: rtrVersionMax, Type: pduCacheReset}, make([]byte, pduHeaderLen))
			require.ErrorIs(t, err, errRtrCacheResetReceivedWillDo)
		}, pduResetQueryLen)

		assert.Equal(t, pduResetQuery, buf[1], "the fallback is a Reset Query")
		assert.Equal(t, uint32(pduResetQueryLen), binary.BigEndian.Uint32(buf[4:8]))
	})

	t.Run("a held serial is used for a serial query, not a reset", func(t *testing.T) {
		// RFC requirement: RFC8210-8.1-3 negative -- a session that holds the serial and session
		// id needed for fast resynchronization does not fall back: its opening PDU is a Serial
		// Query carrying that serial, never a Reset Query.
		buf := openingPDU(t, func(s *RTRSession) {
			s.serial = 42
			s.sessionID = 0x1234
		}, pduSerialQueryLen)

		assert.NotEqual(t, pduResetQuery, buf[1], "a session holding its serial does not reset")
		assert.Equal(t, pduSerialQuery, buf[1])
		assert.Equal(t, uint32(42), binary.BigEndian.Uint32(buf[8:12]), "the held serial is what is asked for")
	})
}
