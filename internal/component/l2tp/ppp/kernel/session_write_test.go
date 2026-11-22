// Design: docs/research/l2tpv2-ze-integration.md -- cancellation of a borrowed PPP writer

package kernel

import (
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

// A pending control write must not hold the owner lock needed to close the
// stream that releases it. This also proves the write's error reaches PPP.
func TestKernelSessionCloseInterruptsBlockedControlWrite(t *testing.T) {
	control := &sessionBlockedWriter{
		entered: make(chan struct{}),
		closed:  make(chan struct{}),
	}
	unit := &sessionTestStream{closed: make(chan struct{})}
	var released []int
	ops := sessionTestOps(nil, &released)
	ops.wrap = func(fd int, _ string) (io.ReadWriteCloser, error) {
		if fd == 31 {
			return control, nil
		}
		return unit, nil
	}
	frames, handle, err := sessionTestNewSession(sessionTestConfig(), ops)
	if err != nil {
		t.Fatal(err)
	}
	writeDone := make(chan error, 1)
	go func() {
		_, writeErr := frames.Control().Write([]byte{0xc0, 0x21})
		writeDone <- writeErr
	}()
	<-control.entered
	closeDone := make(chan error, 1)
	go func() { closeDone <- handle.Close() }()
	select {
	case err := <-closeDone:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(time.Second):
		// Release the test-owned blocked stream before failing, so even the
		// counterfactual joins its two goroutines instead of leaking them.
		if err := control.Close(); err != nil {
			t.Error(err)
		}
		writeErr := <-writeDone
		closeErr := <-closeDone
		t.Fatalf("session close waited for a blocked write instead of interrupting it (write=%v, close=%v)", writeErr, closeErr)
	}
	if err := <-writeDone; !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("blocked write returned %v, want the close error", err)
	}
	if _, err := frames.Control().Write([]byte{0xc0, 0x21}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("write after close returned %v", err)
	}
}

type sessionBlockedWriter struct {
	entered chan struct{}
	closed  chan struct{}
	once    sync.Once
}

func (s *sessionBlockedWriter) Read([]byte) (int, error) {
	<-s.closed
	return 0, io.ErrClosedPipe
}

func (s *sessionBlockedWriter) Write([]byte) (int, error) {
	close(s.entered)
	<-s.closed
	return 0, io.ErrClosedPipe
}

func (s *sessionBlockedWriter) Close() error {
	s.once.Do(func() { close(s.closed) })
	return nil
}
