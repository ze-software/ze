// Design: plan/learned/740-ipsec-7-ikev2-engine.md -- IKE UDP transport
// RFC: rfc/short/rfc7296.md -- IKE uses UDP port 500 (Section 2.1)
package transport

import (
	"errors"
	"log/slog"
	"net"
	"sync"
)

const (
	IKEPort    = 500
	MaxMsgSize = 3000 // RFC 7296 Section 2.1: handle up to 3000 bytes
)

var (
	ErrClosed      = errors.New("transport: closed")
	ErrSendFailed  = errors.New("transport: send failed")
	ErrNoLocalAddr = errors.New("transport: no local address configured")
)

// Packet is an inbound IKE message with its source address.
type Packet struct {
	Data       []byte
	RemoteAddr *net.UDPAddr
	LocalAddr  *net.UDPAddr

	// NATT records that this datagram arrived on the NAT-T socket, port 4500.
	//
	// RFC 7296 Section 2.11 MUST: an implementation
	// "MUST specify the address and port at which the request was received as the source address and port in the response".
	// A handler therefore needs to know which of the two sockets delivered the
	// request, and the socket is what knows. Run stamps it from the transport's own
	// role, so no handler infers it from a port number.
	//
	// The zero value is the plain IKE socket. That is what a hand-built Packet in a
	// test means. It is also the unfloated default of RFC 7296 Section 2.23.
	NATT bool
}

// UDPTransport listens on a UDP socket and dispatches incoming IKE packets.
type UDPTransport struct {
	logger *slog.Logger
	conn   *net.UDPConn

	// natT records that this socket is the NAT-T one, port 4500.
	//
	// RFC 3948 Section 2.2 puts a four-octet non-ESP marker on every IKE message
	// that port carries, so a sender needs to know which socket it holds.
	// The role is fixed at construction and never inferred from the bind port.
	// A port comparison reads the wrong answer under the ze.test.ike.port override,
	// where neither socket carries a well-known port.
	natT bool

	mu     sync.Mutex
	closed bool

	inbound chan Packet
}

// NewUDPTransport creates a transport listening on the given local address.
// The socket carries plain IKE, so it adds no non-ESP marker.
func NewUDPTransport(localAddr string, logger *slog.Logger) (*UDPTransport, error) {
	return newTransport(localAddr, false, logger)
}

// NewNATTTransport creates the NAT-T transport, the one RFC 7296 Section 2.23
// reserves for UDP-encapsulated ESP and IKE.
//
// IsNATT reports true for the result, so every sender that holds it frames its
// messages with the non-ESP marker of RFC 3948 Section 2.2.
func NewNATTTransport(localAddr string, logger *slog.Logger) (*UDPTransport, error) {
	return newTransport(localAddr, true, logger)
}

func newTransport(localAddr string, natT bool, logger *slog.Logger) (*UDPTransport, error) {
	addr, err := net.ResolveUDPAddr("udp4", localAddr)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return nil, err
	}
	return &UDPTransport{
		logger:  logger,
		conn:    conn,
		natT:    natT,
		inbound: make(chan Packet, 64),
	}, nil
}

// IsNATT reports whether this socket is the NAT-T one.
//
// It fails closed. A nil transport reads false, so a caller with no socket adds no
// marker and sends nothing (ai/rules/fail-closed-guards.md).
func (t *UDPTransport) IsNATT() bool {
	if t == nil {
		return false
	}
	return t.natT
}

// Recv returns the channel of inbound packets.
func (t *UDPTransport) Recv() <-chan Packet {
	return t.inbound
}

// Send writes a raw IKE message to the remote address.
func (t *UDPTransport) Send(data []byte, remote *net.UDPAddr) error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return ErrClosed
	}
	t.mu.Unlock()
	_, err := t.conn.WriteToUDP(data, remote)
	if err != nil {
		return errors.Join(ErrSendFailed, err)
	}
	return nil
}

// Conn returns the underlying UDP connection for use by keepalive senders.
func (t *UDPTransport) Conn() *net.UDPConn {
	return t.conn
}

// LocalAddr returns the local address the transport is listening on.
func (t *UDPTransport) LocalAddr() net.Addr {
	return t.conn.LocalAddr()
}

// Run reads packets from the UDP socket until Close is called.
func (t *UDPTransport) Run() {
	buf := make([]byte, MaxMsgSize)
	for {
		n, remoteAddr, err := t.conn.ReadFromUDP(buf)
		if err != nil {
			t.mu.Lock()
			closed := t.closed
			t.mu.Unlock()
			if closed {
				return
			}
			t.logger.Warn("ike transport: read error", "error", err)
			continue
		}
		if n < 28 { // IKE header is 28 bytes minimum
			continue
		}
		pkt := Packet{
			Data:       make([]byte, n),
			RemoteAddr: remoteAddr,
			LocalAddr:  t.localUDPAddr(),
			NATT:       t.natT,
		}
		copy(pkt.Data, buf[:n])

		select {
		case t.inbound <- pkt:
		default:
			t.logger.Warn("ike transport: inbound queue full, dropping packet",
				"remote", remoteAddr)
		}
	}
}

func (t *UDPTransport) localUDPAddr() *net.UDPAddr {
	addr, ok := t.conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil
	}
	return addr
}

// Close shuts down the transport.
func (t *UDPTransport) Close() error {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()
	return t.conn.Close()
}
