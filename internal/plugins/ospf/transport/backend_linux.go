//go:build linux

// Design: plan/learned/957-ospf-3-ip-transport.md -- Linux AF_INET/SOCK_RAW backend
// Related: transport.go -- orchestrator; SendPacketRouted uses SendRouted for virtual links

package transport

import (
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

const (
	rcvTimeout = 500 * time.Millisecond
	rxBufLen   = 65535
)

type linuxBackend struct{}

func NewBackend() Backend { return linuxBackend{} }

var (
	resolveIfaceBinding   = iface.Resolve
	resolveIfaceAddresses = iface.Addresses
	ensureIfaceBackend    = iface.EnsureBackend
)

type resolvedInterface struct {
	ifindex int
	osName  string
	local   [4]byte
}

func resolveOSPFInterface(name string) (resolvedInterface, error) {
	// An OSPF-only config has no interface{} block to load the iface backend, but
	// OSPF still needs it to read the interface's ifindex and IPv4. Ensure a
	// default backend is loaded (no-op when an explicit interface{} backend
	// already loaded one). Without this, opening an active interface fails
	// "iface: no backend loaded" even when the link exists.
	if err := ensureIfaceBackend(); err != nil {
		return resolvedInterface{}, fmt.Errorf("ospf/transport: interface %s: %w", name, err)
	}
	b, err := resolveIfaceBinding(name)
	if err != nil {
		return resolvedInterface{}, fmt.Errorf("ospf/transport: resolve interface %s: %w", name, err)
	}
	osName := b.OsName
	if osName == "" {
		osName = name
	}
	local, err := interfaceIPv4(name)
	if err != nil {
		return resolvedInterface{}, err
	}
	return resolvedInterface{ifindex: b.Ifindex, osName: osName, local: local}, nil
}

func (linuxBackend) OpenInterface(name string, recordDrop dropRecorder) (InterfaceHandle, error) {
	resolved, err := resolveOSPFInterface(name)
	if err != nil {
		return nil, err
	}
	rxFD, err := openInterfaceSocket(resolved.osName)
	if err != nil {
		return nil, err
	}
	if err := joinGroup(rxFD, resolved.ifindex, resolved.local, AllSPFRouters); err != nil {
		closeFD(rxFD)
		return nil, err
	}
	tv := unix.Timeval{Sec: int64(rcvTimeout / time.Second), Usec: int64((rcvTimeout % time.Second) / time.Microsecond)}
	if err := unix.SetsockoptTimeval(rxFD, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv); err != nil {
		closeFD(rxFD)
		return nil, fmt.Errorf("ospf/transport: setsockopt SO_RCVTIMEO: %w", err)
	}
	txFD, err := openInterfaceSocket(resolved.osName)
	if err != nil {
		closeFD(rxFD)
		return nil, err
	}
	if err := setMulticastOptions(txFD, resolved.ifindex, resolved.local); err != nil {
		closeFD(rxFD)
		closeFD(txFD)
		return nil, err
	}
	li := &linuxInterface{rxFD: rxFD, txFD: txFD, osName: resolved.osName, ifindex: resolved.ifindex, local: resolved.local, recvCh: make(chan RawPacket, 64), stop: make(chan struct{}), recordDrop: recordDrop}
	go li.readLoop()
	return li, nil
}

type linuxInterface struct {
	rxFD       int
	txFD       int
	routedFD   int // lazily-opened TX socket with TTL > 1 for virtual-link (routed) sends; 0 = none
	osName     string
	ifindex    int
	local      [4]byte
	recvCh     chan RawPacket
	stop       chan struct{}
	recordDrop dropRecorder
	sendMu     sync.Mutex
	closed     sync.Once
}

// routedTTL is the TTL applied to virtual-link packets. Unlike the TTL-1 link-local socket,
// virtual-link packets are routed across the transit area (RFC 2328 section 8.1), so they
// need a TTL large enough to traverse it.
const routedTTL = 64

// SendRouted sends a unicast packet on a routed TX socket (TTL routedTTL), distinct from the
// TTL-1 link-local txFD, so a virtual-link packet is routed across the transit area rather
// than dropped at the first hop. The routed socket is opened on first use.
func (li *linuxInterface) SendRouted(dst netip.Addr, payload []byte) error {
	if !dst.Is4() {
		return ErrInvalidDestination
	}
	li.sendMu.Lock()
	defer li.sendMu.Unlock()
	if li.routedFD == 0 {
		fd, err := openRoutedSocket(li.osName)
		if err != nil {
			return err
		}
		li.routedFD = fd
	}
	sa := &unix.SockaddrInet4{Addr: dst.As4()}
	if err := unix.Sendto(li.routedFD, payload, 0, sa); err != nil {
		return fmt.Errorf("ospf/transport: routed sendto %s: %w", dst, err)
	}
	return nil
}

// openRoutedSocket opens an AF_INET/SOCK_RAW proto-89 TX socket bound to the transit egress
// with TTL routedTTL, so virtual-link packets are routed rather than link-local.
func openRoutedSocket(name string) (int, error) {
	fd, err := openInterfaceSocket(name)
	if err != nil {
		return -1, err
	}
	if err := unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_TTL, routedTTL); err != nil {
		closeFD(fd)
		return -1, fmt.Errorf("ospf/transport: setsockopt routed IP_TTL: %w", err)
	}
	return fd, nil
}

func (li *linuxInterface) IfIndex() int             { return li.ifindex }
func (li *linuxInterface) Recv() <-chan RawPacket   { return li.recvCh }
func (li *linuxInterface) JoinAllSPFRouters() error { return nil }
func (li *linuxInterface) JoinAllDRouters() error {
	return joinGroup(li.rxFD, li.ifindex, li.local, AllDRouters)
}
func (li *linuxInterface) LeaveAllDRouters() error {
	return leaveGroup(li.rxFD, li.ifindex, li.local, AllDRouters)
}

func (li *linuxInterface) Send(dst netip.Addr, payload []byte) error {
	if !dst.Is4() {
		return ErrInvalidDestination
	}
	li.sendMu.Lock()
	defer li.sendMu.Unlock()
	sa := &unix.SockaddrInet4{Addr: dst.As4()}
	if err := unix.Sendto(li.txFD, payload, 0, sa); err != nil {
		return fmt.Errorf("ospf/transport: sendto %s: %w", dst, err)
	}
	return nil
}

func (li *linuxInterface) Close() error {
	var err error
	li.closed.Do(func() {
		close(li.stop)
		err = unix.Close(li.rxFD)
		if txErr := unix.Close(li.txFD); err == nil {
			err = txErr
		}
		if li.routedFD != 0 {
			if rErr := unix.Close(li.routedFD); err == nil {
				err = rErr
			}
		}
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
	var buf [rxBufLen]byte
	for {
		if li.stopped() {
			return
		}
		n, _, err := unix.Recvfrom(li.rxFD, buf[:], 0)
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
				continue
			}
			if errors.Is(err, unix.EBADF) || errors.Is(err, unix.EINVAL) {
				return
			}
			continue
		}
		if !li.deliverDatagram(buf[:n]) {
			continue
		}
	}
}

func (li *linuxInterface) deliverDatagram(data []byte) bool {
	payload, src, ok := StripIPv4Header(data)
	if !ok {
		if li.recordDrop != nil {
			li.recordDrop(dropMalformedIPv4)
		}
		return false
	}
	cp := make([]byte, len(payload))
	copy(cp, payload)
	select {
	case li.recvCh <- RawPacket{IfIndex: li.ifindex, Src: src, Payload: cp}:
	case <-li.stop:
		return false
	}
	return true
}

func openInterfaceSocket(name string) (int, error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW, Protocol)
	if err != nil {
		return -1, fmt.Errorf("ospf/transport: socket(AF_INET, SOCK_RAW, proto 89) needs CAP_NET_RAW: %w", err)
	}
	if err := unix.SetsockoptString(fd, unix.SOL_SOCKET, unix.SO_BINDTODEVICE, name); err != nil {
		closeFD(fd)
		return -1, fmt.Errorf("ospf/transport: bind socket to interface %s: %w", name, err)
	}
	return fd, nil
}
func interfaceIPv4(name string) ([4]byte, error) {
	addrs, err := resolveIfaceAddresses(name)
	if err != nil {
		return [4]byte{}, fmt.Errorf("ospf/transport: interface %s addresses: %w", name, err)
	}
	for _, addr := range addrs {
		if addr.Family != "" && addr.Family != "ipv4" {
			continue
		}
		ip, err := netip.ParseAddr(addr.Address)
		if err == nil && ip.Is4() {
			return ip.As4(), nil
		}
	}
	return [4]byte{}, fmt.Errorf("ospf/transport: interface %s has no IPv4 address", name)
}

func setMulticastOptions(fd, ifindex int, local [4]byte) error {
	// RFC 2328 Appendix A.1: OSPF packets are link-local; unicast DD and
	// retransmission packets use this TX socket too, so set the unicast TTL.
	if err := unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_TTL, 1); err != nil {
		return fmt.Errorf("ospf/transport: setsockopt IP_TTL: %w", err)
	}
	// RFC 2328 Appendix D.3: OSPF sends link-local multicasts with TTL 1.
	if err := unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_MULTICAST_TTL, 1); err != nil {
		return fmt.Errorf("ospf/transport: setsockopt IP_MULTICAST_TTL: %w", err)
	}
	// RFC 2328 Appendix D.3: multicast loopback must not create a self-neighbor.
	if err := unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_MULTICAST_LOOP, 0); err != nil {
		return fmt.Errorf("ospf/transport: setsockopt IP_MULTICAST_LOOP: %w", err)
	}
	if err := unix.SetsockoptInet4Addr(fd, unix.IPPROTO_IP, unix.IP_MULTICAST_IF, local); err != nil {
		return fmt.Errorf("ospf/transport: setsockopt IP_MULTICAST_IF ifindex %d: %w", ifindex, err)
	}
	return nil
}

func isOSPFMulticast(addr netip.Addr) bool {
	return addr == AllSPFRouters || addr == AllDRouters
}

func joinGroup(fd, ifindex int, local [4]byte, group netip.Addr) error {
	if !group.Is4() || !isOSPFMulticast(group) {
		return ErrInvalidDestination
	}
	mreq := unix.IPMreq{Multiaddr: group.As4(), Interface: local}
	if err := unix.SetsockoptIPMreq(fd, unix.IPPROTO_IP, unix.IP_ADD_MEMBERSHIP, &mreq); err != nil {
		return fmt.Errorf("ospf/transport: join %s on ifindex %d: %w", group, ifindex, err)
	}
	return nil
}

func leaveGroup(fd, ifindex int, local [4]byte, group netip.Addr) error {
	if !group.Is4() || !isOSPFMulticast(group) {
		return ErrInvalidDestination
	}
	mreq := unix.IPMreq{Multiaddr: group.As4(), Interface: local}
	if err := unix.SetsockoptIPMreq(fd, unix.IPPROTO_IP, unix.IP_DROP_MEMBERSHIP, &mreq); err != nil {
		return fmt.Errorf("ospf/transport: leave %s on ifindex %d: %w", group, ifindex, err)
	}
	return nil
}

func closeFD(fd int) {
	if err := unix.Close(fd); err != nil {
		logger().Warn("ospf/transport: close fd", "err", err)
	}
}
