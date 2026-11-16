// Design: docs/architecture/api/process-protocol.md -- fatal runtime correlation errors.
package ipc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRuntimeAbortClosesBeforeFilterAdmissionJoin proves a filter admission
// lock cannot keep the corrupt stream open or delay request cancellation.
// Method: hold the admission read lock, then observe EOF and the cancel hook
// before releasing it and allowing the filter shutdown to complete.
func TestRuntimeAbortClosesBeforeFilterAdmissionJoin(t *testing.T) {
	work, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	engine, client := net.Pipe()
	pc := NewPluginConn(engine, engine)
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	pc.filters.mu.RLock()
	locked := true
	defer func() {
		if locked {
			pc.filters.mu.RUnlock()
		}
	}()
	canceled := make(chan struct{})
	finished := make(chan error, 1)
	go func() { finished <- pc.AbortRuntime(func() { close(canceled) }) }()
	select {
	case <-canceled:
	case <-work.Done():
		t.Fatal("filter admission lock delayed runtime cancellation")
	}
	if reply, err := rpc.NewConn(client, client).ReadRequest(work); err == nil {
		t.Fatalf("corrupt stream remained open: %+v", reply)
	}
	if work.Err() != nil {
		t.Fatal("runtime transport did not close before filter join")
	}
	select {
	case err := <-finished:
		t.Fatalf("filter stop escaped its held admission lock: %v", err)
	default:
	}
	pc.filters.mu.RUnlock()
	locked = false
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-work.Done():
		t.Fatal(work.Err())
	}
}
