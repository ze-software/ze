// Design: docs/research/l2tpv2-ze-integration.md -- Router Advertisement transmission for PPP IPv6.
// RFC: rfc/full/rfc4861.txt -- Sections 6.2.1, 6.2.5. RFC 4861 has no rfc/short/ summary and is not enrolled.
// Related: ra.go -- RA message building.
// Related: ra_linux.go -- opens the ICMPv6 socket and starts these loops.

package ppp

import (
	"context"
	"io"
	"log/slog"
	"net"
	"time"

	"golang.org/x/net/ipv6"
)

const (
	// raInitialCount and raInitialInterval shape the burst sent when IPv6CP
	// opens, so a subscriber finds the router without waiting for the first
	// periodic advertisement.
	raInitialCount    = 5
	raInitialInterval = 3 * time.Second

	// raRouterLifetime is the Router Lifetime advertised to the subscriber,
	// in seconds, and raPeriodicInterval is one third of it. RFC 4861
	// Section 6.2.1 gives AdvDefaultLifetime a default of
	// 3 * MaxRtrAdvInterval, so the subscriber keeps its default route
	// across two lost advertisements. Deriving one from the other keeps
	// that margin if the lifetime ever changes.
	raRouterLifetime   = 1800
	raPeriodicInterval = raRouterLifetime * time.Second / 3

	// raCeaseLifetime is the Router Lifetime of the final advertisement.
	// Zero tells the subscriber this router is no longer a default router.
	// RFC 4861 Section 4.2.
	raCeaseLifetime = 0
)

// raWriter writes one Router Advertisement to the link. *ipv6.PacketConn
// implements it.
//
// The simpler design holds *ipv6.PacketConn directly. It was rejected because
// the order stopRASender guarantees, final advertisement before close, is then
// provable only from a raw ICMPv6 socket, which needs both Linux and root. The
// seam is the same one vppOps uses for the same reason (ai/rules/testing.md).
type raWriter interface {
	WriteTo(b []byte, cm *ipv6.ControlMessage, dst net.Addr) (int, error)
}

// raSender sends Router Advertisements for one PPP subscriber interface.
// Safe for concurrent use: it holds no mutable state, and *ipv6.PacketConn is
// itself safe for concurrent use.
type raSender struct {
	conn    raWriter
	dst     net.Addr
	ifIndex int
	ifname  string
	logger  *slog.Logger
}

// send writes one Router Advertisement carrying lifetime seconds in the Router
// Lifetime field. A failure is logged and not returned: a periodic
// advertisement is repeated by the next tick, and the final one is best effort
// because an abrupt teardown can remove the interface first.
func (s *raSender) send(lifetime uint16) {
	var buf [256]byte
	n := BuildRA(buf[:], RAConfig{
		CurHopLimit:    64,
		Managed:        true,
		OtherConfig:    true,
		RouterLifetime: lifetime,
	})
	cm := &ipv6.ControlMessage{IfIndex: s.ifIndex}
	if _, err := s.conn.WriteTo(buf[:n], cm, s.dst); err != nil {
		s.logger.Debug("ppp: RA send failed", "error", err, "iface", s.ifname)
	}
}

// raSenderLoop sends the initial burst of Router Advertisements, then one for
// every tick of the periodic timer and one for every Router Solicitation
// coalesced onto rsCh. It closes senderDone when it returns, so stopRASender
// can wait for it. It returns only when ctx is canceled, because a session
// advertises for as long as it is up.
func raSenderLoop(ctx context.Context, sender *raSender, rsCh <-chan struct{}, senderDone chan<- struct{}) {
	defer close(senderDone)

	for range raInitialCount {
		sender.send(raRouterLifetime)
		select {
		case <-ctx.Done():
			return
		case <-rsCh:
			sender.send(raRouterLifetime)
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
			sender.send(raRouterLifetime)
		case <-rsCh:
			sender.send(raRouterLifetime)
		}
	}
}

// stopRASender ceases advertisement on the subscriber's interface. It cancels
// the sender goroutine, waits for it to leave so no advertisement can follow
// the final one, sends one Router Advertisement with a Router Lifetime of zero,
// and only then closes the socket. The caller MUST NOT use conn afterwards, and
// MUST pass the senderDone channel that raSenderLoop closes.
//
// RFC 4861 Section 6.2.5: a router whose interface ceases to be an advertising
// interface "SHOULD transmit one or more (but not more than
// MAX_FINAL_RTR_ADVERTISEMENTS) final multicast Router Advertisements on the
// interface with a Router Lifetime field of zero". A PPP session teardown is
// that case: the interface stops advertising, as it does when an operator
// disables it. Without the final advertisement the subscriber keeps this router
// as its default router until raRouterLifetime expires.
//
// Ze sends one of the three the RFC permits. The subscriber drops the default
// route on the first, and RFC 4861 Section 6.2.6 rate limits consecutive
// multicast advertisements to one every MIN_DELAY_BETWEEN_RAS seconds, so the
// other two would hold the session's teardown for six seconds.
func stopRASender(cancel context.CancelFunc, senderDone <-chan struct{}, sender *raSender, conn io.Closer) {
	cancel()
	<-senderDone

	sender.send(raCeaseLifetime)

	if err := conn.Close(); err != nil {
		sender.logger.Warn("ppp: RA close failed", "error", err)
	}
}
