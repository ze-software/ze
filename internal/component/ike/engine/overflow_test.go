// Design: docs/architecture/ike/ipsec-14-responder.md -- IKE responder handshake
// RFC: rfc/short/rfc7296.md -- IKE_SA_INIT notify (Section 2.21, Section 3)
//
// VALIDATES: sendSAInitNotify bounds its fixed 512-byte buffer: an oversized
// notify is dropped (not sent, not truncated) and logged with the peer and the
// required length, and a fitting notify is still emitted byte-for-byte unchanged.
// PREVENTS: an oversized IKE_SA_INIT notify panicking with a slice-bounds error
// inside Message.WriteTo (spec-fixit-fixed-buffer-overflow, AC-1/AC-3).
package engine

import (
	"bytes"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestSendSAInitNotifyOversizedRejected drives sendSAInitNotify with a notify
// whose encoded length exceeds the fixed 512-byte buffer. Before the bound this
// panics inside Message.WriteTo; after it, the send is skipped and logged.
func TestSendSAInitNotifyOversizedRejected(t *testing.T) {
	transportLog := slogutil.DiscardLogger()

	// Loopback receiver standing in for the peer, plus our own sender socket.
	peerTr, err := transport.NewUDPTransport("127.0.0.1:0", transportLog)
	if err != nil {
		t.Fatalf("peer transport: %v", err)
	}
	t.Cleanup(func() { _ = peerTr.Close() })
	go peerTr.Run()

	myTr, err := transport.NewUDPTransport("127.0.0.1:0", transportLog)
	if err != nil {
		t.Fatalf("sender transport: %v", err)
	}
	t.Cleanup(func() { _ = myTr.Close() })

	remote, ok := peerTr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("peer transport local address is not *net.UDPAddr")
	}

	sa := &SA{
		PeerName:     "oversize-peer",
		InitiatorSPI: [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
		ResponderSPI: [8]byte{8, 7, 6, 5, 4, 3, 2, 1},
	}

	// Positive control: a small notify really does traverse the sockets, so a
	// later "nothing received" is a genuine drop rather than a broken harness.
	sendSAInitNotify(sa, myTr, remote, wire.NotifyNoProposalChosen, nil, transportLog)
	select {
	case <-peerTr.Recv():
	case <-time.After(2 * time.Second):
		t.Fatal("control notify was not received; test harness is broken")
	}

	// NotificationData far larger than the 512-byte fixed buffer.
	oversized := make([]byte, 600)
	var logBuf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logBuf, nil))

	didPanic := false
	func() {
		defer func() {
			if recover() != nil {
				didPanic = true
			}
		}()
		sendSAInitNotify(sa, myTr, remote, wire.NotifyInvalidKEPayload, oversized, log)
	}()
	if didPanic {
		t.Fatal("sendSAInitNotify panicked on an oversized notify; the bound is missing")
	}

	// The oversized notify must NOT reach the peer (truncation is malformed).
	select {
	case pkt := <-peerTr.Recv():
		t.Fatalf("oversized notify was transmitted (%d bytes); it must be dropped", len(pkt.Data))
	case <-time.After(300 * time.Millisecond):
	}

	// The drop is logged with the peer and the required length.
	logged := logBuf.String()
	if !strings.Contains(logged, "oversize-peer") {
		t.Errorf("drop log missing peer name: %q", logged)
	}
	if !strings.Contains(logged, "needs") {
		t.Errorf("drop log missing required length: %q", logged)
	}
}

// TestSendSAInitNotifyBytesUnchanged proves the checked encode path is byte-for-
// byte identical to the raw WriteTo for a fitting notify (AC-3): the guard adds a
// bound, it changes no encoding.
func TestSendSAInitNotifyBytesUnchanged(t *testing.T) {
	// The exact message sendSAInitNotify builds for an INVALID_KE notify.
	msg := wire.Message{
		Header: wire.Header{
			InitiatorSPI: [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
			ResponderSPI: [8]byte{8, 7, 6, 5, 4, 3, 2, 1},
			MajorVersion: 2,
			ExchangeType: wire.ExchangeIKESAInit,
			Flags:        wire.FlagResponse,
			MessageID:    0,
		},
		Payloads: []wire.PayloadEntry{{Payload: &wire.PayloadNotify{
			NotifyMsgType:    wire.NotifyInvalidKEPayload,
			NotificationData: []byte{0, 14}, // DH group 14
		}}},
	}

	raw := make([]byte, 512)
	nRaw := msg.WriteTo(raw, 0)

	chk := make([]byte, 512)
	nChk, err := msg.CheckedWriteTo(chk, 0)
	if err != nil {
		t.Fatalf("CheckedWriteTo on a fitting notify errored: %v", err)
	}
	if nRaw != nChk {
		t.Fatalf("length differs: raw=%d checked=%d", nRaw, nChk)
	}
	if !bytes.Equal(raw[:nRaw], chk[:nChk]) {
		t.Fatalf("checked bytes differ from raw WriteTo bytes")
	}
	if got := msg.Len(); got != nRaw {
		t.Fatalf("Len()=%d but WriteTo wrote %d bytes", got, nRaw)
	}

	// The bytes still parse back to the same notify (a valid, unchanged encoding).
	var back wire.Message
	if err := back.ReadFrom(chk[:nChk]); err != nil {
		t.Fatalf("re-parse of encoded notify failed: %v", err)
	}
	if len(back.Payloads) != 1 {
		t.Fatalf("re-parsed payload count = %d, want 1", len(back.Payloads))
	}
	notify, ok := back.Payloads[0].Payload.(*wire.PayloadNotify)
	if !ok {
		t.Fatalf("re-parsed payload is %T, want *wire.PayloadNotify", back.Payloads[0].Payload)
	}
	if notify.NotifyMsgType != wire.NotifyInvalidKEPayload {
		t.Errorf("notify type = %d, want %d", notify.NotifyMsgType, wire.NotifyInvalidKEPayload)
	}
	if !bytes.Equal(notify.NotificationData, []byte{0, 14}) {
		t.Errorf("notify data = %v, want [0 14]", notify.NotificationData)
	}
}
