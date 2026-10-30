// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- IPCP negotiation.
// Related: session.go -- negotiateIPCP and sendIPCPCodeReject under test
// Related: ipcp_reply_test.go -- ipcpFrame and recordedIPCP
//
// Drives the PPPoE client's IPCP negotiation with packets RFC 1332 tells it
// to refuse: a Code IPCP does not use (Section 2), and a request to compress
// the slot identifier (Section 4.1).

package pppoeclient

import (
	"bytes"
	"encoding/binary"
	"log/slog"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestClientIPCPUnusedCodesAreCodeRejected sends the client IPCP packets
// carrying Codes 8 to 12 during negotiation and reads what it writes.
//
// VALIDATES: RFC 1332 Section 2: "Other Codes should be treated as
// unrecognized and should result in Code-Rejects." Each Code-Reject carries a
// fresh Identifier (RFC 1661 Section 5.6), and a Rejected-Packet longer than
// the server's MRU is cut to it.
// PREVENTS: the silent drop the client's IPCP loop gave every Code its switch
// did not name.
//
// RFC requirement: RFC1332-2-2 positive -- during IPCP negotiation the PPPoE client answers an IPCP packet with Code 8, 9, 10, 11 or 12 with one IPCP Code-Reject whose Rejected-Packet starts with that packet's Code and Identifier, and each Code-Reject's Identifier differs from the last.
func TestClientIPCPUnusedCodesAreCodeRejected(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &frameLog{}
		frames := make(chan readFrame)
		stop := make(chan struct{})
		defer close(stop)
		go func() {
			var buf [ppp.MaxFrameBufLen]byte
			_, _ = negotiateIPCP(w, frames, buf[:], 1, 100, nil, stop, slog.Default()) // The test stops it before IPCP opens, so the result says nothing.
		}()
		synctest.Wait()

		var lastID uint8
		for i, code := range []uint8{ppp.LCPProtocolReject, ppp.LCPEchoRequest, ppp.LCPEchoReply, ppp.LCPDiscardRequest, 12} {
			frames <- ipcpFrame(code, 0x40+code, []byte{0xca, 0xfe})
			synctest.Wait()
			written := w.snapshot()
			if len(written) != i+2 {
				t.Fatalf("code %d: %d frames written, want the request and %d Code-Rejects", code, len(written), i+1)
			}
			reject := recordedIPCP(t, w, i+1)
			if reject.Code != ppp.LCPCodeReject {
				t.Fatalf("code %d: reply Code = %d, want Code-Reject", code, reject.Code)
			}
			want := []byte{code, 0x40 + code, 0, 6, 0xca, 0xfe}
			if !bytes.Equal(reject.Data, want) {
				t.Fatalf("code %d: Rejected-Packet = % x, want % x", code, reject.Data, want)
			}
			if i > 0 && reject.Identifier == lastID {
				t.Fatalf("code %d: Code-Reject Identifier %d repeats the last one", code, reject.Identifier)
			}
			lastID = reject.Identifier
		}

		// A packet longer than the server's 100-octet MRU: the Code-Reject's
		// own Information field (its 4-octet header and the copy) stays within it.
		frames <- ipcpFrame(ppp.LCPEchoRequest, 0x70, bytes.Repeat([]byte{0xab}, 200))
		synctest.Wait()
		written := w.snapshot()
		last := written[len(written)-1]
		if got := len(last) - ppp.ProtoFieldLen; got != 100 {
			t.Fatalf("Code-Reject Information field = %d octets, want the 100-octet MRU", got)
		}
		reject := recordedIPCP(t, w, len(written)-1)
		if got := binary.BigEndian.Uint16(reject.Data[2:4]); got != 204 {
			t.Fatalf("Rejected-Packet Length = %d, want the original 204", got)
		}
	})
}

// TestClientIPCPRejectsVanJacobsonCompression sends the client a server IPCP
// Configure-Request asking it to compress TCP/IP headers and the slot
// identifier, and reads its answer.
//
// VALIDATES: the client never agrees to Van Jacobson compression, so it never
// compresses the slot identifier (RFC 1332 Section 4.1).
// PREVENTS: a client IPCP loop that Acks an option it does not implement.
//
// RFC requirement: RFC1332-4.1-1 negative -- the PPPoE client answers a server Configure-Request carrying IP-Compression-Protocol 0x002d with Comp-Slot-Id 1 with a Configure-Reject of exactly that option, never a Configure-Ack.
func TestClientIPCPRejectsVanJacobsonCompression(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &frameLog{}
		frames := make(chan readFrame)
		stop := make(chan struct{})
		defer close(stop)
		go func() {
			var buf [ppp.MaxFrameBufLen]byte
			_, _ = negotiateIPCP(w, frames, buf[:], 1, 0, nil, stop, slog.Default()) // The test stops it before IPCP opens, so the result says nothing.
		}()
		synctest.Wait()

		vj := []byte{2, 6, 0x00, 0x2d, 15, 1}
		request := append([]byte{ppp.IPCPOptIPAddress, 6, 192, 0, 2, 1}, vj...)
		frames <- ipcpFrame(ppp.LCPConfigureRequest, 0x31, request)
		synctest.Wait()
		if written := w.snapshot(); len(written) != 2 {
			t.Fatalf("%d frames written, want the request and one reply", len(written))
		}
		reply := recordedIPCP(t, w, 1)
		if reply.Code != ppp.LCPConfigureReject || reply.Identifier != 0x31 || !bytes.Equal(reply.Data, vj) {
			t.Fatalf("reply = %+v, want a Configure-Reject of exactly % x", reply, vj)
		}
	})
}
