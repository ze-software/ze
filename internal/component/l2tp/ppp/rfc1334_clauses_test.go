// Design: docs/architecture/l2tp/subscriber-session-model.md -- authentication method negotiation
// Related: auth_test.go -- the fallback-order helper the session consults
//
// Drives RFC 1334 Section 2 through LCP: a session that includes CHAP offers
// it in its Configure-Request before it ever offers PAP.

package ppp

import (
	"encoding/binary"
	"slices"
	"testing"
	"time"
)

// authProtoOffered returns the Authentication-Protocol value of the last LCP
// Configure-Request the session wrote, and false when that request carries
// no Authentication-Protocol option.
func authProtoOffered(t *testing.T, rec *frameRecorder) (uint16, bool) {
	t.Helper()
	for _, frame := range slices.Backward(decodeFrames(t, rec)) {
		if frame.Proto != ProtoLCP || frame.Pkt.Code != LCPConfigureRequest {
			continue
		}
		opts, err := ParseLCPOptions(frame.Pkt.Data)
		if err != nil {
			t.Fatalf("Configure-Request options do not parse: %v", err)
		}
		data, ok := lookupOption(opts, LCPOptAuthProto)
		if !ok || len(data) < 2 {
			return 0, false
		}
		return binary.BigEndian.Uint16(data[:2]), true
	}
	t.Fatal("no LCP Configure-Request written")
	return 0, false
}

// TestAuthOffersCHAPBeforePAP opens negotiation with CHAP-MD5 and the default
// fallback order, reads the first Configure-Request, answers it with a
// Configure-Nak that suggests PAP, and reads the next one. The control keeps
// PAP out of the fallback order and answers with the same Nak, after which
// selectAuthFallback omits the option rather than offer PAP.
//
// VALIDATES: RFC 1334 Section 2, CHAP is offered before PAP.
// PREVENTS: a first Configure-Request that offers PAP while CHAP is included,
// and a PAP offer the fallback order does not allow.
//
// RFC requirement: RFC1334-x-1 positive -- the first Configure-Request offers CHAP (0xc223), and only after the peer's Configure-Nak suggests PAP does the next Configure-Request offer PAP (0xc023).
// RFC requirement: RFC1334-x-1 negative -- the first Configure-Request never offers PAP, and when the fallback order holds no PAP, the Configure-Request after the same Nak does not offer PAP.
func TestAuthOffersCHAPBeforePAP(t *testing.T) {
	t.Parallel()

	papNak := LCPPacket{Code: LCPConfigureNak, Data: optStream(LCPOption{Type: LCPOptAuthProto, Data: []byte{0xC0, 0x23}})}
	for _, tc := range []struct {
		name     string
		order    []AuthMethod
		papAfter bool
	}{
		{"default order", defaultAuthFallbackOrder(), true},
		{"order without PAP", []AuthMethod{AuthMethodCHAPMD5, AuthMethodMSCHAPv2}, false},
	} {
		s, rec, _ := newRFC1661Session(LCPStateReqSent)
		s.configuredAuthMethod = AuthMethodCHAPMD5
		s.authFallbackOrder = tc.order
		if !s.sendConfigureRequest() {
			t.Fatalf("%s: sendConfigureRequest failed", tc.name)
		}
		first := decodeFrames(t, rec)[0].Pkt
		if got, ok := authProtoOffered(t, rec); !ok || got != authProtoCHAP {
			t.Fatalf("%s: first Configure-Request offers %#04x (present %v), want CHAP", tc.name, got, ok)
		}
		nak := papNak
		nak.Identifier = first.Identifier
		s.handleLCPPacket(nak)
		got, ok := authProtoOffered(t, rec)
		if offersPAP := ok && got == authProtoPAP; offersPAP != tc.papAfter {
			t.Fatalf("%s: after a PAP Nak the Configure-Request offers %#04x (present %v), want PAP %v",
				tc.name, got, ok, tc.papAfter)
		}
	}
}

// TestConfiguredPAPStillOffersCHAPFirst starts a session through the Driver
// with PAP configured, the way `auth-method pap` reaches spawnSession. The
// peer reads the first Configure-Request, which MUST offer CHAP because the
// session's fallback order includes CHAP-MD5, answers it with a Configure-Nak
// that suggests PAP, and reads the next one, which then offers PAP. The
// control configures a fallback order holding PAP alone, so the session
// includes no stronger method and its first Configure-Request offers PAP.
//
// VALIDATES: RFC 1334 Section 2, a configured PAP preference does not make ze
// offer PAP before CHAP.
// PREVENTS: spawnSession copying StartSession.AuthMethod = PAP into the first
// Configure-Request while CHAP is available to the session.
//
// RFC requirement: RFC1334-x-1 positive -- with PAP configured and CHAP-MD5 in the fallback order, the first Configure-Request built by spawnSession offers CHAP (0xc223), and the one after the peer's PAP Configure-Nak offers PAP (0xc023).
// RFC requirement: RFC1334-x-1 negative -- with PAP configured and CHAP-MD5 in the fallback order, the first Configure-Request built by spawnSession does not offer PAP.
func TestConfiguredPAPStillOffersCHAPFirst(t *testing.T) {
	for _, tc := range []struct {
		name      string
		fd        int
		order     []AuthMethod
		firstWant uint16
	}{
		{"CHAP in order", 16301, defaultAuthFallbackOrder(), authProtoCHAP},
		{"PAP alone", 16302, []AuthMethod{AuthMethodPAP}, authProtoPAP},
	} {
		reg := newPipeRegistry()
		installPipeRegistry(t, reg)
		pair := newPipePair(reg, tc.fd)

		ops, _, _ := newFakeOps()
		d := newTestDriverNoResponder(&fakeBackend{}, ops)
		if err := d.Start(); err != nil {
			t.Fatalf("%s: Start: %v", tc.name, err)
		}

		secondCR := make(chan LCPPacket, 1)
		peerDone := make(chan struct{})
		go authProtoReplyPeer(t, pair.peerEnd, tc.firstWant, LCPConfigureNak, authProtoPAP, nil, secondCR, peerDone)

		d.SessionsIn() <- StartSession{
			TunnelID:          183,
			SessionID:         uint16(tc.fd - 16000),
			ChanFD:            tc.fd,
			UnitFD:            tc.fd - 15000,
			UnitNum:           23,
			LNSMode:           true,
			MaxMRU:            1500,
			AuthMethod:        AuthMethodPAP,
			AuthFallbackOrder: tc.order,
		}

		select {
		case pkt := <-secondCR:
			opts, err := ParseLCPOptions(pkt.Data)
			if err != nil {
				t.Fatalf("%s: second Configure-Request options: %v", tc.name, err)
			}
			data, ok := lookupOption(opts, LCPOptAuthProto)
			if !ok || len(data) < 2 || binary.BigEndian.Uint16(data[:2]) != authProtoPAP {
				t.Fatalf("%s: Configure-Request after the PAP Nak carries Auth-Protocol %x (present %v), want PAP",
					tc.name, data, ok)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%s: no second Configure-Request (the first did not offer %#04x)", tc.name, tc.firstWant)
		}

		<-peerDone
		d.Stop()
		closeConn(pair.peerEnd)
	}
}
