package server

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/plugin/ipc"
	"codeberg.org/thomas-mangin/ze/internal/plugin/process"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// TestConcurrentPluginDispatch verifies that the engine dispatches plugin RPCs
// concurrently, not sequentially.
//
// Uses a barrier pattern: two requests are sent concurrently, and each handler
// blocks until BOTH handlers are active. With sequential dispatch, the second
// request can't be read until the first handler returns — deadlock. With
// concurrent dispatch, both handlers run in parallel and unblock each other.
//
// VALIDATES: AC-4 — Engine receives two requests on Socket A from same plugin;
//
//	both dispatched and processed; responses sent with correct IDs.
//
// VALIDATES: AC-10 — Engine concurrent dispatch with clean shutdown;
//
//	all in-flight dispatches complete before handler returns.
//
// PREVENTS: Regression to sequential dispatch that blocks the read loop.
func TestConcurrentPluginDispatch(t *testing.T) {
	t.Parallel()

	// Create socket pair for Socket A.
	pluginSide, engineSide := net.Pipe()

	// Create Process with engineConnA.
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-concurrent"})
	proc.SetConnA(ipc.NewPluginConn(engineSide, engineSide))

	// Barrier: both handlers must be active simultaneously for either to proceed.
	var barrier sync.WaitGroup
	barrier.Add(2)

	// Create a minimal server with a codec handler that uses the barrier.
	s := &Server{
		subscriptions: NewSubscriptionManager(),
		dispatcher:    NewDispatcher(),
		rpcFallback: func(method string) func(json.RawMessage) (any, error) {
			if method == "test:barrier" {
				return func(_ json.RawMessage) (any, error) {
					barrier.Done() // signal arrival
					barrier.Wait() // wait for both to arrive
					return map[string]string{"status": "ok"}, nil
				}
			}
			return nil
		},
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	// Start the runtime handler in a goroutine.
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		s.handleSingleProcessCommandsRPC(proc)
	}()

	// Plugin side uses MuxConn for concurrent writes.
	pluginConn := rpc.NewConn(pluginSide, pluginSide)
	mux := rpc.NewMuxConn(pluginConn)
	defer func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
	}()

	// Send two concurrent requests. With sequential dispatch, this times out
	// because the barrier requires both handlers to be active simultaneously.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	errs := make([]error, 2)

	for i := range 2 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, callErr := mux.CallRPC(ctx, "test:barrier", nil)
			errs[idx] = callErr
		}(i)
	}

	wg.Wait()

	require.NoError(t, errs[0], "request 0 should succeed")
	require.NoError(t, errs[1], "request 1 should succeed")

	// Clean shutdown: close connections, verify handler exits.
	if err := pluginSide.Close(); err != nil {
		t.Logf("pluginSide close: %v", err)
	}
	if err := engineSide.Close(); err != nil {
		t.Logf("engineSide close: %v", err)
	}

	select {
	case <-handlerDone:
		// Handler exited cleanly.
	case <-time.After(2 * time.Second):
		t.Fatal("handleSingleProcessCommandsRPC did not exit after connection close")
	}
}
