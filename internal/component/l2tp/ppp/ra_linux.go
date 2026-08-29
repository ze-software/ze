// Design: docs/research/l2tpv2-ze-integration.md -- RA sender for PPP IPv6 (Linux)
// Related: ra.go -- RA message building (cross-platform)
// Related: ra_send.go -- the send loop, the advertised lifetimes, and the stop path

//go:build linux

package ppp

import (
	"context"
	"log/slog"
	"net"

	"golang.org/x/net/ipv6"
	"golang.org/x/sys/unix"
)

// startRASender opens a raw ICMPv6 socket on ifname, joins the
// all-routers multicast group (ff02::2), sets ICMP6_FILTER to accept
// only Router Solicitations, and starts a goroutine that sends an
// initial burst of RAs followed by periodic RAs. Returns a cancel
// function that stops the goroutines, sends the final Router
// Advertisement, and closes the socket. See stopRASender in ra_send.go.
func startRASender(ifname string, logger *slog.Logger) (func(), error) {
	conn, err := (&net.ListenConfig{}).ListenPacket(context.Background(), "ip6:ipv6-icmp", "::")
	if err != nil {
		return nil, err
	}

	pc := ipv6.NewPacketConn(conn)

	iface, err := net.InterfaceByName(ifname)
	if err != nil {
		if cerr := conn.Close(); cerr != nil {
			logger.Warn("ppp: RA close failed", "error", cerr)
		}
		return nil, err
	}

	if ipConn, ok := conn.(*net.IPConn); ok {
		rawConn, rcErr := ipConn.SyscallConn()
		if rcErr == nil {
			if ctrlErr := rawConn.Control(func(fd uintptr) {
				//nolint:errcheck // best-effort bind; pppN is point-to-point so scope is already narrow
				unix.SetsockoptString(int(fd), unix.SOL_SOCKET, unix.SO_BINDTODEVICE, ifname)
			}); ctrlErr != nil {
				logger.Debug("ppp: RA bind-to-device control failed", "error", ctrlErr)
			}
		}
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

	sender := &raSender{
		conn:    pc,
		dst:     allNodes,
		ifIndex: iface.Index,
		ifname:  ifname,
		logger:  logger,
	}

	rsCh := make(chan struct{}, 1)
	senderDone := make(chan struct{})
	go rsReaderLoop(ctx, pc, rsCh)
	go raSenderLoop(ctx, sender, rsCh, senderDone)

	return func() {
		stopRASender(cancel, senderDone, sender, conn)
	}, nil
}

func rsReaderLoop(ctx context.Context, pc *ipv6.PacketConn, rsCh chan<- struct{}) {
	var buf [256]byte
	for {
		if ctx.Err() != nil {
			return
		}
		if _, _, _, err := pc.ReadFrom(buf[:]); err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		// capacity-1 channel coalesces RS bursts into one RA send
		select {
		case rsCh <- struct{}{}:
		case <-ctx.Done():
			return
		}
	}
}
