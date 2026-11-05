// Design: docs/guide/l2tp.md -- an unsent Configure-Ack cannot advance negotiation.

package ppp

import (
	"bytes"
	"net/netip"
	"testing"
)

func TestUnsentLCPAckCannotAdvanceNegotiation(t *testing.T) {
	for _, initial := range []LCPState{LCPStateReqSent, LCPStateAckRcvd} {
		for _, oversized := range []bool{true, false} {
			name := "fitting"
			if oversized {
				name = "oversized"
			}
			t.Run(initial.String()+"/"+name, func(t *testing.T) {
				s, rec, events := newRFC1661Session(initial)
				s.maxMRU = MinIPv4MRU
				s.negotiatedMRU = MinIPv4MRU
				// Optional no-wire-auth is a useful permitted control. A
				// false Opened transition must not even ask its handler.
				s.authRespCh <- authResponseMsg{accept: true}
				opts := optStream(mruOpt(MinIPv4MRU))
				copies := 1
				if oversized {
					copies = 11
				}
				for range copies {
					opts = append(opts, optStream(magicOpt(0x11223344))...)
				}
				terminated := s.handleLCPPacket(LCPPacket{Code: LCPConfigureRequest, Identifier: 0x31, Data: opts})
				if oversized {
					if state := s.currentState(); state == LCPStateAckSent || state == LCPStateOpened {
						t.Errorf("unsent Configure-Ack advanced LCP from %v to %v", initial, state)
					}
					if rec.count() != 0 {
						t.Error("oversized request emitted a Configure-Ack or a later-phase frame")
					}
					if len(s.authEventsOut) != 0 {
						t.Error("unsent Configure-Ack reached the authentication handler")
					}
					for len(events) > 0 {
						switch event := (<-events).(type) {
						case EventLCPUp, EventSessionUp, EventSessionIPAssigned:
							t.Errorf("unsent Configure-Ack published later-phase event %T", event)
						}
					}
					if backend, ok := s.backend.(*fakeBackend); !ok || len(backend.MTUCalls()) != 0 || len(backend.P2PCalls()) != 0 {
						t.Error("unsent Configure-Ack installed interface state")
					}
					return
				}
				if terminated {
					t.Fatal("fitting peer request ended its permitted session")
				}
				assertExactConfigureAck(t, rec, ProtoLCP, opts)
				want := LCPStateAckSent
				if initial == LCPStateAckRcvd {
					want = LCPStateOpened
					if len(s.authEventsOut) == 0 {
						t.Fatal("fitting acknowledgment did not reach permitted authentication")
					}
				}
				if state := s.currentState(); state != want {
					t.Fatalf("fitting request reached %v, want %v", state, want)
				}
			})
		}
	}
}

func TestUnsentIPCPAckCannotAssignAddress(t *testing.T) {
	for _, initial := range []LCPState{LCPStateReqSent, LCPStateAckRcvd} {
		for _, oversized := range []bool{true, false} {
			name := "fitting"
			if oversized {
				name = "oversized"
			}
			t.Run(initial.String()+"/"+name, func(t *testing.T) {
				s, rec, events := newRFC1661Session(LCPStateOpened)
				s.maxMRU = MinIPv4MRU
				s.negotiatedMRU = MinIPv4MRU
				s.disableIPCP = false
				s.ncpStarted = true
				s.ipcpState = initial
				s.localIPv4 = netip.MustParseAddr("192.0.2.1")
				s.peerIPv4 = netip.MustParseAddr("192.0.2.2")
				stopRelay := relayLifecycleEvents(s)
				defer stopRelay()
				copies := 1
				if oversized {
					copies = 11
				}
				opts := bytes.Repeat([]byte{IPCPOptIPAddress, 6, 192, 0, 2, 2}, copies)
				terminated := s.handleIPCPPacket(LCPPacket{Code: LCPConfigureRequest, Identifier: 0x31, Data: opts})
				backend, ok := s.backend.(*fakeBackend)
				if !ok {
					t.Fatalf("backend is %T", s.backend)
				}
				if oversized {
					if state := s.ipcpState; state == LCPStateAckSent || state == LCPStateOpened {
						t.Errorf("unsent Configure-Ack advanced IPCP from %v to %v", initial, state)
					}
					if rec.count() != 0 || len(backend.P2PCalls()) != 0 {
						t.Error("unsent IPCP Configure-Ack emitted a reply or installed an address")
					}
					for len(events) > 0 {
						switch event := (<-events).(type) {
						case EventLCPUp, EventSessionUp, EventSessionIPAssigned:
							t.Errorf("unsent IPCP Configure-Ack published later-phase event %T", event)
						}
					}
					if len(s.authEventsOut) != 0 {
						t.Error("unsent IPCP Configure-Ack reached authentication")
					}
					return
				}
				if terminated {
					t.Fatal("fitting peer request ended its permitted IPv4 session")
				}
				assertExactConfigureAck(t, rec, ProtoIPCP, opts)
				want, addresses := LCPStateAckSent, 0
				if initial == LCPStateAckRcvd {
					want, addresses = LCPStateOpened, 1
				}
				if s.ipcpState != want || len(backend.P2PCalls()) != addresses {
					t.Fatalf("fitting request reached state %v, address writes %v; want %v and %d", s.ipcpState, backend.P2PCalls(), want, addresses)
				}
			})
		}
	}
}

func assertExactConfigureAck(t *testing.T, rec *frameRecorder, proto uint16, opts []byte) {
	t.Helper()
	got := decodeFrames(t, rec)
	if len(got) != 1 || got[0].Proto != proto || got[0].Pkt.Code != LCPConfigureAck || got[0].Pkt.Identifier != 0x31 || !bytes.Equal(got[0].Pkt.Data, opts) {
		t.Fatalf("fitting request did not receive its complete matching Ack: %+v", got)
	}
}
