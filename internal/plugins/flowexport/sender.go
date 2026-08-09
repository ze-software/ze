// Design: docs/architecture/flowexport/flow-export-0-umbrella.md -- Buffer pool and UDP sender

package flowexport

import (
	"fmt"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
)

// MaxDatagramSize is the maximum UDP payload to avoid fragmentation.
const MaxDatagramSize = 1400

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
type Sender struct {
	conn *net.UDPConn
	addr *net.UDPAddr

	datagramsSent atomic.Uint64
	bytesSent     atomic.Uint64
	errors        atomic.Uint64
}

// NewSender creates a UDP sender targeting the given collector.
// If sourceAddress is non-empty, the UDP socket binds to that local IP.
func NewSender(address string, port int, sourceAddress string) (*Sender, error) {
	addr, err := netip.ParseAddr(address)
	if err != nil {
		return nil, err
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
		conn: conn,
		addr: udpAddr,
	}, nil
}

// Send transmits buf as a single UDP datagram.
func (s *Sender) Send(buf []byte) error {
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

// Close shuts down the UDP socket.
func (s *Sender) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
