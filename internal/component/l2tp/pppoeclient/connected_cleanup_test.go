// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- sole-consumer shutdown.
// Related: dialer.go -- phase join; receive.go -- receive worker join.
package pppoeclient

import (
	"bytes"
	"io"
	"testing"
	"testing/synctest"
)

// TestConnectedCleanupJoinsConsumer keeps both original input packets owned by
// a paused consumer. Cleanup must stop transport, join that consumer, and then
// join the production reader before releasing the unit.
func TestConnectedCleanupJoinsConsumer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		transport := &cleanupTransport{}
		link := &sessionLink{channel: transport, stopped: make(chan struct{}), closeTransport: func() {}}
		input := make(chan readFrame, 2)
		input <- readFrame{data: []byte{0xc0, 0x21}}
		input <- readFrame{data: []byte{0x80, 0x21}}
		close(input)
		releaseRead := make(chan struct{})
		stop := make(chan struct{})
		reader, err := newFrameReader(func(buf []byte) (int, error) {
			frame, ok := <-input
			if !ok {
				<-releaseRead
				return 0, io.EOF
			}
			return copy(buf, frame.data), frame.err
		}, stop)
		if err != nil {
			t.Fatal(err)
		}
		advance, phaseDone := make(chan struct{}), make(chan struct{})
		phaseResult := make(chan []readFrame, 1)
		go func() {
			<-advance
			var consumed []readFrame
			for range 2 {
				consumed = append(consumed, <-reader.next())
				reader.received()
			}
			phaseResult <- consumed
			reader.next()
			close(phaseDone)
		}()
		cleanupDone := make(chan struct{})
		cleanupEntered, unitReleased := false, false
		go func() {
			link.cleanupConnected(phaseDone, func() {
				cleanupEntered = true
				_ = link.Close() //nolint:errcheck // test transport always succeeds
				<-reader.done
				unitReleased = true
			})
			close(cleanupDone)
		}()
		synctest.Wait()
		if !transport.closed {
			t.Error("cleanup did not stop transport before joining the phase worker")
		}
		if cleanupEntered {
			t.Error("cleanup competed for input before the phase worker exited")
		}
		if len(input) != 2 {
			t.Error("cleanup consumed input owned by the phase worker")
		}
		if unitReleased {
			t.Error("cleanup released the unit before the phase worker exited")
		}
		close(advance)
		synctest.Wait()
		if !cleanupEntered {
			t.Error("cleanup did not join the completed phase")
		}
		if unitReleased {
			t.Error("unit released while production read was still blocked")
		}
		close(stop)
		close(releaseRead)
		<-cleanupDone
		consumed := <-phaseResult
		if !bytes.Equal(consumed[0].data, []byte{0xc0, 0x21}) {
			t.Fatalf("control packet changed: %+v", consumed)
		}
		if !bytes.Equal(consumed[1].data, []byte{0x80, 0x21}) {
			t.Fatalf("unit packet changed: %+v", consumed)
		}
		if _, ok := <-reader.frames; ok {
			t.Fatal("delivery survived cleanup cancellation")
		}
		if !unitReleased {
			t.Fatal("cleanup did not release the unit after both joins")
		}
		if len(input) != 0 {
			t.Fatal("cleanup retained input after joining the phase worker")
		}
	})
}

type cleanupTransport struct {
	bytes.Buffer
	closed bool
}

func (c *cleanupTransport) Close() error { c.closed = true; return nil }
