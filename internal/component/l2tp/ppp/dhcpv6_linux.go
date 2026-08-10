// Design: docs/research/l2tpv2-ze-integration.md -- DHCPv6-PD UDP listener (Linux)
// Related: dhcpv6.go -- DHCPv6 codec (cross-platform)
// Related: ipv6_service.go -- IPv6Service.handleDHCPv6 state machine

//go:build linux

package ppp

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"syscall"

	"golang.org/x/sys/unix"
)

// startDHCPv6Server opens a UDP socket on port 547 bound to ifname,
// joins the DHCPv6 multicast group (ff02::1:2), and starts a
// goroutine that reads DHCPv6 messages and delegates to
// svc.handleDHCPv6. Returns a cancel function.
func startDHCPv6Server(ifname string, svc *IPv6Service, serverID DHCPv6DUID, allocPrefix func() (netip.Prefix, bool), logger *slog.Logger) (func(), error) {
	lc := net.ListenConfig{
		Control: func(_, _ string, c syscall.RawConn) error {
			var opErr error
			if ctrlErr := c.Control(func(fd uintptr) {
				// SO_REUSEADDR allows concurrent sessions to each bind :547 on their own pppN
				if err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err != nil {
					opErr = err
					return
				}
				opErr = unix.SetsockoptString(int(fd), unix.SOL_SOCKET, unix.SO_BINDTODEVICE, ifname)
			}); ctrlErr != nil {
				return ctrlErr
			}
			return opErr
		},
	}

	pc, err := lc.ListenPacket(context.Background(), "udp6", "[::]:547")
	if err != nil {
		return nil, err
	}

	iface, ifErr := net.InterfaceByName(ifname)
	if ifErr == nil {
		group := net.ParseIP("ff02::1:2")
		udpConn, ok := pc.(*net.UDPConn)
		if ok {
			rawConn, rcErr := udpConn.SyscallConn()
			if rcErr == nil {
				if ctrlErr := rawConn.Control(func(fd uintptr) {
					var mreq unix.IPv6Mreq
					copy(mreq.Multiaddr[:], group.To16())
					mreq.Interface = uint32(iface.Index)
					//nolint:errcheck // best-effort multicast join; DHCPv6 still works via link-local unicast
					unix.SetsockoptIPv6Mreq(int(fd), unix.IPPROTO_IPV6, unix.IPV6_JOIN_GROUP, &mreq)
				}); ctrlErr != nil {
					logger.Debug("ppp: DHCPv6 multicast join control failed", "error", ctrlErr)
				}
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	go dhcpv6ServerLoop(ctx, pc, svc, serverID, allocPrefix, logger)

	return func() {
		cancel()
		if cerr := pc.Close(); cerr != nil {
			logger.Warn("ppp: DHCPv6 close failed", "error", cerr)
		}
	}, nil
}

func dhcpv6ServerLoop(ctx context.Context, conn net.PacketConn, svc *IPv6Service, serverID DHCPv6DUID, allocPrefix func() (netip.Prefix, bool), logger *slog.Logger) {
	var buf [1500]byte
	for {
		if ctx.Err() != nil {
			return
		}

		n, addr, err := conn.ReadFrom(buf[:])
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Debug("ppp: DHCPv6 read error", "error", err)
			continue
		}

		if n < 4 {
			continue
		}

		msg, parseErr := parseDHCPv6(buf[:n])
		if parseErr != nil {
			logger.Debug("ppp: DHCPv6 parse error", "error", parseErr)
			continue
		}

		resp, handleErr := svc.handleDHCPv6(msg, serverID, allocPrefix)
		if handleErr != nil {
			logger.Warn("ppp: DHCPv6 handle error", "error", handleErr)
			continue
		}
		if resp == nil {
			continue
		}

		if _, writeErr := conn.WriteTo(resp, addr); writeErr != nil {
			logger.Debug("ppp: DHCPv6 write error", "error", writeErr)
		}
	}
}
