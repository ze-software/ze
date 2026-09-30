package engine

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/transport"
)

// stampingTransport records the loop clock's reading at every Send, so a test
// measures the interval between transmitted Control packets.
type stampingTransport struct {
	clk  *steppedClock
	sent []time.Time
}

func (*stampingTransport) Start() error { return nil }
func (*stampingTransport) Stop() error  { return nil }
func (s *stampingTransport) Send(transport.Outbound) error {
	s.sent = append(s.sent, s.clk.now)
	return nil
}
func (*stampingTransport) RX() <-chan transport.Inbound { return nil }

// jitterTickIntervals runs the periodic transmit path of a lone Active session
// with the given bfd.DetectMult: it ticks once, then steps the clock exactly to
// each next-transmit deadline and ticks again, so every gap between two sends
// is the wait tick scheduled. It returns the gaps and the negotiated
// transmission interval they are measured against.
func jitterTickIntervals(t *testing.T, detectMult uint8, sends int) ([]time.Duration, time.Duration) {
	t.Helper()
	clk := &steppedClock{now: time.Unix(1_000_000, 0)}
	st := &stampingTransport{clk: clk}
	l := NewLoop(st, clk)
	req := reqFor(addrB, addrA)
	req.DetectMult = detectMult
	if _, err := l.EnsureSession(req); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	m := machineFor(t, l, req.Key())

	l.tick()
	for len(st.sent) < sends {
		l.mu.Lock()
		next := m.NextTxDeadline()
		l.mu.Unlock()
		if next.IsZero() {
			t.Fatal("precondition: an Active session must hold a next-transmit deadline")
		}
		clk.now = next
		l.tick()
	}
	l.mu.Lock()
	base := m.TransmitInterval()
	l.mu.Unlock()

	gaps := make([]time.Duration, 0, len(st.sent)-1)
	for i := 1; i < len(st.sent); i++ {
		gaps = append(gaps, st.sent[i].Sub(st.sent[i-1]))
	}
	return gaps, base
}

// RFC requirement: RFC5880-6.8.7-2 positive -- over 200 periodic Control
// packets sent by tick with bfd.DetectMult 3, every interval between two
// transmitted packets is the negotiated transmission interval reduced by 0 to
// 25% (in (75%, 100%] of it), the intervals take many distinct values, and at
// least one is reduced by more than 5%, so the reduction is drawn per packet
// on the path that transmits.
//
// VALIDATES: tick applies applyJitter to every periodic transmission.
// PREVENTS: tick scheduling the next packet a full interval ahead (no jitter),
// or with one reduction reused for every packet, while applyJitter alone still
// passes its own tests.
func TestRFC5880PeriodicTransmitJitteredPerPacket(t *testing.T) {
	gaps, base := jitterTickIntervals(t, 3, 201)
	floor := base - base/4
	distinct := map[time.Duration]bool{}
	reduced := false
	for i, gap := range gaps {
		if gap <= floor {
			t.Fatalf("interval %d = %v, at or below 75%% of %v", i, gap, base)
		}
		if gap > base {
			t.Fatalf("interval %d = %v, above the negotiated %v", i, gap, base)
		}
		distinct[gap] = true
		if gap < base-base/20 {
			reduced = true
		}
	}
	if len(distinct) < 50 {
		t.Fatalf("only %d distinct intervals in %d packets: the reduction is not drawn per packet", len(distinct), len(gaps))
	}
	if !reduced {
		t.Fatalf("no interval in %d packets was reduced by more than 5%% of %v", len(gaps), base)
	}
}

// RFC requirement: RFC5880-6.8.7-3 positive -- with bfd.DetectMult 1, over 200
// periodic Control packets sent by tick, every interval between two
// transmitted packets is no more than 90% and no less than 75% of the
// negotiated transmission interval.
//
// VALIDATES: tick passes the session's DetectMult to applyJitter, so the
// DetectMult 1 window holds on the transmitted packets.
// PREVENTS: tick passing a fixed multiplier (or no jitter), which lets an
// interval above 90% through for a DetectMult 1 session.
func TestRFC5880PeriodicTransmitDetectMultOneWindow(t *testing.T) {
	gaps, base := jitterTickIntervals(t, 1, 201)
	floor := base - base/4
	ceiling := base - base/10
	for i, gap := range gaps {
		if gap < floor {
			t.Fatalf("interval %d = %v, below 75%% of %v", i, gap, base)
		}
		if gap > ceiling {
			t.Fatalf("interval %d = %v, above 90%% of %v", i, gap, base)
		}
	}
}
