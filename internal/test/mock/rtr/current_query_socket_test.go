// Design: docs/guide/rpki.md -- dynamic RTR query framing at the real socket.
package rtr

import (
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestRTRVRPFileQuerySocketLengths proves the header is validated before a
// dynamic query can consume a body or reach the responder. Invalid lengths
// send only the complete header, and must get EOF rather than a fabricated
// serial-zero reply or a wait for an impossible body. Valid zero/changed/equal
// serials and Reset Query remain useful on one real TCP connection.
// MUTATION: remove the dynamic exact-length guard in rtrMockHandleConn;
// serial length eight must then fail because it receives Cache Reset.
func TestRTRVRPFileQuerySocketLengths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "current-vrps")
	require.NoError(t, os.WriteFile(path, []byte("7\n10.254.0.0/24,24,65999\n"), 0o600))
	cache := &rtrMockCache{vrpFile: path, serial: 1, refresh: 1, retry: 1}
	for _, question := range []struct {
		name   string
		kind   byte
		length uint32
	}{
		{"serial missing body", rtrMockPDUSerialQuery, 8},
		{"serial below header", rtrMockPDUSerialQuery, 7},
		{"serial empty", rtrMockPDUSerialQuery, 0},
		{"serial short body", rtrMockPDUSerialQuery, 11},
		{"serial long body", rtrMockPDUSerialQuery, 13},
		{"serial oversized", rtrMockPDUSerialQuery, 0xffffffff},
		{"reset below header", rtrMockPDUResetQuery, 7},
		{"reset empty", rtrMockPDUResetQuery, 0},
		{"reset body", rtrMockPDUResetQuery, 9},
		{"reset serial body", rtrMockPDUResetQuery, 12},
		{"reset oversized", rtrMockPDUResetQuery, 0xffffffff},
	} {
		t.Run(question.name, func(t *testing.T) {
			conn := rtrQuerySocket(t, cache)
			header := []byte{2, question.kind, 0, 1, 0, 0, 0, 0}
			binary.BigEndian.PutUint32(header[4:8], question.length)
			_, err := conn.Write(header)
			require.NoError(t, err)
			var reply [1]byte
			n, err := conn.Read(reply[:])
			require.Zero(t, n, "malformed query reached the responder")
			require.ErrorIs(t, err, io.EOF, "invalid declared length must close before reading a body")
		})
	}
	t.Run("valid queries on same connection", func(t *testing.T) {
		conn := rtrQuerySocket(t, cache)
		for _, serial := range []uint32{0, 6} {
			query := []byte{2, rtrMockPDUSerialQuery, 0, 1, 0, 0, 0, 12, 0, 0, 0, 0}
			binary.BigEndian.PutUint32(query[8:12], serial)
			_, err := conn.Write(query)
			require.NoError(t, err)
			reply := make([]byte, 8)
			_, err = io.ReadFull(conn, reply)
			require.NoError(t, err)
			require.Equal(t, []byte{2, 8, 0, 0, 0, 0, 0, 8}, reply, "valid serial query must receive Cache Reset")
		}
		_, err := conn.Write([]byte{2, rtrMockPDUResetQuery, 0, 0, 0, 0, 0, 8})
		require.NoError(t, err)
		full := make([]byte, 8+20+24)
		_, err = io.ReadFull(conn, full)
		require.NoError(t, err)
		require.Equal(t, byte(rtrMockPDUCacheResp), full[1])
		require.Equal(t, byte(rtrMockPDUIPv4Prefix), full[9])
		require.Equal(t, []byte{1, 24, 24, 0, 10, 254, 0, 0}, full[16:24])
		require.Equal(t, uint32(65999), binary.BigEndian.Uint32(full[24:28]))
		require.Equal(t, byte(rtrMockPDUEndOfData), full[29])
		require.Equal(t, uint32(7), binary.BigEndian.Uint32(full[36:40]))
		_, err = conn.Write([]byte{2, rtrMockPDUSerialQuery, 0, 1, 0, 0, 0, 12, 0, 0, 0, 7})
		require.NoError(t, err)
		delta := make([]byte, 8+24)
		_, err = io.ReadFull(conn, delta)
		require.NoError(t, err)
		require.Equal(t, byte(rtrMockPDUCacheResp), delta[1])
		require.Equal(t, byte(rtrMockPDUEndOfData), delta[9])
		require.Equal(t, uint32(7), binary.BigEndian.Uint32(delta[16:20]))
	})
}

// TestRTRFixedQuerySocketKeepsWholeSet checks the same actual handler without
// vrpFile. Both normal query forms retain fixed-mode whole-set behavior.
func TestRTRFixedQuerySocketKeepsWholeSet(t *testing.T) {
	var vrps vrpList
	require.NoError(t, vrps.Set("10.254.0.0/24,24,65999"))
	conn := rtrQuerySocket(t, &rtrMockCache{vrps: vrps, serial: 7, refresh: 1, retry: 1})
	for _, query := range [][]byte{
		{2, rtrMockPDUResetQuery, 0, 0, 0, 0, 0, 8},
		{2, rtrMockPDUSerialQuery, 0, 1, 0, 0, 0, 12, 0, 0, 0, 7},
	} {
		_, err := conn.Write(query)
		require.NoError(t, err)
		reply := make([]byte, 8+20+24)
		_, err = io.ReadFull(conn, reply)
		require.NoError(t, err)
		require.Equal(t, byte(rtrMockPDUCacheResp), reply[1])
		require.Equal(t, byte(rtrMockPDUIPv4Prefix), reply[9])
		require.Equal(t, []byte{1, 24, 24, 0, 10, 254, 0, 0}, reply[16:24])
		require.Equal(t, uint32(65999), binary.BigEndian.Uint32(reply[24:28]))
		require.Equal(t, byte(rtrMockPDUEndOfData), reply[29])
		require.Equal(t, uint32(7), binary.BigEndian.Uint32(reply[36:40]))
	}
}

// rtrQuerySocket owns one real handler lifecycle. The caller MUST use the
// connection only until test cleanup. Cleanup MUST close both ends and the
// listener, and join the handler even when an assertion fails.
func rtrQuerySocket(t *testing.T, cache *rtrMockCache) net.Conn {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			done <- acceptErr
			return
		}
		if deadlineErr := conn.SetDeadline(time.Now().Add(5 * time.Second)); deadlineErr != nil {
			_ = conn.Close() //nolint:errcheck // Preserve the deadline failure.
			done <- deadlineErr
			return
		}
		rtrMockHandleConn(conn, cache)
		done <- nil
	}()
	var conn net.Conn
	t.Cleanup(func() {
		if conn != nil {
			_ = conn.Close()
		} //nolint:errcheck // Cleanup unblocks the owned handler.
		_ = listener.Close() //nolint:errcheck // Cleanup unblocks Accept if Dial failed.
		select {
		case handlerErr := <-done:
			if handlerErr != nil {
				t.Errorf("query socket handler: %v", handlerErr)
			}
		case <-time.After(6 * time.Second):
			t.Error("query socket handler did not stop after cleanup")
		}
	})
	conn, err = (&net.Dialer{Timeout: time.Second}).DialContext(t.Context(), "tcp", listener.Addr().String())
	require.NoError(t, err)
	require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
	return conn
}
