// VALIDATES: the PPP client's PAP result handling is driven solely by the
// Authenticate-Ack/Nak Code field -- an Ack (Code 2) means success and a Nak
// (Code 3) means failure regardless of any human-readable Message the server
// appends, per RFC 1334 Section 2.3 ("Message ... MUST NOT affect operation").
// PREVENTS: a regression where runClientAuth starts inspecting the advisory
// Message field and lets its content flip a Nak into success (or an Ack into
// failure).

package pppoeclient

import (
	"io"
	"log/slog"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// discardRWC is a no-op io.ReadWriteCloser: runClientAuth only writes to its
// channel argument (the PAP Authenticate-Request and any echo replies), so
// writes are discarded and reads report EOF.
type discardRWC struct{}

func (discardRWC) Read([]byte) (int, error)    { return 0, io.EOF }
func (discardRWC) Write(p []byte) (int, error) { return len(p), nil }
func (discardRWC) Close() error                { return nil }

// papReplyFrame builds a PPP frame carrying a PAP Authenticate-Ack (code 2) or
// Authenticate-Nak (code 3) with the given Identifier and a Message. The
// Message is deliberately non-empty so the test proves it is ignored, not that
// it happens to be absent.
func papReplyFrame(t *testing.T, code, id uint8, message string) []byte {
	t.Helper()
	var pap [64]byte
	var n int
	switch code {
	case ppp.PAPAuthenticateAck:
		n = ppp.WritePAPAck(pap[:], 0, id, []byte(message))
	case ppp.PAPAuthenticateNak:
		n = ppp.WritePAPNak(pap[:], 0, id, []byte(message))
	default:
		t.Fatalf("papReplyFrame: unsupported code %d", code)
	}
	out := make([]byte, ppp.MaxFrameLen)
	off := ppp.WriteFrame(out, 0, ppp.ProtoPAP, pap[:n])
	return out[:off]
}

// RFC requirement: RFC1334-2.3-4 positive -- a PAP Authenticate-Ack (Code 2)
// carrying a non-empty Message still yields auth success: runClientAuth
// branches only on pkt.Code and never reads the Message field (producer
// internal/component/l2tp/pppoeclient/session.go:285-288).
// RFC requirement: RFC1334-2.3-4 negative -- a PAP Authenticate-Nak (Code 3)
// carrying a Message still yields auth failure (session.go:289-290); the same
// Message text on both codes yields opposite outcomes, proving the Message does
// not change the Code-driven outcome.
func TestPAPReplyMessageDoesNotAffectOutcome(t *testing.T) {
	cases := []struct {
		name    string
		code    uint8
		message string
		wantErr bool
	}{
		{"ack with message succeeds", ppp.PAPAuthenticateAck, "welcome aboard", false},
		{"nak with message fails", ppp.PAPAuthenticateNak, "invalid credentials", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			frames := make(chan readFrame, 1)
			frames <- readFrame{data: papReplyFrame(t, tc.code, 1, tc.message)}

			buf := make([]byte, ppp.MaxFrameLen)
			lcp := lcpResult{authProto: ppp.ProtoPAP}
			cfg := sessionConfig{username: "user", password: "pass"}
			stopCh := make(chan struct{})
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))

			err := runClientAuth(discardRWC{}, frames, buf, lcp, cfg, 0, stopCh, logger)

			if tc.wantErr && err == nil {
				t.Fatalf("runClientAuth returned nil, want failure for a Nak bearing message %q", tc.message)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("runClientAuth returned %v, want success for an Ack bearing message %q", err, tc.message)
			}
		})
	}
}
