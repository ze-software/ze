// Design: docs/research/l2tpv2-ze-integration.md -- kernel IPv6 control sockets
// RFC 4861 Sections 6.1.1 and 6.1.2 -- see rfc/short/rfc4861.md
// Related: ipv6.go -- carrier I/O and cancellation ownership
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
	"syscall"

	"golang.org/x/net/ipv6"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/ndp"
)

func openIPv6Carrier(ctx context.Context, ifname string) (ppp.IPv6Carrier, error) {
	device, err := net.InterfaceByName(ifname)
	if err != nil {
		return nil, err
	}
	return acquireIPv6Carrier(ctx, ifname, device.Index,
		func(ctx context.Context) (raSocket, error) { return openRASocket(ctx, ifname, device) },
		func(ctx context.Context) (dhcpSocket, error) { return openDHCPSocket(ctx, ifname, device) })
}

func sessionIPv6Listen(ifname string) net.ListenConfig {
	return net.ListenConfig{Control: func(_, _ string, raw syscall.RawConn) error {
		var optionErr error
		controlErr := raw.Control(func(fd uintptr) {
			if err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err != nil {
				optionErr = err
				return
			}
			optionErr = unix.SetsockoptString(int(fd), unix.SOL_SOCKET, unix.SO_BINDTODEVICE, ifname)
		})
		return errors.Join(controlErr, optionErr)
	}}
}

func openRASocket(ctx context.Context, ifname string, device *net.Interface) (*ipv6.PacketConn, error) {
	config := sessionIPv6Listen(ifname)
	conn, err := config.ListenPacket(ctx, "ip6:ipv6-icmp", "::")
	if err != nil {
		return nil, err
	}
	packet := ipv6.NewPacketConn(conn)
	if err := configureRASocket(packet, device); err != nil {
		return nil, errors.Join(err, packet.Close())
	}
	return packet, nil
}

func configureRASocket(packet *ipv6.PacketConn, device *net.Interface) error {
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
	if err := packet.JoinGroup(device, &net.IPAddr{IP: net.IPv6linklocalallrouters}); err != nil {
		return err
	}
	var filter ipv6.ICMPFilter
	filter.SetAll(true)
	filter.Accept(ipv6.ICMPTypeRouterSolicitation)
	return packet.SetICMPFilter(&filter)
}

func openDHCPSocket(ctx context.Context, ifname string, device *net.Interface) (net.PacketConn, error) {
	config := sessionIPv6Listen(ifname)
	conn, err := config.ListenPacket(ctx, "udp6", "[::]:547")
	if err != nil {
		return nil, err
	}
	packet := ipv6.NewPacketConn(conn)
	group := &net.IPAddr{IP: net.ParseIP("ff02::1:2")}
	if err := packet.JoinGroup(device, group); err != nil {
		return nil, errors.Join(err, conn.Close())
	}
	return conn, nil
}
