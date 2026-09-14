// Design: (none -- RFC 8907 Section 10.5.2, the client half)
// Detail: client.go -- trySend, the only producer that sets a request flag bit
// Detail: packet.go -- (*Packet).MarshalInto, the last function before the socket
//
// VALIDATES: RFC 8907 Section 10.5.2, "Clients MUST be implemented in a way
// that requires explicit configuration to enable the use of
// TAC_PLUS_UNENCRYPTED_FLAG."
// PREVENTS: unencrypted mode reached by configuration alone. The shape that
// would do it is a server entry written without a shared secret: with no key
// there is no pseudo-pad, so a client that sent anyway would put a PAP
// User-Password on the wire in cleartext while nobody asked for unencrypted
// mode.
//
// This file sits beside rfc8907_obfuscation_test.go rather than inside it:
// that file carries `RFC requirement:` tags on every test it holds, and an
// edit there is read against the tags it already carries.

package tacacs

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wireRecorder accepts one connection and reads it to EOF without answering.
// The client under test either writes one request and waits out its timeout, or
// is refused before it writes at all; either way the connection ends with the
// client closing it, so io.ReadAll answers with everything that arrived.
type wireRecorder struct {
	listener net.Listener
	wire     chan []byte
}

func newWireRecorder(t *testing.T) *wireRecorder {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	r := &wireRecorder{listener: ln, wire: make(chan []byte, 1)}
	t.Cleanup(func() { closeIgnore(ln) })
	go r.serve()
	return r
}

func (r *wireRecorder) addr() string { return r.listener.Addr().String() }

func (r *wireRecorder) serve() {
	conn, err := r.listener.Accept()
	if err != nil {
		return // listener closed
	}
	defer func() { closeIgnore(conn) }()

	// A reset connection still answers with the octets that arrived before it,
	// and those are what the test judges, so the read error is not read.
	wire, _ := io.ReadAll(conn)
	select {
	case r.wire <- wire:
	default:
	}
}

// recorded answers the octets the client put on the connection, once the
// connection is over. A client that wrote nothing answers an empty slice.
func (r *wireRecorder) recorded(t *testing.T) []byte {
	t.Helper()
	select {
	case wire := <-r.wire:
		return wire
	case <-time.After(5 * time.Second):
		t.Fatal("the recorder never saw the connection end")
		return nil
	}
}

// TestRFC8907UnencryptedModeIsNotReachableFromConfiguration drives the client
// through its configuration entry point in each of the two states an operator
// can write, and reads the octets that reached the socket in each.
//
// The absent shared secret is the state that matters. It is the only
// configuration from which an implementation could decide, on its own, that the
// body travels in the clear, and Section 4.5 shows why that decision is not
// available: TAC_PLUS_UNENCRYPTED_FLAG is how a sender announces an unobfuscated
// body, and Section 10.5.2 closes it to a client.
//
// RFC requirement: RFC8907-10-1 positive -- neither configuration state ze
// exposes enables TAC_PLUS_UNENCRYPTED_FLAG. With a shared secret configured,
// the request that reaches the socket carries a flag octet with 0x01 clear; with
// the shared secret left out, the client writes no bytes at all rather than
// falling back to an unencrypted send (client.go trySend, packet.go
// (*Packet).MarshalInto).
func TestRFC8907UnencryptedModeIsNotReachableFromConfiguration(t *testing.T) {
	t.Run("a shared secret is configured", func(t *testing.T) {
		recorder := newWireRecorder(t)
		client := NewTacacsClient(TacacsClientConfig{
			Servers: []TacacsServer{{Address: recorder.addr(), Key: []byte("obfuscation-key")}},
			Timeout: 500 * time.Millisecond,
		})

		reply, err := client.Authenticate("admin", "secret", "ssh", "10.0.0.1")
		require.Error(t, err, "the recorder never answers, so the exchange cannot succeed")
		assert.Nil(t, reply)

		wire := recorder.recorded(t)
		require.GreaterOrEqual(t, len(wire), hdrLen, "the client sent no request to judge")
		assert.Zero(t, wire[3]&FlagUnencrypted,
			"the client announced TAC_PLUS_UNENCRYPTED_FLAG on a request, which Section 10.5.2 forbids")
	})

	t.Run("no shared secret is configured", func(t *testing.T) {
		recorder := newWireRecorder(t)
		client := NewTacacsClient(TacacsClientConfig{
			Servers: []TacacsServer{{Address: recorder.addr()}},
			Timeout: 500 * time.Millisecond,
		})

		reply, err := client.Authenticate("admin", "secret", "ssh", "10.0.0.1")
		require.Error(t, err, "a server with no shared secret was used for authentication")
		assert.Nil(t, reply)

		wire := recorder.recorded(t)
		assert.Empty(t, wire,
			"the client sent a packet with no shared secret configured: the PAP body had no pseudo-pad to obfuscate it")
	})
}
