// Design: docs/architecture/flowexport/flow-export-0-umbrella.md -- Buffer pool and UDP sender

package flowexport

import (
	"fmt"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
)

// MaxDatagramSize is the largest UDP payload a collector can be configured
// to receive, and the size of every pooled buffer. It is the ceiling of the
// collector's max-datagram-size leaf, never the value in use: a Sender
// carries that value, and each encoder chunks to Sender.MaxDatagram.
const MaxDatagramSize = 1400

// DatagramSizeMin is the smallest collector payload bound. Together with
// IPv6 and UDP headers it fits the 512-octet unknown-PMTU recommendation.
const DatagramSizeMin = 464

// DatagramSizeDefault is the UDP payload bound when the PMTU is unknown.
// RFC 7011 Section 10.3.3: "If the PMTU is unknown, a maximum packet size
// of 512 octets SHOULD be used." Reserve 40 IPv6 and 8 UDP header octets.
// Operators MUST subtract all headers from a known PMTU before raising it.
const DatagramSizeDefault = DatagramSizeMin

// bufPool provides reusable 1400-byte buffers for datagram encoding.
// Stores *[]byte so the pool does not allocate on Put.
var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, MaxDatagramSize)
		return &b
	},
}

// GetBuf returns a 1400-byte buffer from the pool.
func GetBuf() *[]byte {
	bp, ok := bufPool.Get().(*[]byte)
	if !ok {
		b := make([]byte, MaxDatagramSize)
		return &b
	}
	return bp
}

// PutBuf returns a buffer to the pool.
func PutBuf(b *[]byte) {
	if b == nil {
		return
	}
	bufPool.Put(b)
}

// Sender sends pre-encoded datagrams to a single collector via UDP.
// Send, Stats and Close are safe for concurrent use. Encoders sharing a Sender
// MUST serialize Sequence, encoding, Send and AdvanceSequence as one operation.
type Sender struct {
	conn *net.UDPConn
	addr *net.UDPAddr

	// maxDatagram is the largest payload Send accepts, the collector's
	// configured max-datagram-size. Encoders chunk to it.
	maxDatagram int

	// sequence belongs to the collector's transport session, not to a
	// record template. Counter and flow encoders share it under exporter.mu.
	sequence uint32

	datagramsSent atomic.Uint64
	bytesSent     atomic.Uint64
	errors        atomic.Uint64
}

// NewSender creates a UDP sender targeting the given collector.
// If sourceAddress is non-empty, the UDP socket binds to that local IP.
// maxDatagram is the collector's UDP payload bound; it MUST lie within
// DatagramSizeMin..MaxDatagramSize, which CollectorConfig.validate enforces
// before any Sender is built.
// The owner MUST call Close when the collector is removed or the exporter stops.
func NewSender(address string, port int, sourceAddress string, maxDatagram int) (*Sender, error) {
	addr, err := netip.ParseAddr(address)
	if err != nil {
		return nil, err
	}
	if maxDatagram < DatagramSizeMin || maxDatagram > MaxDatagramSize {
		return nil, fmt.Errorf("max-datagram-size %d out of range %d-%d", maxDatagram, DatagramSizeMin, MaxDatagramSize)
	}

	udpAddr := &net.UDPAddr{
		IP:   addr.AsSlice(),
		Port: port,
	}

	network := "udp4"
	if addr.Is6() {
		network = "udp6"
	}

	var localAddr *net.UDPAddr
	if sourceAddress != "" {
		srcIP := net.ParseIP(sourceAddress)
		if srcIP == nil {
			// Reject rather than fall back to a wildcard (nil IP) bind, which
			// would silently ignore the operator-configured source.
			return nil, fmt.Errorf("invalid source-address %q", sourceAddress)
		}
		localAddr = &net.UDPAddr{IP: srcIP}
	}

	conn, err := net.DialUDP(network, localAddr, udpAddr)
	if err != nil {
		if localAddr != nil {
			// Attribute the failure to the source binding so a misconfigured
			// or not-yet-assigned source-address is diagnosable in the log.
			return nil, fmt.Errorf("bind source-address %s: %w", sourceAddress, err)
		}
		return nil, err
	}

	return &Sender{
		conn:        conn,
		addr:        udpAddr,
		maxDatagram: maxDatagram,
	}, nil
}

// MaxDatagram is the largest UDP payload this collector accepts. Every
// encoder bounds its datagrams to it (RFC 7011 Section 10.3.3).
func (s *Sender) MaxDatagram() int {
	return s.maxDatagram
}

// Sequence returns the next protocol sequence for this transport session.
// The exporter serializes encoding and sending for each collector.
func (s *Sender) Sequence() uint32 {
	return s.sequence
}

// AdvanceSequence accounts for the units defined by the collector's protocol:
// sent data records for IPFIX, sent packets for NetFlow, generated datagrams
// for sFlow. Unsigned addition provides the required modulo-2^32 wrap.
func (s *Sender) AdvanceSequence(count uint32) {
	s.sequence += count
}

// Send transmits buf as a single UDP datagram. Oversized payloads are
// refused and counted as send errors without reaching the socket.
func (s *Sender) Send(buf []byte) error {
	if len(buf) > s.maxDatagram {
		s.errors.Add(1)
		return fmt.Errorf("flow-export datagram size %d exceeds max-datagram-size %d", len(buf), s.maxDatagram)
	}
	_, err := s.conn.Write(buf)
	if err != nil {
		s.errors.Add(1)
		return err
	}
	s.datagramsSent.Add(1)
	s.bytesSent.Add(uint64(len(buf)))
	return nil
}

// Stats returns current export counters.
func (s *Sender) Stats() (datagrams, bytes, errors uint64) {
	return s.datagramsSent.Load(), s.bytesSent.Load(), s.errors.Load()
}

// Close shuts down the UDP socket. The owner MUST call it for every NewSender
// result before discarding the collector.
func (s *Sender) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
