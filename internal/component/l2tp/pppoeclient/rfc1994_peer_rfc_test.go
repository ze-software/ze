// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- CHAP peer role.
//
// VALIDATES: RFC 1994 Section 4.1 and 4.2 for the peer role the PPPoE client
// fills: every Challenge received makes runClientAuth transmit a CHAP Response
// (Code 2) and nothing else does, and a Failure ends authentication whatever
// its Message says.
// PREVENTS: a client that answers only the first Challenge, answers a CHAP
// packet that is not a Challenge, or lets a Failure's Message turn it into a
// success.

package pppoeclient

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// chapPacketsWritten decodes every CHAP packet the client wrote.
func chapPacketsWritten(t *testing.T, w *recordingRWC) []ppp.LCPPacket {
	t.Helper()
	var out []ppp.LCPPacket
	for _, frame := range w.snapshot() {
		proto, payload, _, err := ppp.ParseFrame(frame)
		if err != nil {
			t.Fatalf("ParseFrame(% x): %v", frame, err)
		}
		if proto != ppp.ProtoCHAP {
			continue
		}
		pkt, err := ppp.ParseLCPPacket(payload)
		if err != nil {
			t.Fatalf("ParseLCPPacket(% x): %v", payload, err)
		}
		out = append(out, pkt)
	}
	return out
}

// runCHAPClient runs runClientAuth over the queued frames with CHAP
// negotiated. The frames channel is closed after the queue, so a run the
// frames do not end returns the channel-closed error.
func runCHAPClient(queued ...readFrame) (*recordingRWC, error) {
	frames := make(chan readFrame, len(queued))
	for _, f := range queued {
		frames <- f
	}
	close(frames)
	w := &recordingRWC{}
	buf := make([]byte, ppp.MaxFrameLen)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := runClientAuth(w, frames, buf, lcpResult{authProto: ppp.ProtoCHAP},
		sessionConfig{username: "user", password: "pass"}, 0, make(chan struct{}), logger)
	return w, err
}

// VALIDATES: two Challenges (Identifier 0x42, then a re-challenge 0x43) make
// the client write two CHAP packets, each with Code 2 and the Identifier of
// the Challenge it answers; the Success for the second then ends the run.
// METHOD: runClientAuth writes to a recording writer, and the test decodes
// every CHAP packet written.
//
// RFC requirement: RFC1994-4.1-4 positive -- for each of two received Challenges (Identifier 0x42, then 0x43), runClientAuth transmits one CHAP packet with Code 2 (Response) carrying that Identifier.
func TestRFC1994PeerTransmitsResponseForEveryChallenge(t *testing.T) {
	w, err := runCHAPClient(
		chapChallengeFrame(0x42, []byte{4, 1, 2, 3, 4}),
		chapChallengeFrame(0x43, []byte{4, 5, 6, 7, 8}),
		readFrame{data: chapReplyFrame(t, 3, 0x43, "")},
	)
	if err != nil {
		t.Fatalf("runClientAuth = %v, want success after the Success for 0x43", err)
	}
	got := chapPacketsWritten(t, w)
	if len(got) != 2 {
		t.Fatalf("client wrote %d CHAP packets, want 2 (one per Challenge): %+v", len(got), got)
	}
	for i, wantID := range []uint8{0x42, 0x43} {
		if got[i].Code != 2 {
			t.Errorf("CHAP packet %d Code = %d, want 2 (Response)", i, got[i].Code)
		}
		if got[i].Identifier != wantID {
			t.Errorf("CHAP packet %d Identifier = %#x, want %#x", i, got[i].Identifier, wantID)
		}
	}
}

// VALIDATES: CHAP packets that are not a Challenge (a reflected Response,
// Code 2, and an unassigned Code 5) make the client transmit nothing.
// METHOD: the same recording run; the queue holds no Challenge, so the run
// ends on the closed channel with nothing written.
//
// RFC requirement: RFC1994-4.1-4 negative -- CHAP packets with Code 2 and Code 5 (not a Challenge) make runClientAuth transmit no CHAP packet, so a Response is sent only when a Challenge is received.
func TestRFC1994PeerSendsNoResponseWithoutChallenge(t *testing.T) {
	notChallenge := func(code, id uint8) readFrame {
		var buf [ppp.MaxFrameBufLen]byte
		off := ppp.WriteFrame(buf[:], 0, ppp.ProtoCHAP, nil)
		off += ppp.WriteLCPPacket(buf[:], off, code, id, []byte{4, 1, 2, 3, 4})
		return readFrame{data: buf[:off]}
	}
	w, err := runCHAPClient(notChallenge(2, 0x42), notChallenge(5, 0x43))
	if err == nil {
		t.Fatal("runClientAuth succeeded with no Challenge and no Success")
	}
	if got := chapPacketsWritten(t, w); len(got) != 0 {
		t.Fatalf("client wrote CHAP packets %+v for packets that are not a Challenge", got)
	}
}

// VALIDATES: a Failure (Code 4) ends authentication with an error whatever
// its Message: empty, one that reads as success, and one with non-ASCII
// bytes. The same Message on a Success (Code 3) completes authentication, so
// with the Message held fixed only the Code decides the outcome.
// METHOD: one Challenge/Response exchange, then the reply under test.
//
// RFC requirement: RFC1994-4.2-4 positive -- a CHAP Failure whose Message is empty, "authentication succeeded", or non-ASCII bytes makes runClientAuth return an error, and a Success carrying the same Message returns success: the Message does not change the Code-driven outcome.
func TestRFC1994FailureMessageDoesNotAffectOutcome(t *testing.T) {
	for _, message := range []string{"", "authentication succeeded", "\x00\x03 \xfe\xff"} {
		t.Run(message, func(t *testing.T) {
			_, err := runCHAPClient(
				chapChallengeFrame(0x42, []byte{4, 1, 2, 3, 4}),
				readFrame{data: chapReplyFrame(t, 4, 0x42, message)},
			)
			// The run would also end in an error on the closed channel if the
			// Failure were ignored, so the error must be the Failure's own.
			if err == nil || !strings.Contains(err.Error(), "CHAP auth failed") {
				t.Fatalf("CHAP Failure with Message %q = %v, want the CHAP auth failure", message, err)
			}
			_, err = runCHAPClient(
				chapChallengeFrame(0x42, []byte{4, 1, 2, 3, 4}),
				readFrame{data: chapReplyFrame(t, 3, 0x42, message)},
			)
			if err != nil {
				t.Fatalf("CHAP Success with Message %q = %v, want success", message, err)
			}
		})
	}
}
