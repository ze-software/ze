// Design: docs/research/l2tpv2-ze-integration.md -- RA sender for PPP IPv6 (Linux)
// Related: ra.go -- RA message building (cross-platform)

//go:build linux

package ppp

import (
	"context"
	"log/slog"
	"net"
	"time"

	"golang.org/x/net/ipv6"
	"golang.org/x/sys/unix"
)

const (
	raInitialCount     = 5
	raInitialInterval  = 3 * time.Second
	raPeriodicInterval = 600 * time.Second
)

// startRASender opens a raw ICMPv6 socket on ifname, joins the
// all-routers multicast group (ff02::2), sets ICMP6_FILTER to accept
// only Router Solicitations, and starts a goroutine that sends an
// initial burst of RAs followed by periodic RAs. Returns a cancel
// function to stop the goroutine and close the socket.
func startRASender(ifname string, logger *slog.Logger) (func(), error) {
	conn, err := net.ListenPacket("ip6:ipv6-icmp", "::")
	if err != nil {
		return nil, err
	}

	pc := ipv6.NewPacketConn(conn)

	iface, err := net.InterfaceByName(ifname)
	if err != nil {
		conn.Close()
		return nil, err
	}

	rawConn, rcErr := conn.(*net.IPConn).SyscallConn()
	if rcErr == nil {
		rawConn.Control(func(fd uintptr) {
			//nolint:errcheck // best-effort bind; pppN is point-to-point so scope is already narrow
			unix.SetsockoptString(int(fd), unix.SOL_SOCKET, unix.SO_BINDTODEVICE, ifname)
		})
	}

	// RFC 4861 Section 6.1.1: routers join the all-routers multicast
	// address to receive Router Solicitations.
	allRouters := &net.IPAddr{IP: net.ParseIP("ff02::2")}
	if joinErr := pc.JoinGroup(iface, allRouters); joinErr != nil {
		logger.Warn("ppp: RA failed to join ff02::2", "error", joinErr, "iface", ifname)
	}

	var filter ipv6.ICMPFilter
	filter.SetAll(true)
	filter.Accept(ipv6.ICMPTypeRouterSolicitation)
	if filterErr := pc.SetICMPFilter(&filter); filterErr != nil {
		logger.Warn("ppp: RA failed to set ICMP6_FILTER", "error", filterErr)
	}

	allNodes := &net.UDPAddr{IP: net.ParseIP("ff02::1"), Zone: ifname}

	ctx, cancel := context.WithCancel(context.Background())

	go raSenderLoop(ctx, pc, allNodes, iface, ifname, logger)

	return func() {
		cancel()
		conn.Close()
	}, nil
}

func raSenderLoop(ctx context.Context, pc *ipv6.PacketConn, dst net.Addr, iface *net.Interface, ifname string, logger *slog.Logger) {
	sendRA := func() {
		var buf [256]byte
		n := BuildRA(buf[:], RAConfig{
			CurHopLimit:    64,
			Managed:        true,
			OtherConfig:    true,
			RouterLifetime: 1800,
		})
		cm := &ipv6.ControlMessage{IfIndex: iface.Index}
		if _, err := pc.WriteTo(buf[:n], cm, dst); err != nil {
			logger.Debug("ppp: RA send failed", "error", err, "iface", ifname)
		}
	}

	for range raInitialCount {
		sendRA()
		select {
		case <-ctx.Done():
			return
		case <-time.After(raInitialInterval):
		}
	}

	ticker := time.NewTicker(raPeriodicInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sendRA()
		}
	}
}
