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
	"strings"
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
// Authenticate-Nak (code 3) with the given Identifier and a Message, which
// may be empty.
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

// The cases hold the Code fixed and vary only the Message: three Acks and
// three Naks carry the same three Messages, one empty, one that reads as
// success and one that reads as failure. A Nak must end the run with its own
// rejection error, because an ignored Nak also ends in an error once the
// Authenticate-Request retries run out.
//
// RFC requirement: RFC1334-2.3-4 positive -- a PAP Authenticate-Ack (Code 2) whose Message is empty, "welcome aboard" or "invalid credentials" yields auth success from runClientAuth every time: with the Code fixed, the Message does not change the outcome.
// RFC requirement: RFC1334-2.3-4 negative -- a PAP Authenticate-Nak (Code 3) whose Message is empty, "welcome aboard" or "invalid credentials" yields the PAP rejection error from runClientAuth every time: a success-like Message does not turn a Nak into success.
func TestPAPReplyMessageDoesNotAffectOutcome(t *testing.T) {
	cases := []struct {
		name    string
		code    uint8
		message string
		wantErr bool
	}{
		{"ack with empty message succeeds", ppp.PAPAuthenticateAck, "", false},
		{"ack with success message succeeds", ppp.PAPAuthenticateAck, "welcome aboard", false},
		{"ack with failure message succeeds", ppp.PAPAuthenticateAck, "invalid credentials", false},
		{"nak with empty message fails", ppp.PAPAuthenticateNak, "", true},
		{"nak with success message fails", ppp.PAPAuthenticateNak, "welcome aboard", true},
		{"nak with failure message fails", ppp.PAPAuthenticateNak, "invalid credentials", true},
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

			if tc.wantErr && (err == nil || !strings.Contains(err.Error(), "PAP auth rejected")) {
				t.Fatalf("runClientAuth returned %v, want the PAP rejection for a Nak bearing message %q", err, tc.message)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("runClientAuth returned %v, want success for an Ack bearing message %q", err, tc.message)
			}
		})
	}
}
