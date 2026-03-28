package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeEngine starts a goroutine that reads requests from conn and sends
// responses with matching IDs. The handler func receives the request and
// returns the result to embed in the response. Closing ctx stops the engine.
func fakeEngine(ctx context.Context, conn net.Conn, handler func(*Request) any) {
	rpcConn := NewConn(conn, conn)
	go func() {
		for {
			req, err := rpcConn.ReadRequest(ctx)
			if err != nil {
				return
			}
			result := handler(req)
			if sendErr := rpcConn.SendResult(ctx, req.ID, result); sendErr != nil {
				return
			}
		}
	}()
}

// closePipe closes a net.Conn and logs failures to t.
func closePipe(t *testing.T, name string, c net.Conn) {
	t.Helper()
	if err := c.Close(); err != nil {
		t.Logf("close %s: %v", name, err)
	}
}

// TestMuxConn_SequentialCallRPC verifies that sequential calls work correctly.
//
// VALIDATES: AC-11 -- Conn.CallRPC behavior preserved; sequential MuxConn calls work.
// PREVENTS: Regression in basic call/response matching.
func TestMuxConn_SequentialCallRPC(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "pluginEnd", pluginEnd)
	defer closePipe(t, "engineEnd", engineEnd)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Engine echoes back method name as result.
	fakeEngine(ctx, engineEnd, func(req *Request) any {
		return map[string]string{"method": req.Method}
	})

	conn := NewConn(pluginEnd, pluginEnd)
	mux := NewMuxConn(conn)
	defer func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
	}()

	// Two sequential calls. CallRPC returns result payload directly.
	raw1, err := mux.CallRPC(ctx, "test-method-1", nil)
	require.NoError(t, err)
	var result1 struct {
		Method string `json:"method"`
	}
	require.NoError(t, json.Unmarshal(raw1, &result1))
	assert.Equal(t, "test-method-1", result1.Method)

	raw2, err := mux.CallRPC(ctx, "test-method-2", nil)
	require.NoError(t, err)
	var result2 struct {
		Method string `json:"method"`
	}
	require.NoError(t, json.Unmarshal(raw2, &result2))
	assert.Equal(t, "test-method-2", result2.Method)
}

// TestMuxConn_ConcurrentCallRPC verifies two concurrent calls get correct responses.
//
// VALIDATES: AC-1 -- Two goroutines call MuxConn.CallRPC concurrently; each receives its own response.
// PREVENTS: Response misrouting when multiple callers share a connection.
func TestMuxConn_ConcurrentCallRPC(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "pluginEnd", pluginEnd)
	defer closePipe(t, "engineEnd", engineEnd)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Engine reads both requests first, then sends both responses, ensuring
	// both calls are in-flight simultaneously on the MuxConn side.
	go func() {
		rpcConn := NewConn(engineEnd, engineEnd)
		req1, readErr := rpcConn.ReadRequest(ctx)
		if readErr != nil {
			return
		}
		req2, readErr := rpcConn.ReadRequest(ctx)
		if readErr != nil {
			return
		}
		if sendErr := rpcConn.SendResult(ctx, req1.ID, map[string]string{"method": req1.Method}); sendErr != nil {
			return
		}
		if sendErr := rpcConn.SendResult(ctx, req2.ID, map[string]string{"method": req2.Method}); sendErr != nil {
			return
		}
	}()

	conn := NewConn(pluginEnd, pluginEnd)
	mux := NewMuxConn(conn)
	defer func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
	}()

	type callResult struct {
		method string
		err    error
	}

	ch1 := make(chan callResult, 1)
	ch2 := make(chan callResult, 1)

	// Launch two concurrent calls.
	go func() {
		raw, callErr := mux.CallRPC(ctx, "method-alpha", nil)
		if callErr != nil {
			ch1 <- callResult{err: callErr}
			return
		}
		var result struct {
			Method string `json:"method"`
		}
		if unmarshalErr := json.Unmarshal(raw, &result); unmarshalErr != nil {
			ch1 <- callResult{err: unmarshalErr}
			return
		}
		ch1 <- callResult{method: result.Method}
	}()

	go func() {
		raw, callErr := mux.CallRPC(ctx, "method-beta", nil)
		if callErr != nil {
			ch2 <- callResult{err: callErr}
			return
		}
		var result struct {
			Method string `json:"method"`
		}
		if unmarshalErr := json.Unmarshal(raw, &result); unmarshalErr != nil {
			ch2 <- callResult{err: unmarshalErr}
			return
		}
		ch2 <- callResult{method: result.Method}
	}()

	r1 := <-ch1
	r2 := <-ch2

	require.NoError(t, r1.err)
	require.NoError(t, r2.err)
	assert.Equal(t, "method-alpha", r1.method)
	assert.Equal(t, "method-beta", r2.method)
}

// TestMuxConn_ContextCancellation verifies that context cancellation unblocks waiting callers.
//
// VALIDATES: AC-2 -- CallRPC with canceled context returns context error; pending entry cleaned up.
// PREVENTS: Goroutine leaks when callers time out or cancel.
func TestMuxConn_ContextCancellation(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "pluginEnd", pluginEnd)
	defer closePipe(t, "engineEnd", engineEnd)

	// Engine that never responds -- simulates timeout.
	go func() {
		rpcConn := NewConn(engineEnd, engineEnd)
		for {
			if _, err := rpcConn.ReadRequest(context.Background()); err != nil {
				return
			}
			// Deliberately don't send a response.
		}
	}()

	conn := NewConn(pluginEnd, pluginEnd)
	mux := NewMuxConn(conn)
	defer func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
	}()

	// Call with a short deadline.
	shortCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := mux.CallRPC(shortCtx, "never-responds", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// TestMuxConn_CloseUnblocksPending verifies that Close() unblocks all waiting callers.
//
// VALIDATES: AC-3 -- Close() while CallRPC is waiting; waiting callers unblock with connection-closed error.
// PREVENTS: Goroutine leaks when MuxConn is closed during active RPCs.
func TestMuxConn_CloseUnblocksPending(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "engineEnd", engineEnd)

	// Engine that never responds.
	go func() {
		rpcConn := NewConn(engineEnd, engineEnd)
		for {
			if _, err := rpcConn.ReadRequest(context.Background()); err != nil {
				return
			}
		}
	}()

	conn := NewConn(pluginEnd, pluginEnd)
	mux := NewMuxConn(conn)

	ctx := context.Background()

	// Start a call that will block.
	errCh := make(chan error, 1)
	go func() {
		_, callErr := mux.CallRPC(ctx, "will-be-closed", nil)
		errCh <- callErr
	}()

	// Wait for the call to register in the pending map.
	require.Eventually(t, func() bool {
		found := false
		mux.pending.Range(func(_, _ any) bool {
			found = true
			return false
		})
		return found
	}, 2*time.Second, time.Millisecond, "call should register in pending map")

	// Close the mux -- should unblock the caller.
	require.NoError(t, mux.Close())

	closeCtx, closeCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer closeCancel()

	select {
	case err := <-errCh:
		require.Error(t, err, "CallRPC should return an error after Close()")
	case <-closeCtx.Done():
		t.Fatal("CallRPC did not unblock after Close()")
	}
}

// TestMuxConn_ManyConcurrent verifies 100 concurrent calls all succeed.
//
// VALIDATES: AC-5 -- 100 concurrent MuxConn.CallRPC calls complete without deadlock; each response matches its request ID.
// PREVENTS: Deadlocks or response misrouting under high concurrency.
func TestMuxConn_ManyConcurrent(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "pluginEnd", pluginEnd)
	defer closePipe(t, "engineEnd", engineEnd)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Engine responds with the method name.
	fakeEngine(ctx, engineEnd, func(req *Request) any {
		return map[string]string{"method": req.Method}
	})

	conn := NewConn(pluginEnd, pluginEnd)
	mux := NewMuxConn(conn)
	defer func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
	}()

	const n = 100
	var wg sync.WaitGroup
	results := make([]string, n)
	errs := make([]error, n)

	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			method := fmt.Sprintf("method-%d", idx)
			raw, callErr := mux.CallRPC(ctx, method, nil)
			if callErr != nil {
				errs[idx] = callErr
				return
			}
			// CallRPC returns result payload directly.
			var result struct {
				Method string `json:"method"`
			}
			if unmarshalErr := json.Unmarshal(raw, &result); unmarshalErr != nil {
				errs[idx] = unmarshalErr
				return
			}
			results[idx] = result.Method
		}(i)
	}

	wg.Wait()

	for i := range n {
		require.NoError(t, errs[i], "call %d should succeed", i)
		assert.Equal(t, fmt.Sprintf("method-%d", i), results[i], "call %d should get correct response", i)
	}
}

// TestMuxConn_ReaderError verifies that a connection error unblocks all pending callers.
//
// VALIDATES: AC-6 -- MuxConn background reader encounters connection error; all pending callers unblock with the error; no goroutine leak.
// PREVENTS: Goroutine leaks when the underlying connection breaks.
func TestMuxConn_ReaderError(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()

	// Engine reads one request, waits for both calls to be pending, then closes.
	bothPending := make(chan struct{})
	go func() {
		rpcConn := NewConn(engineEnd, engineEnd)
		if _, err := rpcConn.ReadRequest(context.Background()); err != nil {
			return
		}
		// Wait until test signals both calls are pending.
		<-bothPending
		if err := engineEnd.Close(); err != nil {
			return
		}
	}()

	conn := NewConn(pluginEnd, pluginEnd)
	mux := NewMuxConn(conn)
	defer func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
	}()

	ctx := context.Background()

	// Start two calls that will be pending when the connection breaks.
	errCh1 := make(chan error, 1)
	errCh2 := make(chan error, 1)

	go func() {
		_, callErr := mux.CallRPC(ctx, "call-1", nil)
		errCh1 <- callErr
	}()
	go func() {
		_, callErr := mux.CallRPC(ctx, "call-2", nil)
		errCh2 <- callErr
	}()

	// Wait for both calls to register in the pending map before breaking connection.
	require.Eventually(t, func() bool {
		count := 0
		mux.pending.Range(func(_, _ any) bool {
			count++
			return true
		})
		return count >= 2
	}, 2*time.Second, time.Millisecond, "both calls should register in pending map")
	close(bothPending)

	readerCtx, readerCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer readerCancel()

	select {
	case err := <-errCh1:
		require.Error(t, err, "call-1 should fail after connection error")
	case <-readerCtx.Done():
		t.Fatal("call-1 did not unblock after connection error")
	}

	select {
	case err := <-errCh2:
		require.Error(t, err, "call-2 should fail after connection error")
	case <-readerCtx.Done():
		t.Fatal("call-2 did not unblock after connection error")
	}
}

// TestMuxConn_InboundRequest verifies that MuxConn routes inbound requests
// (verb is a method name, not ok/error) to the Requests() channel.
//
// VALIDATES: Bidirectional MuxConn routes inbound requests to Requests() channel.
// PREVENTS: Inbound requests being dropped as orphaned responses.
func TestMuxConn_InboundRequest(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "pluginEnd", pluginEnd)
	defer closePipe(t, "engineEnd", engineEnd)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn := NewConn(pluginEnd, pluginEnd)
	mux := NewMuxConn(conn)
	defer func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
	}()

	engineConn := NewConn(engineEnd, engineEnd)

	// Engine sends a request to the plugin.
	go func() {
		line := FormatRequest(1, "ze-plugin-callback:configure", json.RawMessage(`{"sections":[]}`))
		_ = engineConn.writeLineWithContext(ctx, line)
	}()

	// Plugin receives the request via Requests() channel.
	select {
	case req := <-mux.Requests():
		assert.Equal(t, uint64(1), req.ID)
		assert.Equal(t, "ze-plugin-callback:configure", req.Method)
		assert.JSONEq(t, `{"sections":[]}`, string(req.Params))
	case <-ctx.Done():
		t.Fatal("timed out waiting for inbound request")
	}
}

// TestMuxConn_MixedTraffic verifies MuxConn correctly separates interleaved
// responses (to our outbound calls) and requests (from the other side).
//
// VALIDATES: Responses routed to CallRPC callers, requests routed to Requests().
// PREVENTS: Responses and requests being confused when interleaved on one connection.
func TestMuxConn_MixedTraffic(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "pluginEnd", pluginEnd)
	defer closePipe(t, "engineEnd", engineEnd)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn := NewConn(pluginEnd, pluginEnd)
	mux := NewMuxConn(conn)
	defer func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
	}()

	engineConn := NewConn(engineEnd, engineEnd)

	// Engine: read our outbound request, send back a response AND an inbound request.
	go func() {
		// Read the outbound request from the plugin.
		req, err := engineConn.ReadRequest(ctx)
		if err != nil {
			return
		}
		// Send response to the outbound request.
		_ = engineConn.SendResult(ctx, req.ID, map[string]string{"status": "ok"})
		// Also send an inbound request from engine to plugin.
		line := FormatRequest(100, "ze-plugin-callback:deliver-batch", json.RawMessage(`{"events":["e1"]}`))
		_ = engineConn.writeLineWithContext(ctx, line)
	}()

	// Plugin sends an outbound call.
	raw, err := mux.CallRPC(ctx, "ze-plugin-engine:declare-registration", nil)
	require.NoError(t, err)
	var result struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(raw, &result))
	assert.Equal(t, "ok", result.Status)

	// Plugin should also receive the inbound request on Requests().
	select {
	case req := <-mux.Requests():
		assert.Equal(t, uint64(100), req.ID)
		assert.Equal(t, "ze-plugin-callback:deliver-batch", req.Method)
	case <-ctx.Done():
		t.Fatal("timed out waiting for inbound request in mixed traffic")
	}
}

// TestMuxConn_SendResultToInbound verifies that SendResult/SendOK/SendError
// can respond to inbound requests received via Requests().
//
// VALIDATES: Plugin can respond to engine-initiated requests on a single connection.
// PREVENTS: Deadlock or failure when responding to inbound requests.
func TestMuxConn_SendResultToInbound(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "pluginEnd", pluginEnd)
	defer closePipe(t, "engineEnd", engineEnd)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn := NewConn(pluginEnd, pluginEnd)
	mux := NewMuxConn(conn)
	defer func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
	}()

	engineConn := NewConn(engineEnd, engineEnd)

	// Engine sends a request and expects a response.
	responseCh := make(chan json.RawMessage, 1)
	go func() {
		raw, callErr := engineConn.CallRPC(ctx, "ze-plugin-callback:configure", nil)
		if callErr != nil {
			return
		}
		responseCh <- raw
	}()

	// Plugin receives the inbound request.
	select {
	case req := <-mux.Requests():
		assert.Equal(t, "ze-plugin-callback:configure", req.Method)
		// Respond via MuxConn.
		require.NoError(t, mux.SendOK(ctx, req.ID))
	case <-ctx.Done():
		t.Fatal("timed out waiting for inbound request")
	}

	// Engine should receive the response.
	select {
	case <-responseCh:
		// Success -- engine got the response.
	case <-ctx.Done():
		t.Fatal("engine timed out waiting for response from plugin")
	}
}

// TestMuxConn_UnexpectedID verifies that orphan responses don't crash or deadlock.
//
// VALIDATES: AC-9 -- MuxConn response ID mismatch (unexpected ID arrives); logged as warning; does not crash or deadlock.
// PREVENTS: Panics or deadlocks when the engine sends a response for an already-canceled request.
func TestMuxConn_UnexpectedID(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "pluginEnd", pluginEnd)
	defer closePipe(t, "engineEnd", engineEnd)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	engineConn := NewConn(engineEnd, engineEnd)

	// Engine sends a spurious response with ID 999 first, then the real response.
	go func() {
		req, err := engineConn.ReadRequest(ctx)
		if err != nil {
			return
		}

		// Send a spurious response with a different ID.
		if sendErr := engineConn.SendOK(ctx, 999); sendErr != nil {
			return
		}

		// Then send the real response.
		if sendErr := engineConn.SendOK(ctx, req.ID); sendErr != nil {
			return
		}
	}()

	conn := NewConn(pluginEnd, pluginEnd)
	mux := NewMuxConn(conn)
	defer func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
	}()

	// The call should succeed despite the spurious response.
	_, err := mux.CallRPC(ctx, "test-method", nil)
	require.NoError(t, err)
}
