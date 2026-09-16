// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- the two IKE sockets, the DF send and the error queue
// RFC: rfc/short/rfc7296.md -- IKE uses UDP port 500 (Section 2.1)
// Related: udp_linux.go, udp_other.go -- the platform half: IP_RECVERR, the DF toggle, the drain
package transport

import (
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"syscall"

	"github.com/ze-software/ze/internal/core/probe"
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

// refusalQueueDepth bounds the refusals channel. One probe holds one SA's
// request window, and a refusal is one entry per DF datagram the path
// turned back, so the depth is the number of SAs that can be probing at
// once with room to spare; an entry that finds the channel full is dropped
// with a log line rather than blocking Run, which must keep reading IKE.
const refusalQueueDepth = 16

// SizeRefusal is one EMSGSIZE the kernel queued on this socket: a datagram
// sent with the DF bit was larger than the path, and either a router on the
// path said so with Fragmentation Needed or this host's kernel refused the
// send against its cached path MTU. Run reads the entry off the error queue
// and delivers it on Refusals for the engine to match against the SA that
// holds a probe.
type SizeRefusal struct {
	// Peer is the address and port the refused datagram was sent to, which
	// is the SA's remote endpoint: the socket is shared by every SA, and this
	// is how the engine tells whose probe was refused. A LOCAL refusal names
	// the address and port 0: the kernel fills that entry from the socket's
	// connected port (net/ipv4/ip_output.c __ip_append_data calls
	// ip_local_error with inet_dport), which an unconnected socket has not
	// got. The engine learns a local refusal from SendDF's own error, so the
	// event is confirmation, never the match.
	Peer netip.AddrPort
	// Outcome is ErrQueueMTUReported when MTU carries the reported next-hop
	// MTU, and ErrQueueMTUUnreported when the refusal named no usable value.
	Outcome probe.ErrQueueOutcome
	// MTU is the reported next-hop MTU in octets, meaningful only under
	// ErrQueueMTUReported.
	MTU uint32
	// Local is true when this host's own kernel refused the send against its
	// cached path MTU (the honor-cache mode), so nothing left the host and
	// MTU is the cache's estimate. Offender is then the zero Addr.
	Local bool
	// Offender is the router that answered, for a refusal from the network.
	Offender netip.Addr
	// NATT records which socket the refusal arrived on, as Packet.NATT does.
	NATT bool
}

// UDPTransport listens on a UDP socket and dispatches incoming IKE packets.
//
// Two goroutines meet on the socket. Run reads it, and every sender writes
// it through Send or SendDF under mu, which also guards closed. SendDF
// toggles a socket-wide option around its one write, so the lock is what
// keeps another SA's datagram from leaving with the probe's DF setting.
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

	// mu serializes every write on the socket and guards closed. Send and
	// SendDF MUST hold it for the whole write: SendDF changes a socket-wide
	// option for the duration of its write, and a write that interleaved
	// from another goroutine would leave under that option.
	mu     sync.Mutex
	closed bool

	inbound chan Packet
	// refusals carries the size refusals Run reads off the error queue.
	// Bounded by refusalQueueDepth; an entry that finds it full is dropped.
	refusals chan SizeRefusal
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
	t := &UDPTransport{
		logger:   logger,
		conn:     conn,
		natT:     natT,
		inbound:  make(chan Packet, 64),
		refusals: make(chan SizeRefusal, refusalQueueDepth),
	}
	if err := t.installErrorQueue(); err != nil {
		conn.Close() //nolint:errcheck // the socket is discarded with the error
		return nil, err
	}
	return t, nil
}

// IsNATT reports whether this socket is the NAT-T one.
//
// It fails closed. A nil transport reads false, so a caller with no socket adds no
// marker and sends nothing (ai/rules/evidence.md).
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

// Send writes a raw IKE message to the remote address under the kernel's
// default DF policy. It holds mu across the write, so it never interleaves
// with a SendDF on the same socket. Safe for concurrent use.
func (t *UDPTransport) Send(data []byte, remote *net.UDPAddr) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return ErrClosed
	}
	return t.write(data, remote)
}

// write is the one write on the socket, under mu, with the error queue's
// side effect handled. With IP_RECVERR on, the kernel hands an ICMP error
// about an EARLIER datagram to the next send on the socket as that send's
// failure (net/core/sock.c sock_alloc_send_pskb returns the pending sk_err
// before it allocates), and the datagram is never sent. So a failed write
// drains the queue first: a drain that found entries says the error was a
// report about earlier traffic, now delivered on Refusals, and the write is
// made once more. A drain that found nothing leaves an error about this
// write, and so does one that found a LOCAL entry: only this socket's own
// refused send queues one, under mu, so it is this write's cache refusal
// and a second attempt would draw the same answer.
func (t *UDPTransport) write(data []byte, remote *net.UDPAddr) error {
	_, err := t.conn.WriteToUDP(data, remote)
	if err == nil {
		return nil
	}
	entries, local := t.drainErrorQueue()
	if entries == 0 {
		return errors.Join(ErrSendFailed, err)
	}
	if local {
		return errors.Join(ErrSendFailed, err)
	}
	_, err = t.conn.WriteToUDP(data, remote)
	if err == nil {
		return nil
	}
	t.drainErrorQueue()
	return errors.Join(ErrSendFailed, err)
}

// SendDF writes a raw IKE message to the remote address with the DF mode df
// installed for that one datagram: DFHonorCache sets the DF bit and lets
// the kernel refuse a datagram larger than its cached path MTU with
// EMSGSIZE, DFBypassCache sets the bit and puts the datagram on the wire at
// full size, DFOff clears the bit so the kernel fragments. The socket's
// IP_MTU_DISCOVER is restored to its prior value before this returns, on
// every path, so a later Send by another SA leaves under the kernel default.
// It holds mu across the toggle and the write, so no other write on the
// socket can leave under the toggled option. Safe for concurrent use.
//
// Off Linux it answers probe.ErrDFUnsupported and writes nothing. A send the
// kernel refused against its cached path MTU is reported with EMSGSIZE
// joined to ErrSendFailed, and that is the caller's signal: the refusal's
// LOCAL entry reaches Refusals as well, with Local set and, on this
// unconnected socket, the peer's port unknown to the kernel.
func (t *UDPTransport) SendDF(data []byte, remote *net.UDPAddr, df probe.DFMode) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return ErrClosed
	}
	return t.writeWithDF(data, remote, df)
}

// Refusals returns the channel of size refusals the kernel queued for
// datagrams this socket sent with the DF bit. The engine reads it beside
// Recv. It is bounded by refusalQueueDepth and never blocks Run.
func (t *UDPTransport) Refusals() <-chan SizeRefusal {
	return t.refusals
}

// deliverQueuedError turns one error-queue entry into a SizeRefusal on the
// channel. Only an EMSGSIZE is a size refusal; the kernel queues every ICMP
// error for the socket once IP_RECVERR is on (a port unreachable, a host
// unreachable), and those are logged at Debug and dropped because nothing
// in the engine acts on them. A full channel drops the entry with a log
// line: Run must return to the socket, and the engine's retransmit timer
// carries the DF-clear copy in any case.
func (t *UDPTransport) deliverQueuedError(entry probe.QueuedError) {
	if entry.Errno != syscall.EMSGSIZE {
		t.logger.Debug("ike transport: queued error is not a size refusal, dropped",
			"peer", entry.Dest, "errno", entry.Errno, "offender", entry.Offender)
		return
	}
	refusal := SizeRefusal{
		Peer:     entry.Dest,
		Outcome:  entry.Outcome,
		MTU:      entry.MTU,
		Local:    entry.Local,
		Offender: entry.Offender,
		NATT:     t.natT,
	}
	select {
	case t.refusals <- refusal:
	default:
		t.logger.Warn("ike transport: refusal queue full, dropping size refusal",
			"peer", refusal.Peer, "mtu", refusal.MTU)
	}
}

// Conn returns the underlying UDP connection, for a socket option set on it
// (EnableESPInUDP). Nothing writes through it: every write goes through Send
// or SendDF, under mu, the NAT keepalive included.
func (t *UDPTransport) Conn() *net.UDPConn {
	return t.conn
}

// LocalAddr returns the local address the transport is listening on.
func (t *UDPTransport) LocalAddr() net.Addr {
	return t.conn.LocalAddr()
}

// Run reads packets from the UDP socket until Close is called. The loop has
// no bound of its own: it ends when the socket is closed.
//
// With IP_RECVERR on the socket, an ICMP error the kernel matched to a
// datagram this socket sent returns from the read as that errno once, with
// the entry on the error queue. A read error that is not the close is
// therefore first taken to the queue: a drain that found entries explains
// the error and delivers the size refusals; a drain that found nothing
// leaves an error the queue did not cause, which is logged as before.
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
			if entries, _ := t.drainErrorQueue(); entries > 0 {
				continue
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
