// Design: docs/guide/l2tp.md -- an unfit mandatory authentication result fails visibly.

package ppp

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

// The RADIUS handler passes an Access-Reject's Reply-Message to AuthResponse
// without adding MS-CHAPv2 fields (plugins/authradius/handler.go, doRADIUS).
// Eighty ASCII octets fit that backend's attribute but exceed the 64-octet
// Message budget on a valid IPv4-only link with MRU 68. Refusing that result
// must happen before the bounded encoder can turn it into a truncated Failure.
// The structured control still sends its complete required fields and fitting
// display-text prefix; rejecting every result cannot satisfy both cases.
// MUTATION: let fitMSCHAPv2FailureMessage accept its unfit-prefix branch; the
// bounded encoder and final writer remain, but wire silence and reason fail.
func TestMSCHAPv2UnstructuredFailureDriverCap(t *testing.T) {
	for _, structured := range []bool{false, true} {
		name := "unstructured Reply-Message"
		if structured {
			name = "structured fitting prefix"
		}
		t.Run(name, func(t *testing.T) {
			reg := newPipeRegistry()
			installPipeRegistry(t, reg)
			pair := newPipePair(reg, 24201)
			ops, _, _ := newFakeOps()
			backend := &fakeBackend{}
			d := newTestDriverNoResponder(backend, ops)
			if err := d.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				closeConn(pair.peerEnd)
				d.Stop()
			})
			proto, authData := authMethodToLCPOptions(AuthMethodMSCHAPv2)
			peer := optStream(mruOpt(1500), magicOpt(0x11223344))
			local := optStream(mruOpt(MinIPv4MRU), magicOpt(0x01020304), authProtoOpt(proto, authData...))
			d.SessionsIn() <- StartSession{
				TunnelID: 24, SessionID: 2, ChanFD: 24201, UnitFD: 24202, UnitNum: 7,
				MaxMRU: MinIPv4MRU, AuthMethod: AuthMethodMSCHAPv2,
				AuthFallbackOrder: []AuthMethod{AuthMethodMSCHAPv2}, AuthRequired: true,
				DisableIPv6CP:       true,
				ProxyLCPInitialRecv: peer, ProxyLCPLastRecv: peer, ProxyLCPLastSent: local,
			}
			challenge := readLifecyclePacket(t, pair.peerEnd, ProtoCHAP, MSCHAPv2CodeChallenge)
			if len(challenge.Data) != 1+mschapv2ChallengeValueLen+len(mschapv2LNSName) || challenge.Data[0] != mschapv2ChallengeValueLen {
				t.Fatalf("MS-CHAPv2 challenge changed: %x", challenge.Data)
			}
			response := buildMSCHAPv2ResponsePayload(challenge.Identifier,
				bytes.Repeat([]byte{0x11}, mschapv2PeerChallengeLen),
				bytes.Repeat([]byte{0x22}, mschapv2NTResponseLen), "alice")
			writeLifecyclePacket(t, pair.peerEnd, ProtoCHAP, MSCHAPv2CodeResponse, challenge.Identifier, response[mschapv2HeaderLen:])
			select {
			case ev := <-d.AuthEventsOut():
				request, ok := ev.(EventAuthRequest)
				if !ok || request.TunnelID != 24 || request.SessionID != 2 || request.Method != AuthMethodMSCHAPv2 || request.Identifier != challenge.Identifier || request.Username != "alice" {
					t.Fatalf("authentication did not reach its real handler boundary: %+v", ev)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("authentication did not reach the handler")
			}
			d.mu.Lock()
			session := d.sessions[sessionKey{tunnelID: 24, sessionID: 2}]
			d.mu.Unlock()
			if session == nil {
				t.Fatal("authentication session disappeared before the handler decision")
			}
			if err := pair.peerEnd.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			type wireResult struct {
				data []byte
				err  error
			}
			wire := make(chan wireResult, 1)
			go func() {
				data, err := io.ReadAll(pair.peerEnd)
				wire <- wireResult{data: data, err: err}
			}()
			message := strings.Repeat("x", 80)
			wantReason := "chap-v2: auth failure fields exceed the transmit MRU"
			wantAuthReason := wantReason
			if structured {
				message = "E=691 R=0 C=" + strings.Repeat("AB", 16) + " V=3 M=" + message
				wantReason = "chap-v2: auth rejected: " + message
				wantAuthReason = message
			}
			if err := d.AuthResponse(24, 2, false, message, nil); err != nil {
				t.Fatal(err)
			}
			select {
			case <-session.done:
			case <-time.After(5 * time.Second):
				t.Fatal("rejected authentication did not terminate the session")
			}
			// Completion closes the actual channel and finishes every event
			// producer, so these assertions need no silence timer or polling.
			captured := <-wire
			if captured.err != nil {
				t.Fatalf("capturing the channel until teardown: %v", captured.err)
			}
			if !structured {
				if len(captured.data) != 0 {
					t.Errorf("unstructured oversized rejection transmitted a complete or truncated Failure: %x", captured.data)
				}
			} else {
				wireProto, payload, _, err := ParseFrame(captured.data)
				if err != nil || wireProto != ProtoCHAP {
					t.Fatalf("structured control wire protocol = %#x, error = %v", wireProto, err)
				}
				result, err := ParseLCPPacket(payload)
				want := message[:int(MinIPv4MRU)-mschapv2HeaderLen]
				if err != nil || result.Code != MSCHAPv2CodeFailure || result.Identifier != challenge.Identifier || string(result.Data) != want || len(payload) != int(MinIPv4MRU) {
					t.Fatalf("structured fitting control lost its full error fields or cap: %+v, payload %d, error %v", result, len(payload), err)
				}
			}
			if len(d.EventsOut()) != 2 {
				t.Fatalf("lifecycle events = %d, want only LCP-up and one failure", len(d.EventsOut()))
			}
			if up, ok := (<-d.EventsOut()).(EventLCPUp); !ok || up.NegotiatedMRU != 1500 || up.NegotiatedAuthMethod != AuthMethodMSCHAPv2 {
				t.Fatalf("proxy negotiation did not reach authentication: %+v", up)
			}
			if down, ok := (<-d.EventsOut()).(EventSessionDown); !ok || down.TunnelID != 24 || down.SessionID != 2 || down.Reason != wantReason {
				t.Errorf("missing informative failure at the mandatory-field size guard: %+v, want %q", down, wantReason)
			}
			if len(d.AuthEventsOut()) != 1 {
				t.Fatalf("auth outcome events = %d, want only failure", len(d.AuthEventsOut()))
			}
			if failure, ok := (<-d.AuthEventsOut()).(eventAuthFailure); !ok || failure.TunnelID != 24 || failure.SessionID != 2 || failure.Reason != wantAuthReason {
				t.Errorf("rejection became auth success or lost its reason: %+v", failure)
			}
			if session.ncpStarted || len(d.IPEventsOut()) != 0 || len(backend.MTUCalls()) != 0 || len(backend.UpCalls()) != 0 || len(backend.P2PCalls()) != 0 {
				t.Error("rejected authentication progressed to NCP or installed network state")
			}
		})
	}
}
