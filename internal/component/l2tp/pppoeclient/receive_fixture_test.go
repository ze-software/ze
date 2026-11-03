// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- packet-source fixtures.
// Related: receive.go -- production pull worker; receive_linux.go -- raw ordering.

package pppoeclient

import (
	"io"
	"testing"
	"testing/synctest"
)

// testFrameReader keeps the original control and unit inputs, including their
// terminal errors, behind the production request/delivery worker. It performs no
// protocol parsing, filtering, phase changes, or delivery-channel injection.
// The channel priority here is a deterministic source fixture, not Linux proof.
// TestOrderedDescriptorDispatch and the real Dial tests exercise raw descriptors.
// Cleanup cancels a blocked source read and joins the production worker.
func testFrameReader(t *testing.T, control, unit <-chan readFrame) *frameReader {
	t.Helper()
	stop := make(chan struct{})
	read := func(buf []byte) (int, error) {
		var frame readFrame
		var ok bool
		select {
		case frame, ok = <-control:
		default:
			select {
			case <-stop:
				return 0, io.ErrClosedPipe
			case frame, ok = <-control:
			case frame, ok = <-unit:
			}
		}
		if !ok {
			return 0, io.EOF
		}
		if len(frame.data) > len(buf) {
			return 0, io.ErrShortBuffer
		}
		return copy(buf, frame.data), frame.err
	}
	reader, err := newFrameReader(read, stop)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		close(stop)
		<-reader.done
	})
	return reader
}

// TestFrameReaderAdmission checks constructor refusal without a worker to leak.
func TestFrameReaderAdmission(t *testing.T) {
	if reader, err := newFrameReader(nil, make(chan struct{})); err == nil || reader != nil {
		t.Fatal("nil read admitted")
	}
	if reader, err := newFrameReader(func([]byte) (int, error) { return 0, io.EOF }, nil); err == nil || reader != nil {
		t.Fatal("nil stop admitted")
	}
}

// TestFrameReaderStopsBlockedRead joins the actual worker after its source
// cancellation, and proves the source was entered before stop.
func TestFrameReaderStopsBlockedRead(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		entered, stop := make(chan struct{}), make(chan struct{})
		reader, err := newFrameReader(func([]byte) (int, error) {
			close(entered)
			<-stop
			return 0, io.ErrClosedPipe
		}, stop)
		if err != nil {
			t.Fatal(err)
		}
		frames := reader.next()
		<-entered
		close(stop)
		<-reader.done
		if _, ok := <-frames; ok {
			t.Fatal("blocked source delivery survived cancellation")
		}
	})
}
