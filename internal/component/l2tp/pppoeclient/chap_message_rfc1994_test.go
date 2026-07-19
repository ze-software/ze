// VALIDATES: the PPP client's CHAP result handling is driven solely by the
// Success/Failure Code field -- a Success (Code 3) means authentication succeeded
// regardless of the advisory human-readable Message the authenticator appends,
// per RFC 1994 Section 4.2 ("Message ... MUST NOT affect operation of the
// protocol").
// PREVENTS: a regression where runClientAuth inspects the advisory CHAP Message
// field and lets its content change the Code-driven outcome.

package pppoeclient

import (
	"encoding/binary"
	"io"
	"log/slog"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/l2tp/ppp"
)

// chapReplyFrame builds a PPP frame carrying a CHAP Success (code 3) or Failure
// (code 4) with the given Identifier and a Message. RFC 1994 Section 4.2:
// Success/Failure has NO Msg-Length octet -- the Message runs from byte 4 to
// Length. The Message is deliberately non-empty so the test proves it is
// ignored, not that it happens to be absent.
func chapReplyFrame(t *testing.T, code, id uint8, message string) []byte {
	t.Helper()
	msg := []byte(message)
	total := 4 + len(msg)
	chap := make([]byte, total)
	chap[0] = code
	chap[1] = id
	binary.BigEndian.PutUint16(chap[2:4], uint16(total)) //nolint:gosec // total bounded by a short test message
	copy(chap[4:], msg)
	out := make([]byte, ppp.MaxFrameLen)
	off := ppp.WriteFrame(out, 0, ppp.ProtoCHAP, chap)
	return out[:off]
}

// RFC requirement: RFC1994-4.2-4 positive -- two CHAP Success frames (Code 3)
// whose Message payloads differ both resolve authentication to the same
// successful outcome (nil error): runClientAuth branches only on pkt.Code and
// never reads the Message field (producer
// internal/component/l2tp/pppoeclient/session.go:270-274), so the advisory
// Message does not affect protocol operation.
func TestCHAPSuccessMessageDoesNotAffectOutcome(t *testing.T) {
	cases := []struct {
		name    string
		message string
	}{
		{"plain ascii message", "welcome aboard"},
		{"different non-ascii message", "\x00\x01 different bytes \xfe\xff"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			frames := make(chan readFrame, 1)
			frames <- readFrame{data: chapReplyFrame(t, 3, 0x42, tc.message)}

			buf := make([]byte, ppp.MaxFrameLen)
			lcp := lcpResult{authProto: ppp.ProtoCHAP}
			cfg := sessionConfig{username: "user", password: "pass"}
			stopCh := make(chan struct{})
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))

			err := runClientAuth(discardRWC{}, frames, buf, lcp, cfg, 0, stopCh, logger)
			if err != nil {
				t.Fatalf("runClientAuth returned %v, want success for a CHAP Success bearing message %q (Message must not affect operation)", err, tc.message)
			}
		})
	}
}
