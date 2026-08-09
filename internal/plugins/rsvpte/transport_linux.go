//go:build linux

// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- Linux raw IP socket for RSVP (proto 46)
// Related: transport.go -- Transport interface and Packet type
// Related: build.go -- message bytes sent over this socket
//
// RFC 2205 Section 3.1: RSVP messages are carried directly in IP with protocol
// number 46. We open an AF_INET SOCK_RAW socket on protocol 46. The kernel
// builds the outgoing IP header (IP_HDRINCL off) so Send only supplies the RSVP
// payload; on receive a raw IPv4 socket delivers the full datagram including the
// IP header, which we strip using the IHL field before handing the payload up.
// Requires CAP_NET_RAW.
package rsvpte

import (
	"fmt"
	"net/netip"
	"sync"
	"sync/atomic"

	"golang.org/x/sys/unix"
)

// rsvpProtocol is the IANA IP protocol number for RSVP (RFC 2205 Section 3.1).
const rsvpProtocol = 46

type rawTransport struct {
	fd     int
	recvCh chan Packet
	closeC chan struct{}
	closed atomic.Bool
	once   sync.Once
}

func openRawTransport(localAddr netip.Addr) (Transport, error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW, rsvpProtocol)
	if err != nil {
		return nil, fmt.Errorf("rsvp-te: open raw socket (proto %d) needs CAP_NET_RAW: %w", rsvpProtocol, err)
	}
	// Bind to the local router-id address so replies and source selection are
	// deterministic. A zero address leaves the kernel to choose per-route.
	if localAddr.Is4() {
		sa := &unix.SockaddrInet4{Addr: localAddr.As4()}
		if err := unix.Bind(fd, sa); err != nil {
			if cerr := unix.Close(fd); cerr != nil {
				logger().Warn("rsvp-te: close raw socket after bind failure", "err", cerr)
			}
			return nil, fmt.Errorf("rsvp-te: bind raw socket to %s: %w", localAddr, err)
		}
	}
	t := &rawTransport{
		fd:     fd,
		recvCh: make(chan Packet, 64),
		closeC: make(chan struct{}),
	}
	go t.readLoop()
	return t, nil
}

func (t *rawTransport) Send(dst netip.Addr, msg []byte) error {
	if !dst.Is4() {
		return fmt.Errorf("rsvp-te: only IPv4 destinations supported, got %s", dst)
	}
	sa := &unix.SockaddrInet4{Addr: dst.As4()}
	if err := unix.Sendto(t.fd, msg, 0, sa); err != nil {
		return fmt.Errorf("rsvp-te: sendto %s: %w", dst, err)
	}
	return nil
}

func (t *rawTransport) Recv() <-chan Packet { return t.recvCh }

func (t *rawTransport) Close() error {
	var err error
	t.once.Do(func() {
		t.closed.Store(true)
		close(t.closeC)
		// Closing the fd unblocks the Recvfrom in readLoop.
		err = unix.Close(t.fd)
	})
	return err
}

func (t *rawTransport) readLoop() {
	defer close(t.recvCh)
	buf := make([]byte, maxRSVPMessage+60) // RSVP payload + max IPv4 header
	for {
		n, from, err := unix.Recvfrom(t.fd, buf, 0)
		if err != nil {
			if t.closed.Load() {
				return
			}
			// Transient error (e.g. EINTR); keep reading.
			continue
		}
		payload, src, ok := stripIPv4Header(buf[:n], from)
		if !ok {
			continue
		}
		// Copy out of the shared buffer before queueing.
		cp := make([]byte, len(payload))
		copy(cp, payload)
		select {
		case t.recvCh <- Packet{Src: src, Payload: cp}:
		case <-t.closeC:
			return
		}
	}
}

// stripIPv4Header removes the leading IPv4 header a raw socket prepends to a
// received datagram and returns the RSVP payload plus the source address. The
// IHL (low nibble of byte 0) gives the header length in 32-bit words.
func stripIPv4Header(data []byte, from unix.Sockaddr) ([]byte, netip.Addr, bool) {
	if len(data) < 20 {
		return nil, netip.Addr{}, false
	}
	ihl := int(data[0]&0x0f) * 4
	if ihl < 20 || ihl > len(data) {
		return nil, netip.Addr{}, false
	}
	var src netip.Addr
	if sa4, ok := from.(*unix.SockaddrInet4); ok {
		src = netip.AddrFrom4(sa4.Addr)
	}
	return data[ihl:], src, true
}
