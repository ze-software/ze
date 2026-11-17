// Design: docs/research/l2tpv2-ze-integration.md -- configured LCP session timing.
// RFC: rfc/short/rfc1661.md -- Sections 4.4 and 4.6.

package ppp

import (
	"bytes"
	"net/netip"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

// Historical fixtures use the RFC's recommended values explicitly. Production
// obtains its defaults from the shared YANG grouping instead.
const (
	defaultRestartTimer = 3 * time.Second
	defaultMaxTerminate = 2
	defaultMaxConfigure = 10
)

func configuredLCPSession(t *testing.T, state LCPState, interval time.Duration, terminate, configure, failure int) (*pppSession, *frameRecorder, chan Event) {
	t.Helper()
	s, rec, events := newRFC1661Session(state)
	var err error
	s.lcpConfig, err = NewLCPConfig(interval, terminate, configure, failure)
	if err != nil {
		t.Fatal(err)
	}
	replyNextAuthRequest(t, s, authResponseMsg{accept: true})
	return s, rec, events
}

// TestConfiguredLCPRestartTimer measures the timer armed by a real SCR, for
// both the schema's recommended interval and a configured shorter interval.
// MUTATION: armRestartTimer resetting a fixed three seconds fails the 500 ms case.
// RFC requirement: RFC1661-4.6-1 positive -- SCR arms the configured 500 ms interval and the default three-second interval exactly.
// RFC requirement: RFC1661-4.6-1 negative -- neither interval expires a nanosecond early, and entering Opened stops the timer.
func TestConfiguredLCPRestartTimer(t *testing.T) {
	for _, interval := range []time.Duration{500 * time.Millisecond, 3 * time.Second} {
		t.Run(interval.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, rec, _ := configuredLCPSession(t, LCPStateClosed, interval, 2, 3, 5)
				s.restartTimer = time.NewTimer(time.Hour)
				s.restartTimer.Stop()
				defer s.restartTimer.Stop()
				if s.applyTransition(LCPStateClosed, LCPDoTransition(LCPStateClosed, LCPEventOpen), LCPPacket{}) {
					t.Fatal("Open ended the session")
				}
				time.Sleep(interval - time.Nanosecond)
				select {
				case <-s.restartTimer.C:
					t.Fatal("Restart timer expired early")
				default:
				}
				time.Sleep(time.Nanosecond)
				select {
				case <-s.restartTimer.C:
					if s.handleRestartTimeout() {
						t.Fatal("first retransmission ended the session")
					}
				default:
					t.Fatal("Restart timer did not expire at the configured interval")
				}
				if got := confRequestsIn(t, rec); got != 2 {
					t.Fatalf("Configure-Requests = %d, want 2", got)
				}
				request := lastLCPConfigureRequest(t, rec)
				s.handleLCPPacket(LCPPacket{Code: LCPConfigureRequest, Identifier: 8})
				s.handleLCPPacket(LCPPacket{Code: LCPConfigureAck, Identifier: request.Identifier, Data: request.Data})
				if s.currentState() != LCPStateOpened {
					t.Fatal("answering peer did not reach Opened")
				}
				if s.restartTimer.Stop() {
					t.Fatal("Restart timer still armed in Opened")
				}
			})
		})
	}
}

// TestConfiguredLCPMaxConfigure spends every configured transmission, then
// contrasts an answering peer that opens before the budget is exhausted.
// MUTATION: applyTransition loading a fixed ten fails both configured budgets.
// RFC requirement: RFC1661-4.6-3 positive -- budgets of three and five transmit exactly that many Configure-Requests before TO- reaches Stopped.
// RFC requirement: RFC1661-4.6-3 negative -- a peer answering the second request opens after exactly two transmissions and causes no third request.
func TestConfiguredLCPMaxConfigure(t *testing.T) {
	for _, maximum := range []int{3, 5} {
		s, rec, _ := configuredLCPSession(t, LCPStateClosed, time.Second, 2, maximum, 5)
		s.applyTransition(LCPStateClosed, LCPDoTransition(LCPStateClosed, LCPEventOpen), LCPPacket{})
		for sent := 1; sent < maximum; sent++ {
			if s.handleRestartTimeout() {
				t.Fatalf("budget %d ended after %d requests", maximum, sent)
			}
		}
		if s.restartCount != 0 {
			t.Fatalf("remaining = %d, want 0 after %d transmissions", s.restartCount, maximum)
		}
		if !s.handleRestartTimeout() || s.currentState() != LCPStateStopped {
			t.Fatalf("budget %d did not end in Stopped", maximum)
		}
		if got := confRequestsIn(t, rec); got != maximum {
			t.Fatalf("requests = %d, want %d", got, maximum)
		}
	}
	s, rec, _ := configuredLCPSession(t, LCPStateClosed, time.Second, 2, 3, 5)
	s.applyTransition(LCPStateClosed, LCPDoTransition(LCPStateClosed, LCPEventOpen), LCPPacket{})
	s.handleRestartTimeout()
	request := lastLCPConfigureRequest(t, rec)
	s.handleLCPPacket(LCPPacket{Code: LCPConfigureRequest, Identifier: 8})
	s.handleLCPPacket(LCPPacket{Code: LCPConfigureAck, Identifier: request.Identifier, Data: request.Data})
	if s.currentState() != LCPStateOpened || confRequestsIn(t, rec) != 2 {
		t.Fatal("answering peer did not open after exactly two Configure-Requests")
	}
}

// TestLCPReplyRenewsConfiguredCounter spends two of three transmissions before
// a valid Nak, so a missing IRC cannot hide behind an untouched initial budget.
// Ack-Rcvd is the table's contrast: its RCN edge sends without resetting.
func TestLCPReplyRenewsConfiguredCounter(t *testing.T) {
	for _, state := range []LCPState{LCPStateReqSent, LCPStateAckSent, LCPStateAckRcvd} {
		s, rec, _ := configuredLCPSession(t, LCPStateClosed, time.Second, 2, 3, 5)
		s.applyTransition(LCPStateClosed, LCPDoTransition(LCPStateClosed, LCPEventOpen), LCPPacket{})
		s.handleRestartTimeout()
		request := lastLCPConfigureRequest(t, rec)
		s.state = state
		if s.handleLCPPacket(LCPPacket{Code: LCPConfigureNak, Identifier: request.Identifier, Data: optStream(mruOption(1400))}) {
			t.Fatalf("%s: valid Nak ended negotiation", state)
		}
		want := 2
		if state == LCPStateAckRcvd {
			want = 0
		}
		if s.restartCount != want || confRequestsIn(t, rec) != 3 {
			t.Fatalf("%s: remaining=%d, transmitted=%d; want %d, 3", state, s.restartCount, confRequestsIn(t, rec), want)
		}
		if state == LCPStateAckRcvd {
			if !s.handleRestartTimeout() {
				t.Fatal("Ack-Rcvd's spent counter was unexpectedly renewed")
			}
			continue
		}
		for range 2 {
			if s.handleRestartTimeout() {
				t.Fatalf("%s: renewed budget exhausted early", state)
			}
		}
		if !s.handleRestartTimeout() || confRequestsIn(t, rec) != 5 {
			t.Fatalf("%s: renewed budget did not finish after exactly five total requests", state)
		}
	}
}

// TestConfiguredLCPMaxTerminate contrasts silent and answering peers with
// budgets of two and four, using Close's actual irc,str transition.
// MUTATION: loading Max-Configure for str fails the exact termination counts.
// RFC requirement: RFC1661-4.6-2 positive -- Close sends exactly the configured two or four Terminate-Requests before TO- reaches Closed.
// RFC requirement: RFC1661-4.6-2 negative -- a matching Terminate-Ack after the first request reaches Closed without a retransmission.
func TestConfiguredLCPMaxTerminate(t *testing.T) {
	for _, maximum := range []int{2, 4} {
		s, rec, _ := configuredLCPSession(t, LCPStateOpened, time.Second, maximum, 7, 5)
		if s.applyTransition(LCPStateOpened, LCPDoTransition(LCPStateOpened, LCPEventClose), LCPPacket{}) {
			t.Fatal("Close ended before its request could be answered")
		}
		for sent := 1; sent < maximum; sent++ {
			if s.handleRestartTimeout() {
				t.Fatalf("termination ended after %d requests, want %d", sent, maximum)
			}
		}
		if !s.handleRestartTimeout() || s.currentState() != LCPStateClosed {
			t.Fatal("exhausted termination did not reach Closed")
		}
		if got := len(decodeFrames(t, rec)); got != maximum {
			t.Fatalf("termination frames = %d, want %d", got, maximum)
		}
		for i, frame := range decodeFrames(t, rec) {
			if frame.Proto != ProtoLCP || frame.Pkt.Code != LCPTerminateRequest {
				t.Fatalf("termination frame %d protocol %#x code %d, want LCP Terminate-Request", i, frame.Proto, frame.Pkt.Code)
			}
		}
	}
	s, rec, _ := configuredLCPSession(t, LCPStateOpened, time.Second, 4, 7, 5)
	s.applyTransition(LCPStateOpened, LCPDoTransition(LCPStateOpened, LCPEventClose), LCPPacket{})
	s.handleLCPPacket(LCPPacket{Code: LCPTerminateAck, Identifier: s.terminateID})
	if s.currentState() != LCPStateClosed || len(decodeFrames(t, rec)) != 1 {
		t.Fatal("matching Terminate-Ack did not stop after exactly one Terminate-Request")
	}
	frame := decodeFrames(t, rec)[0]
	if frame.Proto != ProtoLCP || frame.Pkt.Code != LCPTerminateRequest {
		t.Fatalf("answered termination protocol %#x code %d, want LCP Terminate-Request", frame.Proto, frame.Pkt.Code)
	}
}

// TestConfiguredLCPMaxFailure repeats an unacceptable MRU beside an acceptable
// ACCM, inspecting the original peer bytes in the converted Reject. A later Ack
// must restore the Nak budget rather than poison the next exchange.
// MUTATION: omit Max-Failure conversion; the sixth reply stays a Nak.
// RFC requirement: RFC1661-4.6-4 positive -- the configured number of Naks, including default five, is followed by a Reject containing only the original unacceptable option.
// RFC requirement: RFC1661-4.6-4 negative -- replies through the configured threshold remain Naks, and a successfully sent Configure-Ack renews the Nak budget.
func TestConfiguredLCPMaxFailure(t *testing.T) {
	for _, maximum := range []int{2, 5} {
		s, rec, _ := configuredLCPSession(t, LCPStateReqSent, time.Second, 2, 10, maximum)
		bad := mruOption(2000)
		acceptable := LCPOption{Type: LCPOptACCM, Data: []byte{0, 0, 0, 0}}
		for sent := 0; sent <= maximum; sent++ {
			req := LCPPacket{Code: LCPConfigureRequest, Identifier: uint8(sent + 1), Data: optStream(acceptable, bad)}
			if s.handleLCPPacket(req) {
				t.Fatal("unacceptable peer option ended the session")
			}
			frames := decodeFrames(t, rec)
			if len(frames) != sent+1 {
				t.Fatalf("replies = %d, want %d", len(frames), sent+1)
			}
			got := frames[sent].Pkt
			want := LCPConfigureNak
			if sent == maximum {
				want = LCPConfigureReject
			}
			if got.Code != want || got.Identifier != req.Identifier {
				t.Fatalf("reply %d code/id = %d/%d, want %d/%d", sent+1, got.Code, got.Identifier, want, req.Identifier)
			}
			if want == LCPConfigureReject && !bytes.Equal(got.Data, optStream(bad)) {
				t.Fatalf("Reject = %x, want original MRU alone %x", got.Data, optStream(bad))
			}
		}
		if !s.sendConfigureAck(LCPPacket{Code: LCPConfigureRequest, Identifier: 90}) {
			t.Fatal("could not send the resetting Configure-Ack")
		}
		if !s.sendConfigureNakOrReject(LCPPacket{Code: LCPConfigureRequest, Identifier: 91, Data: optStream(bad)}) {
			t.Fatal("could not send the fresh Nak")
		}
		frames := decodeFrames(t, rec)
		if got := frames[len(frames)-1].Pkt.Code; got != LCPConfigureNak {
			t.Fatalf("reply after Configure-Ack = %d, want Nak", got)
		}
	}
}

// TestConfiguredLCPZeroRestartPeriod observes zrc's configured grace period,
// including the absence of any timer on Closed's sta-only transition.
// MUTATION: armRestartTimer fixed at three seconds fails the 500 ms expiry.
// RFC requirement: RFC1661-4.4-2 positive -- Opened's Terminate-Request zeroes a live counter and arms exactly the configured 500 ms grace period before Stopped.
// RFC requirement: RFC1661-4.4-2 negative -- Closed acknowledges the same request without arming a timer or retransmitting.
func TestConfiguredLCPZeroRestartPeriod(t *testing.T) {
	for _, state := range []LCPState{LCPStateOpened, LCPStateClosed} {
		t.Run(state.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, rec, _ := configuredLCPSession(t, state, 500*time.Millisecond, 2, 3, 5)
				s.restartCount = 2
				s.restartTimer = time.NewTimer(time.Hour)
				s.restartTimer.Stop()
				defer s.restartTimer.Stop()
				s.handleLCPPacket(LCPPacket{Code: LCPTerminateRequest, Identifier: 7})
				time.Sleep(500*time.Millisecond - time.Nanosecond)
				select {
				case <-s.restartTimer.C:
					t.Fatal("zrc grace period expired early")
				default:
				}
				time.Sleep(time.Nanosecond)
				select {
				case <-s.restartTimer.C:
					if state != LCPStateOpened || s.restartCount != 0 {
						t.Fatal("timer armed without zrc, or zrc left a nonzero counter")
					}
					s.handleRestartTimeout()
					if s.currentState() != LCPStateStopped {
						t.Fatal("zrc expiry did not reach Stopped")
					}
				default:
					if state == LCPStateOpened {
						t.Fatal("zrc did not use configured grace period")
					}
				}
				frames := decodeFrames(t, rec)
				if len(frames) != 1 {
					t.Fatalf("frames = %d, want one Terminate-Ack", len(frames))
				}
				frame := frames[0]
				if frame.Proto != ProtoLCP || frame.Pkt.Code != LCPTerminateAck || frame.Pkt.Identifier != 7 {
					t.Fatalf("termination reply protocol/code/id = %#x/%d/%d, want LCP Terminate-Ack/7", frame.Proto, frame.Pkt.Code, frame.Pkt.Identifier)
				}
			})
		})
	}
}

// TestConfiguredLCPConvergenceBound sends valid Naks before every Restart
// expiry. IRC renews the silent-peer counter, but cannot extend the lifetime's
// independently derived convergence deadline.
func TestConfiguredLCPConvergenceBound(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, events := configuredLCPSession(t, LCPStateInitial, 500*time.Millisecond, 2, 3, 5)
		peer := runLifecycleSession(t, s)
		started := time.Now()
		request := readLifecyclePacket(t, peer, ProtoLCP, LCPConfigureRequest)
		for range 11 {
			time.Sleep(250 * time.Millisecond)
			writeLifecyclePacket(t, peer, ProtoLCP, LCPConfigureNak, request.Identifier, optStream(mruOption(1400)))
			request = readLifecyclePacket(t, peer, ProtoLCP, LCPConfigureRequest)
		}
		time.Sleep(250 * time.Millisecond)
		synctest.Wait()
		down := awaitLifecycleDown(t, events)
		if elapsed := time.Since(started); elapsed != 3*time.Second {
			t.Fatalf("convergence ended after %s, want 3s", elapsed)
		}
		if !strings.Contains(down.Reason, "LCP convergence timeout after 3s") {
			t.Fatalf("reason = %q, want configured convergence timeout", down.Reason)
		}
	})
}

// TestConfiguredLCPLongSilentBudget runs the real session loop past the former
// fixed 30-second deadline. It must send exactly three requests 20 seconds apart
// and report the counter-driven termination at 60 seconds.
func TestConfiguredLCPLongSilentBudget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, events := configuredLCPSession(t, LCPStateInitial, 20*time.Second, 2, 3, 5)
		peer := runLifecycleSession(t, s)
		started := time.Now()
		for sent := range 3 {
			if err := peer.SetReadDeadline(time.Now().Add(21 * time.Second)); err != nil {
				t.Fatal(err)
			}
			buf := make([]byte, MaxFrameBufLen)
			n, err := peer.Read(buf)
			if err != nil {
				t.Fatalf("request %d: %v", sent+1, err)
			}
			proto, data, _, err := ParseFrame(buf[:n])
			if err != nil {
				t.Fatal(err)
			}
			packet, err := ParseLCPPacket(data)
			if err != nil {
				t.Fatal(err)
			}
			if proto != ProtoLCP || packet.Code != LCPConfigureRequest {
				t.Fatalf("request %d protocol/code = %x/%d", sent+1, proto, packet.Code)
			}
			if elapsed := time.Since(started); elapsed != time.Duration(sent)*20*time.Second {
				t.Fatalf("request %d at %s", sent+1, elapsed)
			}
		}
		time.Sleep(20 * time.Second)
		synctest.Wait()
		down := awaitLifecycleDown(t, events)
		if elapsed := time.Since(started); elapsed != time.Minute || !strings.Contains(down.Reason, "stopped") {
			t.Fatalf("session ended at %s: %+v", elapsed, down)
		}
		if s.currentState() != LCPStateStopped {
			t.Fatal("silent peer did not exhaust counter into Stopped")
		}
	})
}

// lcpDownOrderWriter observes the upper-layer state at each outgoing frame,
// rather than after handleLCPPacket has already finished all its actions.
type lcpDownOrderWriter struct {
	*frameRecorder
	session         *pppSession
	events          <-chan Event
	writesAfterDown int
}

func (w *lcpDownOrderWriter) Write(frame []byte) (int, error) {
	if !w.session.ncpStarted {
		if !w.session.channelConnected {
			if w.session.ipcpState == LCPStateInitial {
				if len(w.events) == 1 {
					w.writesAfterDown++
				}
			}
		}
	}
	return w.frameRecorder.Write(frame)
}

// TestLCPDownPrecedesRestartWireResponse pins the existing tld implementation:
// both accepted and refused requests tear down NCP before SCR and SCA/SCN.
// Echo is the contrast: it remains Opened and never publishes an LCP-down.
func TestLCPDownPrecedesRestartWireResponse(t *testing.T) {
	for _, refused := range []bool{false, true} {
		s, rec, events := configuredLCPSession(t, LCPStateOpened, time.Second, 2, 3, 5)
		s.ncpStarted = true
		if err := testHandleFor(s).Connect(); err != nil {
			t.Fatal(err)
		}
		s.channelConnected = true
		s.ipcpState = LCPStateOpened
		s.localIPv4 = netip.MustParseAddr("192.0.2.1")
		s.peerIPv4 = netip.MustParseAddr("192.0.2.2")
		local := netip.PrefixFrom(s.localIPv4, 32)
		if err := s.dataplane.AddAddressP2P(local, netip.PrefixFrom(s.peerIPv4, 32)); err != nil {
			t.Fatal(err)
		}
		s.installedIPv4 = local
		writer := &lcpDownOrderWriter{frameRecorder: rec, session: s, events: events}
		s.chanFile = writer
		stopRelay := relayLifecycleEvents(s)
		data := optStream(mruOption(1400))
		if refused {
			data = optStream(mruOption(2000))
		}
		ended := s.handleLCPPacket(LCPPacket{Code: LCPConfigureRequest, Identifier: 7, Data: data})
		stopRelay()
		if ended || writer.writesAfterDown != 2 {
			t.Fatalf("refused=%t: ended=%t, responses after completed LCP-down=%d, want 2", refused, ended, writer.writesAfterDown)
		}
		if len(events) != 1 {
			t.Fatalf("events = %d, want exactly one LCP-down", len(events))
		}
		if down, ok := (<-events).(EventLCPDown); !ok || !down.NetworkPhase {
			t.Fatalf("event = %+v, want network-phase LCP-down", down)
		}
		frames := decodeFrames(t, rec)
		wantReply := LCPConfigureAck
		if refused {
			wantReply = LCPConfigureNak
		}
		if len(frames) != 2 || frames[0].Pkt.Code != LCPConfigureRequest || frames[1].Pkt.Code != wantReply {
			t.Fatalf("refused=%t: frames = %+v, want SCR followed by %d", refused, frames, wantReply)
		}
	}
	s, rec, events := configuredLCPSession(t, LCPStateOpened, time.Second, 2, 3, 5)
	s.peerMagic = 0x11223344
	s.handleLCPPacket(LCPPacket{Code: LCPEchoRequest, Identifier: 8, Data: []byte{0x11, 0x22, 0x33, 0x44}})
	if s.currentState() != LCPStateOpened || len(events) != 0 {
		t.Fatal("Echo changed the LCP lifetime")
	}
	if frames := decodeFrames(t, rec); len(frames) != 1 || frames[0].Pkt.Code != LCPEchoReply {
		t.Fatalf("Echo frames = %+v, want exactly one Echo-Reply", frames)
	}
}
