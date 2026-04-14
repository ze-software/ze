// Design: rfc/short/rfc5881.md -- single-hop UDP encapsulation (port 3784)
// Design: rfc/short/rfc5883.md -- multi-hop UDP encapsulation (port 4784)
// Related: socket.go -- Transport interface
// Related: loopback.go -- in-memory Transport used by engine tests
// Related: udp_linux.go -- Linux socket options (IP_TTL, IP_RECVTTL, SO_BINDTODEVICE)
// Related: udp_other.go -- non-Linux stub for socket options
//
// Production UDP transport. A single UDP socket (per VRF, per port) reads
// packets into pool buffers, extracts IP TTL via IP_RECVTTL control
// messages on Linux, and feeds a channel drained by the engine's express
// loop.
//
// The TTL check for GTSM (single-hop MUST be 255) and the min-TTL check
// for multi-hop are enforced by the engine, not the transport. Keeping
// packet policy out of the transport lets us swap in a different socket
// back end (XDP/eBPF, raw socket) without touching the engine.
//
// The socket-option logic and cmsg parsing are Linux-specific and live
// in udp_linux.go; non-Linux builds fall back to the stubs in
// udp_other.go which cannot bind-to-device and leave TTL=0 on receive
// so the engine fails closed.
package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"syscall"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
)

// transportLog is the lazy logger for the BFD UDP transport. Logged
// events include control-message truncation warnings (emitted once per
// transport lifetime) and any other kernel-visible anomalies that the
// engine cannot observe directly.
var transportLog = slogutil.LazyLogger("bfd.transport")

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
//
// oobTruncOnce ensures the MSG_CTRUNC warning is logged at most once
// per transport lifetime; a noisy per-packet log would hide the
// signal under the noise.
type UDP struct {
	oobTruncOnce sync.Once

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
	// Linux, the VRF binding is applied through the Device field
	// (SO_BINDTODEVICE); this field is the string that appears on
	// Inbound.VRF for engine dispatch.
	VRF string

	// Device is the Linux network device name the socket binds to via
	// SO_BINDTODEVICE. Zero value means no bind-to-device. For
	// single-hop pinned sessions it is the egress interface; for
	// non-default VRF deployments it is the VRF device name.
	Device string

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

// Start binds the socket, applies the GTSM-related socket options, and
// launches the receive goroutine. Start is NOT idempotent; a second call
// returns an error.
func (u *UDP) Start() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.conn != nil {
		return errUDPAlreadyStarted
	}
	if u.closed {
		return errUDPRestart
	}

	// Use ListenConfig.Control to run applySocketOptions on the raw fd
	// before ListenPacket hands the socket off. This is the only place
	// SO_BINDTODEVICE will succeed without CAP_NET_RAW-vs-bind ordering
	// hazards, and it lets IP_RECVTTL / IPV6_RECVHOPLIMIT be set before
	// the first read.
	device := u.Device
	isV6 := u.Bind.Addr().Is6() && !u.Bind.Addr().Is4In6()
	var ctrlErr error
	lc := net.ListenConfig{
		Control: func(_, _ string, c syscall.RawConn) error {
			if isV6 {
				if err := applySocketOptionsV6(c, device); err != nil {
					ctrlErr = err
					return err
				}
				return nil
			}
			if err := applySocketOptions(c, device); err != nil {
				ctrlErr = err
				return err
			}
			return nil
		},
	}

	network := "udp4"
	if isV6 {
		network = "udp6"
	}
	addr := fmt.Sprintf("[%s]:%d", u.Bind.Addr().String(), u.Bind.Port())
	if !isV6 {
		addr = fmt.Sprintf("%s:%d", u.Bind.Addr().String(), u.Bind.Port())
	}
	pc, err := lc.ListenPacket(context.Background(), network, addr)
	if err != nil {
		if ctrlErr != nil {
			return fmt.Errorf("bfd: bind %s (%s): %w", addr, device, ctrlErr)
		}
		return fmt.Errorf("bfd: bind %s: %w", addr, err)
	}
	conn, ok := pc.(*net.UDPConn)
	if !ok {
		if closeErr := pc.Close(); closeErr != nil {
			return fmt.Errorf("bfd: unexpected PacketConn type %T (close failed: %w)", pc, closeErr)
		}
		return fmt.Errorf("bfd: unexpected PacketConn type %T", pc)
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

// Send writes an Outbound to the peer address via the bound socket. The
// destination port is the transport's own bound port (Bind.Port) so an
// echo transport bound to UDPPortEcho sends to 3785 while a control
// transport bound to UDPPortSingleHopControl sends to 3784. The
// IP_TTL=255 socket option applied at Start means every packet leaves
// with the maximum TTL, satisfying RFC 5881 Section 5 for both hop modes
// (multi-hop peers happily accept TTL=255 since their floor is typically
// 254).
func (u *UDP) Send(out Outbound) error {
	u.mu.Lock()
	conn := u.conn
	u.mu.Unlock()
	if conn == nil {
		return errUDPNotStarted
	}
	raddr := &net.UDPAddr{IP: out.To.AsSlice(), Port: int(u.Bind.Port())}
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
// rx backing slice, the oob backing slice, the free-slot channel, and
// the per-slot release closures are all created ONCE at goroutine start
// and reused for the goroutine's lifetime. ReadMsgUDPAddrPort writes
// into pre-existing slices; Inbound carries a pre-built release closure
// picked by slot index. The oob buffer is parsed via parseReceivedTTL
// and discarded per packet -- no allocation.
func (u *UDP) readLoop() {
	defer u.wg.Done()
	defer close(u.rx)

	// One contiguous backing array sliced into rxPoolSize independent
	// buffers, similar to the ze peerPool pattern. Each
	// ReadMsgUDPAddrPort targets one slice; the slice is released via
	// Inbound.Release when the engine has consumed it.
	const rxPoolSize = 16
	const rxBufLen = 128 // enough for 24 + 28 (SHA1) + future TLVs
	backing := make([]byte, rxPoolSize*rxBufLen)
	oobBacking := make([]byte, rxPoolSize*oobBufLen)
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
		oob := oobBacking[idx*oobBufLen : (idx+1)*oobBufLen]
		n, oobn, flags, raddr, err := u.conn.ReadMsgUDPAddrPort(buf, oob)
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
		// MSG_CTRUNC (recvmsg flag bit 0x8) means the kernel
		// truncated the control-message blob because oobBufLen was
		// too small. With Stage 2's single IP_TTL cmsg requirement
		// the 64-byte oob slot has ample headroom, but a future
		// addition (IP_PKTINFO, SCM_TIMESTAMP) could hit the limit
		// silently and lose the TTL. Log once per transport so the
		// operator sees the truncation without flooding the log.
		if flags&syscall.MSG_CTRUNC != 0 {
			u.oobTruncOnce.Do(func() {
				transportLog().Warn("bfd transport oob buffer truncated by kernel (MSG_CTRUNC); increase oobBufLen",
					"oob-capacity", oobBufLen,
					"bind", u.Bind.String())
			})
		}
		ttl := parseReceivedTTL(oob[:oobn])
		in := Inbound{
			From:    raddr.Addr().Unmap(),
			VRF:     u.VRF,
			Mode:    u.Mode,
			TTL:     ttl,
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
