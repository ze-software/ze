// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- CHAP result correlation.
package pppoeclient

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

func chapChallengeFrame(id uint8, data []byte) readFrame {
	var buf [ppp.MaxFrameBufLen]byte
	off := ppp.WriteFrame(buf[:], 0, ppp.ProtoCHAP, nil)
	off += ppp.WriteLCPPacket(buf[:], off, 1, id, data)
	return readFrame{data: buf[:off]}
}

// TestClientCHAPReplyCorrelation requires a successfully written Response
// before a matching Success or Failure can complete authentication.
func TestClientCHAPReplyCorrelation(t *testing.T) {
	for _, code := range []uint8{3, 4} {
		synctest.Test(t, func(t *testing.T) {
			w := &recordingRWC{}
			frames := make(chan readFrame)
			stop := make(chan struct{})
			defer close(stop)
			done := make(chan error, 1)
			go func() {
				var buf [ppp.MaxFrameBufLen]byte
				done <- runClientAuth(w, frames, buf[:], lcpResult{authProto: ppp.ProtoCHAP},
					sessionConfig{username: "alice", password: "secret"}, 1, stop, slog.Default())
			}()
			assertWaiting := func(frame readFrame) {
				t.Helper()
				frames <- frame
				synctest.Wait()
				select {
				case err := <-done:
					t.Fatalf("unmatched exchange completed authentication: %v", err)
				default:
				}
			}
			assertWaiting(readFrame{data: chapReplyFrame(t, code, 42, "premature")})
			assertWaiting(chapChallengeFrame(42, []byte{4, 1}))
			assertWaiting(chapChallengeFrame(42, []byte{0}))
			if len(w.frames) != 0 {
				t.Fatalf("malformed challenge produced response: % x", w.frames)
			}
			assertWaiting(chapChallengeFrame(42, []byte{4, 1, 2, 3, 4}))
			if len(w.frames) != 1 {
				t.Fatalf("valid challenge produced %d responses, want 1", len(w.frames))
			}
			assertWaiting(readFrame{data: chapReplyFrame(t, code, 41, "stale")})
			frames <- readFrame{data: chapReplyFrame(t, code, 42, "current")}
			err := <-done
			if (err == nil) != (code == 3) {
				t.Fatalf("result code %d returned %v", code, err)
			}
		})
	}
}

// TestClientCHAPWriteFailure refuses to wait for a result to an unsent Response.
func TestClientCHAPWriteFailure(t *testing.T) {
	for _, tc := range []struct{ writeErr, want error }{
		{io.ErrClosedPipe, io.ErrClosedPipe},
		{nil, io.ErrShortWrite},
	} {
		frames := make(chan readFrame, 1)
		frames <- chapChallengeFrame(42, []byte{4, 1, 2, 3, 4})
		w := &failingAuthWriter{err: tc.writeErr}
		var buf [ppp.MaxFrameBufLen]byte
		err := runClientAuth(w, frames, buf[:], lcpResult{authProto: ppp.ProtoCHAP},
			sessionConfig{username: "alice", password: "secret"}, 1, make(chan struct{}), slog.Default())
		if !errors.Is(err, tc.want) {
			t.Fatalf("response write error = %v, want %v", err, tc.want)
		}
	}
}
