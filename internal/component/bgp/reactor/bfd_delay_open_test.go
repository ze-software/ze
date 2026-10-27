// Design: docs/architecture/behavior/fsm.md — strict-mode DelayOpen wire actions.
package reactor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/test/sim"
)

// strictDelaySocket starts Idle through the real Start/Accept entries, with a
// TCP recorder and controlled timer clock. No FSM state is installed by hand.
func strictDelaySocket(t *testing.T, passive bool, hold uint16, bfd api.State, strict bool) (*Session, net.Conn, *bfdInitialTransport, *sim.FakeClock, *fsm.ConnectRetryCounter) {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65001, 65002, 0x01020301)
	if passive {
		settings.Connection = ConnectionPassive
	}
	settings.DelayOpen = time.Minute
	settings.ReceiveHoldTime = time.Duration(hold) * time.Second
	settings.DisableASN4 = true
	settings.BFD = &BFDSettings{Enabled: true, Strict: strict, HoldTime: 5}
	if strict {
		settings.Capabilities = []capability.Capability{&capability.BFDStrictMode{}}
	}
	s := NewSession(settings)
	clock := sim.NewFakeClock(time.Now())
	s.SetClock(clock)
	s.setBFDStateReader(func() (api.State, time.Time, bool) { return bfd, clock.Now(), true })
	counter := &fsm.ConnectRetryCounter{}
	s.SetConnectRetryCounter(counter)
	t.Cleanup(func() { s.timers.StopAll(); s.stopSendHoldTimer(); s.closeConn() })
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	far, near := collisionProofTCPPair(t)
	transport := &bfdInitialTransport{Conn: near}
	if err := s.Accept(transport); err != nil {
		t.Fatal(err)
	}
	if len(transport.wire) != 0 || !s.timers.IsDelayOpenTimerRunning() {
		t.Fatal("DelayOpen did not hold the initial OPEN")
	}
	return s, far, transport, clock, counter
}

func strictDelayReceiveOpen(t *testing.T, s *Session, far net.Conn, hold uint16, collision bool) {
	t.Helper()
	body := openBodyWithBFDStrict(hold)
	if collision {
		open, err := message.UnpackOpen(body)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.processOpen(open); err != nil {
			t.Fatal(err)
		}
		return
	}
	wire := append(bytes.Repeat([]byte{0xff}, 16), 0, byte(19+len(body)), 1)
	bfdInitialReceive(t, s, far, append(wire, body...))
}

// TestSessionBFDStrictDelayOpenRelease proves exact wire ordering and the two
// real OPEN dispatch rails, all release events, and zero/nonzero HoldTime.
func TestSessionBFDStrictDelayOpenRelease(t *testing.T) {
	for _, passive := range []bool{false, true} {
		for _, hold := range []uint16{0, 9} {
			for _, collision := range []bool{false, true} {
				for _, event := range []fsm.Event{fsm.EventBfdUp, fsm.EventBfdAdminDown, fsm.EventBfdDisabled} {
					t.Run(fmt.Sprintf("passive=%t/hold=%d/collision=%t/%s", passive, hold, collision, event), func(t *testing.T) {
						s, far, transport, _, _ := strictDelaySocket(t, passive, hold, api.StateDown, true)
						parent := s.State()
						strictDelayReceiveOpen(t, s, far, hold, collision)
						open := delayOpenReadType(t, far, 1)
						if !bytes.Equal(transport.wire, open) || s.State() != parent || s.fsm.BfdSubState() == fsm.SubStateNone || !s.Negotiated().BFDStrictMode || s.timers.IsKeepaliveTimerRunning() {
							t.Fatal("strict Event 20 sent KEEPALIVE or failed to retain the parent state")
						}
						if err := s.handleBFDEvent(event); err != nil {
							t.Fatal(err)
						}
						keepalive := delayOpenReadType(t, far, 4)
						want := append(bytes.Clone(open), keepalive...)
						if !bytes.Equal(transport.wire, want) || s.State() != fsm.StateOpenConfirm || s.fsm.BfdSubState() != fsm.SubStateNone || s.timers.IsBfdHoldTimerRunning() || s.timers.IsKeepaliveTimerRunning() != (hold != 0) {
							t.Fatal("release did not send exactly one KEEPALIVE and leave pending")
						}
						if err := s.handleBFDEvent(event); err != nil || !bytes.Equal(transport.wire, want) {
							t.Fatalf("duplicate BFD release sent extra bytes: %v", err)
						}
						bfdInitialReceive(t, s, far, keepalive)
						if s.State() != fsm.StateEstablished {
							t.Fatal("peer KEEPALIVE did not establish released session")
						}
					})
				}
			}
		}
	}
}

func TestSessionBFDStrictDelayOpenNonwaitingControls(t *testing.T) {
	for _, passive := range []bool{false, true} {
		for _, tc := range []struct {
			state  api.State
			strict bool
		}{{api.StateUp, true}, {api.StateAdminDown, true}, {api.StateDown, false}} {
			s, far, transport, _, _ := strictDelaySocket(t, passive, 9, tc.state, tc.strict)
			strictDelayReceiveOpen(t, s, far, 9, false)
			open := delayOpenReadType(t, far, 1)
			keepalive := delayOpenReadType(t, far, 4)
			if !bytes.Equal(transport.wire, append(open, keepalive...)) || s.State() != fsm.StateOpenConfirm || s.fsm.BfdSubState() != fsm.SubStateNone {
				t.Fatal("nonwaiting control entered a strict substate")
			}
		}
	}
}

// TestSessionBFDStrictDelayOpenBoundedWait drives the actual timer callbacks,
// not just FSM events. Second OPEN and config-change use their real handlers.
func TestSessionBFDStrictDelayOpenBoundedWait(t *testing.T) {
	for _, passive := range []bool{false, true} {
		for _, kind := range []string{"bfd-hold", "bgp-hold", "second-open", "config"} {
			t.Run(fmt.Sprintf("passive=%t/%s", passive, kind), func(t *testing.T) {
				hold := uint16(0)
				if kind == "bgp-hold" {
					hold = 3
				}
				if kind == "second-open" {
					hold = 9
				}
				s, far, transport, clock, counter := strictDelaySocket(t, passive, hold, api.StateDown, true)
				openCalls, negotiationCalls := 0, 0
				s.onOpenRecv = func() { openCalls++ }
				s.onNegotiated = func(_, _ uint32) { negotiationCalls++ }
				strictDelayReceiveOpen(t, s, far, hold, false)
				open := delayOpenReadType(t, far, 1)
				if openCalls != 1 || negotiationCalls != 1 {
					t.Fatalf("first OPEN callbacks: received=%d negotiated=%d", openCalls, negotiationCalls)
				}
				originalNegotiated := s.Negotiated()
				s.mu.RLock()
				originalOpen := s.peerOpen
				s.mu.RUnlock()
				// Nonzero retry history distinguishes config reset from no-op.
				counter.Increment()
				counter.Increment()
				ctx, cancel := context.WithCancel(t.Context())
				done := make(chan struct{})
				go func() {
					defer close(done)
					_ = s.Run(ctx) // The expected teardown error is checked through state and wire.
				}()
				t.Cleanup(func() {
					cancel()
					select {
					case <-done:
					case <-time.After(5 * time.Second):
						t.Error("strict DelayOpen Run did not stop")
					}
				})
				switch kind {
				case "bfd-hold", "bgp-hold":
					bound := 5 * time.Second
					if kind == "bgp-hold" {
						bound = 3 * time.Second
					}
					clock.Add(bound - time.Nanosecond)
					if s.fsm.BfdSubState() == fsm.SubStateNone || s.Conn() == nil {
						t.Fatal("strict wait expired before its negotiated/configured bound")
					}
					clock.Add(time.Nanosecond)
				case "second-open":
					// A different valid proposal would change the negotiated hold,
					// router ID and strict capability if refusal happened too late.
					body := openBodyWithBFDStrict(0)[:10]
					body[8], body[9] = 3, 0 // Identifier 1.2.3.3, no optional parameters.
					if _, err := message.UnpackOpen(body); err != nil {
						t.Fatalf("changed second OPEN is not valid: %v", err)
					}
					wire := append(bytes.Repeat([]byte{0xff}, 16), 0, byte(19+len(body)), 1)
					if _, err := far.Write(append(wire, body...)); err != nil {
						t.Fatal(err)
					}
				case "config":
					if err := s.handleBFDEvent(fsm.EventBfdStrictConfigChanged); err != nil {
						t.Fatal(err)
					}
				}
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Fatal("bounded strict wait did not terminate Run")
				}
				if kind == "second-open" {
					s.mu.RLock()
					peerOpen := s.peerOpen
					s.mu.RUnlock()
					if openCalls != 1 || negotiationCalls != 1 {
						t.Errorf("second OPEN reached validation/negotiation: received=%d negotiated=%d", openCalls, negotiationCalls)
					}
					if peerOpen != originalOpen || peerOpen.BGPIdentifier != 0x01020302 ||
						peerOpen.HoldTime != 9 || s.Negotiated() != originalNegotiated ||
						!originalNegotiated.BFDStrictMode || s.timers.HoldTime() != 9*time.Second {
						t.Error("second OPEN changed the accepted OPEN or negotiated state before refusal")
					}
				}
				wantCounter := uint32(3)
				if kind == "config" {
					wantCounter = 0
				}
				if s.State() != fsm.StateIdle || s.fsm.BfdSubState() != fsm.SubStateNone || counter.Load() != wantCounter || s.Conn() != nil || s.timers.IsBfdHoldTimerRunning() || s.timers.IsHoldTimerRunning() || s.timers.IsKeepaliveTimerRunning() || s.timers.IsDelayOpenTimerRunning() || s.timers.IsConnectRetryTimerRunning() {
					t.Fatalf("wait left resources: state=%s sub=%s counter=%d conn=%v", s.State(), s.fsm.BfdSubState(), counter.Load(), s.Conn())
				}
				rest, err := io.ReadAll(far)
				if err != nil {
					t.Fatal(err)
				}
				if kind == "config" {
					if len(rest) != 0 {
						t.Fatalf("silent config close sent %x", rest)
					}
				} else {
					code, subcode := byte(6), byte(10)
					if kind == "bgp-hold" {
						code, subcode = 4, 0
					}
					if kind == "second-open" {
						code, subcode = 5, 0
					}
					if len(rest) != 21 || rest[18] != 3 || rest[19] != code || rest[20] != subcode {
						t.Fatalf("notification stream=%x, want %d/%d", rest, code, subcode)
					}
				}
				if !bytes.Equal(transport.wire, append(open, rest...)) {
					t.Fatal("TCP recorder differs from received stream")
				}
			})
		}
	}
}

// TestPeerBFDStrictDelayOpenRelease exercises outbound Connect and inbound
// Active through the Peer reactor, including the production BFD event router.
func TestPeerBFDStrictDelayOpenRelease(t *testing.T) {
	for _, passive := range []bool{false, true} {
		t.Run(fmt.Sprintf("passive=%t", passive), func(t *testing.T) {
			peer, session, far, _ := startDelayOpenPeer(t, passive, time.Minute, func(peer *Peer) {
				peer.settings.BFD = &BFDSettings{Enabled: true, Strict: true, HoldTime: 5}
				peer.settings.Capabilities = []capability.Capability{&capability.BFDStrictMode{}}
			})
			session.setBFDStateReader(func() (api.State, time.Time, bool) {
				return api.StateDown, time.Now(), true
			})
			parent := session.State()
			body := openBodyWithBFDStrict(9)
			body[1], body[2] = 0xfd, 0xe9 // Peer fixture expects AS 65001.
			frame := append(bytes.Repeat([]byte{0xff}, 16), 0, byte(19+len(body)), 1)
			if _, err := far.Write(append(frame, body...)); err != nil {
				t.Fatal(err)
			}
			delayOpenReadType(t, far, 1)
			delayOpenAwait(t, func() bool { return session.fsm.BfdSubState() != fsm.SubStateNone })
			if session.State() != parent {
				t.Fatal("Event 20 left its parent before BFD")
			}
			delayOpenNoBytes(t, far)
			released := make(chan struct{})
			go func() {
				defer close(released)
				peer.deliverBFDEvent(fsm.EventBfdUp)
			}()
			delayOpenReadType(t, far, 4)
			select {
			case <-released:
			case <-time.After(5 * time.Second):
				t.Fatal("BFD release blocked")
			}
			if session.State() != fsm.StateOpenConfirm || session.fsm.BfdSubState() != fsm.SubStateNone {
				t.Fatal("Peer BFD delivery failed to release pending Event 20")
			}
			delayOpenWrite(t, far, &message.Keepalive{})
			delayOpenAwait(t, func() bool { return session.State() == fsm.StateEstablished })
		})
	}
}
