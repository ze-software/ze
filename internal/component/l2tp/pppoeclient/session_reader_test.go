// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- bounded reader shutdown.
package pppoeclient

import (
	"io"
	"testing"
	"testing/synctest"
)

// saturatedFrameReader fills the four-frame delivery queue before producing
// either a fifth frame or a read error. It has no blocking transport to close.
type saturatedFrameReader struct {
	reads     int
	lastError bool
}

func (r *saturatedFrameReader) Read(buf []byte) (int, error) {
	r.reads++
	if r.reads > 5 {
		return 0, io.EOF
	}
	if r.reads == 5 && r.lastError {
		return 0, io.ErrClosedPipe
	}
	return copy(buf, []byte{0xc0, 0x21, 1}), nil
}

// TestSessionReaderStopsWithFullQueue stops the reader while either data or
// error delivery is blocked, before any consumer drains the queue.
func TestSessionReaderStopsWithFullQueue(t *testing.T) {
	for _, lastError := range []bool{false, true} {
		synctest.Test(t, func(t *testing.T) {
			reader := &saturatedFrameReader{lastError: lastError}
			stop := make(chan struct{})
			frames := startReader(reader, stop)
			synctest.Wait()
			if reader.reads != 5 {
				t.Fatalf("reads = %d, want blocked fifth delivery", reader.reads)
			}
			close(stop)
			synctest.Wait()
			count := 0
			for frame := range frames {
				count++
				if frame.err != nil || count > 4 {
					t.Fatalf("blocked delivery survived stop: frame %+v, count %d", frame, count)
				}
			}
			if count != 4 {
				t.Fatalf("buffered frames = %d, want 4", count)
			}
		})
	}
}
