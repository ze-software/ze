//go:build linux

// Design: docs/architecture/diagnostics/active-probes.md -- the socket options behind a DF mode
// Related: socket.go -- OpenICMP, which installs the Control this file builds
// Related: socket_other.go -- the non-Linux stub with the same signatures
//
// Linux-specific socket options for the probe socket, and the unprivileged
// ICMP datagram socket. Split from socket.go because IP_MTU_DISCOVER,
// IP_RECVERR, their IPv6 twins and the ping socket exist only on Linux. The
// option shape follows internal/component/bfd/transport/udp_linux.go: a
// net.ListenConfig.Control function that sets each option with
// unix.SetsockoptInt and reports the first failure by name.
//
// The ping socket cannot come from net.ListenConfig, which knows no
// datagram ICMP network, and golang.org/x/net/icmp, whose "udp4" endpoint
// is this socket, is not vendored. So listenDatagramICMP opens it with
// unix.Socket, sets the same options on the bare descriptor, binds it so
// the kernel assigns the echo identifier, and hands it to
// net.FilePacketConn, which yields a *net.UDPConn.

package probe

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// errDatagramICMPUnbound is a Getsockname that named no port: the kernel
// assigns the identifier at bind (net/ipv4/ping.c ping_get_port), so an
// absent port is a socket the probers cannot match replies on.
var errDatagramICMPUnbound = errors.New("probe: the datagram ICMP socket reports no bound identifier")

// dfOptions is the per-family spelling of the two options a DF mode installs.
type dfOptions struct {
	level       int
	mtuDiscover int
	recvErr     int
}

// dfOptionsOf is the option set for f. Both families are always handled: the
// portable OpenICMP already refused FamilyAny before this is reached.
func dfOptionsOf(f Family) dfOptions {
	if f == FamilyIPv6 {
		return dfOptions{level: unix.IPPROTO_IPV6, mtuDiscover: unix.IPV6_MTU_DISCOVER, recvErr: unix.IPV6_RECVERR}
	}
	return dfOptions{level: unix.IPPROTO_IP, mtuDiscover: unix.IP_MTU_DISCOVER, recvErr: unix.IP_RECVERR}
}

// pmtuDiscValue is the IP_MTU_DISCOVER value for a mode. The IPv6 constants
// carry the same values as the IPv4 ones, so one table serves both families.
// DFOff is IP_PMTUDISC_DONT rather than the kernel's default: the default,
// IP_PMTUDISC_WANT, sets the DF bit on every datagram that fits the path.
func pmtuDiscValue(df DFMode) int {
	switch df {
	case DFBypassCache:
		return unix.IP_PMTUDISC_PROBE
	case DFHonorCache:
		return unix.IP_PMTUDISC_DO
	default:
		return unix.IP_PMTUDISC_DONT
	}
}

// dfControl builds the ListenConfig.Control that installs df on a socket of
// family f. The Control sets IP_MTU_DISCOVER for the DF bit and the cache
// policy on every mode, and for a mode that sets DF it also sets IP_RECVERR
// so the kernel queues a router's Fragmentation Needed or Packet Too Big
// answer on the socket's error queue, where drainErrorQueue collects the
// reported next-hop MTU. DFOff installs no IP_RECVERR: with the bit clear
// nothing is refused, and the queue would only carry errors the probers do
// not read.
func dfControl(f Family, df DFMode) (func(network, address string, c syscall.RawConn) error, error) {
	opts := dfOptionsOf(f)
	discover := pmtuDiscValue(df)
	recvErr := df != DFOff
	return func(_, _ string, c syscall.RawConn) error {
		return applyDFOptions(c, opts, discover, recvErr)
	}, nil
}

// applyDFOptions sets the options on the raw socket. A setsockopt failure
// is returned by option name, and OpenICMP closes nothing because ListenPacket
// closes the socket itself when Control fails.
func applyDFOptions(c syscall.RawConn, opts dfOptions, discover int, recvErr bool) error {
	var innerErr error
	controlErr := c.Control(func(fd uintptr) {
		innerErr = setDFOptions(int(fd), opts, discover, recvErr)
	})
	if controlErr != nil {
		return fmt.Errorf("rawconn Control: %w", controlErr)
	}
	return innerErr
}

// setDFOptions sets the options on a bare descriptor: the raw socket's
// Control and the datagram socket's opener both end here.
func setDFOptions(fd int, opts dfOptions, discover int, recvErr bool) error {
	if err := unix.SetsockoptInt(fd, opts.level, opts.mtuDiscover, discover); err != nil {
		return fmt.Errorf("setsockopt IP_MTU_DISCOVER=%d: %w", discover, err)
	}
	if !recvErr {
		return nil
	}
	if err := unix.SetsockoptInt(fd, opts.level, opts.recvErr, 1); err != nil {
		return fmt.Errorf("setsockopt IP_RECVERR: %w", err)
	}
	return nil
}

// listenDatagramICMP opens Linux's unprivileged ICMP socket for family with
// the DF options of df installed, bound to bind (the zero Addr binds the
// unspecified address). The kernel refuses the socket with EACCES when the
// process group is outside net.ipv4.ping_group_range, which the caller
// reports beside the raw refusal. The identifier returned is the port the
// bind assigned, which the kernel writes into every echo the socket sends.
func listenDatagramICMP(family Family, bind netip.Addr, df DFMode) (net.PacketConn, uint16, error) {
	domain, proto := unix.AF_INET, unix.IPPROTO_ICMP
	if family == FamilyIPv6 {
		domain, proto = unix.AF_INET6, unix.IPPROTO_ICMPV6
	}
	fd, err := unix.Socket(domain, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, proto)
	if err != nil {
		return nil, 0, err
	}
	// f owns fd from here; FilePacketConn duplicates the descriptor, so f
	// is closed on every path out.
	f := os.NewFile(uintptr(fd), "icmp-datagram")
	defer f.Close() //nolint:errcheck // the conn holds its own duplicate

	if err := setDFOptions(fd, dfOptionsOf(family), pmtuDiscValue(df), df != DFOff); err != nil {
		return nil, 0, err
	}
	sa, err := datagramBindAddr(family, bind)
	if err != nil {
		return nil, 0, err
	}
	if err := unix.Bind(fd, sa); err != nil {
		return nil, 0, fmt.Errorf("bind %s: %w", bind, err)
	}
	id, err := datagramIdentifier(fd)
	if err != nil {
		return nil, 0, err
	}
	conn, err := net.FilePacketConn(f)
	if err != nil {
		return nil, 0, fmt.Errorf("wrap datagram socket: %w", err)
	}
	return conn, id, nil
}

// datagramBindAddr is the sockaddr the datagram socket binds to: bind at
// port 0, so the kernel picks a free identifier, or the unspecified address
// of the family when bind is zero. An IPv6 zone is resolved to its
// interface index, as ListenPacket does for the raw kind.
func datagramBindAddr(family Family, bind netip.Addr) (unix.Sockaddr, error) {
	if family == FamilyIPv4 {
		sa := &unix.SockaddrInet4{}
		if bind.IsValid() {
			sa.Addr = bind.Unmap().As4()
		}
		return sa, nil
	}
	sa := &unix.SockaddrInet6{}
	if !bind.IsValid() {
		return sa, nil
	}
	sa.Addr = bind.As16()
	if bind.Zone() == "" {
		return sa, nil
	}
	ifi, err := net.InterfaceByName(bind.Zone())
	if err != nil {
		return nil, fmt.Errorf("bind zone %q: %w", bind.Zone(), err)
	}
	sa.ZoneId = uint32(ifi.Index)
	return sa, nil
}

// datagramIdentifier reads the identifier the bind assigned: the port of the
// socket's own name. A name with no port is refused, never answered as 0.
func datagramIdentifier(fd int) (uint16, error) {
	name, err := unix.Getsockname(fd)
	if err != nil {
		return 0, fmt.Errorf("getsockname: %w", err)
	}
	port := 0
	switch sa := name.(type) {
	case *unix.SockaddrInet4:
		port = sa.Port
	case *unix.SockaddrInet6:
		port = sa.Port
	}
	if port == 0 {
		return 0, errDatagramICMPUnbound
	}
	return uint16(port), nil
}
