// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- client LCP handshake transitions.
package pppoeclient

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestClientLCPRepliesInAckReceived follows a first Ack with another valid
// reply. Each RFC 1661 Section 4.1 transition owes a fresh Configure-Request,
// and the first Ack cannot satisfy that new request.
func TestClientLCPRepliesInAckReceived(t *testing.T) {
	for _, code := range []uint8{ppp.LCPConfigureAck, ppp.LCPConfigureNak, ppp.LCPConfigureReject} {
		synctest.Test(t, func(t *testing.T) {
			w := &frameLog{}
			frames := make(chan readFrame)
			stop := make(chan struct{})
			defer close(stop)
			done := make(chan error, 1)
			go func() {
				var buf [ppp.MaxFrameBufLen]byte
				_, err := negotiateLCP(w, frames, buf[:], sessionConfig{mtu: 1492}, clientMagic, stop, slog.Default())
				done <- err
			}()
			synctest.Wait()
			first := w.lcpPackets(t)[0]
			frames <- serverFrame(ppp.LCPConfigureAck, first.Identifier, first.Data)
			synctest.Wait()
			data := first.Data
			switch code {
			case ppp.LCPConfigureNak:
				data = []byte{ppp.LCPOptMRU, 4, 5, 0}
			case ppp.LCPConfigureReject:
				data = first.Data[:4]
			}
			frames <- serverFrame(code, first.Identifier, data)
			synctest.Wait()
			packets := w.lcpPackets(t)
			if len(packets) != 2 || packets[1].Code != ppp.LCPConfigureRequest || packets[1].Identifier == first.Identifier {
				t.Fatalf("reply code %d after Ack: %+v, want fresh request", code, packets)
			}
			frames <- serverFrame(ppp.LCPConfigureRequest, 77, nil)
			synctest.Wait()
			select {
			case err := <-done:
				t.Fatalf("old Ack opened the new request: %v", err)
			default:
			}
			frames <- serverFrame(ppp.LCPConfigureAck, packets[1].Identifier, packets[1].Data)
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestClientLCPRefusalRevokesPeerAck replaces an accepted proposal with one
// that draws a Nak or Reject. A local Ack cannot open LCP until a subsequent
// acceptable peer request arrives, and only that request's options survive.
func TestClientLCPRefusalRevokesPeerAck(t *testing.T) {
	for _, badOptions := range [][]byte{
		{0xfe, 2},
		{ppp.LCPOptMagic, 6, 0, 0, 0, 0},
		{ppp.LCPOptMagic, 5, 0, 0, 0},
	} {
		synctest.Test(t, func(t *testing.T) {
			w := &frameLog{}
			frames := make(chan readFrame)
			stop := make(chan struct{})
			defer close(stop)
			type outcome struct {
				result lcpResult
				err    error
			}
			done := make(chan outcome, 1)
			go func() {
				var buf [ppp.MaxFrameBufLen]byte
				result, err := negotiateLCP(w, frames, buf[:], sessionConfig{mtu: 1492}, clientMagic, stop, slog.Default())
				done <- outcome{result: result, err: err}
			}()
			synctest.Wait()
			request := w.lcpPackets(t)[0]
			frames <- serverFrame(ppp.LCPConfigureRequest, 76, []byte{
				ppp.LCPOptMRU, 4, 4, 0xb0,
				ppp.LCPOptAuthProto, 4, 0xc0, 0x23,
			})
			frames <- serverFrame(ppp.LCPConfigureRequest, 77, badOptions)
			frames <- serverFrame(ppp.LCPConfigureAck, request.Identifier, request.Data)
			synctest.Wait()
			select {
			case got := <-done:
				t.Fatalf("refused peer proposal opened LCP: %+v", got)
			default:
			}
			frames <- serverFrame(ppp.LCPConfigureRequest, 78, nil)
			got := <-done
			if got.err != nil {
				t.Fatal(got.err)
			}
			if got.result.peerMRU != 0 || got.result.authProto != 0 {
				t.Fatalf("superseded peer options survived: %+v", got.result)
			}
		})
	}
}

// TestClientLCPEchoBeforeOpenDiscarded verifies that an Echo cannot elicit a
// reply while either half of LCP is still negotiating.
//
// RFC requirement: RFC1661-5.8-2 negative -- client negotiation sends no Echo-Reply before LCP reaches Opened.
// MUTATION: restore sendEchoReply in negotiateLCP; the recorder gains an Echo-Reply.
func TestClientLCPEchoBeforeOpenDiscarded(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &frameLog{}
		frames := make(chan readFrame)
		stop := make(chan struct{})
		defer close(stop)
		done := make(chan error, 1)
		go func() {
			var buf [ppp.MaxFrameBufLen]byte
			_, err := negotiateLCP(w, frames, buf[:], sessionConfig{mtu: 1492}, clientMagic, stop, slog.Default())
			done <- err
		}()
		synctest.Wait()
		request := w.lcpPackets(t)[0]
		frames <- serverFrame(ppp.LCPEchoRequest, 76, []byte{1, 2, 3, 4})
		synctest.Wait()
		if len(w.frames) != 1 {
			t.Fatalf("Echo before negotiation completed produced frames: % x", w.frames)
		}
		frames <- serverFrame(ppp.LCPConfigureAck, request.Identifier, request.Data)
		frames <- serverFrame(ppp.LCPEchoRequest, 77, []byte{1, 2, 3, 4})
		synctest.Wait()
		if len(w.frames) != 1 {
			t.Fatalf("Echo in Ack-Rcvd produced frames: % x", w.frames)
		}
		frames <- serverFrame(ppp.LCPConfigureRequest, 78, nil)
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	})
}

type failedAckWriter struct {
	frameLog
	err error
}

func (w *failedAckWriter) Write(frame []byte) (int, error) {
	if len(frame) >= 3 && frame[2] == ppp.LCPConfigureAck {
		return 0, w.err
	}
	return w.frameLog.Write(frame)
}

// TestClientLCPAckWriteFailure cannot open LCP after an Ack failed to reach the peer.
func TestClientLCPAckWriteFailure(t *testing.T) {
	for _, tc := range []struct {
		writeErr error
		want     error
	}{
		{io.ErrClosedPipe, io.ErrClosedPipe},
		{nil, io.ErrShortWrite},
	} {
		synctest.Test(t, func(t *testing.T) {
			w := &failedAckWriter{err: tc.writeErr}
			frames := make(chan readFrame)
			stop := make(chan struct{})
			defer close(stop)
			done := make(chan error, 1)
			go func() {
				var buf [ppp.MaxFrameBufLen]byte
				_, err := negotiateLCP(w, frames, buf[:], sessionConfig{mtu: 1492}, clientMagic, stop, slog.Default())
				done <- err
			}()
			synctest.Wait()
			request := w.lcpPackets(t)[0]
			frames <- serverFrame(ppp.LCPConfigureAck, request.Identifier, request.Data)
			frames <- serverFrame(ppp.LCPConfigureRequest, 77, nil)
			if err := <-done; !errors.Is(err, tc.want) {
				t.Fatalf("failed Ack write returned %v, want %v", err, tc.want)
			}
		})
	}
}
