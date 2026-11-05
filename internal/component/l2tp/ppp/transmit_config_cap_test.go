// Design: docs/guide/l2tp.md -- complete configuration replies fit the admitted transmit cap.

package ppp

import (
	"bytes"
	"testing"
)

// Unmodified option reflection must either fit in full or fail the session.
// A fresh equivalent session proves that each fitting Ack/Reject still works.
func TestConfigureRepliesRespectTransmitCap(t *testing.T) {
	for _, kind := range []string{"lcp Ack", "ipcp Ack", "ipv6cp Ack", "lcp Reject", "ipcp Reject", "ipv6cp Reject"} {
		t.Run(kind, func(t *testing.T) {
			s, rec, events := newRFC1661Session(LCPStateOpened)
			s.maxMRU = MinIPv4MRU
			s.negotiatedMRU = 1500
			send := s.sendConfigureAck
			proto := uint16(ProtoLCP)
			code := uint8(LCPConfigureAck)
			switch kind {
			case "lcp Ack":
			case "ipcp Ack":
				proto = ProtoIPCP
				send = func(req LCPPacket) bool { return s.sendNCPConfigureAck(AddressFamilyIPv4, req) }
			case "ipv6cp Ack":
				proto = ProtoIPv6CP
				send = func(req LCPPacket) bool { return s.sendNCPConfigureAck(AddressFamilyIPv6, req) }
			case "lcp Reject":
				code = LCPConfigureReject
				send = s.sendConfigureNakOrReject
			case "ipcp Reject":
				proto, code = ProtoIPCP, LCPConfigureReject
				send = func(req LCPPacket) bool { return s.sendNCPConfigureNakOrReject(AddressFamilyIPv4, req) }
			case "ipv6cp Reject":
				proto, code = ProtoIPv6CP, LCPConfigureReject
				send = func(req LCPPacket) bool { return s.sendNCPConfigureNakOrReject(AddressFamilyIPv6, req) }
			}
			oversized := append([]byte{250, 66}, bytes.Repeat([]byte{0x5a}, 64)...)
			if send(LCPPacket{Code: LCPConfigureRequest, Identifier: 0x31, Data: oversized}) {
				t.Fatal("unrepresentable complete option set reported a successful send")
			}
			if rec.count() != 0 {
				t.Fatal("oversized option set was transmitted or partly acknowledged")
			}
			if len(events) != 1 {
				t.Fatalf("unrepresentable reply did not publish one failure: %d events", len(events))
			}
			if _, ok := (<-events).(EventSessionDown); !ok {
				t.Fatal("unrepresentable reply did not publish a session-down result")
			}
			valid, good, _ := newRFC1661Session(LCPStateOpened)
			valid.maxMRU = MinIPv4MRU
			valid.negotiatedMRU = 1500
			switch kind {
			case "lcp Ack":
				send = valid.sendConfigureAck
			case "ipcp Ack":
				send = func(req LCPPacket) bool { return valid.sendNCPConfigureAck(AddressFamilyIPv4, req) }
			case "ipv6cp Ack":
				send = func(req LCPPacket) bool { return valid.sendNCPConfigureAck(AddressFamilyIPv6, req) }
			case "lcp Reject":
				send = valid.sendConfigureNakOrReject
			case "ipcp Reject":
				send = func(req LCPPacket) bool { return valid.sendNCPConfigureNakOrReject(AddressFamilyIPv4, req) }
			case "ipv6cp Reject":
				send = func(req LCPPacket) bool { return valid.sendNCPConfigureNakOrReject(AddressFamilyIPv6, req) }
			}
			fitting := []byte{250, 2}
			if !send(LCPPacket{Code: LCPConfigureRequest, Identifier: 0x32, Data: fitting}) {
				t.Fatal("fitting complete option set was refused")
			}
			got := decodeFrames(t, good)
			if len(got) != 1 || got[0].Proto != proto || got[0].Pkt.Code != code || got[0].Pkt.Identifier != 0x32 || !bytes.Equal(got[0].Pkt.Data, fitting) {
				t.Fatalf("fitting options did not survive unchanged: %+v", got)
			}
		})
	}
}

func TestWriteFrameRefusesMissedTransmitCap(t *testing.T) {
	s, rec, events := newRFC1661Session(LCPStateOpened)
	s.maxMRU = MinIPv4MRU
	s.negotiatedMRU = 1500
	frame := lcpFrame(ProtoLCP, LCPConfigureAck, 0x41, bytes.Repeat([]byte{0x5a}, 65))
	if s.writeFrame(frame) || rec.count() != 0 {
		t.Fatal("final writer boundary transmitted an oversized complete frame")
	}
	if len(events) != 1 {
		t.Fatalf("writer refusal did not publish one failure: %d events", len(events))
	}
	if _, ok := (<-events).(EventSessionDown); !ok {
		t.Fatal("writer refusal did not publish a session-down result")
	}
	valid, good, _ := newRFC1661Session(LCPStateOpened)
	valid.maxMRU = MinIPv4MRU
	valid.negotiatedMRU = 1500
	frame = lcpFrame(ProtoLCP, LCPConfigureAck, 0x42, bytes.Repeat([]byte{0x5a}, 64))
	if !valid.writeFrame(frame) || good.count() != 1 || !bytes.Equal(good.all()[0], frame) {
		t.Fatal("final writer boundary changed or suppressed a fitting complete frame")
	}
}
