//go:build linux

// Design: plan/learned/970-ospfv3-3-ipv6-transport.md -- Linux ipv6.PacketConn backend
// RFC: rfc/short/rfc5340.md (§2.9 raw IPv6 proto 89, link-local source, ff02::5/6)

package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"golang.org/x/net/ipv6"
	"golang.org/x/sys/unix"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

const (
	rxBufLen    = 65535
	readTimeout = 500 * time.Millisecond
	dropRecvErr = "recv-error"
	// listenNetwork opens a raw IPv6 socket with Next Header = Protocol (89).
	listenNetwork = "ip6:89"
)

type linuxBackend struct{}

// NewBackend returns the Linux raw IPv6 OSPFv3 backend.
func NewBackend() Backend { return linuxBackend{} }

var (
	resolveIfaceBinding   = iface.Resolve
	resolveIfaceAddresses = iface.Addresses
	ensureIfaceBackend    = iface.EnsureBackend
)

type resolvedInterface struct {
	ifi       *net.Interface
	linkLocal netip.Addr
}

// resolveOSPFv3Interface resolves a logical OSPFv3 interface name through the
// shared iface resolver (os-name / mac selectors) to its kernel device index/name
// and link-local source address. A missing link-local source (IPv6 DAD not yet
// complete) returns ErrNoLinkLocal so the orchestrator marks the open pending.
func resolveOSPFv3Interface(name string) (resolvedInterface, error) {
	// An OSPFv3-only config has no interface{} block to load the iface backend,
	// but the resolver below needs it. Ensure a default backend is loaded (no-op
	// when an explicit interface{} backend already loaded one). Mirrors the v2
	// transport; without it an OSPFv3-only config fails "iface: no backend loaded".
	if err := ensureIfaceBackend(); err != nil {
		return resolvedInterface{}, fmt.Errorf("ospfv3/transport: interface %s: %w", name, err)
	}
	b, err := resolveIfaceBinding(name)
	if err != nil {
		return resolvedInterface{}, fmt.Errorf("ospfv3/transport: resolve interface %s: %w", name, err)
	}
	osName := b.OsName
	if osName == "" {
		osName = name
	}
	ll, err := interfaceLinkLocal(name)
	if err != nil {
		return resolvedInterface{}, err
	}
	// A minimal *net.Interface (index + name) is all ipv6.PacketConn needs for
	// JoinGroup / SetMulticastInterface / the WriteTo ControlMessage; avoid a
	// net.InterfaceByName lookup so resolution stays driven by the iface resolver.
	return resolvedInterface{ifi: &net.Interface{Index: b.Ifindex, Name: osName}, linkLocal: ll}, nil
}

// interfaceLinkLocal returns the interface's IPv6 link-local (fe80::/10) source, using the
// resolver's LinkLocal classifier and preferring a DAD-complete address over a tentative one.
// ErrNoLinkLocal means the interface has no link-local at all yet.
func interfaceLinkLocal(name string) (netip.Addr, error) {
	addrs, err := resolveIfaceAddresses(name)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("ospfv3/transport: interface %s addresses: %w", name, err)
	}
	// Prefer a DAD-complete link-local (RFC 4862: a tentative address is not a confirmed
	// source). Fall back to a tentative one only when that is all the interface has: in some
	// environments (bridged containers) IPv6 DAD never completes and the address stays tentative
	// yet is still usable, so binding to it beats never forming an adjacency.
	var tentative netip.Addr
	for _, addr := range addrs {
		if !addr.LinkLocal {
			continue
		}
		ip, perr := netip.ParseAddr(addr.Address)
		if perr != nil || !ip.Is6() {
			continue
		}
		if addr.Tentative {
			if !tentative.IsValid() {
				tentative = ip.WithZone("")
			}
			continue
		}
		return ip.WithZone(""), nil
	}
	if tentative.IsValid() {
		return tentative, nil
	}
	return netip.Addr{}, ErrNoLinkLocal
}

func (linuxBackend) OpenInterface(name string, recordDrop DropRecorder) (InterfaceHandle, error) {
	resolved, err := resolveOSPFv3Interface(name)
	if err != nil {
		return nil, err
	}
	conn, err := (&net.ListenConfig{}).ListenPacket(context.Background(), listenNetwork, "::")
	if err != nil {
		return nil, fmt.Errorf("ospfv3/transport: listen %s needs CAP_NET_RAW: %w", listenNetwork, err)
	}
	pc := ipv6.NewPacketConn(conn)
	if err := setupSocket(conn, pc, resolved.ifi); err != nil {
		closeConn(conn)
		return nil, err
	}
	li := &linuxInterface{
		conn:       conn,
		pc:         pc,
		ifi:        resolved.ifi,
		linkLocal:  resolved.linkLocal,
		recvCh:     make(chan RawPacket, 64),
		stop:       make(chan struct{}),
		recordDrop: recordDrop,
	}
	go li.readLoop()
	return li, nil
}

// setupSocket binds the socket to the interface and sets the OSPFv3 multicast and
// control-message options. RFC 5340 §2.9: hop limit 1 (link-local scope),
// per-interface egress, loopback off (no self-neighbor).
func setupSocket(conn net.PacketConn, pc *ipv6.PacketConn, ifi *net.Interface) error {
	ipc, ok := conn.(*net.IPConn)
	if !ok {
		return fmt.Errorf("ospfv3/transport: unexpected conn type %T", conn)
	}
	raw, err := ipc.SyscallConn()
	if err != nil {
		return fmt.Errorf("ospfv3/transport: syscall conn: %w", err)
	}
	var soErr error
	if cerr := raw.Control(func(fd uintptr) {
		soErr = unix.SetsockoptString(int(fd), unix.SOL_SOCKET, unix.SO_BINDTODEVICE, ifi.Name)
	}); cerr != nil {
		return fmt.Errorf("ospfv3/transport: rawconn control: %w", cerr)
	}
	if soErr != nil {
		return fmt.Errorf("ospfv3/transport: bind socket to interface %s: %w", ifi.Name, soErr)
	}
	if err := pc.SetMulticastHopLimit(1); err != nil {
		return fmt.Errorf("ospfv3/transport: set multicast hop limit: %w", err)
	}
	if err := pc.SetMulticastInterface(ifi); err != nil {
		return fmt.Errorf("ospfv3/transport: set multicast interface %s: %w", ifi.Name, err)
	}
	if err := pc.SetMulticastLoopback(false); err != nil {
		return fmt.Errorf("ospfv3/transport: disable multicast loopback: %w", err)
	}
	if err := pc.SetControlMessage(ipv6.FlagDst|ipv6.FlagInterface|ipv6.FlagHopLimit, true); err != nil {
		return fmt.Errorf("ospfv3/transport: set control message: %w", err)
	}
	return nil
}

type linuxInterface struct {
	conn       net.PacketConn
	pc         *ipv6.PacketConn
	ifi        *net.Interface
	linkLocal  netip.Addr
	recvCh     chan RawPacket
	stop       chan struct{}
	recordDrop DropRecorder
	sendMu     sync.Mutex
	closed     sync.Once
}

func (li *linuxInterface) IfIndex() int                { return li.ifi.Index }
func (li *linuxInterface) LinkLocalSource() netip.Addr { return li.linkLocal }
func (li *linuxInterface) Recv() <-chan RawPacket      { return li.recvCh }

// JoinAllSPFRouters joins ff02::5 on the interface (RFC 5340 §2.9: all routers).
func (li *linuxInterface) JoinAllSPFRouters() error { return li.joinLeave(AllSPFRouters, true) }

// JoinAllDRouters joins ff02::6 (RFC 5340 §2.9: only the DR/BDR).
func (li *linuxInterface) JoinAllDRouters() error { return li.joinLeave(AllDRouters, true) }

// LeaveAllDRouters leaves ff02::6 on losing the DR/BDR role.
func (li *linuxInterface) LeaveAllDRouters() error { return li.joinLeave(AllDRouters, false) }

func (li *linuxInterface) joinLeave(group netip.Addr, join bool) error {
	g := &net.IPAddr{IP: group.AsSlice(), Zone: li.ifi.Name}
	if join {
		if err := li.pc.JoinGroup(li.ifi, g); err != nil {
			return fmt.Errorf("ospfv3/transport: join %s on %s: %w", group, li.ifi.Name, err)
		}
		return nil
	}
	if err := li.pc.LeaveGroup(li.ifi, g); err != nil {
		return fmt.Errorf("ospfv3/transport: leave %s on %s: %w", group, li.ifi.Name, err)
	}
	return nil
}

// Send transmits payload to dst from the bound link-local source. RFC 5340 §A.3.1:
// ControlMessage.Src forces the on-wire source to equal the checksum pseudo-header
// source. Hop limit 1 keeps the packet on the link.
func (li *linuxInterface) Send(dst, src netip.Addr, payload []byte) error {
	if !dst.Is6() {
		return ErrInvalidDestination
	}
	li.sendMu.Lock()
	defer li.sendMu.Unlock()
	cm := &ipv6.ControlMessage{IfIndex: li.ifi.Index, HopLimit: 1}
	if src.IsValid() && src.Is6() {
		cm.Src = src.AsSlice()
	}
	d := &net.IPAddr{IP: dst.AsSlice(), Zone: li.ifi.Name}
	if _, err := li.pc.WriteTo(payload, cm, d); err != nil {
		return fmt.Errorf("ospfv3/transport: writeto %s: %w", dst, err)
	}
	return nil
}

// SendRouted transmits a routed virtual-link packet to the neighbor's GLOBAL address dst
// from the local GLOBAL source src with hop limit > 1 (RFC 5340 §2.9). Unlike Send it does
// NOT set a zone/scope on dst (the packet is routed, not link-scoped) and leaves the
// outgoing interface to the kernel's route lookup rather than pinning IfIndex.
func (li *linuxInterface) SendRouted(dst, src netip.Addr, payload []byte, hopLimit int) error {
	if !dst.Is6() || !src.Is6() {
		return ErrInvalidDestination
	}
	li.sendMu.Lock()
	defer li.sendMu.Unlock()
	cm := &ipv6.ControlMessage{HopLimit: hopLimit, Src: src.AsSlice()}
	d := &net.IPAddr{IP: dst.AsSlice()}
	if _, err := li.pc.WriteTo(payload, cm, d); err != nil {
		return fmt.Errorf("ospfv3/transport: routed writeto %s: %w", dst, err)
	}
	return nil
}

func (li *linuxInterface) Close() error {
	var err error
	li.closed.Do(func() {
		close(li.stop)
		err = li.conn.Close()
	})
	return err
}

func (li *linuxInterface) stopped() bool {
	select {
	case <-li.stop:
		return true
	default:
		return false
	}
}

func (li *linuxInterface) readLoop() {
	defer close(li.recvCh)
	buf := make([]byte, rxBufLen)
	for {
		if li.stopped() {
			return
		}
		if err := li.pc.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
			if li.stopped() {
				return
			}
		}
		n, cm, src, err := li.pc.ReadFrom(buf)
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) {
				continue
			}
			if li.stopped() {
				return
			}
			if li.recordDrop != nil {
				li.recordDrop(dropRecvErr)
			}
			continue
		}
		if !li.deliver(buf[:n], cm, src) {
			continue
		}
	}
}

func (li *linuxInterface) deliver(data []byte, cm *ipv6.ControlMessage, src net.Addr) bool {
	cp := make([]byte, len(data))
	copy(cp, data)
	rp := RawPacket{IfIndex: li.ifi.Index, Payload: cp}
	if ipa, ok := src.(*net.IPAddr); ok {
		if a, ok := netip.AddrFromSlice(ipa.IP); ok {
			rp.Src = a.Unmap()
		}
	}
	if cm != nil {
		if cm.IfIndex != 0 {
			rp.IfIndex = cm.IfIndex
		}
		if a, ok := netip.AddrFromSlice(cm.Dst); ok {
			rp.Dst = a.Unmap()
		}
		if cm.HopLimit >= 0 && cm.HopLimit <= 255 {
			rp.HopLimit = uint8(cm.HopLimit)
		}
	}
	select {
	case li.recvCh <- rp:
	case <-li.stop:
		return false
	}
	return true
}

func closeConn(conn net.PacketConn) {
	if err := conn.Close(); err != nil {
		logger().Warn("ospfv3/transport: close conn", "err", err)
	}
}
