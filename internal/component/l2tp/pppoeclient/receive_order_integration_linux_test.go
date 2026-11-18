//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- ordered delivery ownership.
// Related: kernel/client_dispatch_integration_linux_test.go -- real sockets.

package pppoeclient

import (
	"bytes"
	"io"
	"testing"
	"testing/synctest"
)

// TestOrderedDescriptorDispatch retains the CPE half of the old combined root:
// one useful channel result, no second source read until dispatch acknowledges,
// then the exact useful unit result. The same-name shared kernel native root
// retains the original two actual socket descriptors, packet bytes, channel
// priority and MSG_PEEK assertion. These two roots MUST be judged together;
// this private logical protocol port grants no public original-file authority.
func TestOrderedDescriptorDispatch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		controlPacket := []byte{0xc2, 0x23, 3, 63, 0, 4}
		unitPacket := []byte{0x80, 0x21, 1, 71, 0, 10, 3, 6, 192, 0, 2, 1}
		packets := [][]byte{controlPacket, unitPacket}
		reads := 0
		client := testClientProtocol(&recordingRWC{})
		client.read = func(buffer []byte) (int, error) {
			if reads >= len(packets) {
				return 0, io.ErrUnexpectedEOF
			}
			packet := packets[reads]
			reads++
			return copy(buffer, packet), nil
		}
		link := &sessionLink{client: client, stopped: make(chan struct{}), closeTransport: func() error { return nil }}
		reader, err := newFrameReader(link.readPacket, link.stopped)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := link.Close(); err != nil {
				t.Error(err)
			}
			<-reader.done
		})
		delivery := reader.next()
		frame := <-delivery
		if frame.err != nil {
			t.Fatal(frame.err)
		}
		if !bytes.Equal(frame.data, controlPacket) {
			t.Fatalf("channel packet changed: %x", frame.data)
		}
		synctest.Wait()
		if reads != 1 {
			t.Fatalf("unit source read before dispatch: %d", reads)
		}
		if reader.next() != delivery {
			t.Fatal("outstanding delivery acquired another receive owner")
		}
		synctest.Wait()
		if reads != 1 {
			t.Fatalf("repeated next read unit before acknowledgment: %d", reads)
		}
		reader.received()
		frame = <-reader.next()
		reader.received()
		if frame.err != nil {
			t.Fatal(frame.err)
		}
		if !bytes.Equal(frame.data, unitPacket) {
			t.Fatalf("unit packet lost: %x", frame.data)
		}
		synctest.Wait()
		if reads != 2 {
			t.Fatalf("second dispatch source reads = %d", reads)
		}
	})
}
