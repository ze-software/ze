// Design: docs/guide/l2tp.md -- LCP interruption of authentication/NCP

package ppp

import (
	"net/netip"
	"testing"
	"testing/synctest"
	"time"
)

// A real Configure-Ack enters PAP and then NCP synchronously. An intervening
// LCP termination or refused renegotiation must invalidate that authenticated
// phase before a queued PAP repeat is handled, and must not publish SessionUp.
func TestPAPDecisionEndsWhenLCPInterruptsNCP(t *testing.T) {
	for _, interruption := range []LCPPacket{
		{Code: LCPTerminateRequest, Identifier: 0x41},
		{Code: LCPConfigureRequest, Identifier: 0x42, Data: []byte{0xfe, 2}},
	} {
		t.Run(LCPCodeName(interruption.Code), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, recorder, events := newRFC1661Session(LCPStateAckSent)
				authRequests := make(chan AuthEvent, 16)
				ipRequests := make(chan IPEvent, 16)
				s.authEventsOut = authRequests
				s.ipEventsOut = ipRequests
				stopRelay := relayLifecycleEvents(s)
				defer stopRelay()
				s.configuredAuthMethod = AuthMethodPAP
				s.disableIPCP = false
				s.ipTimeout = time.Second
				frames := make(chan []byte, 3)
				pap := getFrameBuf()
				frames <- pap[:copy(pap, papAuthRequestFrame())]
				s.framesIn = frames
				if !s.sendConfigureRequest() {
					t.Fatal("initial LCP request failed")
				}
				request := lastLCPConfigureRequest(t, recorder)
				stopReplies, repliesDone := make(chan struct{}), make(chan struct{})
				repliesStopped := false
				stopReplyWorker := func() {
					if !repliesStopped {
						close(stopReplies)
						repliesStopped = true
					}
					<-repliesDone
				}
				defer stopReplyWorker()
				go func() {
					defer close(repliesDone)
					select {
					case raw := <-authRequests:
						auth, ok := raw.(EventAuthRequest)
						if !ok {
							t.Errorf("authentication event %T, want actual PAP request", raw)
							return
						}
						if err := auth.answer(authResponseMsg{accept: true}); err != nil {
							t.Errorf("answer actual PAP request: %v", err)
							return
						}
					case <-stopReplies:
						return
					}
					select {
					case raw := <-ipRequests:
						ip, ok := raw.(EventIPRequest)
						if !ok {
							t.Errorf("address event %T, want actual IPv4 request", raw)
							return
						}
						if err := ip.answer(ipResponseMsg{
							accept: true, family: AddressFamilyIPv4,
							local: netip.MustParseAddr("192.0.2.1"), peer: netip.MustParseAddr("192.0.2.2"),
						}); err != nil {
							t.Errorf("answer actual IPv4 request: %v", err)
							return
						}
					case <-stopReplies:
						return
					}
					// Reaching the actual IP wait proves PAP acceptance was
					// consumed. Only now may LCP interrupt the nested NCP.
					control := getFrameBuf()
					n := WriteFrame(control, 0, ProtoLCP, nil)
					n += WriteLCPPacket(control, n, interruption.Code, interruption.Identifier, interruption.Data)
					repeat := getFrameBuf()
					repeat = repeat[:copy(repeat, papAuthRequestFrame())]
					frames <- control[:n]
					frames <- repeat
				}()
				terminated := s.handleLCPPacket(LCPPacket{Code: LCPConfigureAck, Identifier: request.Identifier, Data: request.Data})
				stopReplyWorker()
				// The outer PPP loop consumes anything left after NCP unwinds.
				for len(frames) != 0 {
					frame := <-frames
					s.handleFrame(frame)
					putFrameBuf(frame)
				}
				synctest.Wait()
				papReplies := 0
				for _, frame := range decodeFrames(t, recorder) {
					if frame.Proto == ProtoPAP {
						papReplies++
						if frame.Pkt.Code != PAPAuthenticateAck {
							t.Fatalf("initial PAP authentication failed: %+v", frame.Pkt)
						}
					}
				}
				if papReplies != 1 {
					t.Fatalf("PAP replied after its LCP lifetime ended: %d replies", papReplies)
				}
				if terminated {
					t.Fatal("nested NCP interruption bypassed the resumed LCP FSM")
				}
				for len(events) != 0 {
					if _, ok := (<-events).(EventSessionUp); ok {
						t.Fatal("interrupted NCP brought the session up")
					}
				}
			})
		})
	}
}
