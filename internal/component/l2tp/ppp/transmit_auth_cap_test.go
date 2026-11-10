// Design: docs/guide/l2tp.md -- authentication results obey the admitted transmit cap.

package ppp

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// TestAuthenticationResultTransmitCap uses the Driver's real authentication
// exchange and handler response route. Display text can shrink; decisions,
// Identifiers and MS-CHAPv2's mutual-authentication response cannot change.
func TestAuthenticationResultTransmitCap(t *testing.T) {
	for _, method := range []AuthMethod{AuthMethodPAP, AuthMethodCHAPMD5, AuthMethodMSCHAPv2} {
		for _, accept := range []bool{true, false} {
			for _, tc := range []struct {
				name string
				cap  uint16
				peer uint16
				text string
			}{
				{"minimum cap", 68, 1500, strings.Repeat("x", 2000)},
				{"1492 cap", 1492, 1500, strings.Repeat("x", 2000)},
				{"smaller peer", 1492, 1400, strings.Repeat("x", 2000)},
				{"fitting text", 68, 1500, "ok"},
			} {
				outcome := "accept"
				if !accept {
					outcome = "reject"
				}
				t.Run(method.String()+"/"+outcome+"/"+tc.name, func(t *testing.T) {
					reg := newPipeRegistry()
					installPipeRegistry(t, reg)
					pair := newPipePair(reg, 24101)
					ops, _, _ := newFakeOps()
					d := newTestDriverNoResponder(&fakeBackend{}, ops)
					if err := d.Start(); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						closeConn(pair.peerEnd)
						d.Stop()
					})
					proto, authData := authMethodToLCPOptions(method)
					peer := optStream(mruOpt(tc.peer), magicOpt(0x11223344))
					local := optStream(mruOpt(tc.cap), magicOpt(0x01020304), authProtoOpt(proto, authData...))
					d.SessionsIn() <- StartSession{
						TunnelID: 24, SessionID: 1, ChanFD: 24101, UnitFD: 24102, UnitNum: 7,
						MaxMRU: tc.cap, AuthMethod: method, AuthFallbackOrder: []AuthMethod{method}, AuthRequired: true,
						DisableIPCP: true, DisableIPv6CP: true,
						ProxyLCPInitialRecv: peer, ProxyLCPLastRecv: peer, ProxyLCPLastSent: local,
					}
					identifier := uint8(0x5a)
					wireProto := ProtoPAP
					if method == AuthMethodPAP {
						if _, err := pair.peerEnd.Write(papAuthRequestFrame()); err != nil {
							t.Fatal(err)
						}
					} else {
						wireProto = ProtoCHAP
						challenge := readLifecyclePacket(t, pair.peerEnd, ProtoCHAP, CHAPCodeChallenge)
						identifier = challenge.Identifier
						if len(challenge.Data) != 1+16+len(chapLNSName) || challenge.Data[0] != 16 {
							t.Fatalf("challenge value/name changed: %x", challenge.Data)
						}
						var response []byte
						if method == AuthMethodCHAPMD5 {
							response = append([]byte{chapMD5DigestLen}, bytes.Repeat([]byte{0x22}, chapMD5DigestLen)...)
							response = append(response, "alice"...)
						} else {
							packet := buildMSCHAPv2ResponsePayload(identifier, bytes.Repeat([]byte{0x11}, mschapv2PeerChallengeLen), bytes.Repeat([]byte{0x22}, mschapv2NTResponseLen), "alice")
							response = packet[chapHeaderLen:]
						}
						writeLifecyclePacket(t, pair.peerEnd, ProtoCHAP, CHAPCodeResponse, identifier, response)
					}
					select {
					case ev := <-d.AuthEventsOut():
						request, ok := ev.(EventAuthRequest)
						if !ok || request.Method != method || request.Identifier != identifier || request.Username != "alice" {
							t.Fatalf("auth request = %+v", ev)
						}
					case <-time.After(5 * time.Second):
						t.Fatal("authentication did not reach the handler")
					}
					message := tc.text
					if method == AuthMethodMSCHAPv2 && !accept {
						message = "E=691 R=0 C=" + strings.Repeat("AB", 16) + " V=3 M=" + message
					}
					blob := bytes.Repeat([]byte{0xcd}, mschapv2AuthenticatorResponseLen)
					if err := d.AuthResponse(24, 1, accept, message, blob); err != nil {
						t.Fatal(err)
					}
					code := CHAPCodeSuccess
					if !accept {
						code = CHAPCodeFailure
					}
					if method == AuthMethodPAP {
						code = PAPAuthenticateAck
						if !accept {
							code = PAPAuthenticateNak
						}
					}
					result := readLifecyclePacket(t, pair.peerEnd, wireProto, code)
					if result.Identifier != identifier {
						t.Fatalf("reply Identifier = %d, want %d", result.Identifier, identifier)
					}
					limit := int(min(tc.cap, tc.peer))
					var want string
					switch method {
					case AuthMethodNone:
						t.Fatal("AuthMethodNone sends no auth result to cap")
					case AuthMethodPAP:
						want = message[:min(len(message), papMaxFieldLen, limit-papHeaderLen-1)]
						if len(result.Data) != 1+len(want) || int(result.Data[0]) != len(want) || string(result.Data[1:]) != want {
							t.Fatalf("PAP result has incorrect optional Message: %x", result.Data)
						}
					case AuthMethodCHAPMD5:
						want = message[:min(len(message), limit-chapHeaderLen)]
						if string(result.Data) != want {
							t.Fatal("CHAP changed its fitting display text")
						}
					case AuthMethodMSCHAPv2:
						want = message
						if accept {
							want = "S=" + strings.Repeat("CD", 20) + " M=" + message
						}
						want = want[:min(len(want), limit-mschapv2HeaderLen)]
						if string(result.Data) != want {
							t.Fatal("MS-CHAPv2 lost the authenticator/error fields or fitting text")
						}
					}
					if lcpHeaderLen+len(result.Data) > limit {
						t.Fatalf("auth result size %d exceeds transmit limit %d", lcpHeaderLen+len(result.Data), limit)
					}
					select {
					case ev := <-d.AuthEventsOut():
						if accept {
							if _, ok := ev.(eventAuthSuccess); !ok {
								t.Fatalf("accepted handler outcome = %T", ev)
							}
						} else if failure, ok := ev.(eventAuthFailure); !ok || failure.Reason != message {
							t.Fatalf("rejected handler outcome changed: %+v", ev)
						}
					case <-time.After(5 * time.Second):
						t.Fatal("authentication outcome was not published")
					}
				})
			}
		}
	}
}

func TestCachedPAPResultFitsMinimumTransmitCap(t *testing.T) {
	for _, code := range []uint8{PAPAuthenticateAck, PAPAuthenticateNak} {
		s, rec, _ := newRFC1661Session(LCPStateOpened)
		s.maxMRU = MinIPv4MRU
		s.negotiatedMRU = 1500
		s.papReplyCode = code
		payload := papAuthRequestFrame()[2:]
		if !s.reanswerPAP(payload) {
			t.Fatal("cached PAP reply failed")
		}
		got := decodeFrames(t, rec)
		if len(got) != 1 || got[0].Proto != ProtoPAP || got[0].Pkt.Code != code || got[0].Pkt.Identifier != payload[1] || len(got[0].Pkt.Data) != 1 || got[0].Pkt.Data[0] != 0 {
			t.Fatalf("cached PAP outcome changed: %+v", got)
		}
	}
}

func TestMSCHAPv2FailurePrefaceCannotBeTruncated(t *testing.T) {
	message := "E=691 R=0 C=" + strings.Repeat("AB", 16) + " V=3 M=" + strings.Repeat("x", 2000)
	if fitted, ok := fitMSCHAPv2FailureMessage(message, 64); !ok || fitted != message[:64] {
		t.Fatalf("fitting mandatory prefix was not retained: %q, %v", fitted, ok)
	}
	if _, ok := fitMSCHAPv2FailureMessage(message, 20); ok {
		t.Fatal("truncation inside mandatory challenge fields was permitted")
	}
	if _, ok := fitMSCHAPv2FailureMessage(strings.Repeat("invalid", 20), 64); ok {
		t.Fatal("oversized unstructured failure was silently cut")
	}
}
