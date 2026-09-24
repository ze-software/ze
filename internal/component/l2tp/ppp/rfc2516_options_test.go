// VALIDATES: PPPoE's prohibited LCP options are rejected on the AC wire path,
// while ordinary PPP negotiation retains its transport-specific behavior.
// RFC: rfc/short/rfc2516.md
package ppp

import (
	"bytes"
	"testing"
)

// RFC requirement: RFC2516-7-2 positive -- the AC rejects ACCM, ACFC and FCS Alternatives verbatim and never proposes them in its own Configure-Request.
// RFC requirement: RFC2516-7-2 negative -- acceptable MRU and PFC options are acknowledged; L2TP still acknowledges ACCM and ACFC.
// MUTATION: remove the PPPoE guard in negotiatePeerOption to make the forbidden-option reply an Ack.
func TestRFC2516ACRejectsForbiddenLCPOptions(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"ACCM", []byte{LCPOptACCM, 6, 0xff, 0xff, 0xff, 0xff}},
		{"ACFC", []byte{LCPOptACFC, 2}},
		{"FCS Alternatives", []byte{9, 3, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, rec, _ := newRFC1661Session(LCPStateReqSent)
			s.pppoe = true
			request := append([]byte{LCPOptMRU, 4, 5, 0xd4}, tc.data...)
			if s.handleFrame(lcpReqFrame(0x71, request)) {
				t.Fatal("Configure-Request terminated the session")
			}
			frames := decodeFrames(t, rec)
			if len(frames) != 1 || frames[0].Pkt.Code != LCPConfigureReject {
				t.Fatalf("reply = %+v, want one Configure-Reject", frames)
			}
			if frames[0].Pkt.Identifier != 0x71 || !bytes.Equal(frames[0].Pkt.Data, tc.data) {
				t.Fatalf("Reject = %+v, want identifier 0x71 and options %x", frames[0].Pkt, tc.data)
			}
		})
	}

	for _, tc := range []struct {
		name  string
		pppoe bool
		data  []byte
	}{
		{"PPPoE MRU and PFC", true, []byte{LCPOptMRU, 4, 5, 0xd4, LCPOptPFC, 2}},
		{"L2TP ACCM and ACFC", false, []byte{LCPOptACCM, 6, 0, 0, 0, 0, LCPOptACFC, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, rec, _ := newRFC1661Session(LCPStateReqSent)
			s.pppoe = tc.pppoe
			if s.handleFrame(lcpReqFrame(0x72, tc.data)) {
				t.Fatal("acceptable options terminated the session")
			}
			frames := decodeFrames(t, rec)
			if len(frames) != 1 || frames[0].Pkt.Code != LCPConfigureAck {
				t.Fatalf("reply = %+v, want one Configure-Ack", frames)
			}
			if !bytes.Equal(frames[0].Pkt.Data, tc.data) {
				t.Fatalf("Ack options = %x, want %x", frames[0].Pkt.Data, tc.data)
			}
		})
	}

	s, rec, _ := newRFC1661Session(LCPStateReqSent)
	s.pppoe = true
	if !s.sendConfigureRequest() {
		t.Fatal("local Configure-Request failed")
	}
	request, ok := findCode(t, rec, LCPConfigureRequest)
	if !ok {
		t.Fatal("AC sent no Configure-Request")
	}
	opts, err := ParseLCPOptions(request.Data)
	if err != nil {
		t.Fatal(err)
	}
	for _, opt := range opts {
		switch opt.Type {
		case LCPOptACCM, LCPOptACFC, 9:
			t.Fatalf("AC requested prohibited option %d", opt.Type)
		}
	}
}
