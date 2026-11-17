// Design: docs/architecture/api/ipc_protocol.md -- failed cancellation is not settlement.
package rpc

import (
	"context"
	"errors"
	"io"
	"math"
	"net"
	"testing"
	"time"
)

// TestMuxCancellableRefusesInvalidSettlement proves a missing, mismatched or
// refused remote receipt is an unresolved barrier, preserving caller cause.
// Method: cancel an admitted request and return each invalid control answer.
func TestMuxCancellableRefusesInvalidSettlement(t *testing.T) {
	for _, kind := range []string{"missing", "wrong-id", "unknown", "refused"} {
		t.Run(kind, func(t *testing.T) {
			work, stop := context.WithTimeout(t.Context(), 5*time.Second)
			defer stop()
			client, engine := net.Pipe()
			mux := NewMuxConn(NewConn(client, client))
			t.Cleanup(func() { closePipe(t, "client", client); closePipe(t, "engine", engine) })
			started := make(chan struct{})
			serverDone := make(chan error, 1)
			go func() {
				server := NewConn(engine, engine)
				request, err := server.ReadRequest(work)
				if err != nil {
					serverDone <- err
					return
				}
				close(started)
				cancelRequest, err := server.ReadRequest(work)
				if err != nil {
					serverDone <- err
					return
				}
				output := &CancelRequestOutput{RequestID: request.ID, Outcome: CancelRequestSettled}
				switch kind {
				case "missing":
					output = nil
				case "wrong-id":
					output.RequestID++
				case "unknown":
					output.Outcome = 99
				case "refused":
					serverDone <- server.SendError(work, cancelRequest.ID, "cannot join")
					return
				}
				serverDone <- server.SendResult(work, cancelRequest.ID, output)
			}()
			ctx, cancel := context.WithCancel(work)
			defer cancel()
			finished := make(chan error, 1)
			go func() { _, err := mux.CallRPCCancellable(ctx, MethodSelectRoutes, nil); finished <- err }()
			select {
			case <-started:
			case <-work.Done():
				t.Fatal(work.Err())
			}
			cancel()
			select {
			case err := <-finished:
				if _, unsettled := errors.AsType[*CancellationUnsettledError](err); !unsettled {
					t.Fatalf("invalid receipt claimed settlement: %v", err)
				}
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("caller cause lost: %v", err)
				}
			case <-work.Done():
				t.Fatal(work.Err())
			}
			if err := <-serverDone; err != nil {
				t.Fatal(err)
			}
		})
	}
}

type partialRequestConn struct{ net.Conn }

func (conn partialRequestConn) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	n, err := conn.Conn.Write(data[:1])
	if err != nil {
		return n, err
	}
	return n, io.ErrUnexpectedEOF
}

// TestMuxCancellablePartialWriteEndsStream proves an incomplete request cannot
// leave a live reusable stream or claim a remote join. Method: deliver one byte
// of the frame, fail its writer, and require EOF at the remote request reader.
func TestMuxCancellablePartialWriteEndsStream(t *testing.T) {
	work, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	client, engine := net.Pipe()
	mux := NewMuxConn(NewConn(client, partialRequestConn{client}))
	t.Cleanup(func() { closePipe(t, "client", client); closePipe(t, "engine", engine) })
	serverDone := make(chan error, 1)
	go func() { _, err := NewConn(engine, engine).ReadRequest(work); serverDone <- err }()
	_, err := mux.CallRPCCancellable(work, MethodSelectRoutes, nil)
	if _, unsettled := errors.AsType[*CancellationUnsettledError](err); !unsettled {
		t.Fatalf("partial send claimed settlement: %v", err)
	}
	select {
	case err := <-serverDone:
		if err == nil {
			t.Fatal("partial frame was accepted as a request")
		}
	case <-work.Done():
		t.Fatal("partial request left remote reader alive")
	}
}

// TestMuxCancellableRequestIDExhaustion refuses wrap before a frame is sent.
// Method: place the existing connection counter at its maximum and require the
// stream to end instead of reusing a prior request's correlation identity.
func TestMuxCancellableRequestIDExhaustion(t *testing.T) {
	client, engine := net.Pipe()
	conn := NewConn(client, client)
	conn.idSeq.Store(math.MaxUint64)
	mux := NewMuxConn(conn)
	t.Cleanup(func() { closePipe(t, "client", client); closePipe(t, "engine", engine) })
	if _, err := mux.CallRPCCancellable(t.Context(), MethodSelectRoutes, nil); err == nil {
		t.Fatal("wrapped request ID was admitted")
	}
}
