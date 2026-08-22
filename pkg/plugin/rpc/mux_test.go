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

// TestMuxConnDeliversEveryRecordToOneCaller checks that every line of one
// answer reaches the caller that asked for it. The method: a peer answers a
// single request with a head, two record lines and a terminator, all under the
// caller's id, and the caller counts the records it took delivery of.
//
// VALIDATES: R-1 -- the pending entry survives every line of an answer, not
// only the first.
// PREVENTS: an answer arriving as its head alone, with every record orphaned by
// the LoadAndDelete that routed the head.
func TestMuxConnDeliversEveryRecordToOneCaller(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "pluginEnd", pluginEnd)
	defer closePipe(t, "engineEnd", engineEnd)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		engineConn := NewConn(engineEnd, engineEnd)
		req, err := engineConn.ReadRequest(ctx)
		if err != nil {
			return
		}
		answer := []string{
			wireLine(req.ID, "ok status=done type=ndjson key=peers\n"),
			wireLine(req.ID, "ok item={\"peer\":\"10.0.0.1\"}\n"),
			wireLine(req.ID, "ok item={\"peer\":\"10.0.0.2\"}\n"),
			wireLine(req.ID, "ok count=2\n"),
		}
		for _, line := range answer {
			if _, writeErr := engineEnd.Write([]byte(line)); writeErr != nil {
				return
			}
		}
	}()

	mux := NewMuxConn(NewConn(pluginEnd, pluginEnd))
	defer func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
	}()

	received, err := mux.CallAnswer(ctx, "ze-bgp:peer-list", nil)
	require.NoError(t, err)

	var items []string
	for record := range received.Records {
		items = append(items, string(record.Item))
	}

	assert.Equal(t, "peers", received.Key, "the head names the envelope its records belong under")
	assert.Equal(t, []string{`{"peer":"10.0.0.1"}`, `{"peer":"10.0.0.2"}`}, items,
		"every record line of the answer must reach the one caller waiting on that id")
	assert.Equal(t, VerdictDone, received.Verdict(), "the terminator carried count=2 faults=0")
	assert.NoError(t, received.Err(), "an answer that reached its terminator ended with no fault")
}

// writeLines writes each line to conn and stops at the first write error, which
// is what a closed pipe gives a peer goroutine.
func writeLines(conn net.Conn, lines ...string) {
	for _, line := range lines {
		if _, err := conn.Write([]byte(line)); err != nil {
			return
		}
	}
}

// TestSingleResponseCallRPCPathUnchanged checks that a peer answering with
// today's one-line frame still reaches its caller byte for byte. The method: a
// peer answers a CallRPC with #<len>:<id> ok <json>, and the test compares the
// returned payload against the bytes it wrote and then checks the pending entry
// is gone.
//
// VALIDATES: AC-13 -- a peer that has not negotiated the answer shape sees the
// path it saw before, and its entry still lives for exactly one line.
// PREVENTS: the pending lifetime a record sequence needs leaking into the
// single-response path, where it would leave every completed call registered.
func TestSingleResponseCallRPCPathUnchanged(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "pluginEnd", pluginEnd)
	defer closePipe(t, "engineEnd", engineEnd)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const payload = `{"peers":[{"peer":"10.0.0.1"}],"count":1}`

	go func() {
		engineConn := NewConn(engineEnd, engineEnd)
		req, err := engineConn.ReadRequest(ctx)
		if err != nil {
			return
		}
		writeLines(engineEnd, wireLine(req.ID, fmt.Sprintf("ok %s\n", payload)))
	}()

	mux := NewMuxConn(NewConn(pluginEnd, pluginEnd))
	defer func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
	}()

	result, err := mux.CallRPC(ctx, "ze-bgp:peer-list", nil)
	require.NoError(t, err)
	assert.Equal(t, payload, string(result), "the caller gets the peer's payload unchanged")

	_, found := mux.pending.Load(pendingKey(1))
	assert.False(t, found, "a CallRPC entry is released as its one response is routed")
}

// TestVerbPredicateUnchangedForResponses checks that readLoop still splits
// responses from requests on the verb alone. The method: the peer sends a line
// whose verb is a method name under the id of a call that is waiting, and the
// test asserts it arrives as an inbound request while the call keeps waiting.
//
// VALIDATES: readLoop's isResponse predicate is still verb == StatusOK ||
// verb == StatusError, so no plugin's inbound dispatch moved.
// PREVENTS: an answer-shape reader claiming a line by its id and swallowing a
// request the plugin was meant to serve.
func TestVerbPredicateUnchangedForResponses(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "pluginEnd", pluginEnd)
	defer closePipe(t, "engineEnd", engineEnd)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		engineConn := NewConn(engineEnd, engineEnd)
		req, err := engineConn.ReadRequest(ctx)
		if err != nil {
			return
		}
		// Same id as the waiting call, but the verb is a method name.
		writeLines(engineEnd,
			wireLine(req.ID, "deliver-event {\"kind\":\"route\"}\n"),
			wireLine(req.ID, "ok {\"served\":true}\n"),
		)
	}()

	mux := NewMuxConn(NewConn(pluginEnd, pluginEnd))
	defer func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
	}()

	result, err := mux.CallRPC(ctx, "ze-bgp:peer-list", nil)
	require.NoError(t, err)
	assert.Equal(t, `{"served":true}`, string(result), "only the ok line answered the call")

	select {
	case req := <-mux.Requests():
		assert.Equal(t, "deliver-event", req.Method, "a method verb is an inbound request whatever its id")
	case <-ctx.Done():
		t.Fatal("the line whose verb is a method name never reached Requests()")
	}
}

// TestOrphanRecordDoesNotCloseConnection checks that record lines for an answer
// nobody waits on leave the connection alive. The method: the peer writes more
// orphaned record lines than the flood guard's threshold, then answers a real
// call on the same connection.
//
// VALIDATES: AC-16, R-2 -- a record for an unknown id is discarded without
// counting toward maxConsecutiveBadLines.
// PREVENTS: a canceled answer's in-flight records killing a live plugin
// connection, which takes every other id down with it.
func TestOrphanRecordDoesNotCloseConnection(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "pluginEnd", pluginEnd)
	defer closePipe(t, "engineEnd", engineEnd)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const orphans = maxConsecutiveBadLines + 50

	go func() {
		engineConn := NewConn(engineEnd, engineEnd)
		req, err := engineConn.ReadRequest(ctx)
		if err != nil {
			return
		}
		for i := range orphans {
			line := wireLine(999, fmt.Sprintf("ok item={\"row\":%d}\n", i))
			if _, writeErr := engineEnd.Write([]byte(line)); writeErr != nil {
				return
			}
		}
		writeLines(engineEnd, wireLine(req.ID, "ok {\"alive\":true}\n"))
	}()

	mux := NewMuxConn(NewConn(pluginEnd, pluginEnd))
	defer func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
	}()

	result, err := mux.CallRPC(ctx, "ze-bgp:peer-list", nil)
	require.NoError(t, err, "%d orphaned records must not close the connection", orphans)
	assert.Equal(t, `{"alive":true}`, string(result))
}

// TestOrphanJunkStillClosesConnection checks that the flood guard was narrowed
// rather than removed. The method: the peer writes more orphaned lines than the
// threshold, none of which reads as an answer tail, and the test asserts the
// call fails because readLoop closed the connection.
//
// VALIDATES: maxConsecutiveBadLines still fires for junk, which is what makes
// TestOrphanRecordDoesNotCloseConnection a statement about records rather than
// about a guard that no longer works.
// PREVENTS: silently disabling the flood guard while making orphaned records
// harmless.
func TestOrphanJunkStillClosesConnection(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "pluginEnd", pluginEnd)
	defer closePipe(t, "engineEnd", engineEnd)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const junk = maxConsecutiveBadLines + 50

	go func() {
		engineConn := NewConn(engineEnd, engineEnd)
		if _, err := engineConn.ReadRequest(ctx); err != nil {
			return
		}
		for i := range junk {
			line := wireLine(999, fmt.Sprintf("ok {\"row\":%d}\n", i))
			if _, writeErr := engineEnd.Write([]byte(line)); writeErr != nil {
				return
			}
		}
	}()

	mux := NewMuxConn(NewConn(pluginEnd, pluginEnd))
	defer func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
	}()

	_, err := mux.CallRPC(ctx, "ze-bgp:peer-list", nil)
	require.Error(t, err, "%d orphaned JSON responses must still trip the flood guard", junk)
	assert.Contains(t, err.Error(), "consecutive malformed lines")
}

// TestSlowConsumerDoesNotStallReadLoop checks that one caller that stops
// reading its records cannot stop the connection. The method: a peer writes
// more record lines than the queue holds for a caller that reads none, then
// answers a second call, and the test asserts the second call returns and the
// first is told its answer was abandoned.
//
// VALIDATES: AC-17, R-5 -- readLoop never waits on a consumer, and a record it
// cannot deliver becomes a reported fault rather than silence.
// PREVENTS: the reader goroutine blocking on a chan send, which stops every
// other id on the connection with it.
func TestSlowConsumerDoesNotStallReadLoop(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "pluginEnd", pluginEnd)
	defer closePipe(t, "engineEnd", engineEnd)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	const records = answerQueueDepth + 100

	go func() {
		engineConn := NewConn(engineEnd, engineEnd)
		slow, err := engineConn.ReadRequest(ctx)
		if err != nil {
			return
		}
		writeLines(engineEnd, wireLine(slow.ID, "ok status=done type=ndjson key=peers\n"))
		for i := range records {
			line := wireLine(slow.ID, fmt.Sprintf("ok item={\"row\":%d}\n", i))
			if _, writeErr := engineEnd.Write([]byte(line)); writeErr != nil {
				return
			}
		}
		writeLines(engineEnd, wireLine(slow.ID, fmt.Sprintf("ok count=%d\n", records)))

		other, err := engineConn.ReadRequest(ctx)
		if err != nil {
			return
		}
		writeLines(engineEnd, wireLine(other.ID, "ok {\"flowing\":true}\n"))
	}()

	mux := NewMuxConn(NewConn(pluginEnd, pluginEnd))
	defer func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
	}()

	// The slow caller takes its head and then reads nothing until the second
	// call has been answered.
	slow, err := mux.CallAnswer(ctx, "ze-bgp:peer-list", nil)
	require.NoError(t, err)

	result, err := mux.CallRPC(ctx, "ze-bgp:overview", nil)
	require.NoError(t, err, "a caller that stopped reading must not stall the reader for every other id")
	assert.Equal(t, `{"flowing":true}`, string(result))

	delivered := 0
	for range slow.Records {
		delivered++
	}
	assert.Less(t, delivered, records, "the queue is bounded, so the abandoned answer is short")
	assert.ErrorIs(t, slow.Err(), ErrAnswerQueueFull, "the records it did not get are reported, never dropped in silence")
	assert.Equal(t, VerdictTruncated, slow.Verdict(), "an abandoned answer never reads as complete")
}

// TestAnswerWithoutTerminatorReportsTruncation checks that an answer cut short
// is reported as truncated. The method: a peer writes a head and one record and
// then closes the connection, and the test asserts the consumer sees the
// records it got plus a truncated verdict.
//
// VALIDATES: AC-9 -- an answer whose connection dies before the terminator is
// truncation, not a short answer.
// PREVENTS: a consumer rendering half a table as the whole table.
func TestAnswerWithoutTerminatorReportsTruncation(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "pluginEnd", pluginEnd)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		engineConn := NewConn(engineEnd, engineEnd)
		req, err := engineConn.ReadRequest(ctx)
		if err != nil {
			return
		}
		writeLines(engineEnd,
			wireLine(req.ID, "ok status=done type=ndjson key=peers\n"),
			wireLine(req.ID, "ok item={\"peer\":\"10.0.0.1\"}\n"),
		)
		if closeErr := engineEnd.Close(); closeErr != nil {
			return
		}
	}()

	mux := NewMuxConn(NewConn(pluginEnd, pluginEnd))
	defer func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
	}()

	cut, err := mux.CallAnswer(ctx, "ze-bgp:peer-list", nil)
	require.NoError(t, err)

	delivered := 0
	for range cut.Records {
		delivered++
	}

	assert.Equal(t, 1, delivered, "the record that did arrive is still delivered")
	assert.Equal(t, VerdictTruncated, cut.Verdict(), "no terminator arrived, so the answer is truncated")
	assert.Error(t, cut.Err(), "the consumer is told why the answer stopped")
}

// TestNotUnderstoodAnswerReachesTheCaller checks that the error verb ends an
// answer as an error rather than as an empty result. The method: a peer answers
// a CallAnswer with one #<len>:<id> error message= line, and the test asserts the
// call fails with that message.
//
// VALIDATES: AC-4 -- the not-understood answer is the only line for its id, and
// CallAnswer surfaces it instead of waiting for a head.
// PREVENTS: a mistyped command hanging the caller until its context expires.
func TestNotUnderstoodAnswerReachesTheCaller(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "pluginEnd", pluginEnd)
	defer closePipe(t, "engineEnd", engineEnd)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		engineConn := NewConn(engineEnd, engineEnd)
		req, err := engineConn.ReadRequest(ctx)
		if err != nil {
			return
		}
		writeLines(engineEnd, wireLine(req.ID, "error message=unknown command: shwo bgp peers\n"))
	}()

	mux := NewMuxConn(NewConn(pluginEnd, pluginEnd))
	defer func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
	}()

	_, err := mux.CallAnswer(ctx, "shwo bgp peers", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command: shwo bgp peers")
}

// TestCallAnswerStopsGeneratorOnConsumerStop checks that a consumer which stops
// reading stops the walk behind it, on both transports. The method: one row
// table drives a peer on a pipe and an in-process producer, the consumer takes
// one record and leaves the range, and the test reads how far each producer got.
//
// Over the socket the walk runs in the peer, so what stopping buys is detaching
// from the answer: the pending entry goes, the lines the peer still writes are
// discarded rather than counted as junk, and the connection carries the next
// call. In process the walk runs under the consumer's own range, so stopping it
// stops the generator on the row it stopped at.
//
// VALIDATES: `| first 10` over a table nobody wants whole costs ten rows, and a
// stop leaves no terminator, so Verdict reads truncated on both transports.
// PREVENTS: a consumer stop that lets the producer run to the end of a million
// rows, and a stop that leaves the connection unusable for the next call.
func TestCallAnswerStopsGeneratorOnConsumerStop(t *testing.T) {
	t.Parallel()

	items := []string{`{"peer":"10.0.0.1"}`, `{"peer":"10.0.0.2"}`, `{"peer":"10.0.0.3"}`}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "pluginEnd", pluginEnd)
	defer closePipe(t, "engineEnd", engineEnd)

	// The peer writes the whole answer and then serves one more call, which is
	// what proves the discarded lines left the connection usable.
	go func() {
		engineConn := NewConn(engineEnd, engineEnd)
		request, err := engineConn.ReadRequest(ctx)
		if err != nil {
			return
		}
		lines := [][]byte{AppendAnswerHead(nil, request.ID, StatusDone, AnswerTypeNDJSON, "peers", nil)}
		for _, item := range items {
			lines = append(lines, AppendAnswerItem(nil, request.ID, json.RawMessage(item)))
		}
		lines = append(lines, AppendAnswerTerminator(nil, request.ID, uint64(len(items)), 0, ""))
		for _, line := range lines {
			if _, writeErr := engineEnd.Write(append(line, '\n')); writeErr != nil {
				return
			}
		}
		next, err := engineConn.ReadRequest(ctx)
		if err != nil {
			return
		}
		if sendErr := engineConn.SendResult(ctx, next.ID, json.RawMessage(`{"awake":true}`)); sendErr != nil {
			return
		}
	}()

	mux := NewMuxConn(NewConn(pluginEnd, pluginEnd))
	defer func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
	}()

	overSocket, err := mux.CallAnswer(ctx, MethodDispatchCommand, &DispatchCommandInput{Command: "show bgp neighbor summary"})
	require.NoError(t, err)

	socketRead := 0
	for range overSocket.Records {
		socketRead++
		break
	}
	assert.Equal(t, 1, socketRead, "the consumer took one record and left")
	assert.Equal(t, VerdictTruncated, overSocket.Verdict(), "a stopped range reaches no terminator")
	assert.NoError(t, overSocket.Err(), "a consumer that stopped the range itself reports no fault")

	result, err := mux.CallRPC(ctx, "ze-bgp:awake", nil)
	require.NoError(t, err, "the lines the stopped answer left behind must not close the connection")
	assert.JSONEq(t, `{"awake":true}`, string(result))

	// In process the same stop reaches the generator, because the consumer's
	// range is what pulls each row.
	produced := 0
	rows := func(yield func(Record) bool) {
		for _, item := range items {
			produced++
			if !yield(Record{Item: json.RawMessage(item)}) {
				return
			}
		}
	}
	head := AnswerTail{Status: StatusDone, Type: AnswerTypeNDJSON, Key: "peers"}
	terminator := AnswerTail{Count: uint64(len(items))}
	inProcess := NewAnswer(head, terminator, rows)

	processRead := 0
	for range inProcess.Records {
		processRead++
		break
	}
	assert.Equal(t, 1, processRead, "the consumer took one record and left")
	assert.Equal(t, 1, produced, "the generator stopped on the row the consumer stopped at")
	assert.Equal(t, VerdictTruncated, inProcess.Verdict(), "a stopped range reaches no terminator")
	assert.NoError(t, inProcess.Err(), "a consumer that stopped the range itself reports no fault")
}

// TestMuxReadLoopSeparatesAnswerFromResponse checks that one reader carries
// both line families over one connection once every line states its id length.
// The method: a CallRPC and a CallAnswer run at once, the peer writes the plain
// response between two records of the answer, and each caller is required to
// receive its own lines whole.
//
// VALIDATES: A-3 -- the mux read loop takes the id field by arithmetic and
// still tells an answer line from a plain response line, so one reader serves
// both without a second discriminator.
// PREVENTS: the length-prefixed id costing the reader its routing, which would
// deliver a record to a CallRPC caller or strand an answer behind a response.
func TestMuxReadLoopSeparatesAnswerFromResponse(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "pluginEnd", pluginEnd)
	defer closePipe(t, "engineEnd", engineEnd)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const (
		answerMethod = "ze-plugin-engine:dispatch-command"
		rpcMethod    = "ze-bgp:peer-list"
	)

	go func() {
		engineConn := NewConn(engineEnd, engineEnd)
		ids := make(map[string]uint64, 2)
		for len(ids) < 2 {
			req, err := engineConn.ReadRequest(ctx)
			if err != nil {
				return
			}
			ids[req.Method] = req.ID
		}
		answerID, rpcID := ids[answerMethod], ids[rpcMethod]
		writeLines(engineEnd,
			wireLine(answerID, "ok status=done type=ndjson key=peers\n"),
			wireLine(answerID, "ok item={\"peer\":\"10.0.0.1\"}\n"),
			wireLine(rpcID, "ok {\"peers\":1}\n"),
			wireLine(answerID, "ok item={\"peer\":\"10.0.0.2\"}\n"),
			wireLine(answerID, "ok count=2\n"),
		)
	}()

	mux := NewMuxConn(NewConn(pluginEnd, pluginEnd))
	defer func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
	}()

	type answerRead struct {
		items   []string
		verdict string
		err     error
	}
	answered := make(chan answerRead, 1)
	go func() {
		received, err := mux.CallAnswer(ctx, answerMethod, nil)
		if err != nil {
			answered <- answerRead{err: err}
			return
		}
		var got answerRead
		for record := range received.Records {
			got.items = append(got.items, string(record.Item))
		}
		got.verdict = received.Verdict()
		got.err = received.Err()
		answered <- got
	}()

	result, err := mux.CallRPC(ctx, rpcMethod, nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"peers":1}`, string(result),
		"the plain response reaches its CallRPC caller whole, from between two records")

	got := <-answered
	require.NoError(t, got.err)
	assert.Equal(t, []string{`{"peer":"10.0.0.1"}`, `{"peer":"10.0.0.2"}`}, got.items,
		"both records reach the CallAnswer caller, with the other id's response between them")
	assert.Equal(t, VerdictDone, got.verdict, "the answer reached its terminator")
}
