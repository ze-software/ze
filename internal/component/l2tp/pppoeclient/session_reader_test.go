// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- bounded reader shutdown.
package pppoeclient

import (
	"io"
	"testing"
	"testing/synctest"
)

// saturatedFrameReader supplies four packets, then either a packet or an error.
// The pull reader must not read ahead while the consumer dispatches a packet.
// It has no blocking transport to close.
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

// TestSessionReaderStopsWithFullQueue preserves cancellation of blocked data
// and error delivery. With no read-ahead queue, the full boundary is one pending
// delivery. Four dispatched packets precede the blocked fifth delivery.
func TestSessionReaderStopsWithFullQueue(t *testing.T) {
	for _, lastError := range []bool{false, true} {
		synctest.Test(t, func(t *testing.T) {
			reader := &saturatedFrameReader{lastError: lastError}
			stop := make(chan struct{})
			owner, err := newFrameReader(reader.Read, stop)
			if err != nil {
				t.Fatal(err)
			}
			synctest.Wait()
			if reader.reads != 0 {
				t.Fatal("read before consumer authorization")
			}
			for count := 1; count <= 4; count++ {
				frame := <-owner.next()
				owner.received()
				if frame.err != nil {
					t.Fatalf("packet %d: %v", count, frame.err)
				}
				synctest.Wait()
				if reader.reads != count {
					t.Fatalf("reads = %d, want %d without read-ahead", reader.reads, count)
				}
			}
			frames := owner.next()
			if owner.next() != frames {
				t.Fatal("outstanding request changed its delivery source")
			}
			synctest.Wait()
			if reader.reads != 5 {
				t.Fatalf("reads = %d, want blocked fifth delivery", reader.reads)
			}
			close(stop)
			<-owner.done
			if frame, ok := <-frames; ok {
				t.Fatalf("blocked delivery survived stop: %+v", frame)
			}
			for range 3 {
				owner.received()
				if _, ok := <-owner.next(); ok {
					t.Fatal("terminal reader reopened")
				}
			}
		})
	}
}
