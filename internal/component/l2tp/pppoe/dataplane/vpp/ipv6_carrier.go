// Design: docs/architecture/l2tp/bng-5-pppoe.md -- IPv6 control over the sole PPP reader
// Related: ipv6_wire.go -- bounded IPv6 admission and encoding; socket.go -- owner
package vpp

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/ndp"
)

// Each session retains at most eight pending messages per service. A full
// queue drops and counts the newest message; PPP negotiation never waits for
// either consumer. Fixed-value messages own bytes after the PPP read returns.
const ipv6ControlQueueDepth = 8
const ipv6DHCPPayloadMax = ppp.MaxFrameLen - 40 - 8

// ipv6Carrier borrows its session's socket and starts no goroutine. Safe for
// concurrent use with one reader per queue. Close unregisters and wakes reads;
// common PPP MUST join readers and send its final RA before calling Close.
type ipv6Carrier struct {
	socket  *controlSocket
	local   netip.Addr
	peer    netip.Addr
	rs      chan struct{}
	dhcp    chan ipv6DHCPMessage
	done    chan struct{}
	closed  atomic.Bool
	writeMu sync.Mutex
	once    sync.Once
}

type ipv6DHCPMessage struct {
	payload [ipv6DHCPPayloadMax]byte
	length  int
	peer    netip.AddrPort
}

func (s *controlSocket) openIPv6Carrier(local, peer netip.Addr) (*ipv6Carrier, error) {
	if !local.Is6() {
		return nil, errors.New("pppoe-vpp: carrier local address must be IPv6")
	}
	if !peer.Is6() {
		return nil, errors.New("pppoe-vpp: carrier peer address must be IPv6")
	}
	if local.Is4In6() {
		return nil, errors.New("pppoe-vpp: carrier local address cannot be mapped IPv4")
	}
	if peer.Is4In6() {
		return nil, errors.New("pppoe-vpp: carrier peer address cannot be mapped IPv4")
	}
	if !local.IsLinkLocalUnicast() {
		return nil, errors.New("pppoe-vpp: IPv6 carrier requires a local link-local address")
	}
	if !peer.IsLinkLocalUnicast() {
		return nil, errors.New("pppoe-vpp: IPv6 carrier requires a peer link-local address")
	}
	if local == peer {
		return nil, errors.New("pppoe-vpp: IPv6 carrier link-local addresses coincide")
	}
	s.carrierMu.Lock()
	defer s.carrierMu.Unlock()
	if s.closed {
		return nil, io.ErrClosedPipe
	}
	if s.carrier != nil {
		return nil, errors.New("pppoe-vpp: IPv6 control carrier is already open")
	}
	carrier := &ipv6Carrier{socket: s, local: local, peer: peer,
		rs:   make(chan struct{}, ipv6ControlQueueDepth),
		dhcp: make(chan ipv6DHCPMessage, ipv6ControlQueueDepth), done: make(chan struct{})}
	s.carrier = carrier
	return carrier, nil
}

func (c *ipv6Carrier) ReadRS(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.closed.Load() {
		return io.ErrClosedPipe
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return io.ErrClosedPipe
	case <-c.rs:
		if c.closed.Load() {
			return io.ErrClosedPipe
		}
		return nil
	}
}

func (c *ipv6Carrier) ReadDHCPv6(ctx context.Context, buf []byte) (int, netip.AddrPort, error) {
	if err := ctx.Err(); err != nil {
		return 0, netip.AddrPort{}, err
	}
	if c.closed.Load() {
		return 0, netip.AddrPort{}, io.ErrClosedPipe
	}
	select {
	case <-ctx.Done():
		return 0, netip.AddrPort{}, ctx.Err()
	case <-c.done:
		return 0, netip.AddrPort{}, io.ErrClosedPipe
	case message := <-c.dhcp:
		if c.closed.Load() {
			return 0, netip.AddrPort{}, io.ErrClosedPipe
		}
		if len(buf) < message.length {
			return 0, netip.AddrPort{}, io.ErrShortBuffer
		}
		return copy(buf, message.payload[:message.length]), message.peer, nil
	}
}

// RFC 4861 Section 4.2: "MUST be the link-local address assigned to the
// interface from which this message is sent."
func (c *ipv6Carrier) WriteRA(payload []byte, hopLimit int) error {
	// RFC 4861 Section 6.1.2: "The IP Hop Limit field has a value of 255,
	// i.e., the packet could not possibly have been forwarded by a router."
	if hopLimit != ndp.MessageHopLimit {
		return errors.New("pppoe-vpp: RA hop limit must be 255")
	}
	if len(payload) < 16 {
		return errors.New("pppoe-vpp: RA body is shorter than its header")
	}
	if payload[0] != ndp.ICMPv6TypeRouterAdvertisement {
		return errors.New("pppoe-vpp: RA body has another ICMPv6 type")
	}
	if payload[1] != 0 {
		return errors.New("pppoe-vpp: RA body has a nonzero code")
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.closed.Load() {
		return io.ErrClosedPipe
	}
	var frame [ppp.MaxFrameBufLen]byte
	// RFC 4861 Section 4.2; RFC 8200 Section 3.
	n, err := writeIPv6Control(frame[:], c.local, ipv6AllNodes, 58, byte(hopLimit), payload)
	if err != nil {
		return err
	}
	body := frame[ppp.ProtoFieldLen+40 : n]
	body[2], body[3] = 0, 0
	// RFC 4443 Section 2.3.
	binary.BigEndian.PutUint16(body[2:4], ipv6Checksum(c.local, ipv6AllNodes, 58, body))
	_, err = c.socket.Write(frame[:n])
	return err
}

// RFC 9915 Section 7.2: "Clients MUST listen for DHCP messages on UDP port
// 546."
func (c *ipv6Carrier) WriteDHCPv6(payload []byte, peer netip.AddrPort) (int, error) {
	// RFC 9915 Section 18.3.10. The worker passes the actual request source
	// returned by ReadDHCPv6, through this same session-bound PPP endpoint.
	if !validIPv6ControlPeer(peer.Addr()) {
		return 0, errors.New("pppoe-vpp: DHCPv6 response target must be an IPv6 unicast address")
	}
	if peer.Port() != 546 {
		return 0, errors.New("pppoe-vpp: DHCPv6 response target port must be 546")
	}
	if len(payload) < 4 {
		return 0, errors.New("pppoe-vpp: DHCPv6 payload lacks a message header")
	}
	if len(payload) > ipv6DHCPPayloadMax {
		return 0, errors.New("pppoe-vpp: DHCPv6 payload exceeds the PPP frame bound")
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.closed.Load() {
		return 0, io.ErrClosedPipe
	}
	var datagram [ppp.MaxFrameLen - 40]byte
	binary.BigEndian.PutUint16(datagram[0:2], 547)
	binary.BigEndian.PutUint16(datagram[2:4], 546)
	binary.BigEndian.PutUint16(datagram[4:6], uint16(8+len(payload)))
	copy(datagram[8:], payload)
	// RFC 8200 Section 8.1.
	checksum := ipv6Checksum(c.local, peer.Addr(), 17, datagram[:8+len(payload)])
	if checksum == 0 {
		checksum = 0xffff
	}
	binary.BigEndian.PutUint16(datagram[6:8], checksum)
	var frame [ppp.MaxFrameBufLen]byte
	// RFC 8200 Section 3.
	n, err := writeIPv6Control(frame[:], c.local, peer.Addr(), 17, 1, datagram[:8+len(payload)])
	if err != nil {
		return 0, err
	}
	if _, err := c.socket.Write(frame[:n]); err != nil {
		return 0, err
	}
	return len(payload), nil
}

func (c *ipv6Carrier) Close() error {
	c.once.Do(func() {
		c.writeMu.Lock()
		defer c.writeMu.Unlock()
		c.socket.carrierMu.Lock()
		defer c.socket.carrierMu.Unlock()
		c.closed.Store(true)
		if c.socket.carrier == c {
			c.socket.carrier = nil
		}
		close(c.done)
	})
	return nil
}

var _ ppp.IPv6Carrier = (*ipv6Carrier)(nil)
