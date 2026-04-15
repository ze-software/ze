// Design: docs/research/l2tpv2-ze-integration.md -- L2TP UDP transport
// Related: reactor.go -- consumes UDPListener.RX() and calls Send

package l2tp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

// rxPoolSize is the number of in-flight receive slots. 64 was chosen over
// BFD's 16 because L2TP control traffic can burst during bulk session
// setup (one SCCRQ + many ICRQs). When full, readLoop blocks until the
// reactor releases a slot; that backpressure is preferable to unbounded
// allocation when a peer floods us. Tune upward if a future BNG workload
// shows steady-state RX queue depth approaching this limit.
const rxPoolSize = 64

// rxBufLen matches phase-1's pooled buffer size (see `pool.go`'s
// `poolBufSize`). RFC 2661 caps a single control message body via a
// 10-bit Length field (max 1023), but UDP datagrams can legitimately
// arrive larger (e.g., L2TP data frames carrying PPP). 1500 is the sweet
// spot: one Ethernet MTU, ample for control, bounded for DOS.
const rxBufLen = 1500

// UDPListener is the L2TP UDP transport. It binds a single unconnected UDP
// socket, reads datagrams into a pre-allocated slot pool, and delivers
// (peer, bytes, release) tuples over a channel. Send writes outbound bytes
// to the caller-supplied peer addr:port using `sendto()`-style semantics.
//
// Caller MUST call Stop after Start; Start is not idempotent. RX is safe
// for concurrent read only by a single consumer (the reactor).
type UDPListener struct {
	bind   netip.AddrPort
	logger *slog.Logger

	mu     sync.Mutex
	conn   *net.UDPConn
	rx     chan rxPacket
	stop   chan struct{}
	wg     sync.WaitGroup
	closed bool
}

// rxPacket carries one received datagram. The bytes slice aliases a slot
// from the listener's pool; the reactor MUST call release when done so the
// slot can be reused by readLoop.
type rxPacket struct {
	from    netip.AddrPort
	bytes   []byte
	release func()
}

var (
	errListenerAlreadyStarted = errors.New("l2tp: UDP listener already started")
	errListenerNotStarted     = errors.New("l2tp: UDP listener not started")
	errListenerRestart        = errors.New("l2tp: UDP listener was stopped and cannot restart")
)

// NewUDPListener constructs a listener bound to the given address. Start
// must be called before RX yields any packets.
func NewUDPListener(bind netip.AddrPort, logger *slog.Logger) *UDPListener {
	if logger == nil {
		logger = slog.Default()
	}
	return &UDPListener{bind: bind, logger: logger}
}

// Start binds the UDP socket and launches the read goroutine. Bind errors
// are reported synchronously; subsequent read errors are surfaced via the
// logger.
func (u *UDPListener) Start(ctx context.Context) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.conn != nil {
		return errListenerAlreadyStarted
	}
	if u.closed {
		return errListenerRestart
	}

	network := "udp4"
	isV6 := u.bind.Addr().Is6() && !u.bind.Addr().Is4In6()
	if isV6 {
		network = "udp6"
	}

	// Force IPV6_V6ONLY on IPv6 listeners so operator intent is honored:
	// a `[::]:1701` binding accepts only IPv6 traffic, leaving IPv4 on the
	// same port free for a separate `0.0.0.0:1701` listener. Without this
	// option Linux's default (`net.ipv6.bindv6only=0`) silently makes the
	// socket dual-stack.
	var lc net.ListenConfig
	if isV6 {
		lc.Control = func(_, _ string, c syscall.RawConn) error {
			var opErr error
			ctrlErr := c.Control(func(fd uintptr) {
				opErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_V6ONLY, 1)
			})
			if ctrlErr != nil {
				return ctrlErr
			}
			return opErr
		}
	}
	pc, err := lc.ListenPacket(ctx, network, u.bind.String())
	if err != nil {
		return fmt.Errorf("l2tp: bind %s: %w", u.bind, err)
	}
	conn, ok := pc.(*net.UDPConn)
	if !ok {
		if closeErr := pc.Close(); closeErr != nil {
			return fmt.Errorf("l2tp: unexpected PacketConn type %T (close failed: %w)", pc, closeErr)
		}
		return fmt.Errorf("l2tp: unexpected PacketConn type %T", pc)
	}

	u.conn = conn
	u.rx = make(chan rxPacket, rxPoolSize)
	u.stop = make(chan struct{})

	u.wg.Add(1)
	go u.readLoop()
	return nil
}

// Stop closes the socket, signals the reader, and waits for it to exit.
// Idempotent.
func (u *UDPListener) Stop() error {
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
	var closeErr error
	if conn != nil {
		closeErr = conn.Close()
	}
	u.wg.Wait()
	return closeErr
}

// RX returns the inbound channel. Closed after Stop drains the read loop.
func (u *UDPListener) RX() <-chan rxPacket { return u.rx }

// Addr returns the locally bound address. Returns a zero-value AddrPort
// if called before Start or after Stop. Useful for tests that bind to an
// ephemeral port (port=0) and need to learn the kernel-assigned port.
func (u *UDPListener) Addr() netip.AddrPort {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.conn == nil {
		return netip.AddrPort{}
	}
	udpAddr, ok := u.conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return netip.AddrPort{}
	}
	a, ok := netip.AddrFromSlice(udpAddr.IP)
	if !ok {
		return netip.AddrPort{}
	}
	if udpAddr.Port < 0 || udpAddr.Port > 65535 {
		return netip.AddrPort{}
	}
	return netip.AddrPortFrom(a.Unmap(), uint16(udpAddr.Port))
}

// Send writes bytes to the given peer. Returns errListenerNotStarted if
// Start has not run (or Stop has already been called).
func (u *UDPListener) Send(to netip.AddrPort, bytes []byte) error {
	u.mu.Lock()
	conn := u.conn
	u.mu.Unlock()
	if conn == nil {
		return errListenerNotStarted
	}
	raddr := &net.UDPAddr{IP: to.Addr().AsSlice(), Port: int(to.Port())}
	_, err := conn.WriteToUDP(bytes, raddr)
	return err
}

// readLoop is the listener's receiver goroutine. It owns the per-socket
// slot pool and pushes rxPacket values onto the rx channel. The consumer
// (reactor) MUST call release() on each packet when done.
//
// Allocation discipline: the backing array, free-slot channel, and
// per-slot release closures are created ONCE at goroutine start. No
// per-packet heap allocation.
func (u *UDPListener) readLoop() {
	defer u.wg.Done()
	defer close(u.rx)

	backing := make([]byte, rxPoolSize*rxBufLen)
	freeCh := make(chan int, rxPoolSize)
	releases := make([]func(), rxPoolSize)
	for i := range rxPoolSize {
		freeCh <- i
		slot := i
		releases[i] = func() { freeCh <- slot }
	}

	for {
		var idx int
		select {
		case idx = <-freeCh:
		case <-u.stop:
			return
		}

		buf := backing[idx*rxBufLen : (idx+1)*rxBufLen]
		n, raddr, err := u.conn.ReadFromUDPAddrPort(buf)
		if err != nil {
			freeCh <- idx
			u.mu.Lock()
			closed := u.closed
			u.mu.Unlock()
			if closed {
				return
			}
			continue
		}

		pkt := rxPacket{
			from:    raddr,
			bytes:   buf[:n],
			release: releases[idx],
		}
		select {
		case u.rx <- pkt:
		case <-u.stop:
			releases[idx]()
			return
		}
	}
}
