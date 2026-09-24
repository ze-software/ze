// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- IPCP reply correlation.
package pppoeclient

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

func ipcpFrame(code, id uint8, options []byte) readFrame {
	var buf [ppp.MaxFrameBufLen]byte
	off := ppp.WriteFrame(buf[:], 0, ppp.ProtoIPCP, nil)
	off += ppp.WriteLCPPacket(buf[:], off, code, id, options)
	return readFrame{data: bytes.Clone(buf[:off])}
}

func recordedIPCP(t *testing.T, w *frameLog, index int) ppp.LCPPacket {
	t.Helper()
	proto, payload, _, err := ppp.ParseFrame(w.frames[index])
	if err != nil || proto != ppp.ProtoIPCP {
		t.Fatalf("frame %d = % x, protocol %#x, error %v", index, w.frames[index], proto, err)
	}
	packet, err := ppp.ParseLCPPacket(payload)
	if err != nil {
		t.Fatal(err)
	}
	return packet
}

// TestClientIPCPReplyCorrelation prevents stale and malformed replies from
// assigning addresses or terminating a still-usable negotiation.
func TestClientIPCPReplyCorrelation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &frameLog{}
		frames := make(chan readFrame)
		stop := make(chan struct{})
		defer close(stop)
		type outcome struct {
			result ipcpResult
			err    error
		}
		done := make(chan outcome, 1)
		go func() {
			var buf [ppp.MaxFrameBufLen]byte
			result, err := negotiateIPCP(w, frames, buf[:], 1, stop, slog.Default())
			done <- outcome{result, err}
		}()
		synctest.Wait()
		first := recordedIPCP(t, w, 0)
		assigned := netip.MustParseAddr("192.0.2.2")
		peer := netip.MustParseAddr("192.0.2.1")
		options := buildIPCPRequest(1, assigned)[4:]
		for _, invalid := range []readFrame{
			ipcpFrame(ppp.LCPConfigureAck, first.Identifier+1, first.Data),
			ipcpFrame(ppp.LCPConfigureAck, first.Identifier, options),
			ipcpFrame(ppp.LCPConfigureNak, first.Identifier+1, options),
			ipcpFrame(ppp.LCPConfigureNak, first.Identifier, append(bytes.Clone(options), 99)),
			ipcpFrame(ppp.LCPConfigureReject, first.Identifier+1, first.Data),
			ipcpFrame(ppp.LCPConfigureReject, first.Identifier, options),
		} {
			frames <- invalid
			synctest.Wait()
			select {
			case got := <-done:
				t.Fatalf("invalid reply completed IPCP: %+v", got)
			default:
			}
			if len(w.frames) != 1 {
				t.Fatalf("invalid reply emitted a replacement: % x", w.frames)
			}
		}
		// sleep(timer): advance the outstanding IPCP request's restart timer.
		time.Sleep(3 * time.Second)
		synctest.Wait()
		if len(w.frames) != 2 || !bytes.Equal(w.frames[0], w.frames[1]) {
			t.Fatalf("timeout did not retransmit the same request: % x", w.frames)
		}
		frames <- ipcpFrame(ppp.LCPConfigureNak, first.Identifier, options)
		synctest.Wait()
		next := recordedIPCP(t, w, 2)
		if next.Identifier == first.Identifier || !bytes.Equal(next.Data, options) {
			t.Fatalf("replacement request = %+v, want new Identifier and assigned address", next)
		}
		frames <- ipcpFrame(ppp.LCPConfigureRequest, 77, buildIPCPRequest(77, peer)[4:])
		frames <- ipcpFrame(ppp.LCPConfigureAck, first.Identifier, first.Data)
		synctest.Wait()
		select {
		case got := <-done:
			t.Fatalf("stale Ack opened IPCP: %+v", got)
		default:
		}
		frames <- ipcpFrame(ppp.LCPConfigureRequest, 78, []byte{99, 2})
		synctest.Wait()
		reject := recordedIPCP(t, w, len(w.frames)-1)
		if reject.Code != ppp.LCPConfigureReject || reject.Identifier != 78 || !bytes.Equal(reject.Data, []byte{99, 2}) {
			t.Fatalf("unsupported peer proposal = %+v, want exact Configure-Reject", reject)
		}
		frames <- ipcpFrame(ppp.LCPConfigureAck, next.Identifier, next.Data)
		synctest.Wait()
		select {
		case got := <-done:
			t.Fatalf("superseded peer proposal opened IPCP: %+v", got)
		default:
		}
		frames <- ipcpFrame(ppp.LCPConfigureRequest, 79, buildIPCPRequest(79, peer)[4:])
		got := <-done
		if got.err != nil || got.result.localIP != assigned || got.result.peerIP != peer {
			t.Fatalf("IPCP result = %+v, want %v/%v", got, assigned, peer)
		}
	})
}

// TestClientIPCPRejectAndWriteFailure distinguishes a valid refusal from an
// unrelated reply, and stops immediately when the initial request cannot be sent.
func TestClientIPCPRejectAndWriteFailure(t *testing.T) {
	frames := make(chan readFrame, 1)
	frames <- ipcpFrame(ppp.LCPConfigureReject, 1, buildIPCPRequest(1, netip.IPv4Unspecified())[4:])
	var buf [ppp.MaxFrameBufLen]byte
	if _, err := negotiateIPCP(io.Discard, frames, buf[:], 1, make(chan struct{}), slog.Default()); err == nil {
		t.Fatal("valid IP-Address rejection opened IPCP")
	}
	for _, tc := range []struct{ writeErr, want error }{
		{io.ErrClosedPipe, io.ErrClosedPipe},
		{nil, io.ErrShortWrite},
	} {
		w := &failingAuthWriter{err: tc.writeErr}
		_, err := negotiateIPCP(w, make(chan readFrame), buf[:], 1, make(chan struct{}), slog.Default())
		if !errors.Is(err, tc.want) {
			t.Fatalf("request write error = %v, want %v", err, tc.want)
		}
	}
}
