// Design: docs/architecture/testing/interop.md -- RFC 1661 Code-Reject wire evidence.
// Related: ppp_code_reject.go -- the predicates under test.
package l2tp

import (
	"encoding/binary"
	"testing"
)

// TestPPPCodeRejectEvidence drives every predicate of checkPPPCodeReject over a
// relay log built here: the conforming exchange passes, and each wrong reading
// the checker exists to refuse fails the predicate that owns it, and only that
// one.
func TestPPPCodeRejectEvidence(t *testing.T) {
	// rejects holds Ze's two LCP Code-Rejects, then its two IPCP ones.
	type exchange struct {
		probes  [4][]byte
		rejects [4][]byte
		extra   []string
	}
	conforming := func() exchange {
		var e exchange
		for index, p := range []struct {
			protocol uint16
			code     byte
		}{{protocolLCP, probeCodeLCP}, {protocolLCP, probeCodeLCP}, {protocolIPCP, probeCodeIPCP}, {protocolIPCP, probeCodeIPCP}} {
			e.probes[index] = pppDatagram(p.protocol, codeRejectProbe(p.code))
		}
		for index := range 2 {
			e.rejects[index] = pppDatagram(protocolLCP, codeReject(byte(index+1), codeRejectProbe(probeCodeLCP), pppCodeRejectMRU))
			e.rejects[2+index] = pppDatagram(protocolIPCP, codeReject(byte(index+1), codeRejectProbe(probeCodeIPCP), pppCodeRejectMRU))
		}
		return e
	}
	log := func(e exchange) string {
		text := ""
		for _, probe := range e.probes {
			text += wireLine(t, relayInjected, nil, probe)
		}
		for _, reject := range e.rejects {
			text += wireLine(t, relayFromZe, reject, reject)
		}
		for _, line := range e.extra {
			text += line
		}
		return text
	}
	type verdicts struct{ probes, identifiers, mru, kept bool }
	judge := func(t *testing.T, text string) verdicts {
		t.Helper()
		frames, err := relayPPPFrames(text)
		if err != nil {
			t.Fatal(err)
		}
		// As checkPPPCodeReject judges them: each verdict holds for both protocols.
		lcp, ipcp := zeCodeRejects(frames, protocolLCP), zeCodeRejects(frames, protocolIPCP)
		return verdicts{
			probes:      injectedProbes(frames) == nil,
			identifiers: distinctIdentifiers(lcp) == nil && distinctIdentifiers(ipcp) == nil,
			mru:         rejectsWithinMRU(lcp, probeCodeLCP) == nil && rejectsWithinMRU(ipcp, probeCodeIPCP) == nil,
			kept:        sessionKept(frames) == nil,
		}
	}
	for _, tc := range []struct {
		name   string
		change func(*exchange)
		want   verdicts
	}{
		{name: "conforming", want: verdicts{true, true, true, true}},
		{name: "Identifier copied from the probe", change: func(e *exchange) {
			for index := range 2 {
				e.rejects[index] = pppDatagram(protocolLCP, codeReject(probeIdentifier, codeRejectProbe(probeCodeLCP), pppCodeRejectMRU))
			}
		}, want: verdicts{true, false, true, true}},
		{name: "IPCP Identifier copied from the probe", change: func(e *exchange) {
			for index := 2; index < 4; index++ {
				e.rejects[index] = pppDatagram(protocolIPCP, codeReject(probeIdentifier, codeRejectProbe(probeCodeIPCP), pppCodeRejectMRU))
			}
		}, want: verdicts{true, false, true, true}},
		{name: "IPCP Rejected-Packet not cut", change: func(e *exchange) {
			e.rejects[3] = pppDatagram(protocolIPCP, codeReject(2, codeRejectProbe(probeCodeIPCP), 4+probeOctets))
		}, want: verdicts{true, true, false, true}},
		{name: "IPCP probes unanswered, the unit descriptor unread", change: func(e *exchange) {
			e.rejects[2] = pppDatagram(protocolLCP, []byte{9, 1, 0, 4})
			e.rejects[3] = pppDatagram(protocolLCP, []byte{9, 2, 0, 4})
		}, want: verdicts{true, false, false, false}},
		{name: "Rejected-Packet not cut", change: func(e *exchange) {
			e.rejects[0] = pppDatagram(protocolLCP, codeReject(1, codeRejectProbe(probeCodeLCP), 4+probeOctets))
		}, want: verdicts{true, true, false, true}},
		{name: "Rejected-Packet Length rewritten to the cut", change: func(e *exchange) {
			reject := codeReject(1, codeRejectProbe(probeCodeLCP), pppCodeRejectMRU)
			binary.BigEndian.PutUint16(reject[6:], uint16(len(reject)-4))
			e.rejects[0] = pppDatagram(protocolLCP, reject)
		}, want: verdicts{true, true, false, true}},
		{name: "Rejected-Packet Code of another probe", change: func(e *exchange) {
			e.rejects[1] = pppDatagram(protocolLCP, codeReject(2, codeRejectProbe(probeCodeIPCP), pppCodeRejectMRU))
		}, want: verdicts{true, true, false, true}},
		{name: "one Code-Reject missing", change: func(e *exchange) {
			e.rejects[1] = pppDatagram(protocolLCP, []byte{9, 1, 0, 4})
		}, want: verdicts{true, false, true, false}},
		{name: "a Code-Reject sent on IPCP is not an LCP one", change: func(e *exchange) {
			// It also joins the IPCP answers, where its Rejected-Packet holds
			// an LCP probe's Code, so the IPCP MRU judgement refuses it too.
			e.rejects[1] = pppDatagram(protocolIPCP, codeReject(2, codeRejectProbe(probeCodeLCP), pppCodeRejectMRU))
		}, want: verdicts{true, false, false, false}},
		{name: "a probe never sent", change: func(e *exchange) {
			e.probes[3] = pppDatagram(protocolLCP, []byte{9, 1, 0, 4})
		}, want: verdicts{false, true, true, true}},
		{name: "ze renegotiated LCP", change: func(e *exchange) {
			request := pppDatagram(protocolLCP, []byte{codeConfigureRequest, 9, 0, 4})
			e.extra = append(e.extra, wireLine(t, relayFromZe, request, request))
		}, want: verdicts{true, true, true, false}},
		{name: "pppd terminated IPCP", change: func(e *exchange) {
			request := pppDatagram(protocolIPCP, []byte{5, 9, 0, 4})
			e.extra = append(e.extra, wireLine(t, relayFromLAC, request, request))
		}, want: verdicts{true, true, true, false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := conforming()
			if tc.change != nil {
				tc.change(&e)
			}
			if got := judge(t, log(e)); got != tc.want {
				t.Fatalf("verdicts %+v, want %+v", got, tc.want)
			}
		})
	}
	ip := pppDatagram(0x0021, []byte{0x45, 0, 0, 20})
	frames, err := relayPPPFrames(wireLine(t, relayFromLAC, ip, ip))
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 0 {
		t.Fatalf("an IPv4 datagram was read as a PPP control frame: %+v", frames)
	}
	short := []byte{0x00, 0x02, 0, 7}
	if _, err := relayPPPFrames(wireLine(t, relayFromZe, short, short)); err == nil {
		t.Fatal("an unreadable data datagram was passed over")
	}
	if _, err := relayPPPFrames("{not json"); err == nil {
		t.Fatal("an unreadable relay log was accepted")
	}
	if err := rejectsWithinMRU(nil, probeCodeLCP); err == nil {
		t.Fatal("no Code-Reject at all was measured as within the MRU")
	}
}

// TestPPPTerminateEvidence drives the assertion 6 predicates: the relay's
// fifth injected frame is the IPCP Terminate-Request, and Ze's answer is an
// IPCP Terminate-Ack carrying its Identifier. Each wrong reading fails.
func TestPPPTerminateEvidence(t *testing.T) {
	request := pppDatagram(protocolIPCP, []byte{codeTerminateRequest, terminateIdentifier, 0, 4})
	probes := ""
	for range 4 {
		probe := pppDatagram(protocolLCP, codeRejectProbe(probeCodeLCP))
		probes += wireLine(t, relayInjected, nil, probe)
	}
	frames := func(t *testing.T, text string) []pppFrame {
		t.Helper()
		parsed, err := relayPPPFrames(text)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}
	ack := func(protocol uint16, id byte) string {
		datagram := pppDatagram(protocol, []byte{codeTerminateAck, id, 0, 4})
		return wireLine(t, relayFromZe, datagram, datagram)
	}
	sent := probes + wireLine(t, relayInjected, nil, request)

	if err := injectedTerminate(frames(t, sent)); err != nil {
		t.Fatalf("the Terminate-Request after four probes was refused: %v", err)
	}
	if err := injectedTerminate(frames(t, probes)); err == nil {
		t.Fatal("four probes and no Terminate-Request were read as the injection")
	}
	lcpRequest := pppDatagram(protocolLCP, []byte{codeTerminateRequest, terminateIdentifier, 0, 4})
	if err := injectedTerminate(frames(t, probes+wireLine(t, relayInjected, nil, lcpRequest))); err == nil {
		t.Fatal("an LCP Terminate-Request was read as the IPCP one")
	}

	if err := zeTerminateAck(frames(t, sent+ack(protocolIPCP, terminateIdentifier))); err != nil {
		t.Fatalf("Ze's IPCP Terminate-Ack was refused: %v", err)
	}
	if err := zeTerminateAck(frames(t, sent)); err == nil {
		t.Fatal("no answer was read as a Terminate-Ack")
	}
	if err := zeTerminateAck(frames(t, sent+ack(protocolIPCP, terminateIdentifier+1))); err == nil {
		t.Fatal("a Terminate-Ack with another Identifier was accepted")
	}
	if err := zeTerminateAck(frames(t, sent+ack(protocolLCP, terminateIdentifier))); err == nil {
		t.Fatal("an LCP Terminate-Ack was accepted for the IPCP request")
	}
}

// TestPPPInformationReadsEveryHeaderForm reads the PPP frame behind each
// optional L2TP data header field, so a sequenced or offset header Ze sends
// is decoded rather than misread.
func TestPPPInformationReadsEveryHeaderForm(t *testing.T) {
	ppp := []byte{0xff, 0x03, 0xc0, 0x21, 7, 1, 0, 4}
	for _, header := range [][]byte{
		{0x00, 0x02, 0, 7, 0, 9},
		{0x40, 0x02, 0, 16, 0, 7, 0, 9},
		{0x08, 0x02, 0, 7, 0, 9, 0, 1, 0, 2},
		{0x02, 0x02, 0, 7, 0, 9, 0, 1, 0},
	} {
		protocol, packet, err := pppInformation(append(append([]byte(nil), header...), ppp...))
		if err != nil {
			t.Fatalf("%x: %v", header, err)
		}
		if protocol != protocolLCP || len(packet) != 4 || packet[0] != codeCodeReject {
			t.Fatalf("%x: protocol %04x packet %x", header, protocol, packet)
		}
	}
	if _, _, err := pppInformation([]byte{0x00, 0x02, 0, 7, 0, 9, 0xfe, 0x03, 0xc0, 0x21}); err == nil {
		t.Fatal("a frame without the PPP Address octet was read")
	}
}

// codeRejectProbe is the 204-octet probe the relay sends, Data octet i holding i.
func codeRejectProbe(code byte) []byte {
	packet := make([]byte, probeOctets)
	packet[0] = code
	packet[1] = probeIdentifier
	binary.BigEndian.PutUint16(packet[2:], probeOctets)
	for index := range packet[4:] {
		packet[4+index] = byte(index)
	}
	return packet
}

// codeReject is a Code-Reject of rejected with Identifier id, cut so its
// Information field is at most limit octets, rejected's own Length kept.
func codeReject(id byte, rejected []byte, limit int) []byte {
	copied := min(len(rejected), limit-4)
	packet := make([]byte, 4+copied)
	packet[0] = codeCodeReject
	packet[1] = id
	binary.BigEndian.PutUint16(packet[2:], uint16(len(packet)))
	copy(packet[4:], rejected[:copied])
	return packet
}

// pppDatagram wraps packet as an L2TP data message on tunnel 7, session 9.
func pppDatagram(protocol uint16, packet []byte) []byte {
	datagram := []byte{0x00, 0x02, 0, 7, 0, 9, 0xff, 0x03, byte(protocol >> 8), byte(protocol)}
	return append(datagram, packet...)
}
