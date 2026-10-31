// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- network phase: IPCP on the PPP unit descriptor
//
// Goal: prove that the client answers an IPCP packet the kernel delivers on
// the PPP unit descriptor once the channel is connected. Linux ppp_input
// queues a frame whose protocol is below 0xc000 on the unit, not the channel,
// after PPPIOCCONNECT (drivers/net/ppp/ppp_generic.c, v6.12 lines 2318 and
// 2472), so the network phase reads IPCP from the unit's reader.
//
// Method: keepaliveLoop runs in a synctest bubble with its channel frames
// silent and IPCP fed on unitFrames, against a recording writer.

package pppoeclient

import (
	"io"
	"log/slog"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// runUnitNetworkPhase starts keepaliveLoop with an IPCP Code-Reject counter at
// lastCodeRejectID, as negotiateIPCP leaves it, and answers the writer, the
// unit's frame feed and the done channel.
func runUnitNetworkPhase(t *testing.T, stop chan struct{}, lastCodeRejectID uint8) (*recordingRWC, chan readFrame, chan struct{}) {
	t.Helper()
	w := &recordingRWC{}
	unit := make(chan readFrame)
	done := make(chan struct{})
	result := &sessionResult{
		frames:           make(chan readFrame),
		unitFrames:       unit,
		magic:            1,
		peerMRU:          1492,
		ipcpCodeRejectID: lastCodeRejectID,
	}
	go keepaliveLoop(w, result, done, stop, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return w, unit, done
}

// TestClientUnitIPCPCodeRejected feeds two IPCP packets with the unused Code
// 12 on the unit and requires two IPCP Code-Rejects that go on from the
// Identifier negotiateIPCP last used, with the session kept.
func TestClientUnitIPCPCodeRejected(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		stop := make(chan struct{})
		w, unit, done := runUnitNetworkPhase(t, stop, 4)
		unit <- ipcpFrame(12, 0x42, []byte{1, 2, 3, 4})
		unit <- ipcpFrame(12, 0x42, []byte{1, 2, 3, 4})
		synctest.Wait()
		if sessionEnded(done) {
			t.Fatal("an IPCP Code 12 ended the session")
		}
		close(stop)
		<-done
		if got := len(w.snapshot()); got != 2 {
			t.Fatalf("the client wrote %d frames for two IPCP Code 12 packets, want two Code-Rejects", got)
		}
		for index, wantID := range []uint8{5, 6} {
			reject := recordedIPCP(t, &w.frameLog, index)
			if reject.Code != ppp.LCPCodeReject {
				t.Fatalf("frame %d has IPCP Code %d, want Code-Reject", index, reject.Code)
			}
			if reject.Identifier != wantID {
				t.Fatalf("Code-Reject %d has Identifier %d, want %d", index, reject.Identifier, wantID)
			}
			if len(reject.Data) < 2 || reject.Data[0] != 12 || reject.Data[1] != 0x42 {
				t.Fatalf("Code-Reject %d rejects % x, not the Code 12 packet", index, reject.Data)
			}
		}
	})
}

// TestClientUnitIPCPTerminateAcked feeds an IPCP Terminate-Request on the unit
// and requires an IPCP Terminate-Ack carrying its Identifier, and the session
// ended.
func TestClientUnitIPCPTerminateAcked(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		stop := make(chan struct{})
		defer close(stop)
		w, unit, done := runUnitNetworkPhase(t, stop, 0)
		unit <- ipcpFrame(ppp.LCPTerminateRequest, 0x77, nil)
		synctest.Wait()
		if !sessionEnded(done) {
			t.Fatal("the session outlived an IPCP Terminate-Request")
		}
		if got := len(w.snapshot()); got != 1 {
			t.Fatalf("the client wrote %d frames for one Terminate-Request, want one Terminate-Ack", got)
		}
		ack := recordedIPCP(t, &w.frameLog, 0)
		if ack.Code != ppp.LCPTerminateAck {
			t.Fatalf("answer has IPCP Code %d, want Terminate-Ack", ack.Code)
		}
		if ack.Identifier != 0x77 {
			t.Fatalf("Terminate-Ack Identifier = 0x%02x, want the request's 0x77", ack.Identifier)
		}
	})
}

// TestClientIPCPCodeZeroCodeRejected feeds an IPCP packet with Code 0, which
// RFC 1332 Section 2 leaves outside the seven Codes IPCP uses, during
// negotiation and in the network phase, and requires a Code-Reject each time
// with the session kept. Before ipcpCodeUnrecognized, negotiation dropped it
// and the network phase ended the session on it.
func TestClientIPCPCodeZeroCodeRejected(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &frameLog{}
		frames := make(chan readFrame)
		stop := make(chan struct{})
		defer close(stop)
		go func() {
			var buf [ppp.MaxFrameBufLen]byte
			_, _ = negotiateIPCP(w, frames, buf[:], 1, 0, nil, stop, slog.Default()) // Stopped before IPCP opens.
		}()
		synctest.Wait()
		frames <- ipcpFrame(0, 0x30, []byte{1, 2})
		synctest.Wait()
		if got := len(w.snapshot()); got != 2 {
			t.Fatalf("negotiation wrote %d frames for a Code 0 packet, want the request and one Code-Reject", got)
		}
		if reject := recordedIPCP(t, w, 1); reject.Code != ppp.LCPCodeReject {
			t.Fatalf("negotiation answered Code 0 with Code %d, want Code-Reject", reject.Code)
		}
	})
	synctest.Test(t, func(t *testing.T) {
		stop := make(chan struct{})
		w, unit, done := runUnitNetworkPhase(t, stop, 0)
		unit <- ipcpFrame(0, 0x31, []byte{1, 2})
		synctest.Wait()
		if sessionEnded(done) {
			t.Fatal("an IPCP Code 0 ended the network phase")
		}
		close(stop)
		<-done
		if got := len(w.snapshot()); got != 1 {
			t.Fatalf("the network phase wrote %d frames for a Code 0 packet, want one Code-Reject", got)
		}
		if reject := recordedIPCP(t, &w.frameLog, 0); reject.Code != ppp.LCPCodeReject {
			t.Fatalf("the network phase answered Code 0 with Code %d, want Code-Reject", reject.Code)
		}
	})
}
