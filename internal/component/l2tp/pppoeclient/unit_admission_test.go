// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- ordered receive boundaries.
// Related: receive.go -- bounded requests and terminal delivery.

package pppoeclient

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestUnitReaderPhaseBoundary consumes a premature unit packet in LCP and another
// during authentication, then proves a fresh unit request survives the transition
// to IPCP through the same production receive owner.
func TestUnitReaderPhaseBoundary(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		s, initial := w.session(t)
		reader := s.reader
		done, stop := make(chan struct{}), make(chan struct{})
		defer func() { close(stop); <-done }()
		go keepaliveLoop(s, done, stop, slog.New(slog.NewTextHandler(io.Discard, nil)))
		w.openLCP(t, ppp.ProtoCHAP, initial.magic)
		// The callback is now waiting for input while authentication is active.
		synctest.Wait()
		if !reader.pending {
			t.Fatal("authentication has no outstanding source read")
		}
		w.send(ppp.ProtoIPCP, 5, 33, nil)
		if len(w.unit) != 0 {
			t.Fatal("authentication retained premature unit input")
		}
		w.quiet(t)
		w.authenticate(t, ppp.ProtoCHAP, true)
		request := w.take(t, ppp.ProtoIPCP, 1)
		w.send(ppp.ProtoIPCP, 3, request[3], []byte{3, 6, 192, 0, 2, 2})
		request = w.take(t, ppp.ProtoIPCP, 1)
		fresh := exchangeFrame(ppp.ProtoIPCP, 1, 94, []byte{3, 6, 192, 0, 2, 1})
		w.unit <- readFrame{data: fresh}
		ack := w.take(t, ppp.ProtoIPCP, 2)
		want := exchangeFrame(ppp.ProtoIPCP, 2, 94, fresh[6:])
		if !bytes.Equal(ack, want) {
			t.Fatalf("fresh packet Ack %x, want %x", ack, want)
		}
		w.send(ppp.ProtoIPCP, 2, request[3], request[6:])
		if s.reader != reader {
			t.Fatal("phase transition replaced the receive owner")
		}
		synctest.Wait()
		if w.passes() != 0 {
			t.Fatalf("IPv4 passed before the interface applied the result: %v", w.modes)
		}
		applyPending(t, s)
		if w.passes() != 1 {
			t.Fatalf("fresh IPCP failed to resume IPv4: %v", w.modes)
		}
	})
}

// TestUnitReaderTerminalDelivery preserves terminal errors at idle and blocked
// delivery boundaries. Four packets precede the terminal fifth request where
// the old mechanism filled its queue. No phase-based admission can hide EOF.
func TestUnitReaderTerminalDelivery(t *testing.T) {
	for _, readErr := range []error{io.EOF, io.ErrUnexpectedEOF} {
		for _, name := range []string{"suspended", "full", "stop"} {
			t.Run(name+"/"+readErr.Error(), func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					input := make(chan readFrame, 5)
					stop := make(chan struct{})
					read := func(buf []byte) (int, error) {
						select {
						case frame := <-input:
							return copy(buf, frame.data), frame.err
						case <-stop:
							return 0, io.ErrClosedPipe
						}
					}
					reader, err := newFrameReader(read, stop)
					if err != nil {
						t.Fatal(err)
					}
					defer func() { close(stop); <-reader.done }()
					if name != "suspended" {
						for range 4 {
							packet := []byte{0x80, 0x21, 5, 33, 0, 4}
							input <- readFrame{data: packet}
							frame := <-reader.next()
							reader.received()
							if frame.err != nil {
								t.Fatal(frame.err)
							}
							if !bytes.Equal(frame.data, packet) {
								t.Fatalf("packet changed: %x", frame.data)
							}
						}
					}
					input <- readFrame{err: readErr}
					frames := reader.next()
					synctest.Wait()
					if name == "stop" {
						close(stop)
						<-reader.done
						if _, ok := <-frames; ok {
							t.Fatal("terminal delivery survived cancellation")
						}
						stop = make(chan struct{})
						return
					}
					frame := <-frames
					reader.received()
					if !errors.Is(frame.err, readErr) {
						t.Fatalf("terminal result %v, want %v", frame.err, readErr)
					}
					<-reader.done
					if _, ok := <-reader.next(); ok {
						t.Fatal("reader did not close after terminal error")
					}
				})
			})
		}
	}
}
