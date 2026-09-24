// Design: docs/architecture/l2tp/bng-5-pppoe.md -- RFC 1334 conformance coverage
//
// Proves RFC 1334 Section 2.2.1 on the peer side the PPPoE client fills: the
// Authentication phase opens with a PAP Authenticate-Request when PAP was
// negotiated, and with no PAP packet otherwise.

package pppoeclient

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// recordingRWC records every frame runClientAuth writes and reads EOF.
type recordingRWC struct{ frameLog }

func (*recordingRWC) Read([]byte) (int, error) { return 0, io.EOF }
func (*recordingRWC) Close() error             { return nil }

// papPackets decodes every PAP packet the client wrote.
func (f *frameLog) papPackets(t *testing.T) []ppp.LCPPacket {
	t.Helper()
	var out []ppp.LCPPacket
	for _, frame := range f.frames {
		proto, payload, _, err := ppp.ParseFrame(frame)
		if err != nil {
			t.Fatalf("ParseFrame(% x): %v", frame, err)
		}
		if proto != ppp.ProtoPAP {
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

// TestClientOpensAuthPhaseWithPAPRequest runs the client's Authentication
// phase with PAP negotiated and with CHAP negotiated, stops it at once, and
// reads the PAP packets it wrote.
//
// RFC requirement: RFC1334-2.2.1-1 positive — with PAP negotiated, runClientAuth transmits one PAP packet whose Code is 1 (Authenticate-Request) carrying the configured Peer-ID as the phase opens.
// RFC requirement: RFC1334-2.2.1-1 negative — with CHAP negotiated, runClientAuth transmits no PAP packet at all.
func TestClientOpensAuthPhaseWithPAPRequest(t *testing.T) {
	t.Parallel()

	run := func(t *testing.T, authProto uint16) []ppp.LCPPacket {
		t.Helper()
		w := &recordingRWC{}
		frames := make(chan readFrame)
		stopCh := make(chan struct{})
		close(stopCh)
		buf := make([]byte, ppp.MaxFrameLen)
		cfg := sessionConfig{username: "alice", password: "secret"}
		err := runClientAuth(w, frames, buf, lcpResult{authProto: authProto}, cfg, 0x01020304, stopCh, slog.Default())
		if err == nil {
			t.Fatal("runClientAuth returned nil after stop")
		}
		return w.papPackets(t)
	}

	pap := run(t, ppp.ProtoPAP)
	if len(pap) != 1 {
		t.Fatalf("client wrote %d PAP packets with PAP negotiated, want 1 Authenticate-Request", len(pap))
	}
	if pap[0].Code != 1 {
		t.Fatalf("PAP Code = %d, want 1 (Authenticate-Request)", pap[0].Code)
	}
	if len(pap[0].Data) < 1 || int(pap[0].Data[0]) != len("alice") || string(pap[0].Data[1:1+len("alice")]) != "alice" {
		t.Fatalf("Authenticate-Request Data = % x, want Peer-ID-Length 5 then \"alice\"", pap[0].Data)
	}

	if chap := run(t, ppp.ProtoCHAP); len(chap) != 0 {
		t.Fatalf("client wrote %d PAP packets with CHAP negotiated, want none", len(chap))
	}
}

type failingAuthWriter struct {
	recordingRWC
	err error
}

func (w *failingAuthWriter) Write([]byte) (int, error) { return 0, w.err }

// TestClientPAPWriteFailure aborts authentication on a socket error or
// short write, before waiting for a reply to a packet that was never sent.
func TestClientPAPWriteFailure(t *testing.T) {
	for _, tc := range []struct {
		name     string
		writeErr error
		want     error
	}{
		{"socket error", io.ErrClosedPipe, io.ErrClosedPipe},
		{"short write", nil, io.ErrShortWrite},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stopCh := make(chan struct{})
			close(stopCh)
			w := &failingAuthWriter{err: tc.writeErr}
			var buf [ppp.MaxFrameBufLen]byte
			err := runClientAuth(w, make(chan readFrame), buf[:],
				lcpResult{authProto: ppp.ProtoPAP}, sessionConfig{
					username: "alice", password: "secret",
				}, 0x01020304, stopCh, slog.Default())
			if !errors.Is(err, tc.want) {
				t.Fatalf("runClientAuth = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestClientPAPRetriesUntilMatchingReply drops the initial exchange and answers
// the retransmission on the same transport, using virtual time for the timer.
//
// RFC requirement: RFC1334-2.2.1-2 positive -- a lost request or reply causes another Authenticate-Request on the same transport, and a matching Ack completes authentication.
// RFC requirement: RFC1334-2.2.1-2 negative -- a valid matching Ack stops the retry loop; no later request is written.
// RFC requirement: RFC1334-2.2-1 positive -- a retransmitted Authenticate-Request changes its Identifier while preserving credentials.
// RFC requirement: RFC1334-2.2-1 negative -- the second request never reuses the first request's Identifier.
// MUTATION: disable the retry arm or the Identifier increment; this exchange cannot finish or repeats the old Identifier.
func TestClientPAPRetriesUntilMatchingReply(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &recordingRWC{}
		frames := make(chan readFrame)
		stop := make(chan struct{})
		defer close(stop)
		done := make(chan error, 1)
		go func() {
			var buf [ppp.MaxFrameBufLen]byte
			done <- runClientAuth(w, frames, buf[:], lcpResult{authProto: ppp.ProtoPAP},
				sessionConfig{username: "alice", password: "secret"}, 1, stop, slog.Default())
		}()
		synctest.Wait()
		first := w.papPackets(t)
		if len(first) != 1 {
			t.Fatalf("initial requests = %d, want 1", len(first))
		}
		// sleep(timer): advance the production PAP retransmission timer.
		time.Sleep(papRetryInterval)
		synctest.Wait()
		packets := w.papPackets(t)
		if len(packets) != 2 {
			t.Fatalf("requests after loss = %d, want 2", len(packets))
		}
		if packets[1].Code != ppp.PAPAuthenticateRequest {
			t.Fatalf("retry code = %d, want Authenticate-Request", packets[1].Code)
		}
		if packets[0].Identifier == packets[1].Identifier {
			t.Fatal("retry reused the initial Identifier")
		}
		if !bytes.Equal(packets[0].Data, packets[1].Data) {
			t.Fatalf("credentials changed: % x -> % x", packets[0].Data, packets[1].Data)
		}
		frames <- readFrame{data: papReplyFrame(t, ppp.PAPAuthenticateAck, packets[1].Identifier, "")}
		if err := <-done; err != nil {
			t.Fatalf("matching Ack: %v", err)
		}
		// sleep(timer): prove the stopped retry timer emits no further request.
		time.Sleep(papRetryInterval)
		synctest.Wait()
		if got := len(w.papPackets(t)); got != 2 {
			t.Fatalf("requests after Ack = %d, want 2", got)
		}
	})
}

// TestClientPAPDiscardsInvalidReplies feeds malformed and unmatched replies,
// then a stale first-request reply after a retry, before a matching Nak.
//
// MUTATION: remove the Identifier check; the unmatched Ack authenticates the client.
func TestClientPAPDiscardsInvalidReplies(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &recordingRWC{}
		frames := make(chan readFrame)
		stop := make(chan struct{})
		defer close(stop)
		done := make(chan error, 1)
		go func() {
			var buf [ppp.MaxFrameBufLen]byte
			done <- runClientAuth(w, frames, buf[:], lcpResult{authProto: ppp.ProtoPAP},
				sessionConfig{username: "alice", password: "secret"}, 1, stop, slog.Default())
		}()
		synctest.Wait()
		assertWaiting := func(frame []byte) {
			t.Helper()
			frames <- readFrame{data: frame}
			synctest.Wait()
			select {
			case err := <-done:
				t.Fatalf("invalid reply completed authentication: % x, error %v", frame, err)
			default:
			}
		}
		for _, code := range []uint8{ppp.PAPAuthenticateAck, ppp.PAPAuthenticateNak} {
			assertWaiting(papReplyFrame(t, code, 77, "unmatched"))
			missingLength := papReplyFrame(t, code, 1, "")
			missingLength[5] = 4
			assertWaiting(missingLength[:6])
			badLength := papReplyFrame(t, code, 1, "x")
			badLength[6] = 2
			assertWaiting(badLength)
			unknownCode := papReplyFrame(t, code, 1, "")
			unknownCode[2] = 99
			assertWaiting(unknownCode)
		}
		// sleep(timer): produce a newer request before delivering the old reply.
		time.Sleep(papRetryInterval)
		synctest.Wait()
		assertWaiting(papReplyFrame(t, ppp.PAPAuthenticateAck, 1, "stale"))
		packets := w.papPackets(t)
		if len(packets) != 2 {
			t.Fatalf("requests = %d, want 2", len(packets))
		}
		frames <- readFrame{data: papReplyFrame(t, ppp.PAPAuthenticateNak, packets[1].Identifier, "")}
		if err := <-done; err == nil {
			t.Fatal("matching Nak authenticated the client")
		}
	})
}

// TestClientPAPRetryLimit checks that a silent peer consumes a bounded number
// of attempts and never authenticates.
func TestClientPAPRetryLimit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &recordingRWC{}
		var buf [ppp.MaxFrameBufLen]byte
		err := runClientAuth(w, make(chan readFrame), buf[:], lcpResult{authProto: ppp.ProtoPAP},
			sessionConfig{username: "alice", password: "secret"}, 1, make(chan struct{}), slog.Default())
		if err == nil {
			t.Fatal("silent peer authenticated")
		}
		packets := w.papPackets(t)
		if len(packets) != papRequestsMax {
			t.Fatalf("requests = %d, want %d", len(packets), papRequestsMax)
		}
	})
}

type retryFailingAuthWriter struct {
	recordingRWC
	err error
}

func (w *retryFailingAuthWriter) Write(frame []byte) (int, error) {
	if len(w.frames) != 0 {
		return 0, w.err
	}
	return w.frameLog.Write(frame)
}

// TestClientPAPRetryWriteFailure covers failed and short writes on a retry
// after a successful initial request.
func TestClientPAPRetryWriteFailure(t *testing.T) {
	for _, tc := range []struct {
		name     string
		writeErr error
		want     error
	}{
		{"socket error", io.ErrClosedPipe, io.ErrClosedPipe},
		{"short write", nil, io.ErrShortWrite},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				w := &retryFailingAuthWriter{err: tc.writeErr}
				var buf [ppp.MaxFrameBufLen]byte
				err := runClientAuth(w, make(chan readFrame), buf[:], lcpResult{authProto: ppp.ProtoPAP},
					sessionConfig{username: "alice", password: "secret"}, 1, make(chan struct{}), slog.Default())
				if !errors.Is(err, tc.want) {
					t.Fatalf("retry error = %v, want %v", err, tc.want)
				}
			})
		})
	}
}

// TestClientPAPStopsOnLCPTermination covers a lost Nak followed by the
// authenticator's LCP termination, which must end the retrying exchange.
func TestClientPAPStopsOnLCPTermination(t *testing.T) {
	w := &recordingRWC{}
	frames := make(chan readFrame, 1)
	frames <- serverFrame(ppp.LCPTerminateRequest, 0x37, nil)
	var buf [ppp.MaxFrameBufLen]byte
	err := runClientAuth(w, frames, buf[:], lcpResult{authProto: ppp.ProtoPAP},
		sessionConfig{username: "alice", password: "secret"}, 1, make(chan struct{}), slog.Default())
	if err == nil {
		t.Fatal("terminated peer authenticated")
	}
	packets := w.lcpPackets(t)
	if len(packets) != 1 || packets[0].Code != ppp.LCPTerminateAck || packets[0].Identifier != 0x37 {
		t.Fatalf("termination replies = %+v, want one matching Terminate-Ack", packets)
	}
}
