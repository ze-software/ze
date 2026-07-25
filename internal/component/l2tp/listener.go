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
	"os"
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

// tunnelRxPoolSize is the slot count for an adopted per-tunnel socket. Far
// smaller than rxPoolSize: the kernel consumes this tunnel's DATA frames and
// only passes CONTROL frames up, and control traffic on an established tunnel is
// a trickle (ICRQ/ICCN/CDN/HELLO/StopCCN), not a burst. 8 slots x rxBufLen is
// 12 KiB per tunnel, which stays bounded on a BNG with thousands of tunnels.
const tunnelRxPoolSize = 8

// UDPListener is the L2TP UDP transport. It binds a single unconnected UDP
// socket, reads datagrams into a pre-allocated slot pool, and delivers
// (peer, bytes, release) tuples over a channel. Send writes outbound bytes
// to the caller-supplied peer addr:port using `sendto()`-style semantics.
//
// It ALSO reads control frames from the per-tunnel connected sockets handed to
// the kernel L2TP module (AdoptTunnelSocket). That is not an optimisation, it is
// required for correctness: see AdoptTunnelSocket.
//
// Caller MUST call Stop after Start; Start is not idempotent. RX is safe
// for concurrent read only by a single consumer (the reactor).
type UDPListener struct {
	bind   netip.AddrPort
	logger *slog.Logger

	mu      sync.Mutex
	conn    *net.UDPConn
	rx      chan rxPacket
	stop    chan struct{}
	wg      sync.WaitGroup
	closed  bool
	tunnels map[uint16]*net.UDPConn // adopted per-tunnel sockets, keyed by local tunnel id
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
	lc.Control = func(_, _ string, c syscall.RawConn) error {
		var opErr error
		ctrlErr := c.Control(func(fd uintptr) {
			// SO_REUSEPORT allows the kernel tunnel worker to create a
			// second connected socket on the same local port for
			// L2TP_CMD_TUNNEL_CREATE.
			opErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
			if opErr != nil {
				return
			}
			if isV6 {
				opErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_V6ONLY, 1)
			}
		})
		if ctrlErr != nil {
			return ctrlErr
		}
		return opErr
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
	// Adopted per-tunnel sockets must be closed here too: their readers block in
	// a socket read, NOT on u.stop, so without this Close the wg.Wait below never
	// returns.
	adopted := make([]*net.UDPConn, 0, len(u.tunnels))
	for tid, tc := range u.tunnels {
		adopted = append(adopted, tc)
		delete(u.tunnels, tid)
	}
	u.mu.Unlock()

	for _, tc := range adopted {
		tc.Close() //nolint:errcheck,gosec // shutdown; unblocks tunnelReadLoop
	}

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

// SocketFD returns the raw file descriptor of the UDP socket. Used by
// the kernel worker for L2TP_CMD_TUNNEL_CREATE which requires the fd
// of the connected UDP socket. The fd is valid for the lifetime of the
// listener. Phase 5 kernel integration.
func (u *UDPListener) SocketFD() (int, error) {
	u.mu.Lock()
	conn := u.conn
	u.mu.Unlock()
	if conn == nil {
		return -1, errListenerNotStarted
	}
	raw, err := conn.SyscallConn()
	if err != nil {
		return -1, fmt.Errorf("l2tp: syscall conn: %w", err)
	}
	var fd int
	if ctrlErr := raw.Control(func(fdp uintptr) {
		fd = int(fdp)
	}); ctrlErr != nil {
		return -1, fmt.Errorf("l2tp: control: %w", ctrlErr)
	}
	return fd, nil
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

// AdoptTunnelSocket starts reading control frames from fd, the connected UDP
// socket that the kernel worker created for tunnel tid and handed to the kernel
// L2TP module via L2TP_ATTR_FD.
//
// This is REQUIRED for correctness, not an optimisation. That socket is bound to
// the listener's own local address:port (SO_REUSEPORT) and connect()ed to the
// peer. Linux scores a connected socket ABOVE an unconnected one when demuxing an
// inbound datagram (compute_score in net/ipv4/udp.c gives +8 for a matching
// inet_daddr/inet_dport), so from the moment it exists EVERY datagram from that
// peer's 4-tuple is delivered to it and no longer to this listener. The kernel
// L2TP module consumes only DATA frames (l2tp_udp_encap_recv returns non-zero for
// a T-bit control frame, which falls through to the socket's normal receive
// queue), so the tunnel's remaining CONTROL messages -- a second ICRQ, CDN,
// HELLO, StopCCN -- land on this socket and are lost unless somebody reads them.
// Without this, ze goes deaf to a peer's control plane the instant that peer's
// first session reaches the kernel data plane.
//
// fd stays owned by the caller: this dups it, so the caller's own Close on
// tunnel delete is still correct. Call ReleaseTunnelSocket(tid) before that Close
// so the reader stops. Adopting a tid that is already adopted is a no-op.
func (u *UDPListener) AdoptTunnelSocket(tid uint16, fd int) error {
	u.mu.Lock()
	if u.closed || u.conn == nil {
		u.mu.Unlock()
		return errListenerNotStarted
	}
	if _, exists := u.tunnels[tid]; exists {
		u.mu.Unlock()
		return nil
	}
	u.mu.Unlock()

	// Dup so the kernel worker's closeFD on tunnel delete does not yank the
	// descriptor out from under the reader goroutine.
	dup, err := unix.Dup(fd)
	if err != nil {
		return fmt.Errorf("l2tp: dup tunnel socket fd %d: %w", fd, err)
	}
	f := os.NewFile(uintptr(dup), "l2tp-tunnel-socket")
	if f == nil {
		unix.Close(dup) //nolint:errcheck // rollback
		return fmt.Errorf("l2tp: invalid tunnel socket fd %d", dup)
	}
	// FileConn dups again internally and returns a runtime-poller-managed conn,
	// so Close reliably unblocks the reader. Close our own handle either way.
	pc, err := net.FileConn(f)
	closeErr := f.Close()
	if err != nil {
		return fmt.Errorf("l2tp: adopt tunnel socket: %w", err)
	}
	if closeErr != nil {
		pc.Close() //nolint:errcheck,gosec // rollback
		return fmt.Errorf("l2tp: close dup handle: %w", closeErr)
	}
	conn, ok := pc.(*net.UDPConn)
	if !ok {
		pc.Close() //nolint:errcheck,gosec // rollback
		return fmt.Errorf("l2tp: tunnel socket is %T, want *net.UDPConn", pc)
	}

	u.mu.Lock()
	if u.closed {
		u.mu.Unlock()
		conn.Close() //nolint:errcheck,gosec // listener stopped while we were adopting
		return errListenerNotStarted
	}
	if _, exists := u.tunnels[tid]; exists {
		// Lost a race with a concurrent adopt; keep the first.
		u.mu.Unlock()
		conn.Close() //nolint:errcheck,gosec // duplicate adopt
		return nil
	}
	if u.tunnels == nil {
		u.tunnels = make(map[uint16]*net.UDPConn)
	}
	u.tunnels[tid] = conn
	// Add UNDER u.mu, which is the same lock Stop takes to set closed: that orders
	// this Add before any wg.Wait a concurrent Stop can reach, so the reader can
	// never be registered on a WaitGroup that Stop has already finished waiting on.
	u.wg.Add(1)
	u.mu.Unlock()

	go u.tunnelReadLoop(tid, conn)
	return nil
}

// ReleaseTunnelSocket stops reading tunnel tid's adopted socket and closes this
// listener's dup of it. Idempotent; unknown tids are ignored. The caller still
// owns (and must close) the original fd it passed to AdoptTunnelSocket.
func (u *UDPListener) ReleaseTunnelSocket(tid uint16) {
	u.mu.Lock()
	conn := u.tunnels[tid]
	delete(u.tunnels, tid)
	u.mu.Unlock()
	if conn != nil {
		conn.Close() //nolint:errcheck,gosec // best-effort; unblocks tunnelReadLoop
	}
}

// tunnelReadLoop reads control frames from one adopted per-tunnel socket and
// feeds them to the same rx channel as the main listener, so the reactor handles
// them through its single existing path. The socket is connected, so its peer is
// fixed: the address is resolved once from RemoteAddr and stamped on every
// rxPacket, keeping the shape identical to the listener's own.
//
// It reads with Read, not ReadFromUDPAddrPort. On a conn derived from an adopted
// fd via net.FileConn, ReadFromUDPAddrPort blocked indefinitely even with a
// datagram queued (verified on darwin, go1.26.5, by
// TestListener_AdoptedTunnelSocketDeliversToRX against the raw-recvfrom premise
// in TestListener_connectedTunnelSocketReceives); Read is the call that matches a
// connected socket and delivers.
//
// Allocation discipline matches readLoop: backing array, free-slot channel, and
// release closures are created ONCE at goroutine start.
func (u *UDPListener) tunnelReadLoop(tid uint16, conn *net.UDPConn) {
	defer u.wg.Done()
	defer conn.Close() //nolint:errcheck,gosec // owned dup; Release may have closed it already

	// The socket is connected, so every datagram on it comes from this one peer.
	// Resolve it once instead of paying a sockaddr copy per read.
	var peer netip.AddrPort
	if ra, ok := conn.RemoteAddr().(*net.UDPAddr); ok {
		if a, ok := netip.AddrFromSlice(ra.IP); ok && ra.Port >= 0 && ra.Port <= 65535 {
			peer = netip.AddrPortFrom(a.Unmap(), uint16(ra.Port))
		}
	}

	backing := make([]byte, tunnelRxPoolSize*rxBufLen)
	freeCh := make(chan int, tunnelRxPoolSize)
	releases := make([]func(), tunnelRxPoolSize)
	for i := range tunnelRxPoolSize {
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
		// Read, not ReadFromUDPAddrPort: the socket is connect()ed, so its peer is
		// fixed and already known (captured as peer above). Read is the call that
		// matches a connected socket.
		n, err := conn.Read(buf)
		if err != nil {
			// Released, or the listener is stopping: either way this socket is
			// done. Unlike the main readLoop there is nothing to retry -- a dead
			// tunnel socket never recovers.
			u.mu.Lock()
			_, stillAdopted := u.tunnels[tid]
			closed := u.closed
			u.mu.Unlock()
			if !stillAdopted || closed {
				return
			}
			u.logger.Debug("l2tp: tunnel socket read error", "tunnel-id", tid, "error", err.Error())
			return
		}

		pkt := rxPacket{
			from:    peer,
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
			// Check closed BEFORE recycling the slot so a concurrent
			// Stop() path that already closed the socket exits this
			// loop deterministically rather than spinning once more.
			u.mu.Lock()
			closed := u.closed
			u.mu.Unlock()
			if closed {
				return
			}
			freeCh <- idx
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
