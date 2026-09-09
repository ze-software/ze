// Design: docs/research/l2tpv2-ze-integration.md -- RA sender for PPP IPv6 (Linux)
// Related: ra.go -- RA message building (cross-platform)
// Related: ra_send.go -- the send loop and the stop path
// Related: ra_schedule.go -- the advertised lifetimes and the RFC 4861 send schedule

//go:build linux

package ppp

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"net"
	"time"

	"golang.org/x/net/ipv6"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/ndp"
	"github.com/ze-software/ze/internal/core/pacer"
)

// startRASender opens a raw ICMPv6 socket on ifname, joins the all-routers
// multicast group (ff02::2), sets ICMP6_FILTER to accept only Router
// Solicitations, and starts a goroutine that advertises on the schedule of RFC
// 4861 Sections 6.2.4 and 6.2.6. Returns a cancel function that stops the
// goroutines, sends the final Router Advertisement, and closes the socket. See
// raSenderLoop and stopRASender in ra_send.go, and raSchedule in
// ra_schedule.go.
func startRASender(ifname string, logger *slog.Logger) (func(), error) {
	conn, err := (&net.ListenConfig{}).ListenPacket(context.Background(), "ip6:ipv6-icmp", "::")
	if err != nil {
		return nil, err
	}

	pc := ipv6.NewPacketConn(conn)

	// RFC 4861 Section 6.1.2: "A node MUST silently discard any received
	// Router Advertisement messages that do not satisfy all of the following
	// validity checks: ... The IP Hop Limit field has a value of 255". The
	// Linux default multicast hop limit is 1, so a socket that never sets this
	// advertises to nobody. ra_send.go sets the same field per packet in the
	// control message; both paths are written because a subscriber that acts on
	// no advertisement looks exactly like a subscriber that received none.
	if hlErr := pc.SetMulticastHopLimit(ndp.MessageHopLimit); hlErr != nil {
		logger.Warn("ppp: RA failed to set multicast hop limit", "error", hlErr, "iface", ifname)
	}
	if hlErr := pc.SetHopLimit(ndp.MessageHopLimit); hlErr != nil {
		logger.Warn("ppp: RA failed to set hop limit", "error", hlErr, "iface", ifname)
	}

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

	// RFC 4861 Section 6.2.4 randomizes the interval so routers on one link
	// do not synchronize. The interface index seeds the second word, so two
	// subscribers that come up in the same nanosecond still diverge.
	//nolint:gosec // the seed drives timer jitter, never a security decision
	random := rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), uint64(iface.Index)))
	sched := newRASchedule(clock.RealClock{}, random)

	rsCh := make(chan struct{}, 1)
	senderDone := make(chan struct{})
	go rsReaderLoop(ctx, pc, rsCh, ifname, logger)
	go raSenderLoop(ctx, sender, sched, rsCh, senderDone)

	return func() {
		stopRASender(cancel, senderDone, sender, sched, conn)
	}, nil
}

// rsReaderLoop reads Router Solicitations off the ICMPv6 socket and signals
// raSenderLoop to answer one. A read error that is neither ctx being done
// nor recovered by the next attempt is logged, counted on
// ze_ppp_reader_errors_total{loop="ra"} (metrics.go), and paced by p, so a
// socket that fails forever costs a bounded slice of a core rather than all
// of it. The wait observes ctx.Done() directly: a context satisfies
// <-chan struct{} through that method, so it needs no adapter to the
// pacer's Wait.
func rsReaderLoop(ctx context.Context, pc *ipv6.PacketConn, rsCh chan<- struct{}, ifname string, logger *slog.Logger) {
	var buf [256]byte
	var p pacer.Pacer
	for {
		if ctx.Err() != nil {
			return
		}
		if _, _, _, err := pc.ReadFrom(buf[:]); err != nil {
			if ctx.Err() != nil {
				return
			}
			// Not context-done, so this is an error the loop cannot
			// classify: it may clear on the next read or it may
			// persist. Log it and count it before pacing the retry, so
			// a socket that never recovers is visible in the log and
			// on the counter rather than only in CPU use.
			logger.Debug("ppp: RA reader read error", "interface", ifname, "error", err.Error())
			countReaderError(loopRA)
			if p.Wait(ctx.Done()) {
				return
			}
			continue
		}
		p.Succeed()
		// capacity-1 channel coalesces RS bursts into one RA send
		select {
		case rsCh <- struct{}{}:
		case <-ctx.Done():
			return
		}
	}
}
