package tacacs

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

// closeIgnore closes the given closer, discarding any error.
// Used in test defers where the close error is irrelevant.
func closeIgnore(c io.Closer) { c.Close() } //nolint:errcheck // test cleanup

// listenTCP creates a test TCP listener using ListenConfig (noctx-safe).
func listenTCP(t *testing.T) net.Listener {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	return ln
}

// testTacacsServer is a minimal TACACS+ server for testing.
// It accepts one connection, reads a packet, and sends a canned reply.
type testTacacsServer struct {
	listener net.Listener
	key      []byte
	replyFn  func(hdr PacketHeader, body []byte) []byte // returns reply body
	headerFn func(req PacketHeader, reply PacketHeader) PacketHeader
}

func newTestServer(t *testing.T, key []byte, replyFn func(PacketHeader, []byte) []byte) *testTacacsServer {
	t.Helper()
	ln := listenTCP(t)

	srv := &testTacacsServer{listener: ln, key: key, replyFn: replyFn}
	go srv.serve()
	return srv
}

func newTestServerWithHeader(t *testing.T, key []byte, replyFn func(PacketHeader, []byte) []byte, headerFn func(PacketHeader, PacketHeader) PacketHeader) *testTacacsServer {
	t.Helper()
	srv := newTestServer(t, key, replyFn)
	srv.headerFn = headerFn
	return srv
}

func (s *testTacacsServer) addr() string { return s.listener.Addr().String() }

func (s *testTacacsServer) close() { closeIgnore(s.listener) }

func (s *testTacacsServer) serve() {
	conn, err := s.listener.Accept()
	if err != nil {
		return // listener closed
	}
	defer func() { closeIgnore(conn) }()

	// Read header.
	hdrBuf := make([]byte, hdrLen)
	if _, err := io.ReadFull(conn, hdrBuf); err != nil {
		return
	}
	hdr, err := UnmarshalPacketHeader(hdrBuf)
	if err != nil {
		return
	}

	// Read body.
	body := make([]byte, hdr.Length)
	if _, err := io.ReadFull(conn, body); err != nil {
		return
	}

	// Decrypt request.
	if len(s.key) > 0 {
		Encrypt(body, hdr.SessionID, s.key, hdr.Version, hdr.SeqNo)
	}

	// Generate reply.
	replyBody := s.replyFn(hdr, body)

	// Build reply packet.
	replyHdr := PacketHeader{
		Version:   hdr.Version,
		Type:      hdr.Type,
		SeqNo:     hdr.SeqNo + 1, // server increments
		SessionID: hdr.SessionID,
		Length:    uint32(len(replyBody)),
	}
	if s.headerFn != nil {
		replyHdr = s.headerFn(hdr, replyHdr)
	}

	replyWire := replyHdr.MarshalBinary()
	if len(s.key) > 0 {
		encrypted := make([]byte, len(replyBody))
		copy(encrypted, replyBody)
		Encrypt(encrypted, replyHdr.SessionID, s.key, replyHdr.Version, replyHdr.SeqNo)
		replyWire = append(replyWire, encrypted...)
	} else {
		replyWire = append(replyWire, replyBody...)
	}

	if _, err := conn.Write(replyWire); err != nil {
		return // test server best-effort
	}
}

// passReply returns an AuthenReply body with PASS status.
func passReply() func(PacketHeader, []byte) []byte {
	return func(_ PacketHeader, _ []byte) []byte {
		msg := "Welcome"
		body := make([]byte, 6+len(msg))
		body[0] = 0x01 // PASS
		body[1] = 0x00
		binary.BigEndian.PutUint16(body[2:4], uint16(len(msg)))
		binary.BigEndian.PutUint16(body[4:6], 0)
		copy(body[6:], msg)
		return body
	}
}

// failReply returns an AuthenReply body with FAIL status.
func failReply() func(PacketHeader, []byte) []byte {
	return func(_ PacketHeader, _ []byte) []byte {
		msg := "Access denied"
		body := make([]byte, 6+len(msg))
		body[0] = 0x02 // FAIL
		body[1] = 0x00
		binary.BigEndian.PutUint16(body[2:4], uint16(len(msg)))
		binary.BigEndian.PutUint16(body[4:6], 0)
		copy(body[6:], msg)
		return body
	}
}

// VALIDATES: AC-1 -- TACACS+ auth with valid credentials succeeds.
// PREVENTS: client unable to authenticate against a working server.
func TestTacacsClientAuthenticatePass(t *testing.T) {
	key := []byte("test-key")
	srv := newTestServer(t, key, passReply())
	defer srv.close()

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})

	reply, err := client.Authenticate("admin", "secret", "ssh", "10.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, uint8(0x01), reply.Status, "should be PASS")
	assert.Equal(t, "Welcome", reply.ServerMsg)
}

// VALIDATES: AC-2 -- TACACS+ auth with invalid credentials returns FAIL.
// PREVENTS: FAIL status interpreted as connection error.
func TestTacacsClientAuthenticateFail(t *testing.T) {
	key := []byte("test-key")
	srv := newTestServer(t, key, failReply())
	defer srv.close()

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})

	reply, err := client.Authenticate("admin", "wrong", "ssh", "10.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, uint8(0x02), reply.Status, "should be FAIL")
}

// VALIDATES: AC-11 -- first server down, second server responds.
// PREVENTS: client giving up after first server failure.
func TestTacacsClientServerFailover(t *testing.T) {
	key := []byte("test-key")
	srv := newTestServer(t, key, passReply())
	defer srv.close()

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{
			{Address: "127.0.0.1:1", Key: key}, // unreachable (port 1)
			{Address: srv.addr(), Key: key},    // working
		},
		Timeout: 500 * time.Millisecond,
	})

	reply, err := client.Authenticate("admin", "secret", "ssh", "10.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, uint8(0x01), reply.Status, "should get PASS from second server")
}

// VALIDATES: AC-4 -- all servers unreachable returns error.
// PREVENTS: silent success when no server responds.
func TestTacacsClientAllServersDown(t *testing.T) {
	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{
			{Address: "127.0.0.1:1", Key: []byte("key")},
			{Address: "127.0.0.1:2", Key: []byte("key")},
		},
		Timeout: 200 * time.Millisecond,
	})

	reply, err := client.Authenticate("admin", "secret", "ssh", "10.0.0.1")
	assert.Error(t, err)
	assert.Nil(t, reply)
	assert.Contains(t, err.Error(), "unreachable")
}

// TestTacacsClientRejectsBadResponseHeader verifies that a syntactically valid
// TACACS+ packet with mismatched response header fields is rejected before body
// parsing.
//
// VALIDATES: response type, version, sequence, and flags are checked strictly.
// PREVENTS: accepting spoofed or desynchronized TACACS+ responses.
func TestTacacsClientRejectsBadResponseHeader(t *testing.T) {
	key := []byte("test-key")
	tests := []struct {
		name   string
		mutate func(PacketHeader, PacketHeader) PacketHeader
		want   string
	}{
		{
			name: "wrong type",
			mutate: func(_ PacketHeader, reply PacketHeader) PacketHeader {
				reply.Type = typeAuthorization
				return reply
			},
			want: "type mismatch",
		},
		{
			name: "wrong major version",
			mutate: func(_ PacketHeader, reply PacketHeader) PacketHeader {
				reply.Version = 0xD0
				return reply
			},
			want: "major version mismatch",
		},
		{
			name: "wrong sequence",
			mutate: func(_ PacketHeader, reply PacketHeader) PacketHeader {
				reply.SeqNo = 7
				return reply
			},
			want: "sequence mismatch",
		},
		{
			name: "unknown flags",
			mutate: func(_ PacketHeader, reply PacketHeader) PacketHeader {
				reply.Flags = 0x80
				return reply
			},
			want: "unsupported response flags",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServerWithHeader(t, key, passReply(), tt.mutate)
			defer srv.close()
			start := NewPAPAuthenStart("admin", "secret", "ssh", "10.0.0.1")
			client := NewTacacsClient(TacacsClientConfig{
				Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
				Timeout: 2 * time.Second,
			})
			buf := client.bufs.Get()
			defer client.bufs.Put(buf)
			pkt := &Packet{Header: PacketHeader{
				Version:   start.Version(),
				Type:      typeAuthentication,
				SeqNo:     1,
				SessionID: 0x01020304,
			}}

			reply, err := client.sendReceive(buf, start.MarshalBinaryInto,
				TacacsServer{Address: srv.addr(), Key: key}, pkt)
			require.Error(t, err)
			assert.Nil(t, reply)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

// VALIDATES: client timeout is respected.
// PREVENTS: client hanging indefinitely on slow server.
func TestTacacsClientTimeout(t *testing.T) {
	// Start a server that accepts but never responds.
	// listenTCP calls require.NoError internally.
	ln := listenTCP(t)
	defer func() { closeIgnore(ln) }()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		// Hold connection open but never send response.
		time.Sleep(5 * time.Second)
		closeIgnore(conn)
	}()

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: ln.Addr().String(), Key: []byte("key")}},
		Timeout: 200 * time.Millisecond,
	})

	start := time.Now()
	reply, err := client.Authenticate("admin", "secret", "ssh", "10.0.0.1")
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Nil(t, reply)
	assert.Less(t, elapsed, 2*time.Second, "should timeout quickly, not wait 5s")
}
