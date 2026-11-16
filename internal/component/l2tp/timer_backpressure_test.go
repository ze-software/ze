// Design: docs/research/l2tpv2-ze-integration.md -- original timer backpressure and reliable deadlines
package l2tp

import (
	"context"
	"log/slog"
	"net/netip"
	"testing"
	"testing/synctest"
	"time"
)

// Fake-clock advancement is explicit timer input, never a wall-clock sleep or
// guessed scheduling delay. synctest.Wait proves all worker/helper channels
// durably blocked before each state assertion. No network/native calls occur.
func fillOriginalTimerTicks(t *testing.T, timer *tunnelTimer, ticks <-chan tickReq, updates chan<- heapUpdate) {
	t.Helper()
	if err := timer.Start(); err != nil {
		t.Fatal(err)
	}
	updates <- heapUpdate{tunnelID: 1, deadline: time.Now()}
	synctest.Sleep(time.Nanosecond)
	if len(ticks) != 1 {
		t.Fatal("actual first due timer did not fill its original tick channel")
	}
	updates <- heapUpdate{tunnelID: 2, deadline: time.Now()}
	synctest.Sleep(time.Nanosecond)
	// First remains buffered. Actual second due timer is now pending delivery;
	// the old worker is durably blocked in its nested send at this point.
	if len(ticks) != 1 {
		t.Fatal("tick fixture did not retain the actual original first output")
	}
}

func TestTunnelTimerAcceptsClosingBurstWhileTickBlocked(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ticks := make(chan tickReq, 1)
		updates := make(chan heapUpdate, 16)
		timer := newTunnelTimer(ticks, updates)
		defer timer.Stop()
		fillOriginalTimerTicks(t, timer, ticks, updates)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		done := make(chan struct{})
		deadline := time.Now().Add(time.Hour)
		go func() {
			defer close(done)
			for index := range 24 {
				select {
				case updates <- heapUpdate{tunnelID: uint16(100 + index), deadline: deadline.Add(time.Duration(index) * time.Second)}:
				case <-ctx.Done():
					return
				}
			}
		}()
		defer func() { cancel(); timer.Stop(); <-done }()
		synctest.Wait()
		select {
		case <-done:
		default:
			t.Fatal("closing burst blocked after filling updateCh while original tick delivery was blocked")
		}
		timer.Stop() // joins before inspecting its private heap ownership
		if len(timer.byTunnel) != 24 {
			t.Fatalf("closing deadlines lost/coalesced: %d,want24", len(timer.byTunnel))
		}
		for index := range 24 {
			entry := timer.byTunnel[uint16(100+index)]
			if entry == nil || !entry.deadline.Equal(deadline.Add(time.Duration(index)*time.Second)) {
				t.Fatalf("original deadline %d was not retained exactly", index)
			}
		}
		if len(ticks) != 1 {
			t.Fatal("worker Stop fabricated a delivery into the still-full output")
		}
	})
}

func TestTunnelTimerStopJoinsWithOriginalTickBlocked(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ticks := make(chan tickReq, 1)
		updates := make(chan heapUpdate, 16)
		timer := newTunnelTimer(ticks, updates)
		defer timer.Stop()
		fillOriginalTimerTicks(t, timer, ticks, updates)
		stopped := make(chan struct{})
		go func() { timer.Stop(); close(stopped) }()
		synctest.Wait()
		select {
		case <-stopped:
		default:
			t.Fatal("Stop joined a blocked tick delivery instead of signaling its existing worker")
		}
		timer.Stop()
		if len(ticks) != 1 {
			t.Fatal("Stop lost/replaced the original buffered tick")
		}
	})
}

func TestTunnelTimerPendingClosedPollPreservesReusedTunnelDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Real existing reactor/reliable-engine constructors, no active socket or
		// native owner. A tick is a clock observation, not a session capability.
		logger := slog.New(slog.DiscardHandler)
		listener := newUDPListener(netip.MustParseAddrPort("127.0.0.1:1701"), logger)
		reactor := newL2TPReactor(listener, logger, reactorParams{})
		ticks := make(chan tickReq, 1)
		timer := newTunnelTimer(ticks, reactor.updateCh)
		defer timer.Stop()
		fillOriginalTimerTicks(t, timer, ticks, reactor.updateCh)
		now := time.Now()
		cfg := ReliableConfig{MaxRetransmit: 3, RTimeout: time.Second, RTimeoutCap: 4 * time.Second, RecvWindow: 4}
		original := newTunnel(2, 200, netip.MustParseAddrPort("127.0.0.1:1702"), cfg, reactor.logger, now.Add(-time.Hour))
		original.state = L2TPTunnelClosed
		original.engine.Close(now.Add(-time.Hour))
		reactor.tunnelsByLocalID[2] = original
		reactor.handleTick(tickReq{tunnelID: 2}) // actual expiry/reaper produces removal updates
		if reactor.tunnelsByLocalID[2] != nil {
			t.Fatal("actual expired closed root was not reaped")
		}
		reactor.nextLocalTID = 1
		tid, err := reactor.allocateLocalTID()
		if err != nil || tid != 2 {
			t.Fatalf("actual label reuse=%d,%v", tid, err)
		}
		successor := newTunnel(tid, 201, netip.MustParseAddrPort("127.0.0.1:1703"), cfg, reactor.logger, now)
		successor.state = L2TPTunnelEstablished
		if _, err := successor.engine.Enqueue(0, bodyOf(MsgHello), now, false); err != nil {
			t.Fatal(err)
		}
		deadline := successor.engine.NextDeadline()
		reactor.tunnelsByLocalID[tid] = successor
		reactor.updateCh <- heapUpdate{tunnelID: tid, deadline: deadline}
		synctest.Wait()
		first := <-ticks
		if first.tunnelID != 1 {
			t.Fatalf("first original tick=%d", first.tunnelID)
		}
		pending := <-ticks
		if pending.tunnelID != 2 {
			t.Fatalf("pending original closed poll=%d", pending.tunnelID)
		}
		// The old observation runs through the actual current reactor. Its real
		// engine clock refuses premature retransmit/expiry and republishes the
		// successor's OWN existing deadline, never a guessed owner/zero fallback.
		reactor.handleTick(pending)
		if reactor.tunnelsByLocalID[tid] != successor || successor.engine.closed || successor.engine.NextDeadline() != deadline {
			t.Fatal("pending old closed poll mutated the reused reliable lifetime")
		}
		actual := <-ticks // durable wait advances only to the actual future deadline
		if actual.tunnelID != tid || !time.Now().Equal(deadline) {
			t.Fatalf("successor deadline lost or shifted: tick%d at%s,want%s", actual.tunnelID, time.Now(), deadline)
		}
		if reactor.tunnelsByLocalID[tid] != successor {
			t.Fatal("timer delivery replaced original current engine root")
		}
	})
}
