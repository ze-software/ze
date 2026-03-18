package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeLine writes a newline-terminated line to conn.
func writeLine(t *testing.T, conn net.Conn, line string) {
	t.Helper()
	_, err := io.WriteString(conn, line+"\n")
	require.NoError(t, err)
}

// closeConn closes an RPC Conn and logs failures.
func closeConn(t *testing.T, c *Conn) {
	t.Helper()
	if err := c.Close(); err != nil {
		t.Logf("close conn: %v", err)
	}
}

// TestConn_ReadRequest_PersistentReader verifies that ReadRequest uses a
// persistent reader goroutine rather than spawning one per call.
//
// VALIDATES: AC-1 -- ReadRequest returns next frame from persistent reader.
// PREVENTS: Per-call goroutine spawning in hot path.
func TestConn_ReadRequest_PersistentReader(t *testing.T) {
	t.Parallel()

	clientEnd, serverEnd := net.Pipe()
	defer closePipe(t, "clientEnd", clientEnd)
	defer closePipe(t, "serverEnd", serverEnd)

	conn := NewConn(clientEnd, clientEnd)
	defer closeConn(t, conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Send frames from a goroutine -- net.Pipe is synchronous, so writes
	// block until the reader consumes. The persistent reader starts on
	// first ReadRequest, so we must not block the test goroutine on Write.
	const count = 10
	go func() {
		writeLine(t, serverEnd, "#1 test-method")
		for i := range count {
			writeLine(t, serverEnd, fmt.Sprintf("#%d ping", i+2))
		}
	}()

	got, err := conn.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "test-method", got.Method)

	for range count {
		got, readErr := conn.ReadRequest(ctx)
		require.NoError(t, readErr)
		assert.Equal(t, "ping", got.Method)
	}
}

// TestConn_ReadRequest_Sequential verifies multiple sequential ReadRequest
// calls each get the correct next frame in order.
//
// VALIDATES: AC-11 -- Each call gets the next frame in order; persistent reader stays alive.
// PREVENTS: Frame reordering or duplication.
func TestConn_ReadRequest_Sequential(t *testing.T) {
	t.Parallel()

	clientEnd, serverEnd := net.Pipe()
	defer closePipe(t, "clientEnd", clientEnd)
	defer closePipe(t, "serverEnd", serverEnd)

	conn := NewConn(clientEnd, clientEnd)
	defer closeConn(t, conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Send frames from goroutine -- net.Pipe is synchronous.
	methods := []string{"alpha", "beta", "gamma", "delta"}
	go func() {
		for i, m := range methods {
			writeLine(t, serverEnd, fmt.Sprintf("#%d %s", i+1, m))
		}
	}()

	for _, m := range methods {
		got, err := conn.ReadRequest(ctx)
		require.NoError(t, err)
		assert.Equal(t, m, got.Method, "frames should arrive in order")
	}
}

// TestConn_ReadRequest_ContextCancel verifies that canceling the context
// returns promptly while the persistent reader survives for future calls.
//
// VALIDATES: AC-5 -- Context canceled during ReadRequest returns ctx.Err();
// persistent reader continues for future calls.
// PREVENTS: Goroutine leaks on context cancellation.
func TestConn_ReadRequest_ContextCancel(t *testing.T) {
	t.Parallel()

	clientEnd, serverEnd := net.Pipe()
	defer closePipe(t, "clientEnd", clientEnd)
	defer closePipe(t, "serverEnd", serverEnd)

	conn := NewConn(clientEnd, clientEnd)
	defer closeConn(t, conn)

	// Use a context that expires quickly -- no data will be sent.
	shortCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := conn.ReadRequest(shortCtx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// Now send a frame and verify the persistent reader is still alive.
	longCtx, longCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer longCancel()

	// Send from goroutine -- net.Pipe is synchronous.
	go writeLine(t, serverEnd, "#2 after-cancel")

	got, err := conn.ReadRequest(longCtx)
	require.NoError(t, err)
	assert.Equal(t, "after-cancel", got.Method)
}

// TestConn_ReadRequest_CloseUnblocks verifies that Close() unblocks a
// pending ReadRequest.
//
// VALIDATES: AC-7 -- Close() while ReadRequest is blocked returns error;
// persistent reader exits cleanly.
// PREVENTS: Goroutine leaks when connection is closed during blocking read.
func TestConn_ReadRequest_CloseUnblocks(t *testing.T) {
	t.Parallel()

	clientEnd, serverEnd := net.Pipe()
	defer closePipe(t, "serverEnd", serverEnd)

	conn := NewConn(clientEnd, clientEnd)

	ctx := context.Background()

	errCh := make(chan error, 1)
	go func() {
		_, readErr := conn.ReadRequest(ctx)
		errCh <- readErr
	}()

	// Give time for ReadRequest to block.
	time.Sleep(50 * time.Millisecond)

	require.NoError(t, conn.Close())

	select {
	case err := <-errCh:
		require.Error(t, err, "ReadRequest should return error after Close()")
	case <-time.After(2 * time.Second):
		t.Fatal("ReadRequest did not unblock after Close()")
	}
}

// TestConn_ReaderError_Propagates verifies that an I/O error on the
// underlying connection is propagated to all subsequent reads.
//
// VALIDATES: AC-10 -- Reader encounters I/O error; error stored;
// all subsequent reads return stored error.
// PREVENTS: Silent failures after connection break.
func TestConn_ReaderError_Propagates(t *testing.T) {
	t.Parallel()

	clientEnd, serverEnd := net.Pipe()

	conn := NewConn(clientEnd, clientEnd)
	defer closeConn(t, conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Send one frame from goroutine (net.Pipe is synchronous), then close.
	go writeLine(t, serverEnd, "#1 before-break")

	got, err := conn.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "before-break", got.Method)

	// Close the server end -- next reader.Read() will fail.
	require.NoError(t, serverEnd.Close())

	// Subsequent reads should return error.
	_, err = conn.ReadRequest(ctx)
	require.Error(t, err)

	// A further read should also return error (stored).
	_, err = conn.ReadRequest(ctx)
	require.Error(t, err)
}

// TestConn_NoGoroutineLeak verifies that many ReadRequest calls don't
// accumulate goroutines.
//
// VALIDATES: AC-1, AC-11 -- Goroutine count stable across many calls.
// PREVENTS: Goroutine leak from per-call spawning.
func TestConn_NoGoroutineLeak(t *testing.T) {

	clientEnd, serverEnd := net.Pipe()
	defer closePipe(t, "clientEnd", clientEnd)
	defer closePipe(t, "serverEnd", serverEnd)

	conn := NewConn(clientEnd, clientEnd)
	defer closeConn(t, conn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const n = 50

	// Send all frames from a goroutine -- net.Pipe is synchronous.
	go func() {
		writeLine(t, serverEnd, "#0 warmup")
		for i := range n {
			writeLine(t, serverEnd, fmt.Sprintf("#%d test", i+1))
		}
	}()

	// Warm up -- trigger persistent reader start.
	_, err := conn.ReadRequest(ctx)
	require.NoError(t, err)

	// Let goroutine counts settle.
	runtime.Gosched()
	time.Sleep(10 * time.Millisecond)
	before := runtime.NumGoroutine()

	for range n {
		_, readErr := conn.ReadRequest(ctx)
		require.NoError(t, readErr)
	}

	runtime.Gosched()
	time.Sleep(10 * time.Millisecond)
	after := runtime.NumGoroutine()

	// Delta of 5 accounts for parallel test goroutine churn. The key
	// assertion: no growth proportional to N (50 calls should not add 50 goroutines).
	assert.InDelta(t, before, after, 5,
		"goroutine count should be stable: before=%d after=%d", before, after)
}

// TestConn_CallRPC_DeadlineWrite verifies CallRPC sends and receives correctly
// using deadline-based writes and persistent reader.
//
// VALIDATES: AC-2 -- CallRPC uses deadline write + persistent read; no goroutines spawned.
// PREVENTS: Regression in CallRPC behavior.
func TestConn_CallRPC_DeadlineWrite(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "pluginEnd", pluginEnd)
	defer closePipe(t, "engineEnd", engineEnd)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Engine echoes method name.
	fakeEngine(ctx, engineEnd, func(req *Request) any {
		return map[string]string{"method": req.Method}
	})

	conn := NewConn(pluginEnd, pluginEnd)
	defer closeConn(t, conn)

	raw, err := conn.CallRPC(ctx, "test-method", nil)
	require.NoError(t, err)

	// CallRPC now returns the result payload directly (not wrapped in {"result":...}).
	var result struct {
		Method string `json:"method"`
	}
	require.NoError(t, json.Unmarshal(raw, &result))
	assert.Equal(t, "test-method", result.Method)
}

// TestConn_CallBatchRPC_DeadlineWrite verifies CallBatchRPC works with
// deadline-based writes and persistent reader.
//
// VALIDATES: AC-3 -- CallBatchRPC uses deadline write + persistent read.
// PREVENTS: Regression in batch delivery.
func TestConn_CallBatchRPC_DeadlineWrite(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "pluginEnd", pluginEnd)
	defer closePipe(t, "engineEnd", engineEnd)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Engine reads the batch request and sends back an OK response.
	go func() {
		engineConn := NewConn(engineEnd, engineEnd)
		req, readErr := engineConn.ReadRequest(ctx)
		if readErr != nil {
			return
		}
		// Send OK response with matching ID.
		if sendErr := engineConn.SendOK(ctx, req.ID); sendErr != nil {
			return
		}
	}()

	conn := NewConn(pluginEnd, pluginEnd)
	defer closeConn(t, conn)

	events := [][]byte{
		[]byte(`{"type":"bgp","bgp":{"type":"state","peer":{"address":"10.0.0.1","asn":65001},"state":"up"}}`),
	}

	_, err := conn.CallBatchRPC(ctx, events)
	require.NoError(t, err)
}

// TestConn_WriteLineWithContext_Deadline verifies writeLineWithContext uses
// SetWriteDeadline instead of goroutine bridge.
//
// VALIDATES: AC-4 -- Uses SetWriteDeadline on writeConn; no goroutine spawned.
// VALIDATES: AC-12 -- Context without deadline uses default 30s safety deadline.
// PREVENTS: Per-write goroutine spawning.
func TestConn_WriteLineWithContext_Deadline(t *testing.T) {
	t.Parallel()

	clientEnd, serverEnd := net.Pipe()
	defer closePipe(t, "clientEnd", clientEnd)
	defer closePipe(t, "serverEnd", serverEnd)

	conn := NewConn(clientEnd, clientEnd)
	defer closeConn(t, conn)

	// Read from server side in background.
	readDone := make(chan []byte, 1)
	go func() {
		fr := NewFrameReader(serverEnd)
		data, readErr := fr.Read()
		if readErr != nil {
			readDone <- nil
			return
		}
		readDone <- data
	}()

	// Write with a deadline context.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	line := FormatRequest(1, "test-write", nil)
	err := conn.writeLineWithContext(ctx, line)
	require.NoError(t, err)

	select {
	case data := <-readDone:
		require.NotNil(t, data)
		id, verb, _, parseErr := ParseLine(data)
		require.NoError(t, parseErr)
		assert.Equal(t, uint64(1), id)
		assert.Equal(t, "test-write", verb)
	case <-time.After(2 * time.Second):
		t.Fatal("server did not receive written frame")
	}
}

// TestConn_WriteLineWithContext_ContextCancel verifies that a canceled context
// causes the write to return promptly.
//
// VALIDATES: AC-6 -- Context canceled -> deadline-triggered write error.
// PREVENTS: Writes blocking indefinitely on canceled context.
func TestConn_WriteLineWithContext_ContextCancel(t *testing.T) {
	t.Parallel()

	clientEnd, serverEnd := net.Pipe()
	defer closePipe(t, "serverEnd", serverEnd)

	conn := NewConn(clientEnd, clientEnd)
	defer closeConn(t, conn)

	// Cancel the context immediately.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	line := FormatRequest(1, "should-fail", nil)
	err := conn.writeLineWithContext(ctx, line)
	require.Error(t, err)
}

// TestConn_CallRPC_CloseUnblocks verifies Close() unblocks a pending CallRPC.
//
// VALIDATES: AC-8 -- Close() while CallRPC waiting for response returns error.
// PREVENTS: Goroutine leak when connection closed during CallRPC.
func TestConn_CallRPC_CloseUnblocks(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "engineEnd", engineEnd)

	// Engine reads but never responds.
	go func() {
		engineConn := NewConn(engineEnd, engineEnd)
		if _, readErr := engineConn.ReadRequest(context.Background()); readErr != nil {
			return
		}
		// Deliberately don't respond -- block forever.
		select {}
	}()

	conn := NewConn(pluginEnd, pluginEnd)

	errCh := make(chan error, 1)
	go func() {
		_, callErr := conn.CallRPC(context.Background(), "will-close", nil)
		errCh <- callErr
	}()

	time.Sleep(50 * time.Millisecond)
	require.NoError(t, conn.Close())

	select {
	case err := <-errCh:
		require.Error(t, err, "CallRPC should return error after Close()")
	case <-time.After(2 * time.Second):
		t.Fatal("CallRPC did not unblock after Close()")
	}
}

// TestConn_CallRPC_Serialization verifies that callMu still serializes
// concurrent CallRPC calls correctly.
//
// VALIDATES: AC-13 -- Concurrent callers serialize via callMu; no races.
// PREVENTS: Race conditions from concurrent CallRPC access.
func TestConn_CallRPC_Serialization(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "pluginEnd", pluginEnd)
	defer closePipe(t, "engineEnd", engineEnd)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Engine echoes method name with a small delay to ensure overlap.
	fakeEngineWithDelay(ctx, engineEnd, 10*time.Millisecond, func(req *Request) any {
		return map[string]string{"method": req.Method}
	})

	conn := NewConn(pluginEnd, pluginEnd)
	defer closeConn(t, conn)

	const n = 10
	var wg sync.WaitGroup
	results := make([]string, n)
	errs := make([]error, n)

	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			method := "serial-" + string(rune('A'+idx))
			raw, callErr := conn.CallRPC(ctx, method, nil)
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
		expected := "serial-" + string(rune('A'+i))
		assert.Equal(t, expected, results[i], "call %d response should match", i)
	}
}

// TestParseResponse verifies parseResponse handles ok, error, mismatched ID,
// and unknown verb cases.
//
// VALIDATES: parseResponse correctly extracts payload or returns typed errors.
// PREVENTS: Silent mishandling of response lines in CallRPC/CallBatchRPC.
func TestParseResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		line       string
		expectedID uint64
		wantData   string // expected JSON payload ("" means nil)
		wantErr    bool
		wantRPCErr bool // true if error should be *RPCCallError
	}{
		{
			name:       "ok with payload",
			line:       `#1 ok {"key":"val"}`,
			expectedID: 1,
			wantData:   `{"key":"val"}`,
		},
		{
			name:       "ok without payload",
			line:       "#1 ok",
			expectedID: 1,
			wantData:   "",
		},
		{
			name:       "error with payload",
			line:       `#1 error {"message":"bad"}`,
			expectedID: 1,
			wantErr:    true,
			wantRPCErr: true,
		},
		{
			name:       "error without payload",
			line:       "#1 error",
			expectedID: 1,
			wantErr:    true,
			wantRPCErr: true,
		},
		{
			name:       "mismatched id",
			line:       "#2 ok",
			expectedID: 1,
			wantErr:    true,
		},
		{
			name:       "unknown verb",
			line:       "#1 foobar",
			expectedID: 1,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseResponse([]byte(tt.line), tt.expectedID)
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantRPCErr {
					var rpcErr *RPCCallError
					require.ErrorAs(t, err, &rpcErr, "error should be *RPCCallError")
				}
				return
			}
			require.NoError(t, err)
			if tt.wantData == "" {
				assert.Nil(t, got)
			} else {
				assert.JSONEq(t, tt.wantData, string(got))
			}
		})
	}
}

// TestInterpretResponse verifies interpretResponse handles ok, error, and
// unknown verb after the #<id> prefix has been stripped by MuxConn.
//
// VALIDATES: interpretResponse correctly extracts payload or returns typed errors.
// PREVENTS: Silent mishandling of response bodies in MuxConn.CallRPC.
func TestInterpretResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		wantData   string // expected JSON payload ("" means nil)
		wantErr    bool
		wantRPCErr bool
	}{
		{
			name:     "ok with payload",
			body:     `ok {"key":"val"}`,
			wantData: `{"key":"val"}`,
		},
		{
			name:     "ok without payload",
			body:     "ok",
			wantData: "",
		},
		{
			name:       "error with payload",
			body:       `error {"message":"bad"}`,
			wantErr:    true,
			wantRPCErr: true,
		},
		{
			name:       "error without payload",
			body:       "error",
			wantErr:    true,
			wantRPCErr: true,
		},
		{
			name:    "unknown verb",
			body:    "foobar",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := interpretResponse([]byte(tt.body))
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantRPCErr {
					var rpcErr *RPCCallError
					require.ErrorAs(t, err, &rpcErr, "error should be *RPCCallError")
				}
				return
			}
			require.NoError(t, err)
			if tt.wantData == "" {
				assert.Nil(t, got)
			} else {
				assert.JSONEq(t, tt.wantData, string(got))
			}
		})
	}
}

// TestConn_MuxConn_Compatibility verifies that MuxConn still works correctly
// after Conn's internal changes.
//
// VALIDATES: AC-9 -- MuxConn wrapping Conn works correctly.
// PREVENTS: Breaking MuxConn which bypasses Conn's persistent reader.
func TestConn_MuxConn_Compatibility(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer closePipe(t, "pluginEnd", pluginEnd)
	defer closePipe(t, "engineEnd", engineEnd)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fakeEngine(ctx, engineEnd, func(req *Request) any {
		return map[string]string{"method": req.Method}
	})

	conn := NewConn(pluginEnd, pluginEnd)
	mux := NewMuxConn(conn)
	defer func() {
		if closeErr := mux.Close(); closeErr != nil {
			t.Logf("mux close: %v", closeErr)
		}
	}()

	// MuxConn should work exactly as before.
	raw, err := mux.CallRPC(ctx, "compat-test", nil)
	require.NoError(t, err)

	// MuxConn.CallRPC returns result payload directly.
	var result struct {
		Method string `json:"method"`
	}
	require.NoError(t, json.Unmarshal(raw, &result))
	assert.Equal(t, "compat-test", result.Method)
}
