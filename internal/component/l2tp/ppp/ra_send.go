// Design: docs/research/l2tpv2-ze-integration.md -- Router Advertisement transmission for PPP IPv6.
// RFC: rfc/full/rfc4861.txt -- Sections 6.2.4, 6.2.5, 6.2.6. RFC 4861 has no rfc/short/ summary and is not enrolled.
// Related: ra.go -- RA message building.
// Related: ra_schedule.go -- when each advertisement is due.
// Related: ra_linux.go -- opens the ICMPv6 socket and starts these loops.

package ppp

import (
	"context"
	"io"
	"log/slog"
	"net"

	"golang.org/x/net/ipv6"
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
// Lifetime field. A failure is logged and not returned: an unsolicited
// advertisement is repeated at the next scheduled one, and the final one is
// best effort because an abrupt teardown can remove the interface first.
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

// raSenderLoop advertises for as long as the session is up. It sends one
// advertisement as soon as the interface becomes an advertising interface, then
// one whenever sched says the next is due. A Router Solicitation coalesced onto
// rsCh moves that time earlier, never later, and sched decides by how much.
//
// The loop closes senderDone when it returns, so stopRASender can wait for it
// and then own sched. It returns only when ctx is canceled.
//
// A fresh timer is armed on each pass rather than one timer reset in place: the
// schedule moves whenever a solicitation arrives, and a timer that is created,
// waited on, and stopped inside one iteration can deliver nothing to the next
// one. The cost is one timer per advertisement, on a path that carries a few
// events per subscriber per raMinRtrAdvInterval.
func raSenderLoop(ctx context.Context, sender *raSender, sched *raSchedule, rsCh <-chan struct{}, senderDone chan<- struct{}) {
	defer close(senderDone)

	// RFC 4861 Section 6.2.4: an interface that becomes an advertising
	// interface advertises at once, so the subscriber finds the router
	// without waiting out an interval or sending a solicitation. It is the
	// first multicast advertisement on this interface, so no rate limit
	// applies to it.
	sender.send(raRouterLifetime)
	sched.advertised()

	for {
		due := make(chan struct{})
		timer := sched.clk.AfterFunc(sched.wait(), func() { close(due) })

		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-rsCh:
			timer.Stop()
			sched.solicit()
		case <-due:
			// A failed write is recorded as a send, which delays the
			// next advertisement and never brings it forward. Treating
			// it as unsent would turn a dead interface into a tight
			// retry loop, and sender.send already logs the failure.
			sender.send(raRouterLifetime)
			sched.advertised()
		}
	}
}

// stopRASender ceases advertisement on the subscriber's interface. It cancels
// the sender goroutine, waits for it to leave so no advertisement can follow
// the final one, waits out the rate limit, sends one Router Advertisement with
// a Router Lifetime of zero, and only then closes the socket. The caller MUST
// NOT use conn afterwards, and MUST pass the senderDone channel that
// raSenderLoop closes together with the sched that loop used.
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
// other two would hold the session's teardown for six seconds. That rate limit
// covers the final advertisement as well, which is what sched.ceaseWait
// measures: it is zero in steady state and at most MIN_DELAY_BETWEEN_RAS when a
// session is torn down just after it advertised.
func stopRASender(cancel context.CancelFunc, senderDone <-chan struct{}, sender *raSender, sched *raSchedule, conn io.Closer) {
	cancel()
	<-senderDone

	sched.clk.Sleep(sched.ceaseWait())
	sender.send(raCeaseLifetime)

	if err := conn.Close(); err != nil {
		sender.logger.Warn("ppp: RA close failed", "error", err)
	}
}
