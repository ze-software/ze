// VALIDATES: a session that enters AdminDown, by Shutdown or by the last
// client's ReleaseSession, keeps sending Control packets that carry State
// AdminDown and Diagnostic 7 at its transmit cadence for at least a Detection
// Time, so the remote system learns of the change.
// PREVENTS: tick skipping every AdminDown session, which left the peer to time
// the session out as a path failure (RFC 5880 Section 6.8.16).
package engine

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/packet"
	"github.com/ze-software/ze/internal/component/bfd/transport"
)

// adminDownSend is one Control packet the loop sent, parsed, with the clock
// reading at the Send.
type adminDownSend struct {
	at      time.Time
	control packet.Control
}

// parsingTransport parses every Control packet the loop sends. A packet that
// does not parse fails the test at once.
type parsingTransport struct {
	t    *testing.T
	clk  *steppedClock
	sent []adminDownSend
}

func (*parsingTransport) Start() error { return nil }
func (*parsingTransport) Stop() error  { return nil }
func (p *parsingTransport) Send(o transport.Outbound) error {
	c, _, err := packet.ParseControl(o.Bytes)
	if err != nil {
		p.t.Fatalf("ParseControl of a sent packet: %v", err)
	}
	p.sent = append(p.sent, adminDownSend{at: p.clk.now, control: c})
	return nil
}
func (*parsingTransport) RX() <-chan transport.Inbound { return nil }

// peerRequiredMinRx is the Required Min RX Interval inboundControlState
// declares for the peer, in microseconds.
const peerRequiredMinRx = 300_000

// adminDownWindowDetectionTimes is the number of Detection Times Ze sends
// AdminDown Control packets for (owner decision 2026-09-30 under the MAY of
// RFC 5880 Section 6.8.16). The tests pin it by value, so a change to the
// producer's factor reddens them.
const adminDownWindowDetectionTimes = 3

// RFC requirement: RFC5880-6.8.16-2 positive -- after an Up session moves to
// AdminDown, through handle.Shutdown and through the last client's
// ReleaseSession, tick keeps transmitting: the first packet leaves at the
// transition, every packet carries State AdminDown and Diagnostic 7, no gap
// between two packets exceeds the transmission interval the packets announce,
// packets still leave after one and after two Detection Times, and the last
// one leaves no earlier than one interval before three Detection Times, where
// the Detection Time is the longer of the local one and the one the remote
// system derives from those packets (their Detect Mult times the larger of
// their Desired Min TX Interval and the peer's Required Min RX Interval).
func TestRFC5880AdminDownControlTransmittedForADetectionTime(t *testing.T) {
	cases := []struct {
		name  string
		enter func(*testing.T, *Loop, api.SessionHandle)
	}{
		{"shutdown", func(t *testing.T, _ *Loop, h api.SessionHandle) {
			if err := h.Shutdown(); err != nil {
				t.Fatalf("Shutdown: %v", err)
			}
		}},
		{"release", func(t *testing.T, l *Loop, h api.SessionHandle) {
			if err := l.ReleaseSession(h); err != nil {
				t.Fatalf("ReleaseSession: %v", err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clk := &steppedClock{now: time.Unix(1_000_000, 0)}
			pt := &parsingTransport{t: t, clk: clk}
			l := NewLoop(pt, clk)
			req := reqFor(addrB, addrA)
			h, err := l.EnsureSession(req)
			if err != nil {
				t.Fatalf("EnsureSession: %v", err)
			}
			key := req.Key()
			m := machineFor(t, l, key)
			discr := m.LocalDiscriminator()
			l.handleInbound(inboundControlState(key.Peer, key.Local, key.Interface, 0, packet.StateDown))
			l.handleInbound(inboundControlState(key.Peer, key.Local, key.Interface, discr, packet.StateInit))
			if got := m.State(); got != packet.StateUp {
				t.Fatalf("precondition: state = %s, want Up", got)
			}
			localDetect := m.DetectionInterval()

			start := clk.now
			tc.enter(t, l, h)
			pt.sent = nil
			for clk.now.Sub(start) <= 30*time.Second {
				l.tick()
				clk.now = clk.now.Add(10 * time.Millisecond)
			}

			if len(pt.sent) == 0 {
				t.Fatal("no Control packet was sent after the transition to AdminDown")
			}
			if !pt.sent[0].at.Equal(start) {
				t.Fatalf("first AdminDown packet sent at +%v, want at the transition", pt.sent[0].at.Sub(start))
			}
			var need, interval time.Duration
			for i, s := range pt.sent {
				if s.control.State != packet.StateAdminDown {
					t.Fatalf("packet %d carries State %s, want AdminDown", i, s.control.State)
				}
				if s.control.Diag != packet.DiagAdminDown {
					t.Fatalf("packet %d carries Diag %d, want %d", i, s.control.Diag, packet.DiagAdminDown)
				}
				interval = time.Duration(max(s.control.DesiredMinTxInterval, peerRequiredMinRx)) * time.Microsecond
				remoteDetect := time.Duration(s.control.DetectMult) * interval
				need = max(localDetect, remoteDetect)
				if i > 0 && s.at.Sub(pt.sent[i-1].at) > interval {
					t.Fatalf("gap %v between AdminDown packets %d and %d exceeds the interval %v", s.at.Sub(pt.sent[i-1].at), i-1, i, interval)
				}
			}
			last := pt.sent[len(pt.sent)-1].at.Sub(start)
			for times := 1; times < adminDownWindowDetectionTimes; times++ {
				mark := time.Duration(times) * need
				if last <= mark {
					t.Fatalf("last AdminDown packet at +%v; none sent after %d Detection Times (%v)", last, times, mark)
				}
			}
			window := adminDownWindowDetectionTimes * need
			if last < window-interval {
				t.Fatalf("last AdminDown packet at +%v; transmission must run for %d Detection Times, %v (interval %v)", last, adminDownWindowDetectionTimes, window, interval)
			}
		})
	}
}
