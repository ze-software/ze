// Design: docs/architecture/api/ipc_protocol.md -- responses queued before reader completion.
// Related: mux.go -- CallRPC and its terminal queuedResponse branch.

package rpc

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

// muxTerminalWriter holds CallRPC in its request write until the test's reader
// has routed the response and stopped. It is used by one caller at a time.
type muxTerminalWriter struct {
	write func([]byte) (int, error)
}

func (w muxTerminalWriter) Write(data []byte) (int, error) {
	return w.write(data)
}

func (muxTerminalWriter) Close() error {
	return nil
}

// TestMuxConn_QueuedResponseAfterReaderStops exercises the terminal branch
// directly after the real reader has queued a response and exited, so the
// regression does not depend on which ready case a select happens to choose.
func TestMuxConn_QueuedResponseAfterReaderStops(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		response string
		payload  string
		rpcError bool
	}{
		{name: "result", response: `ok {"accepted":true}`, payload: `{"accepted":true}`},
		{name: "empty-result", response: "ok"},
		{name: "rpc-error", response: `error {"code":"refused","message":"budget refused"}`, rpcError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			reader, peer := io.Pipe()
			mux := NewMuxConn(NewConn(reader, muxTerminalWriter{write: func(data []byte) (int, error) {
				return len(data), nil
			}}))
			defer closeConn(t, mux.conn)
			defer func() {
				if err := peer.Close(); err != nil {
					t.Errorf("close peer: %v", err)
				}
			}()

			response := make(chan []byte, 1)
			mux.pending.Store(pendingKey(1), response)
			if _, err := io.WriteString(peer, wireLine(1, tc.response)+"\n"); err != nil {
				t.Fatalf("write response: %v", err)
			}
			if err := peer.Close(); err != nil {
				t.Fatalf("close peer after response: %v", err)
			}
			select {
			case <-mux.Done():
			case <-ctx.Done():
				t.Fatal("reader did not stop after peer closed")
			}
			if len(response) != 1 {
				t.Fatal("reader stopped without queuing the response")
			}

			payload, err := mux.queuedResponse(response)
			if tc.rpcError {
				var callErr *RPCCallError
				if !errors.As(err, &callErr) {
					t.Fatalf("queued response error = %v, want RPCCallError", err)
				}
				if callErr.Code != "refused" || callErr.Message != "budget refused" {
					t.Fatalf("queued RPC error = %+v", callErr)
				}
			} else if err != nil {
				t.Fatalf("queued response: %v", err)
			}
			if string(payload) != tc.payload {
				t.Errorf("payload = %s, want %s", payload, tc.payload)
			}
			if _, err := mux.queuedResponse(response); !errors.Is(err, io.EOF) {
				t.Errorf("drained response error = %v, want EOF", err)
			}
		})
	}
}

// TestMuxConn_CallRPCResponseBeforeReaderStops gates the request write on reader
// completion: CallRPC cannot resume until the response and terminal signal are
// both ready. A peer that sends no response must still produce EOF.
func TestMuxConn_CallRPCResponseBeforeReaderStops(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		response string
		payload  string
		rpcError bool
	}{
		{name: "result", response: `ok {"accepted":true}`, payload: `{"accepted":true}`},
		{name: "rpc-error", response: `error {"code":"refused","message":"budget refused"}`, rpcError: true},
		{name: "no-response"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			reader, peer := io.Pipe()
			var mux *MuxConn
			writer := muxTerminalWriter{write: func(data []byte) (int, error) {
				id, _, _, err := ParseLine(data)
				if err != nil {
					return 0, err
				}
				if tc.response != "" {
					if _, err := io.WriteString(peer, wireLine(id, tc.response)+"\n"); err != nil {
						return 0, err
					}
				}
				if err := peer.Close(); err != nil {
					return 0, err
				}
				select {
				case <-mux.Done():
					return len(data), nil
				case <-ctx.Done():
					return 0, ctx.Err()
				}
			}}
			mux = NewMuxConn(NewConn(reader, writer))
			defer closeConn(t, mux.conn)
			defer func() {
				if err := peer.Close(); err != nil {
					t.Errorf("close peer: %v", err)
				}
			}()

			payload, err := mux.CallRPC(ctx, "test-response-before-close", nil)
			if tc.response == "" {
				if !errors.Is(err, io.EOF) {
					t.Fatalf("unanswered call error = %v, want EOF", err)
				}
				return
			}
			if tc.rpcError {
				var callErr *RPCCallError
				if !errors.As(err, &callErr) {
					t.Fatalf("call error = %v, want RPCCallError", err)
				}
				if callErr.Code != "refused" || callErr.Message != "budget refused" {
					t.Fatalf("RPC error = %+v", callErr)
				}
			} else if err != nil {
				t.Fatalf("call with queued response: %v", err)
			}
			if string(payload) != tc.payload {
				t.Errorf("payload = %s, want %s", payload, tc.payload)
			}
		})
	}
}

// TestMuxConn_CallRPCCancellationWithoutResponse cancels after the request write
// while the reader remains live, distinguishing cancellation from reader EOF.
func TestMuxConn_CallRPCCancellationWithoutResponse(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader, peer := io.Pipe()
	mux := NewMuxConn(NewConn(reader, muxTerminalWriter{write: func(data []byte) (int, error) {
		cancel()
		return len(data), nil
	}}))
	defer closeConn(t, mux.conn)
	defer func() {
		if err := peer.Close(); err != nil {
			t.Errorf("close peer: %v", err)
		}
	}()

	if _, err := mux.CallRPC(ctx, "test-cancel-without-response", nil); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled call error = %v, want context.Canceled", err)
	}
	if _, found := mux.pending.Load(pendingKey(1)); found {
		t.Error("canceled call left its pending entry registered")
	}
}
