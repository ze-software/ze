// Design: docs/research/l2tpv2-ze-integration.md -- original options after Max-Failure.

package ppp

import (
	"bytes"
	"testing"
	"time"
)

// TestLCPMaxFailureRejectsOnlyTheNakInstance exhausts Max-Failure, then
// compares the complete reply with the original instance that earned the Nak.
// An acceptable duplicate or an acceptable instance before a malformed duplicate
// must not become part of the Reject merely because its Type matches.
func TestLCPMaxFailureRejectsOnlyTheNakInstance(t *testing.T) {
	bad := optStream(mruOption(2000))
	good := optStream(mruOption(1400))
	malformed := []byte{LCPOptMRU, 1}
	for _, tc := range []struct {
		name    string
		request []byte
		want    []byte
	}{
		{"bad then acceptable duplicate", append(append([]byte{}, bad...), good...), bad},
		{"acceptable then malformed duplicate", append(append([]byte{}, good...), malformed...), malformed},
		{"bad then malformed duplicate", append(append([]byte{}, bad...), malformed...), bad},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, rec, _ := configuredLCPSession(t, LCPStateReqSent, time.Second, 2, 10, 1)
			if s.handleLCPPacket(LCPPacket{Code: LCPConfigureRequest, Identifier: 1, Data: bad}) {
				t.Fatal("first request ended LCP")
			}
			if s.handleLCPPacket(LCPPacket{Code: LCPConfigureRequest, Identifier: 2, Data: tc.request}) {
				t.Fatal("threshold request ended LCP")
			}
			frames := decodeFrames(t, rec)
			if len(frames) != 2 {
				t.Fatalf("replies = %d, want 2", len(frames))
			}
			if frames[0].Pkt.Code != LCPConfigureNak {
				t.Fatal("budget was not spent by a Nak")
			}
			reply := frames[1].Pkt
			if reply.Code != LCPConfigureReject {
				t.Fatalf("reply code = %d, want Configure-Reject", reply.Code)
			}
			if reply.Identifier != 2 {
				t.Fatalf("reply identifier = %d, want 2", reply.Identifier)
			}
			if !bytes.Equal(reply.Data, tc.want) {
				t.Fatalf("Reject options = %x, want original instance %x", reply.Data, tc.want)
			}
		})
	}
}

// TestLCPMaxFailureStillAcknowledgesAcceptableOptions contrasts the exhausted
// failure budget with a request whose options all remain acceptable.
func TestLCPMaxFailureStillAcknowledgesAcceptableOptions(t *testing.T) {
	s, rec, _ := configuredLCPSession(t, LCPStateReqSent, time.Second, 2, 10, 1)
	s.handleLCPPacket(LCPPacket{Code: LCPConfigureRequest, Identifier: 1, Data: optStream(mruOption(2000))})
	good := optStream(mruOption(1400))
	s.handleLCPPacket(LCPPacket{Code: LCPConfigureRequest, Identifier: 2, Data: good})
	frames := decodeFrames(t, rec)
	if len(frames) != 2 {
		t.Fatalf("replies = %d, want 2", len(frames))
	}
	if frames[1].Pkt.Code != LCPConfigureAck {
		t.Fatalf("acceptable options received code %d", frames[1].Pkt.Code)
	}
	if !bytes.Equal(frames[1].Pkt.Data, good) {
		t.Fatalf("Ack options = %x, want %x", frames[1].Pkt.Data, good)
	}
}
