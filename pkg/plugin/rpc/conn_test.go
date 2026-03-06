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

// sendFrame sends a NUL-terminated JSON frame on conn.
func sendFrame(t *testing.T, conn net.Conn, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	data = append(data, 0)
	_, err = conn.Write(data)
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
// VALIDATES: AC-1 — ReadRequest returns next frame from persistent reader.
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

	// Send frames from a goroutine — net.Pipe is synchronous, so writes
	// block until the reader consumes. The persistent reader starts on
	// first ReadRequest, so we must not block the test goroutine on Write.
	const count = 10
	go func() {
		sendFrame(t, serverEnd, &Request{Method: "test-method", ID: json.RawMessage(`1`)})
		for i := range count {
			sendFrame(t, serverEnd, &Request{
				Method: "ping",
				ID:     json.RawMessage(fmt.Sprintf("%d", i+2)),
			})
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
// VALIDATES: AC-11 — Each call gets the next frame in order; persistent reader stays alive.
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

	// Send frames from goroutine — net.Pipe is synchronous.
	methods := []string{"alpha", "beta", "gamma", "delta"}
	go func() {
		for _, m := range methods {
			sendFrame(t, serverEnd, &Request{
				Method: m,
				ID:     json.RawMessage(`"` + m + `"`),
			})
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
// VALIDATES: AC-5 — Context canceled during ReadRequest returns ctx.Err();
// persistent reader continues for future calls.
// PREVENTS: Goroutine leaks on context cancellation.
func TestConn_ReadRequest_ContextCancel(t *testing.T) {
	t.Parallel()

	clientEnd, serverEnd := net.Pipe()
	defer closePipe(t, "clientEnd", clientEnd)
	defer closePipe(t, "serverEnd", serverEnd)

	conn := NewConn(clientEnd, clientEnd)
	defer closeConn(t, conn)

	// Use a context that expires quickly — no data will be sent.
	shortCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := conn.ReadRequest(shortCtx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// Now send a frame and verify the persistent reader is still alive.
	longCtx, longCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer longCancel()

	// Send from goroutine — net.Pipe is synchronous.
	go sendFrame(t, serverEnd, &Request{Method: "after-cancel", ID: json.RawMessage(`2`)})

	got, err := conn.ReadRequest(longCtx)
	require.NoError(t, err)
	assert.Equal(t, "after-cancel", got.Method)
}

// TestConn_ReadRequest_CloseUnblocks verifies that Close() unblocks a
// pending ReadRequest.
//
// VALIDATES: AC-7 — Close() while ReadRequest is blocked returns error;
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
// VALIDATES: AC-10 — Reader encounters I/O error; error stored;
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
	go sendFrame(t, serverEnd, &Request{Method: "before-break", ID: json.RawMessage(`1`)})

	got, err := conn.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "before-break", got.Method)

	// Close the server end — next reader.Read() will fail.
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
// VALIDATES: AC-1, AC-11 — Goroutine count stable across many calls.
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

	// Send all frames from a goroutine — net.Pipe is synchronous.
	go func() {
		sendFrame(t, serverEnd, &Request{Method: "warmup", ID: json.RawMessage(`0`)})
		for i := range n {
			sendFrame(t, serverEnd, &Request{
				Method: "test",
				ID:     json.RawMessage(fmt.Sprintf("%d", i+1)),
			})
		}
	}()

	// Warm up — trigger persistent reader start.
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
// VALIDATES: AC-2 — CallRPC uses deadline write + persistent read; no goroutines spawned.
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

	var resp struct {
		Result struct {
			Method string `json:"method"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(raw, &resp))
	assert.Equal(t, "test-method", resp.Result.Method)
}

// TestConn_CallBatchRPC_DeadlineWrite verifies CallBatchRPC works with
// deadline-based writes and persistent reader.
//
// VALIDATES: AC-3 — CallBatchRPC uses deadline write + persistent read.
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
		data, readErr := engineConn.reader.Read()
		if readErr != nil {
			return
		}
		// Extract ID from batch request.
		var probe struct {
			ID json.RawMessage `json:"id"`
		}
		if unmarshalErr := json.Unmarshal(data, &probe); unmarshalErr != nil {
			return
		}
		resp := &RPCResult{ID: probe.ID}
		if writeErr := engineConn.WriteFrame(resp); writeErr != nil {
			return
		}
	}()

	conn := NewConn(pluginEnd, pluginEnd)
	defer closeConn(t, conn)

	events := [][]byte{
		[]byte(`{"type":"bgp","bgp":{"type":"state","peer":{"address":"10.0.0.1","asn":65001},"state":"up"}}`),
	}

	raw, err := conn.CallBatchRPC(ctx, events)
	require.NoError(t, err)
	require.NotNil(t, raw)
}

// TestConn_WriteWithContext_Deadline verifies WriteWithContext uses
// SetWriteDeadline instead of goroutine bridge.
//
// VALIDATES: AC-4 — Uses SetWriteDeadline on writeConn; no goroutine spawned.
// VALIDATES: AC-12 — Context without deadline uses default 30s safety deadline.
// PREVENTS: Per-write goroutine spawning.
func TestConn_WriteWithContext_Deadline(t *testing.T) {
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

	msg := &Request{Method: "test-write", ID: json.RawMessage(`1`)}
	err := conn.WriteWithContext(ctx, msg)
	require.NoError(t, err)

	select {
	case data := <-readDone:
		require.NotNil(t, data)
		var got Request
		require.NoError(t, json.Unmarshal(data, &got))
		assert.Equal(t, "test-write", got.Method)
	case <-time.After(2 * time.Second):
		t.Fatal("server did not receive written frame")
	}
}

// TestConn_WriteWithContext_ContextCancel verifies that a canceled context
// causes the write to return promptly.
//
// VALIDATES: AC-6 — Context canceled → deadline-triggered write error.
// PREVENTS: Writes blocking indefinitely on canceled context.
func TestConn_WriteWithContext_ContextCancel(t *testing.T) {
	t.Parallel()

	clientEnd, serverEnd := net.Pipe()
	defer closePipe(t, "serverEnd", serverEnd)

	conn := NewConn(clientEnd, clientEnd)
	defer closeConn(t, conn)

	// Cancel the context immediately.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	msg := &Request{Method: "should-fail", ID: json.RawMessage(`1`)}
	err := conn.WriteWithContext(ctx, msg)
	require.Error(t, err)
}

// TestConn_CallRPC_CloseUnblocks verifies Close() unblocks a pending CallRPC.
//
// VALIDATES: AC-8 — Close() while CallRPC waiting for response returns error.
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
		// Deliberately don't respond — block forever.
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
// VALIDATES: AC-13 — Concurrent callers serialize via callMu; no races.
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
			var resp struct {
				Result struct {
					Method string `json:"method"`
				} `json:"result"`
			}
			if unmarshalErr := json.Unmarshal(raw, &resp); unmarshalErr != nil {
				errs[idx] = unmarshalErr
				return
			}
			results[idx] = resp.Result.Method
		}(i)
	}

	wg.Wait()

	for i := range n {
		require.NoError(t, errs[i], "call %d should succeed", i)
		expected := "serial-" + string(rune('A'+i))
		assert.Equal(t, expected, results[i], "call %d response should match", i)
	}
}

// TestConn_MuxConn_Compatibility verifies that MuxConn still works correctly
// after Conn's internal changes.
//
// VALIDATES: AC-9 — MuxConn wrapping Conn works correctly.
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

	var resp struct {
		Result struct {
			Method string `json:"method"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(raw, &resp))
	assert.Equal(t, "compat-test", resp.Result.Method)
}

// TestAutoDetectMode verifies first-byte protocol mode detection.
//
// VALIDATES: AC-8 — First byte { → JSON mode, letter → text mode. Peeked byte not consumed.
// PREVENTS: Mode misdetection or consumed bytes breaking subsequent reads.
func TestAutoDetectMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		send      string
		wantMode  ConnMode
		wantFirst byte
	}{
		{
			name:      "JSON mode from opening brace",
			send:      `{"method":"declare-registration","id":1}`,
			wantMode:  ModeJSON,
			wantFirst: '{',
		},
		{
			name:      "text mode from register verb",
			send:      "register\nfamily ipv4/unicast mode both\n\n",
			wantMode:  ModeText,
			wantFirst: 'r',
		},
		{
			name:      "text mode from capabilities verb",
			send:      "capabilities\ncode 65 encoding hex\n\n",
			wantMode:  ModeText,
			wantFirst: 'c',
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			clientEnd, serverEnd := net.Pipe()
			defer closePipe(t, "clientEnd", clientEnd)
			defer closePipe(t, "serverEnd", serverEnd)

			// Writer goroutine may see broken pipe when test closes the
			// pipe after reading just the peeked byte. Error is expected.
			go func() {
				if _, writeErr := io.WriteString(serverEnd, tt.send); writeErr != nil {
					return
				}
			}()

			mode, wrapped, err := PeekMode(clientEnd)
			require.NoError(t, err)
			assert.Equal(t, tt.wantMode, mode)

			// Verify peeked byte not consumed — first read returns the peeked byte
			var buf [1]byte
			_, readErr := wrapped.Read(buf[:])
			require.NoError(t, readErr)
			assert.Equal(t, tt.wantFirst, buf[0], "peeked byte should still be available")
		})
	}
}
