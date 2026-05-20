// Design: plan/spec-ipsec-7-ikev2-engine.md -- IKE UDP transport
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
}

// UDPTransport listens on a UDP socket and dispatches incoming IKE packets.
type UDPTransport struct {
	logger *slog.Logger
	conn   *net.UDPConn

	mu     sync.Mutex
	closed bool

	inbound chan Packet
}

// NewUDPTransport creates a transport listening on the given local address.
func NewUDPTransport(localAddr string, logger *slog.Logger) (*UDPTransport, error) {
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
		inbound: make(chan Packet, 64),
	}, nil
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
