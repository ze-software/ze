// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- receive phase transitions.
// Related: receive.go -- production request/delivery worker.

package pppoeclient

import (
	"bytes"
	"io"
	"log/slog"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestReaderTransitionHeldFrame sends five premature Terminate-Requests through
// the production reader during authentication. All five must be consumed without
// a reply before Success. Fresh IPCP then completes through that same owner.
// TestSessionReaderStopsWithFullQueue separately proves there is no fifth read
// held behind a userspace queue under the replacement mechanism.
func TestReaderTransitionHeldFrame(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		reader := testFrameReader(t, w.frames, w.unit)
		stop := make(chan struct{})
		defer close(stop)
		result := make(chan error, 1)
		go func() {
			var buf [ppp.MaxFrameBufLen]byte
			cfg := sessionConfig{mtu: 1492, username: "subscriber", password: "secret"}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			// RFC 1661 Section 3.5: premature IPCP must not survive authentication.
			err := runClientAuth(w, reader, buf[:], lcpResult{authProto: ppp.ProtoCHAP}, cfg, 1, stop, logger, new(uint8))
			if err == nil {
				// RFC 1332 Section 2: fresh input must complete both directions.
				_, err = negotiateIPCP(w, reader, buf[:], 1, 1492, nil, new(uint8), stop, logger, new(uint8), nil)
			}
			result <- err
		}()
		for range 5 {
			w.unit <- readFrame{data: exchangeFrame(ppp.ProtoIPCP, 5, 33, nil)}
		}
		synctest.Wait()
		if len(w.unit) != 0 {
			t.Fatal("premature population was not consumed during authentication")
		}
		w.quiet(t)
		w.authenticate(t, ppp.ProtoCHAP, true)
		request := w.take(t, ppp.ProtoIPCP, 1)
		w.send(ppp.ProtoIPCP, 3, request[3], []byte{3, 6, 192, 0, 2, 2})
		request = w.take(t, ppp.ProtoIPCP, 1)
		peer := []byte{3, 6, 192, 0, 2, 1}
		w.send(ppp.ProtoIPCP, 1, 94, peer)
		ack := w.take(t, ppp.ProtoIPCP, 2)
		if !bytes.Equal(ack, exchangeFrame(ppp.ProtoIPCP, 2, 94, peer)) {
			t.Fatalf("fresh Ack: %x", ack)
		}
		w.send(ppp.ProtoIPCP, 2, request[3], request[6:])
		if err := <-result; err != nil {
			t.Fatalf("fresh IPCP exchange failed after premature population: %v", err)
		}
	})
}
