// Design: docs/architecture/l2tp/bng-5-pppoe.md -- PPPoX transport lifetime
// Related: transport.go -- serialized close and borrowed relay descriptor

package kernel

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
)

// TestTransportCloseAndRelayBorrow proves all concurrent closes report the
// original error and execute one release; no descriptor can be borrowed later.
func TestTransportCloseAndRelayBorrow(t *testing.T) {
	want := errors.New("close failed")
	var closes atomic.Int32
	owner := &pppoxTransport{fd: 41, closeFD: func(int) error { closes.Add(1); return want }}
	fd, err := owner.RelayDescriptor()
	if fd != 41 || err != nil {
		t.Fatalf("open borrow: fd=%d error=%v", fd, err)
	}
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			if err := owner.Close(); !errors.Is(err, want) {
				t.Errorf("close result: %v", err)
			}
		})
	}
	workers.Wait()
	if closes.Load() != 1 {
		t.Fatalf("descriptor released %d times", closes.Load())
	}
	if fd, err := owner.RelayDescriptor(); fd != -1 || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("closed borrow: fd=%d error=%v", fd, err)
	}
}
