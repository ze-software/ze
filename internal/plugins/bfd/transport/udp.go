// Design: rfc/short/rfc5881.md -- single-hop UDP encapsulation (port 3784)
// Design: rfc/short/rfc5883.md -- multi-hop UDP encapsulation (port 4784)
// Related: socket.go -- Transport interface
// Related: loopback.go -- in-memory Transport used by engine tests
//
// Production UDP transport. A single UDP socket (per VRF, per port) reads
// packets into pool buffers, parses the mandatory section header to extract
// TTL metadata through IP_RECVTTL/IPV6_RECVHOPLIMIT, and feeds a channel
// drained by the engine's express loop.
//
// The TTL check for GTSM (single-hop MUST be 255) and the min-TTL check
// for multi-hop are enforced by the engine, not the transport. Keeping
// packet policy out of the transport lets us swap in a different socket
// back end (XDP/eBPF, raw socket) without touching the engine.
package transport

import (
	"errors"
	"net"
	"net/netip"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
)

// Port numbers from RFC 5881 Section 4 and RFC 5883 Section 5.
const (
	UDPPortSingleHopControl uint16 = 3784
	UDPPortEcho             uint16 = 3785
	UDPPortMultiHopControl  uint16 = 4784
)

// UDP is a production Transport bound to a single UDP port. One instance
// serves either the single-hop port or the multi-hop port; create two
// instances if the engine needs both.
//
// The transport does NOT open the socket in its zero value; call Start
// to bind and begin reading. Stop closes the socket, signals the reader
// goroutine, and waits for it to exit.
type UDP struct {
	// Bind is the local address to bind. Use netip.AddrPort with the
	// desired IP and the correct port (3784 or 4784). Pass an unspecified
	// address (netip.AddrFrom4([4]byte{}) or equivalent) to listen on
	// all interfaces.
	Bind netip.AddrPort

	// Mode records whether this socket handles single-hop or multi-hop
	// traffic. The engine uses this to route Inbounds to the correct
	// session key.
	Mode api.HopMode

	// VRF is the routing/VRF instance name for Inbound tagging. On
	// Linux, the caller is responsible for making the process see
	// the right VRF via SO_BINDTODEVICE or network namespace; this
	// field is a label, not a kernel primitive.
	VRF string

	// CloseErr is set if the receive goroutine observed a close error
	// from the kernel during shutdown. Read after Stop returns.
	CloseErr error

	mu     sync.Mutex
	conn   *net.UDPConn
	rx     chan Inbound
	stop   chan struct{}
	closed bool
	wg     sync.WaitGroup
}

// errUDPAlreadyStarted is returned by Start if the transport is already
// running.
var errUDPAlreadyStarted = errors.New("bfd: UDP transport already started")

// errUDPRestart is returned by Start if Stop has already been called on
// this instance.
var errUDPRestart = errors.New("bfd: UDP transport was stopped and cannot restart")

// errUDPNotStarted is returned by Send when called before Start.
var errUDPNotStarted = errors.New("bfd: UDP transport not started")

// Start binds the socket and launches the receive goroutine. Start is
// NOT idempotent; a second call returns an error.
func (u *UDP) Start() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.conn != nil {
		return errUDPAlreadyStarted
	}
	if u.closed {
		return errUDPRestart
	}

	laddr := &net.UDPAddr{IP: u.Bind.Addr().AsSlice(), Port: int(u.Bind.Port())}
	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return err
	}
	u.conn = conn
	u.rx = make(chan Inbound, 256)
	u.stop = make(chan struct{})

	u.wg.Add(1)
	go u.readLoop()
	return nil
}

// Stop signals the read goroutine, closes the socket, and waits for the
// goroutine to exit. Stop is idempotent. Any close error from the kernel
// is reported via UDP.CloseErr.
func (u *UDP) Stop() error {
	u.mu.Lock()
	if u.closed {
		u.mu.Unlock()
		return nil
	}
	u.closed = true
	conn := u.conn
	stop := u.stop
	u.mu.Unlock()

	if stop != nil {
		close(stop)
	}
	if conn != nil {
		if err := conn.Close(); err != nil {
			u.CloseErr = err
		}
	}
	u.wg.Wait()
	return u.CloseErr
}

// Send writes an Outbound to the peer address via the bound socket.
func (u *UDP) Send(out Outbound) error {
	u.mu.Lock()
	conn := u.conn
	u.mu.Unlock()
	if conn == nil {
		return errUDPNotStarted
	}
	port := UDPPortSingleHopControl
	if out.Mode == api.MultiHop {
		port = UDPPortMultiHopControl
	}
	raddr := &net.UDPAddr{IP: out.To.AsSlice(), Port: int(port)}
	_, err := conn.WriteToUDP(out.Bytes, raddr)
	return err
}

// RX returns the inbound-packet channel. The channel is closed when Stop
// has drained the read goroutine.
func (u *UDP) RX() <-chan Inbound { return u.rx }

// readLoop is the transport's receiver goroutine. It owns the per-socket
// pool buffers and pushes Inbounds onto the engine channel. The engine is
// responsible for calling Inbound.Release when done.
//
// Allocation discipline: every per-packet allocation is eliminated. The
// rx backing slice, the free-slot channel, and the per-slot release
// closures are all created ONCE at goroutine start and reused for the
// goroutine's lifetime. ReadFromUDP writes into a pre-existing slice;
// Inbound carries a pre-built release closure picked by slot index.
func (u *UDP) readLoop() {
	defer u.wg.Done()
	defer close(u.rx)

	// One contiguous backing array sliced into rxPoolSize independent
	// buffers, similar to the ze peerPool pattern. Each ReadFromUDP
	// targets one slice; the slice is released via Inbound.Release
	// when the engine has consumed it.
	const rxPoolSize = 16
	const rxBufLen = 128 // enough for 24 + 28 (SHA1) + future TLVs
	backing := make([]byte, rxPoolSize*rxBufLen)
	freeCh := make(chan int, rxPoolSize)
	releases := make([]func(), rxPoolSize)
	for i := range rxPoolSize {
		freeCh <- i
		slot := i // capture per iteration so each closure binds its own slot
		releases[i] = func() { freeCh <- slot }
	}

	for {
		// Acquire a free slot; block until the engine releases one.
		var idx int
		select {
		case idx = <-freeCh:
		case <-u.stop:
			return
		}

		buf := backing[idx*rxBufLen : (idx+1)*rxBufLen]
		n, raddr, err := u.conn.ReadFromUDP(buf)
		if err != nil {
			// Return the slot; Conn closed or stopping.
			freeCh <- idx
			u.mu.Lock()
			closed := u.closed
			u.mu.Unlock()
			if closed {
				return
			}
			continue
		}
		from, ok := netip.AddrFromSlice(raddr.IP)
		if !ok {
			freeCh <- idx
			continue
		}
		in := Inbound{
			From:    from.Unmap(),
			VRF:     u.VRF,
			Mode:    u.Mode,
			TTL:     0, // TTL extraction via IP_RECVTTL is future work
			Bytes:   buf[:n],
			release: releases[idx], // pre-built once; no per-packet alloc
		}
		select {
		case u.rx <- in:
		case <-u.stop:
			freeCh <- idx
			return
		}
	}
}
