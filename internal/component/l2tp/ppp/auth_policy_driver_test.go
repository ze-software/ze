package ppp

import (
	"encoding/binary"
	"strconv"
	"strings"
	"testing"
	"time"
)

// policyOffer reads the actual last Configure-Request, including its algorithm.
func policyOffer(t *testing.T, rec *frameRecorder) AuthMethod {
	t.Helper()
	req := lastLCPConfigureRequest(t, rec)
	opts, err := ParseLCPOptions(req.Data)
	if err != nil {
		t.Fatal(err)
	}
	data, ok := lookupOption(opts, LCPOptAuthProto)
	if !ok {
		return AuthMethodNone
	}
	if len(data) < 2 {
		t.Fatal("short auth option")
	}
	return authMethodFromAuthProto(binary.BigEndian.Uint16(data), data[2:])
}

// TestAuthAllowSelectionMatrix drives validated wire replies through the real
// packet dispatcher, then observes the selected method published at Opened.
// RFC requirement: RFC1661-6.2-2 positive -- each Nak advances to an untried permitted method, with configured CHAP first and PAP last.
// RFC requirement: RFC1661-6.2-2 negative -- repeated, unknown and outside suggestions never repeat an offered method or admit an unlisted method.
// RFC requirement: RFC1334-x-1 positive -- with PAP configured and the default permitted methods, the first wire Configure-Request offers CHAP-MD5, a correlated PAP Configure-Nak then selects PAP, and the final Configure-Ack publishes PAP at Opened.
// RFC requirement: RFC1334-x-1 negative -- with PAP configured and [pap, ms-chap-v2, chap-md5] permitted, repeated correlated PAP Configure-Naks cannot select PAP while CHAP-MD5 remains untried: the wire offers MS-CHAPv2, then CHAP-MD5, then PAP, and the final Configure-Ack publishes PAP at Opened.
func TestAuthAllowSelectionMatrix(t *testing.T) {
	for _, tc := range []struct {
		name        string
		method      AuthMethod
		names       []string
		suggestions []AuthMethod
		offers      []AuthMethod
		optional    bool
		reject      bool
		refused     bool
	}{
		{name: "CHAP default ack", method: AuthMethodCHAPMD5, offers: []AuthMethod{AuthMethodCHAPMD5}},
		{name: "CHAP default refuses PAP", method: AuthMethodCHAPMD5, suggestions: []AuthMethod{AuthMethodPAP}, offers: []AuthMethod{AuthMethodCHAPMD5}, refused: true},
		{name: "explicit PAP permitted", method: AuthMethodCHAPMD5, names: []string{"chap-md5", "pap"}, suggestions: []AuthMethod{AuthMethodPAP}, offers: []AuthMethod{AuthMethodCHAPMD5, AuthMethodPAP}},
		{name: "all CHAP before PAP", method: AuthMethodCHAPMD5, names: []string{"chap-md5", "ms-chap-v2", "pap"}, suggestions: []AuthMethod{AuthMethodPAP, AuthMethodPAP}, offers: []AuthMethod{AuthMethodCHAPMD5, AuthMethodMSCHAPv2, AuthMethodPAP}},
		{name: "configured MSCHAP first", method: AuthMethodMSCHAPv2, names: []string{"chap-md5", "ms-chap-v2"}, suggestions: []AuthMethod{AuthMethodCHAPMD5}, offers: []AuthMethod{AuthMethodMSCHAPv2, AuthMethodCHAPMD5}},
		{name: "PAP default", method: AuthMethodPAP, suggestions: []AuthMethod{AuthMethodPAP}, offers: []AuthMethod{AuthMethodCHAPMD5, AuthMethodPAP}},
		{name: "PAP written first", method: AuthMethodPAP, names: []string{"pap", "ms-chap-v2", "chap-md5"}, suggestions: []AuthMethod{AuthMethodPAP, AuthMethodPAP}, offers: []AuthMethod{AuthMethodMSCHAPv2, AuthMethodCHAPMD5, AuthMethodPAP}},
		{name: "repeated suggestion", method: AuthMethodCHAPMD5, names: []string{"chap-md5", "ms-chap-v2", "pap"}, suggestions: []AuthMethod{AuthMethodCHAPMD5, AuthMethodCHAPMD5, AuthMethodPAP}, offers: []AuthMethod{AuthMethodCHAPMD5, AuthMethodMSCHAPv2, AuthMethodPAP}, refused: true},
		{name: "unknown advances", method: AuthMethodCHAPMD5, names: []string{"chap-md5", "pap"}, suggestions: []AuthMethod{AuthMethodNone}, offers: []AuthMethod{AuthMethodCHAPMD5, AuthMethodPAP}},
		{name: "outside advances", method: AuthMethodCHAPMD5, names: []string{"chap-md5", "pap"}, suggestions: []AuthMethod{AuthMethodMSCHAPv2}, offers: []AuthMethod{AuthMethodCHAPMD5, AuthMethodPAP}},
		{name: "optional exhausted", method: AuthMethodCHAPMD5, suggestions: []AuthMethod{AuthMethodPAP}, offers: []AuthMethod{AuthMethodCHAPMD5, AuthMethodNone}, optional: true},
		{name: "optional reject with unused choices", method: AuthMethodCHAPMD5, names: []string{"chap-md5", "pap"}, offers: []AuthMethod{AuthMethodCHAPMD5, AuthMethodNone}, optional: true, reject: true},
		{name: "required reject with unused choices", method: AuthMethodCHAPMD5, names: []string{"chap-md5", "pap"}, offers: []AuthMethod{AuthMethodCHAPMD5}, reject: true, refused: true},
		{name: "explicit noauth", method: AuthMethodNone, offers: []AuthMethod{AuthMethodNone}, optional: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			order, err := ResolveAuthAllow(tc.method, tc.names)
			if err != nil {
				t.Fatal(err)
			}
			s, rec, events := newRFC1661Session(LCPStateAckSent)
			s.authRequired = !tc.optional
			s.authFallbackOrder = order
			s.configuredAuthMethod = initialAuthMethod(tc.method, order)
			if !s.sendConfigureRequest() {
				t.Fatal("first request failed")
			}
			if got := policyOffer(t, rec); got != tc.offers[0] {
				t.Fatalf("first offer %v want %v", got, tc.offers[0])
			}
			steps := len(tc.suggestions)
			if tc.reject {
				steps = 1
			}
			for i := range steps {
				req := lastLCPConfigureRequest(t, rec)
				proto, data := authMethodToLCPOptions(AuthMethodNone)
				if !tc.reject {
					proto, data = authMethodToLCPOptions(tc.suggestions[i])
					if proto == 0 {
						proto = 0xc227
					}
				}
				pkt := LCPPacket{Code: LCPConfigureNak, Identifier: req.Identifier, Data: optStream(authProtoOpt(proto, data...))}
				if tc.reject {
					opts, _ := ParseLCPOptions(req.Data)
					auth, _ := lookupOption(opts, LCPOptAuthProto)
					pkt.Code = LCPConfigureReject
					pkt.Data = optStream(LCPOption{Type: LCPOptAuthProto, Data: auth})
				}
				if s.handleLCPPacket(pkt) {
					t.Fatal("session exited before termination grace")
				}
				if tc.refused && i == steps-1 {
					frames := decodeFrames(t, rec)
					if got := frames[len(frames)-1].Pkt.Code; got != LCPTerminateRequest {
						t.Fatalf("last code %d want TerminateRequest", got)
					}
					if s.currentState() != LCPStateClosing {
						t.Fatalf("state %v", s.currentState())
					}
					if len(events) != 0 || len(s.authEventsOut) != 0 {
						t.Fatal("refusal published Opened/auth")
					}
					if !strings.Contains(s.authRefusalReason, "allowed [") {
						t.Fatal("missing refusal policy reason")
					}
					return
				}
				if got := policyOffer(t, rec); got != tc.offers[i+1] {
					t.Fatalf("offer %d = %v want %v", i+1, got, tc.offers[i+1])
				}
			}
			req := lastLCPConfigureRequest(t, rec)
			done := make(chan bool, 1)
			go func() {
				done <- s.handleLCPPacket(LCPPacket{Code: LCPConfigureAck, Identifier: req.Identifier, Data: req.Data})
			}()
			select {
			case ev := <-events:
				up, ok := ev.(EventLCPUp)
				if !ok {
					t.Fatalf("event %T", ev)
				}
				if up.NegotiatedAuthMethod != tc.offers[len(tc.offers)-1] || up.NegotiatedMRU != 1500 {
					t.Fatalf("actual negotiation %+v", up)
				}
			case <-time.After(time.Second):
				t.Fatal("no LCPUp")
			}
			close(s.sessStop)
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("auth phase did not stop")
			}
		})
	}
}

// TestAuthRefusalTerminationCleanup proves each negotiating state uses normal
// Close and releases the session only on Terminate-Ack or Restart exhaustion.
// RFC requirement: RFC1661-3.5-6 positive -- unacceptable authentication closes LCP with Terminate-Request before any authentication or LCPUp event.
// RFC requirement: RFC1661-3.5-6 negative -- the permitted-alternative control continues negotiation with PAP instead of terminating the link.
func TestAuthRefusalTerminationCleanup(t *testing.T) {
	for _, state := range []LCPState{LCPStateReqSent, LCPStateAckSent, LCPStateAckRcvd} {
		for _, ack := range []bool{true, false} {
			t.Run(state.String()+textBool(ack), func(t *testing.T) {
				s, rec, events := newRFC1661Session(state)
				s.configuredAuthMethod = AuthMethodCHAPMD5
				s.authFallbackOrder = []AuthMethod{AuthMethodCHAPMD5}
				s.authRequired = true
				s.restartTimer = time.NewTimer(time.Hour)
				s.restartTimer.Stop()
				defer s.restartTimer.Stop()
				s.sendConfigureRequest()
				req := lastLCPConfigureRequest(t, rec)
				if s.handleLCPPacket(LCPPacket{Code: LCPConfigureNak, Identifier: req.Identifier, Data: optStream(authProtoOpt(authProtoPAP))}) {
					t.Fatal("refusal skipped grace")
				}
				if s.currentState() != LCPStateClosing || s.restartCount != defaultMaxTerminate-1 {
					t.Fatal("not normal closing lifecycle")
				}
				if len(events) != 0 || len(s.authEventsOut) != 0 {
					t.Fatal("refusal emitted pre-termination event")
				}
				if ack {
					if !s.handleLCPPacket(LCPPacket{Code: LCPTerminateAck, Identifier: s.terminateID}) {
						t.Fatal("TerminateAck did not close")
					}
				} else {
					if s.handleRestartTimeout() {
						t.Fatal("first timeout ended before retry")
					}
					if !s.handleRestartTimeout() {
						t.Fatal("restart exhaustion did not close")
					}
				}
				if s.currentState() != LCPStateClosed {
					t.Fatal("closing left live state")
				}
				if s.restartTimer.Stop() {
					t.Fatal("closing left live restart timer")
				}
				ev := <-events
				down, ok := ev.(EventSessionDown)
				if !ok || !strings.Contains(down.Reason, "auth-method-refused") {
					t.Fatalf("termination event %+v", ev)
				}
			})
		}
	}
	t.Run("permitted alternative does not close", func(t *testing.T) {
		s, rec, events := newRFC1661Session(LCPStateReqSent)
		s.configuredAuthMethod = AuthMethodCHAPMD5
		s.authFallbackOrder = []AuthMethod{AuthMethodCHAPMD5, AuthMethodPAP}
		s.authRequired = true
		if !s.sendConfigureRequest() {
			t.Fatal("request failed")
		}
		req := lastLCPConfigureRequest(t, rec)
		if s.handleLCPPacket(LCPPacket{Code: LCPConfigureNak, Identifier: req.Identifier, Data: optStream(authProtoOpt(authProtoPAP))}) {
			t.Fatal("permitted PAP terminated")
		}
		if s.currentState() != LCPStateReqSent || policyOffer(t, rec) != AuthMethodPAP {
			t.Fatal("permitted alternative did not continue")
		}
		if len(events) != 0 {
			t.Fatal("permitted Nak published termination")
		}
	})
}

func textBool(v bool) string {
	if v {
		return "/ack"
	}
	return "/timeout"
}

// TestAuthOfferRetransmitAndLifetime proves timeouts do not consume another
// method and an actual LCP restart restores the initial policy and correlation.
func TestAuthOfferRetransmitAndLifetime(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateReqSent)
	s.configuredAuthMethod = AuthMethodCHAPMD5
	s.authFallbackOrder = []AuthMethod{AuthMethodCHAPMD5, AuthMethodPAP}
	s.sendConfigureRequest()
	first := lastLCPConfigureRequest(t, rec)
	s.sendConfigureRequest()
	if got := lastLCPConfigureRequest(t, rec); got.Identifier != first.Identifier {
		t.Fatal("retransmit changed identifier")
	}
	if s.authMethodsTried != 1<<AuthMethodCHAPMD5 {
		t.Fatal("retransmit consumed methods")
	}
	s.handleLCPPacket(LCPPacket{Code: LCPConfigureReject, Identifier: first.Identifier, Data: optStream(authProtoOpt(authProtoCHAP, chapAlgorithmMD5))})
	if policyOffer(t, rec) != AuthMethodNone {
		t.Fatal("reject retried option")
	}
	s.mu.Lock()
	s.state = LCPStateOpened
	s.mu.Unlock()
	if s.handleLCPPacket(LCPPacket{Code: LCPConfigureRequest, Identifier: 33, Data: optStream(mruOpt(1400))}) {
		t.Fatal("restart failed")
	}
	if policyOffer(t, rec) != AuthMethodCHAPMD5 {
		t.Fatal("new lifetime did not restore initial auth")
	}
	before := rec.count()
	s.handleLCPPacket(LCPPacket{Code: LCPConfigureAck, Identifier: first.Identifier, Data: first.Data})
	if rec.count() != before || s.currentState() == LCPStateOpened {
		t.Fatal("old reply reopened new lifetime")
	}
}

// TestConfiguredMRUNakInformationFloor checks 68/69 in Information-field units
// and confirms both the requested MRU and peer-side cap use the configured value.
func TestConfiguredMRUNakInformationFloor(t *testing.T) {
	for _, mru := range []uint16{68, 69, 1492, 1500} {
		t.Run(strconv.Itoa(int(mru)), func(t *testing.T) {
			s, rec, _ := newRFC1661Session(LCPStateReqSent)
			s.maxMRU = 1492
			if mru == 1500 {
				s.maxMRU = 1500
			}
			s.sendConfigureRequest()
			req := lastLCPConfigureRequest(t, rec)
			s.handleLCPPacket(LCPPacket{Code: LCPConfigureNak, Identifier: req.Identifier, Data: optStream(mruOpt(mru))})
			opts, _ := ParseLCPOptions(lastLCPConfigureRequest(t, rec).Data)
			got, ok := lookupMRUOption(opts)
			if !ok || got != mru {
				t.Fatalf("MRU %d want %d", got, mru)
			}
		})
	}
	s, rec, _ := newRFC1661Session(LCPStateReqSent)
	s.maxMRU = 1492
	s.handleLCPPacket(LCPPacket{Code: LCPConfigureRequest, Identifier: 42, Data: optStream(mruOpt(1500))})
	frames := decodeFrames(t, rec)
	last := frames[len(frames)-1].Pkt
	opts, _ := ParseLCPOptions(last.Data)
	got, _ := lookupMRUOption(opts)
	if last.Code != LCPConfigureNak || got != 1492 {
		t.Fatalf("peer cap reply %+v MRU %d", last, got)
	}
}
