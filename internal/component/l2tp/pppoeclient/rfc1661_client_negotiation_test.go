// Related: session.go -- negotiateLCP, the client's Configure-Request judge
// Related: rfc1661_option_length_test.go -- runClientLCP and the frame log

package pppoeclient

import (
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// clientReplyTo returns the LCP Code the client answered the server's
// Configure-Request with, and whether it answered at all.
func clientReplyTo(t *testing.T, log *frameLog, id uint8) (uint8, []ppp.LCPOption, bool) {
	t.Helper()
	for _, pkt := range log.lcpPackets(t) {
		if pkt.Identifier != id {
			continue
		}
		switch pkt.Code {
		case ppp.LCPConfigureAck, ppp.LCPConfigureNak, ppp.LCPConfigureReject:
			opts, err := ppp.ParseLCPOptions(pkt.Data)
			if err != nil {
				t.Fatalf("the client's %s Data %% x does not parse: %v", ppp.LCPCodeName(pkt.Code), err)
			}
			return pkt.Code, opts, true
		}
	}
	return 0, nil, false
}

// VALIDATES: the PPPoE client judges the VALUES in a well-formed server
//
//	Configure-Request, and answers a Magic-Number of zero with a
//	Configure-Nak.
//
// PREVENTS: the client acknowledging every parseable Configure-Request unread,
// which stood here until 2026-09-20. negotiateLCP called sendLCPAck as soon as
// the option walk was clean, so a Magic-Number of zero was acknowledged by the
// client and Nak'd by the BNG side: one negotiator, two answers to one peer.
//
// RFC requirement: RFC1661-6.4-3 positive -- RFC 1661 Section 6.4: "A
// Magic-Number of zero is illegal and MUST always be Nak'd, if it is not
// Rejected outright." The client runs the same ppp.NegotiatePeerOptions the
// LNS runs (producer negotiateLCP, session.go), so the zero draws a
// Configure-Nak carrying a Magic-Number ze accepts, with no invalid option
// Length anywhere in the request to route the reply another way.
func TestRFC1661ClientNaksZeroMagicInAWellFormedRequest(t *testing.T) {
	// Every option here is well formed for its Type. Only the Magic-Number's
	// VALUE is illegal, so nothing but a value judgement can produce a reply.
	request := []byte{
		ppp.LCPOptMRU, 4, 0x05, 0xD4,
		ppp.LCPOptMagic, 6, 0, 0, 0, 0,
	}
	log := runClientLCP(t, serverFrame(ppp.LCPConfigureRequest, 0x21, request))

	code, opts, ok := clientReplyTo(t, log, 0x21)
	if !ok {
		t.Fatal("the client did not answer the server's Configure-Request at all")
	}
	if code != ppp.LCPConfigureNak {
		t.Fatalf("the client answered a Magic-Number of zero with %s; RFC 1661 Section 6.4 requires a Configure-Nak", ppp.LCPCodeName(code))
	}
	if len(opts) != 1 || opts[0].Type != ppp.LCPOptMagic {
		t.Fatalf("the Configure-Nak carries %+v, want the Magic-Number option alone", opts)
	}
	if len(opts[0].Data) != 4 {
		t.Fatalf("Nak'd Magic-Number carries %d octets, want the 4 RFC 1661 Section 6.4 gives it", len(opts[0].Data))
	}
	if v := binary.BigEndian.Uint32(opts[0].Data); v == 0 {
		t.Fatal("the client offered a Magic-Number of zero, the value RFC 1661 Section 6.4 calls illegal")
	}
}

// VALIDATES: a well-formed server Configure-Request whose values ze accepts is
//
//	still acknowledged, so the Nak above is the value and not the new branch
//	refusing everything.
//
// RFC requirement: RFC1661-6.4-3 negative -- a non-zero Magic-Number is not
// the illegal value, so Section 6.4 asks for no Nak and negotiateLCP
// (session.go) acknowledges the request.
func TestRFC1661ClientAcksANonZeroMagic(t *testing.T) {
	request := []byte{
		ppp.LCPOptMRU, 4, 0x05, 0xD4,
		ppp.LCPOptMagic, 6, 0xDE, 0xAD, 0xBE, 0xEF,
	}
	log := runClientLCP(t, serverFrame(ppp.LCPConfigureRequest, 0x22, request))

	code, _, ok := clientReplyTo(t, log, 0x22)
	if !ok {
		t.Fatal("the client did not answer the server's Configure-Request at all")
	}
	if code != ppp.LCPConfigureAck {
		t.Fatalf("the client answered an acceptable Configure-Request with %s, want Configure-Ack", ppp.LCPCodeName(code))
	}
}

// RFC requirement: RFC2516-7-2 positive -- the client rejects ACCM, ACFC and FCS Alternatives and proposes none of them.
// RFC requirement: RFC2516-7-2 negative -- a request carrying only MRU and PFC still receives Configure-Ack.
// MUTATION: remove PPPoE from clientLCPPolicy to let the client acknowledge ACCM and ACFC.
func TestRFC2516ClientRejectsForbiddenLCPOptions(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"ACCM", []byte{ppp.LCPOptACCM, 6, 0xff, 0xff, 0xff, 0xff}},
		{"ACFC", []byte{ppp.LCPOptACFC, 2}},
		{"FCS Alternatives", []byte{9, 3, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log := runClientLCP(t, serverFrame(ppp.LCPConfigureRequest, 0x73, tc.data))
			code, opts, ok := clientReplyTo(t, log, 0x73)
			if !ok || code != ppp.LCPConfigureReject {
				t.Fatalf("reply = %d, present %v; want Configure-Reject", code, ok)
			}
			if len(opts) != 1 || opts[0].Type != tc.data[0] {
				t.Fatalf("rejected options = %+v, want option %d", opts, tc.data[0])
			}
			if string(opts[0].Data) != string(tc.data[2:]) {
				t.Fatalf("rejected data = %x, want %x", opts[0].Data, tc.data[2:])
			}
			foundRequest := false
			for _, packet := range log.lcpPackets(t) {
				if packet.Code != ppp.LCPConfigureRequest {
					continue
				}
				foundRequest = true
				local, err := ppp.ParseLCPOptions(packet.Data)
				if err != nil {
					t.Fatal(err)
				}
				for _, opt := range local {
					switch opt.Type {
					case ppp.LCPOptACCM, ppp.LCPOptACFC, 9:
						t.Fatalf("client requested forbidden option %d", opt.Type)
					}
				}
			}
			if !foundRequest {
				t.Fatal("client sent no Configure-Request")
			}
		})
	}
	log := runClientLCP(t, serverFrame(ppp.LCPConfigureRequest, 0x74,
		[]byte{ppp.LCPOptMRU, 4, 5, 0xd4, ppp.LCPOptPFC, 2}))
	code, _, ok := clientReplyTo(t, log, 0x74)
	if !ok || code != ppp.LCPConfigureAck {
		t.Fatalf("MRU/PFC reply = %d, present %v; want Configure-Ack", code, ok)
	}
}
