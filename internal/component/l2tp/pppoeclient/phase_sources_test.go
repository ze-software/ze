// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- connected negotiation sources.
package pppoeclient

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestClientIPCPConnectedSources opens IPCP from unit replies. The exact Ack,
// negotiated addresses and retained Identifier prove the connected input path.
func TestClientIPCPConnectedSources(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		frames := make(chan readFrame)
		unit := make(chan readFrame, 3)
		unit <- ipcpFrame(ppp.LCPConfigureNak, 8, []byte{3, 6, 192, 0, 2, 2})
		unit <- ipcpFrame(ppp.LCPConfigureAck, 9, []byte{3, 6, 192, 0, 2, 2})
		unit <- ipcpFrame(ppp.LCPConfigureRequest, 0x70, []byte{3, 6, 192, 0, 2, 1})
		w := &recordingRWC{}
		var buf [ppp.MaxFrameBufLen]byte
		id := uint8(7)
		got, err := negotiateIPCP(w, testFrameReader(t, frames, unit), buf[:], clientMagic, 1492, nil, &id, make(chan struct{}), slog.New(slog.NewTextHandler(io.Discard, nil)), new(uint8))
		if err != nil {
			t.Fatal(err)
		}
		if got.localIP != netip.MustParseAddr("192.0.2.2") {
			t.Fatalf("local address = %v", got.localIP)
		}
		if got.peerIP != netip.MustParseAddr("192.0.2.1") {
			t.Fatalf("peer address = %v", got.peerIP)
		}
		if id != 9 {
			t.Fatalf("last Identifier = %d, want 9", id)
		}
		want := []byte{0x80, 0x21, 2, 0x70, 0, 10, 3, 6, 192, 0, 2, 1}
		for _, frame := range w.snapshot() {
			if bytes.Equal(frame, want) {
				return
			}
		}
		t.Fatalf("missing exact IPCP Ack: % x", w.snapshot())
	})
}

// TestClientIPCPConnectedControl admits LCP reentry on the channel while the
// unit has no frame. A connected IPCP exchange must still serve LCP promptly.
func TestClientIPCPConnectedControl(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		frames := make(chan readFrame, 1)
		frames <- clientRenegotiationRequest()
		var buf [ppp.MaxFrameBufLen]byte
		_, err := negotiateIPCP(io.Discard, testFrameReader(t, frames, make(chan readFrame)), buf[:], clientMagic, 1492, nil, new(uint8), make(chan struct{}), slog.New(slog.NewTextHandler(io.Discard, nil)), new(uint8))
		var pending *lcpReentry
		if !errors.As(err, &pending) {
			t.Fatalf("channel reentry = %v", err)
		}
		if pending.request.Identifier != 0x73 {
			t.Fatalf("pending Identifier = %x", pending.request.Identifier)
		}
		if !bytes.Equal(pending.request.Data, []byte{5, 6, 0x55, 0x66, 0x77, 0x88}) {
			t.Fatalf("pending options = % x", pending.request.Data)
		}
	})
}

// TestClientConnectedUnitFailure ends each phase on unit EOF or read failure,
// without waiting for its protocol timeout or treating EOF as an empty frame.
func TestClientConnectedUnitFailure(t *testing.T) {
	for _, phase := range []string{"lcp", "auth", "ipcp"} {
		for _, readErr := range []error{io.EOF, io.ErrUnexpectedEOF} {
			t.Run(phase+"/"+readErr.Error(), func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					unit := make(chan readFrame, 1)
					if errors.Is(readErr, io.EOF) {
						close(unit)
					} else {
						unit <- readFrame{err: readErr}
					}
					w := &recordingRWC{}
					var buf [ppp.MaxFrameBufLen]byte
					logger := slog.New(slog.NewTextHandler(io.Discard, nil))
					var err error
					switch phase {
					case "lcp":
						_, err = negotiateLCP(w, testFrameReader(t, make(chan readFrame), unit), buf[:], sessionConfig{mtu: 1492}, clientMagic, nil, make(chan struct{}), logger, new(uint8))
					case "auth":
						err = runClientAuth(w, testFrameReader(t, make(chan readFrame), unit), buf[:], lcpResult{authProto: ppp.ProtoCHAP}, sessionConfig{}, clientMagic, make(chan struct{}), logger, new(uint8))
					case "ipcp":
						_, err = negotiateIPCP(w, testFrameReader(t, make(chan readFrame), unit), buf[:], clientMagic, 1492, nil, new(uint8), make(chan struct{}), logger, new(uint8))
					}
					if !errors.Is(err, readErr) {
						t.Fatalf("phase ended with %v, want %v", err, readErr)
					}
				})
			})
		}
	}
}

// TestClientEstablishmentDiscardsUnitFrames sends non-LCP frames followed by a
// unit read error. Each phase must discard the frames and observe the error.
func TestClientEstablishmentDiscardsUnitFrames(t *testing.T) {
	for _, phase := range []string{"lcp", "auth"} {
		t.Run(phase, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				unit := make(chan readFrame, 3)
				unit <- ipcpFrame(ppp.LCPConfigureRequest, 1, []byte{3, 6, 192, 0, 2, 1})
				unit <- readFrame{data: []byte{0, 0x21, 0x45, 0, 0, 20}}
				unit <- readFrame{err: io.ErrUnexpectedEOF}
				w := &recordingRWC{}
				var buf [ppp.MaxFrameBufLen]byte
				logger := slog.New(slog.NewTextHandler(io.Discard, nil))
				var err error
				if phase == "lcp" {
					_, err = negotiateLCP(w, testFrameReader(t, make(chan readFrame), unit), buf[:], sessionConfig{mtu: 1492}, clientMagic, nil, make(chan struct{}), logger, new(uint8))
				} else {
					err = runClientAuth(w, testFrameReader(t, make(chan readFrame), unit), buf[:], lcpResult{authProto: ppp.ProtoCHAP}, sessionConfig{}, clientMagic, make(chan struct{}), logger, new(uint8))
				}
				if !errors.Is(err, io.ErrUnexpectedEOF) {
					t.Fatalf("phase ended with %v, want unit read error", err)
				}
				for _, frame := range w.snapshot() {
					if frame[0] != 0xc0 {
						t.Fatalf("answered non-LCP packet: % x", frame)
					}
				}
			})
		})
	}
}
