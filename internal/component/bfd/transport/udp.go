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
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
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

	// ifNames caches the ingress ifindex-to-name mapping readLoop stamps on a
	// single-hop Inbound. net.InterfaceByIndex is a netlink round trip, and the
	// index set of a box is small and near-static, so it is resolved once per
	// index rather than once per packet. Only successful lookups are stored: a
	// failure is a moment and must not be remembered as an answer. Each entry
	// expires, because an ifindex is REUSED after a veth or VLAN is deleted and
	// recreated, and a cached name for a reused index is wrong in the direction
	// that drops every packet on that link.
	ifNamesMu sync.RWMutex
	ifNames   map[int]ifName

	// egressPins caches, per interface name, the IP_PKTINFO / IPV6_PKTINFO
	// control message Send attaches to pin a single-hop packet to that
	// interface when the socket is bound to no device. The message is built
	// once per name and read-only after, so every Send shares it without a
	// lock around the write and without an allocation per packet. Entries
	// expire on ifNameTTL for the reason ifNames does: a name can come back
	// with another index after the device is deleted and recreated.
	egressPinsMu sync.RWMutex
	egressPins   map[string]*egressLink

	// Clock is the time source the ifindex cache ages entries against. Nil
	// means clock.RealClock{}, which is what every production caller wants; a
	// test sets it to drive expiry without sleeping.
	Clock clock.Clock

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
	var bAddr textbuf.Buffer
	if isV6 {
		bAddr.Reset().Byte('[').Addr(u.Bind.Addr()).Str("]:").Int(int64(u.Bind.Port()))
	} else {
		bAddr.Reset().Addr(u.Bind.Addr()).Byte(':').Int(int64(u.Bind.Port()))
	}
	addr := bAddr.String()
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
//
// A single-hop packet that names an interface is first checked against that
// interface's subnets (egressLink.reaches): a peer on none of them is refused
// with errUDPOffSubnet and nothing is written. On a socket bound to no device
// the packet then leaves with an ifindex control message naming the session's
// interface (see pinsToLink), so the kernel sends it on that link whatever the
// routing table says about the peer. A multi-hop packet is routed and is
// neither checked nor pinned.
func (u *UDP) Send(out Outbound) error {
	u.mu.Lock()
	conn := u.conn
	u.mu.Unlock()
	if conn == nil {
		return errUDPNotStarted
	}
	to := u.destination(out)
	if u.Mode != api.SingleHop {
		_, err := conn.WriteToUDP(out.Bytes, to)
		return err
	}
	if out.Interface == "" {
		_, err := conn.WriteToUDP(out.Bytes, to)
		return err
	}
	link, err := u.egressLinkOf(out.Interface)
	if err != nil {
		return err
	}
	// RFC 5881 Section 6: "On a multiaccess network, BFD Control packets MUST
	// be transmitted with source and destination addresses that are part of
	// the subnet (addressed from and to interfaces on the subnet)."
	if !link.reaches(out.To) {
		return fmt.Errorf("%w: peer %s, interface %q", errUDPOffSubnet, out.To, out.Interface)
	}
	if !u.pinsToLink(out, to) {
		_, err = conn.WriteToUDP(out.Bytes, to)
		return err
	}
	// RFC 5881 Section 6: "Implementations MUST ensure that all BFD Control
	// packets are transmitted over the one-hop path being protected by BFD."
	_, _, err = conn.WriteMsgUDP(out.Bytes, link.oob, to)
	return err
}

// pinsToLink answers whether Send must pin this packet to out.Interface with
// an ifindex control message.
//
// Only a single-hop socket pins: RFC 5883 multi-hop traffic is routed. A
// socket bound to a device (resolveLoopDevices in bfd.go binds a loop whose
// single-hop sessions all name one interface, and a non-default VRF loop to
// its VRF device) already has the kernel's answer, and a second, different
// ifindex on an IPv6 send is refused with EINVAL. A session that names no
// interface has no link to pin to. A zoned destination is already pinned by
// its zone (destination, sendZone).
func (u *UDP) pinsToLink(out Outbound, to *net.UDPAddr) bool {
	if u.Mode != api.SingleHop {
		return false
	}
	if u.Device != "" {
		return false
	}
	if out.Interface == "" {
		return false
	}
	return to.Zone == ""
}

// destination builds the socket address one Outbound is written to: the peer
// the session names, the port this transport binds, and, for a link-local
// peer, the link the session runs on.
//
// RFC 4007 Section 6: "Because the same non-global address may be in use in
// more than one zone of the same scope (e.g., the use of link-local address
// fe80::1 in two separate physical links) and a node may have interfaces
// attached to different zones of the same scope (e.g., a router normally has
// multiple interfaces attached to different links), a node requires an
// internal means to identify to which zone a non-global address belongs."
// net.UDPAddr.Zone is that means, and out.To.AsSlice() cannot carry it: a
// link-local destination sent with no zone leaves the kernel to pick a link or
// to refuse the write.
//
// The session's own address is the first answer, and the interface is the
// second. A session reaches here with an interface and a zoneless peer,
// because RFC 5881 Section 2 puts a single-hop session on "a single IP hop
// that is associated with an incoming interface" while the config leaf that
// names the peer holds an address alone. Only a scoped destination gets a
// zone: a global address is reachable by the routing table, and naming a link
// for it would narrow a decision that is not this layer's to make.
func (u *UDP) destination(out Outbound) *net.UDPAddr {
	return &net.UDPAddr{
		IP:   out.To.AsSlice(),
		Port: int(u.Bind.Port()),
		Zone: sendZone(out),
	}
}

// sendZone answers the zone destination puts on the address, empty for every
// address that is not scoped to one link.
func sendZone(out Outbound) string {
	if !out.To.IsLinkLocalUnicast() {
		return ""
	}
	if zone := out.To.Zone(); zone != "" {
		return zone
	}
	return out.Interface
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
		// too small. Two messages are enabled and both arrive on
		// every packet: measured in CMSG_SPACE they take 56 bytes on
		// IPv4 and 64 on IPv6 (udp_linux.go, oobBufLen). Truncation
		// loses the TTL and the local address, which is the RFC 5880
		// Section 6.8.6 demux input, so log once per transport --
		// enough for the operator to see it, not enough to flood.
		if flags&syscall.MSG_CTRUNC != 0 {
			u.oobTruncOnce.Do(func() {
				transportLog().Warn("bfd transport oob buffer truncated by kernel (MSG_CTRUNC); increase oobBufLen",
					"oob-capacity", oobBufLen,
					"bind", u.Bind.String())
			})
		}
		ttl := parseReceivedTTL(oob[:oobn])
		// RFC 5880 Section 6.8.6: "If the Your Discriminator field is zero, the
		// session MUST be selected based on some combination of other fields."
		// Ze selects on (peer, local, interface, vrf, mode), so the receiver has
		// to know which of its own addresses the packet reached and on which
		// interface. Both come from IP_PKTINFO. u.Bind.Addr() cannot answer: the
		// socket binds the wildcard, so it reads 0.0.0.0 for every packet, and a
		// session whose key carries a real local address never matched.
		local, ifindex := parseReceivedPktinfo(oob[:oobn])
		in := Inbound{
			From:      raddr.Addr().Unmap(),
			Local:     local.Unmap(),
			Interface: u.ingressInterface(ifindex),
			VRF:       u.VRF,
			Mode:      u.Mode,
			TTL:       ttl,
			Bytes:     buf[:n],
			release:   releases[idx], // pre-built once; no per-packet alloc
		}
		select {
		case u.rx <- in:
		case <-u.stop:
			freeCh <- idx
			return
		}
	}
}

// now reads the transport's time source, defaulting to the real clock so a
// zero-value UDP behaves as production does.
func (u *UDP) now() time.Time {
	if u.Clock == nil {
		return clock.RealClock{}.Now()
	}
	return u.Clock.Now()
}

// ifName is one cached ifindex-to-name answer and the moment it was resolved.
type ifName struct {
	name string
	at   time.Time
}

// ifNameTTL bounds how long a cached ifindex-to-name answer is trusted.
//
// An ifindex is reused: delete a veth or a VLAN and create another, and the new
// device can take the old index. A cached name then answers for a device that
// no longer exists, the first-packet key stops matching, and every packet whose
// Your Discriminator is zero is dropped on that link. The alternative to a TTL
// is a netlink round trip per packet, which is a syscall on the receive path of
// a protocol that runs at 300 packets a second per session.
//
// Thirty seconds is chosen against BFD's own numbers rather than arbitrarily: a
// session whose detection time is under a second is Down long before this
// expires, so the window costs a session that is already down a little more
// down, and never costs a live session anything.
const ifNameTTL = 30 * time.Second

// ingressInterface answers the name of the interface a packet arrived on, from
// the ifindex IP_PKTINFO carried, and ONLY for a single-hop socket.
//
// A multi-hop session is routed, so api.SessionRequest.Canonical clears its
// interface and its key carries none. Inbound.Interface documents itself as
// single-hop only; this is where that holds.
//
// The empty answer, here and for an index nothing resolves, is not a fallback
// to a guess: it says the interface is unknown, and the engine matches such a
// packet against a session that named no interface rather than inventing one.
func (u *UDP) ingressInterface(ifindex int) string {
	if u.Mode != api.SingleHop || ifindex <= 0 {
		return ""
	}
	now := u.now()
	u.ifNamesMu.RLock()
	cached, ok := u.ifNames[ifindex]
	u.ifNamesMu.RUnlock()
	if ok && now.Sub(cached.at) < ifNameTTL {
		return cached.name
	}
	link, err := net.InterfaceByIndex(ifindex)
	if err != nil {
		// NOT cached. A netlink error is a moment, not a fact: caching the
		// empty answer would drop every zero-discriminator packet on that link
		// for the life of the daemon, which is the silently-wrong value this
		// whole path was repaired for (ai/rules/principles.md). Retrying costs
		// one netlink round trip on a packet that would otherwise be lost.
		transportLog().Debug("bfd transport could not name the ingress interface; this packet cannot match a session keyed on one",
			"ifindex", ifindex, "err", err)
		return ""
	}
	u.ifNamesMu.Lock()
	if u.ifNames == nil {
		u.ifNames = make(map[int]ifName, 4)
	}
	u.ifNames[ifindex] = ifName{name: link.Name, at: now}
	u.ifNamesMu.Unlock()
	return link.Name
}

// egressLink is one cached answer about the interface a single-hop session
// names: the control message that pins a packet to it, the subnets its
// addresses sit in, whether it is a point-to-point link, and the moment it was
// resolved. Every field is read-only once cached, so Send shares the slices
// without a lock and without an allocation per packet.
type egressLink struct {
	oob         []byte
	subnets     []netip.Prefix
	pointToLink bool
	at          time.Time
}

// reaches answers whether peer is a destination RFC 5881 Section 6 lets a
// single-hop Control packet on this interface be addressed to.
//
// The subnet rule is stated "On a multiaccess network", so a point-to-point
// interface (IFF_POINTOPOINT, such as an unnumbered or /32-peer tunnel) always
// answers true. A link-local peer is on the link by its own scope (RFC 4291
// Section 2.5.6), and destination already pins it with its zone. Otherwise the
// peer MUST sit in a subnet of one of the interface's addresses; an interface
// with no address has no subnet, and answers false for every global peer.
func (l *egressLink) reaches(peer netip.Addr) bool {
	if l.pointToLink {
		return true
	}
	target := peer.Unmap()
	if target.IsLinkLocalUnicast() {
		return true
	}
	for _, subnet := range l.subnets {
		if subnet.Contains(target) {
			return true
		}
	}
	return false
}

// errUDPEgressUnknown is returned by Send when the interface a single-hop
// session names cannot be resolved to an index.
var errUDPEgressUnknown = errors.New("bfd: single-hop session interface cannot be resolved")

// errUDPOffSubnet is returned by Send when a single-hop session's peer is on
// none of the subnets of the interface the session names (RFC 5881 Section 6).
var errUDPOffSubnet = errors.New("bfd: single-hop peer is on no subnet of the session interface")

// egressLinkOf answers what Send needs to know about the interface named
// name, from the cache when the entry is younger than ifNameTTL.
//
// An interface that does not resolve is an error, never an unpinned or
// unchecked send: the link the session protects is gone, and a packet routed
// over another link would keep the session Up with no one-hop path under it
// (RFC 5881 Section 6). The failure is not cached, for the reason
// ingressInterface gives. The subnets age on the same TTL, so an address the
// operator adds or removes is seen within ifNameTTL; reading them per packet
// would be a netlink dump on a path that runs 300 times a second per session.
func (u *UDP) egressLinkOf(name string) (*egressLink, error) {
	now := u.now()
	u.egressPinsMu.RLock()
	cached, ok := u.egressPins[name]
	u.egressPinsMu.RUnlock()
	if ok && now.Sub(cached.at) < ifNameTTL {
		return cached, nil
	}
	link, err := net.InterfaceByName(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %q: %w", errUDPEgressUnknown, name, err)
	}
	subnets, err := interfaceSubnets(link)
	if err != nil {
		return nil, fmt.Errorf("%w: %q addresses: %w", errUDPEgressUnknown, name, err)
	}
	isV6 := u.Bind.Addr().Is6() && !u.Bind.Addr().Is4In6()
	resolved := &egressLink{
		oob:         pktinfoPin(link.Index, isV6),
		subnets:     subnets,
		pointToLink: link.Flags&net.FlagPointToPoint != 0,
		at:          now,
	}
	u.egressPinsMu.Lock()
	if u.egressPins == nil {
		u.egressPins = make(map[string]*egressLink, 4)
	}
	u.egressPins[name] = resolved
	u.egressPinsMu.Unlock()
	return resolved, nil
}

// interfaceSubnets answers the subnet of every address the interface holds,
// both families, masked to its prefix length.
func interfaceSubnets(link *net.Interface) ([]netip.Prefix, error) {
	addrs, err := link.Addrs()
	if err != nil {
		return nil, err
	}
	subnets := make([]netip.Prefix, 0, len(addrs))
	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		addr, ok := netip.AddrFromSlice(ipNet.IP)
		if !ok {
			continue
		}
		ones, _ := ipNet.Mask.Size()
		subnets = append(subnets, netip.PrefixFrom(addr.Unmap(), ones).Masked())
	}
	return subnets, nil
}
