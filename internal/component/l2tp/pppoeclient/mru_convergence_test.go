// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- IPv4 MRU convergence.
// Related: restart_admission_test.go -- result admission remains a final guard.

package pppoeclient

import (
	"bytes"
	"encoding/binary"
	"io"
	"log/slog"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestClientMRUConvergence checks exact Nak 68 followed by a corrected request,
// both initially and after Opened. No IPCP or PASS is permitted before correction.
func TestClientMRUConvergence(t *testing.T) {
	for _, restart := range []bool{false, true} {
		name := "initial"
		if restart {
			name = "restart"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				w := newConnectedExchange()
				s, initial := w.session(t)
				stop := make(chan struct{})
				defer close(stop)
				result := make(chan error, 1)
				logger := slog.New(slog.NewTextHandler(io.Discard, nil))
				options := exchangePeerOptions(0)
				binary.BigEndian.PutUint16(options[2:], 66)
				if restart {
					go func() {
						result <- s.restart(&lcpReentry{request: ppp.LCPPacket{Code: 1, Identifier: 91, Data: options}}, stop, logger)
					}()
				} else {
					w.unit = w.frames
					go func() {
						var ipcp, lcp, pap, reject uint8
						next, err := negotiatePhases(w, s.reader, s.cfg, 0, nil, &ipcp, stop, logger, &lcp, &pap, &reject)
						if err == nil {
							if next.negMTU != 68 {
								t.Errorf("initial MTU = %d, want 68", next.negMTU)
							}
						}
						result <- err
					}()
				}
				request := w.take(t, ppp.ProtoLCP, 1)
				if !restart {
					w.send(ppp.ProtoLCP, 1, 91, options)
				}
				nak := w.take(t, ppp.ProtoLCP, 3)
				if !bytes.Equal(nak, exchangeFrame(ppp.ProtoLCP, 3, 91, []byte{1, 4, 0, 68})) {
					t.Fatalf("MRU Nak = %x", nak)
				}
				w.send(ppp.ProtoLCP, 2, request[3], request[6:])
				w.quiet(t)
				if w.passes() != 0 {
					t.Fatal("IPv4 enabled before peer correction")
				}
				corrected := bytes.Clone(options)
				binary.BigEndian.PutUint16(corrected[2:], 68)
				w.send(ppp.ProtoLCP, 1, 92, corrected)
				ack := w.take(t, ppp.ProtoLCP, 2)
				if !bytes.Equal(ack, exchangeFrame(ppp.ProtoLCP, 2, 92, corrected)) {
					t.Fatalf("corrected MRU Ack = %x", ack)
				}
				previous := uint8(0)
				if restart {
					previous = initial.ipcpIdentifier
				}
				w.openIPCP(t, previous, 0)
				if err := <-result; err != nil {
					t.Fatal(err)
				}
				if restart {
					if w.passes() != 1 {
						t.Fatalf("PASS count = %d", w.passes())
					}
					if got := <-s.changes; got.NegMTU != 68 {
						t.Fatalf("published MTU = %d", got.NegMTU)
					}
				}
			})
		})
	}
}

// TestSessionTransportRequired refuses a transport that cannot serialize
// descriptor lifetime before any packet write, read authorization or ioctl.
func TestSessionTransportRequired(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		w.unit = w.frames
		stop := make(chan struct{})
		defer close(stop)
		reader := testFrameReader(t, w.frames, nil)
		_, err := negotiateSession(w, reader, -1, -1,
			sessionConfig{mtu: 1492, chanFD: -1}, stop, slog.New(slog.NewTextHandler(io.Discard, nil)))
		if len(w.writes) != 0 {
			t.Fatal("unsupported transport wrote a packet")
		}
		if reader.pending {
			t.Fatal("unsupported transport authorized a read")
		}
		if err == nil {
			t.Fatal("transport without lifecycle lock admitted")
		}
		if !strings.Contains(err.Error(), "requires sessionLink") {
			t.Fatalf("transport refusal = %v", err)
		}
	})
}
