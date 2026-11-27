// Design: docs/research/l2tpv2-ze-integration.md -- Router Advertisement carrier workers
// Related: ra_send.go -- unchanged advertisement schedule and final-RA ordering
// Related: ipv6_carrier.go -- receive cancellation preserves the write path

package ppp

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net"
	"time"

	"golang.org/x/net/ipv6"

	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/pacer"
)

type rsReader interface{ ReadRS(context.Context) error }

type carrierRAWriter struct{ carrier IPv6Carrier }

// WriteTo translates the sender's per-packet hop limit to the session carrier;
// interface and multicast destination selection belong to the created carrier.
func (w carrierRAWriter) WriteTo(payload []byte, control *ipv6.ControlMessage, _ net.Addr) (int, error) {
	if control == nil {
		return 0, errors.New("PPP Router Advertisement has no hop limit")
	}
	if err := w.carrier.WriteRA(payload, control.HopLimit); err != nil {
		return 0, err
	}
	return len(payload), nil
}

// raCarrierClose preserves the carrier close failure while the existing RA
// stop helper reports it and retains its final-advertisement ordering.
type raCarrierClose struct {
	carrier IPv6Carrier
	err     error
}

func (c *raCarrierClose) Close() error { c.err = c.carrier.Close(); return c.err }

func startRASender(carrier IPv6Carrier, identity SessionIdentity, logger *slog.Logger) func() error {
	ctx, cancel := context.WithCancel(context.Background())
	sender := &raSender{ctx: ctx, conn: carrierRAWriter{carrier}, ifname: identity.InterfaceName, logger: logger}
	//nolint:gosec // Timer jitter does not make a security decision.
	random := rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), uint64(identity.NativeIndex)))
	sched := newRASchedule(clock.RealClock{}, random)
	rsCh := make(chan struct{}, 1)
	rsDone := make(chan struct{})
	senderDone := make(chan struct{})
	go func() { defer close(rsDone); rsReaderLoop(ctx, carrier, rsCh, identity.InterfaceName, logger) }()
	go raSenderLoop(ctx, sender, sched, rsCh, senderDone)
	closer := &raCarrierClose{carrier: carrier}
	return func() error {
		cancel()
		<-rsDone
		stopRASender(cancel, senderDone, sender, sched, closer)
		return closer.err
	}
}

// rsReaderLoop consumes only validated solicitations from the carrier. The
// capacity-one notification lane coalesces bursts; receive failures retain the
// existing metric and bounded retry pacing.
func rsReaderLoop(ctx context.Context, reader rsReader, rsCh chan<- struct{}, ifname string, logger *slog.Logger) {
	var p pacer.Pacer
	// One worker lives until its session context ends.
	for {
		if ctx.Err() != nil {
			return
		}
		if err := reader.ReadRS(ctx); err != nil {
			if normalIPv6CarrierStop(ctx, err) {
				return
			}
			reportIPv6Failure(logger, ifname, "Router Solicitation read", err)
			countReaderError(loopRA)
			if p.Wait(ctx.Done()) {
				return
			}
			continue
		}
		p.Succeed()
		select {
		case rsCh <- struct{}{}:
		case <-ctx.Done():
			return
		}
	}
}
