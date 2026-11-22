// Design: docs/research/l2tpv2-ze-integration.md -- kernel IPv6 control sockets
// RFC 4861 Sections 6.1.1 and 6.1.2 -- see rfc/short/rfc4861.md
// Related: ipv6.go -- carrier I/O and cancellation ownership
// Related: created_unit_control_socket.go -- original-unit control socket constructor
// Linux receive contract: net/ipv6/raw.c rawv6_init_sk enables ICMPv6 checksum
// verification; rawv6_rcv_skb rejects bad checksums before socket delivery.
// Source checked: https://github.com/torvalds/linux/blob/v6.18/net/ipv6/raw.c
// The shipped kernel's runtime receiver proof is separate from this source check.

//go:build linux

package kernel

import (
	"context"
	"errors"
	"net"
	"os"

	"golang.org/x/net/ipv6"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/ndp"
)

// RFC 9915 Section 7.2: "Servers and relay agents MUST listen for DHCP
// messages on UDP port 547."
const dhcpv6ServerPort = 547

// openIPv6Carrier opens both control sockets from the original PPP unit, inside
// its original namespace. The kernel binds each socket to that unit's device
// and refuses it once the unit retires, so no interface name or index lookup
// supplies authority: a renamed original keeps its carrier, and a replacement
// that takes the old name never reaches it. index is the unit's captured
// native index, used only to filter receive metadata the kernel reports.
func openIPv6Carrier(ctx context.Context, unit *CreatedUnit, index int) (ppp.IPv6Carrier, error) {
	var carrier ppp.IPv6Carrier
	err := unit.WithOriginalNamespace(ctx, func() error {
		var acquireErr error
		carrier, acquireErr = acquireIPv6Carrier(ctx, index,
			func(ctx context.Context) (raSocket, error) { return openRASocket(ctx, unit) },
			func(ctx context.Context) (dhcpSocket, error) { return openDHCPSocket(ctx, unit) })
		return acquireErr
	})
	if err == nil {
		return carrier, nil
	}
	acquired, usable := carrier.(*ipv6Carrier)
	if !usable {
		// Nil, or the non-usable partial owner acquisition already returned.
		return carrier, err
	}
	// The namespace helper failed after a complete acquisition, while
	// restoring the caller's namespace: the carrier is never published, and
	// its cleanup owner stays reachable.
	return failedIPv6Acquisition(acquired.ra, acquired.dhcp, err)
}

// openOriginalIPv6Conn converts one original-unit control socket to a Go
// connection. prepare runs on the owned descriptor before conversion. The
// constructor's returned file is closed on every outcome: the connection holds
// its own duplicate, and a file returned with an error is still owned here.
func openOriginalIPv6Conn(ctx context.Context, unit *CreatedUnit, kind IPv6ControlSocketKind, prepare func(fd int) error) (net.PacketConn, error) {
	file, err := unit.OpenIPv6ControlSocket(ctx, kind)
	if file == nil {
		return nil, err
	}
	if err == nil {
		if prepare != nil {
			err = controlOriginalIPv6File(file, prepare)
		}
	}
	var conn net.PacketConn
	if err == nil {
		conn, err = net.FilePacketConn(file)
	}
	return conn, errors.Join(err, file.Close())
}

func controlOriginalIPv6File(file *os.File, prepare func(fd int) error) error {
	raw, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var prepareErr error
	controlErr := raw.Control(func(fd uintptr) { prepareErr = prepare(int(fd)) })
	return errors.Join(controlErr, prepareErr)
}

func openRASocket(ctx context.Context, unit *CreatedUnit) (raSocket, error) {
	conn, err := openOriginalIPv6Conn(ctx, unit, IPv6ControlSocketICMP6Raw, nil)
	if conn == nil {
		return nil, err
	}
	packet := ipv6.NewPacketConn(conn)
	owner := &ownedIPv6RASocket{PacketConn: packet, cleanup: &nativeIPv6SocketCleanup{owner: conn}}
	if err != nil {
		// acquireIPv6Carrier consumes this partial owner even on error.
		return owner, err
	}
	if err := configureRASocket(packet); err != nil {
		return owner, err
	}
	return owner, nil
}

func configureRASocket(packet *ipv6.PacketConn) error {
	if err := packet.SetControlMessage(ipv6.FlagHopLimit|ipv6.FlagInterface, true); err != nil {
		return err
	}
	// RFC 4861 Section 6.1.2: "The IP Hop Limit field has a value of 255".
	// Both defaults and the per-message control value select this hop limit.
	if err := packet.SetMulticastHopLimit(ndp.MessageHopLimit); err != nil {
		return err
	}
	if err := packet.SetHopLimit(ndp.MessageHopLimit); err != nil {
		return err
	}
	// Interface zero: the kernel joins on the original unit's device, which
	// the socket retains, and refuses once that unit retires.
	if err := packet.JoinGroup(nil, &net.IPAddr{IP: net.IPv6linklocalallrouters}); err != nil {
		return err
	}
	var filter ipv6.ICMPFilter
	filter.SetAll(true)
	filter.Accept(ipv6.ICMPTypeRouterSolicitation)
	return packet.SetICMPFilter(&filter)
}

// prepareDHCPSocket binds the owned UDP socket to the server port. Every
// session's socket binds the same port; the kernel already bound each one to
// its own original device, which keeps them apart.
func prepareDHCPSocket(fd int) error {
	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err != nil {
		return err
	}
	return unix.Bind(fd, &unix.SockaddrInet6{Port: dhcpv6ServerPort})
}

func openDHCPSocket(ctx context.Context, unit *CreatedUnit) (net.PacketConn, error) {
	conn, err := openOriginalIPv6Conn(ctx, unit, IPv6ControlSocketUDP6, prepareDHCPSocket)
	if conn == nil {
		return nil, err
	}
	owner := &ownedIPv6DHCPSocket{PacketConn: conn, cleanup: &nativeIPv6SocketCleanup{owner: conn}}
	if err != nil {
		// The acquisition boundary owns cleanup of this partial socket.
		return owner, err
	}
	// Interface zero: the kernel joins on the original unit's device.
	group := &net.IPAddr{IP: net.ParseIP("ff02::1:2")}
	if err := ipv6.NewPacketConn(conn).JoinGroup(nil, group); err != nil {
		return owner, err
	}
	return owner, nil
}

type ownedIPv6RASocket struct {
	*ipv6.PacketConn
	cleanup *nativeIPv6SocketCleanup
}

func (s *ownedIPv6RASocket) Close() error { return s.cleanup.Close() }
func (s *ownedIPv6RASocket) RetryIPv6SocketCleanup(ctx context.Context) error {
	return s.cleanup.RetryIPv6SocketCleanup(ctx)
}
