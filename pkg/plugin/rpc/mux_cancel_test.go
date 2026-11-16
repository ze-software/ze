// Design: docs/architecture/api/ipc_protocol.md -- acknowledged request cancellation.
package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"
)

// TestMuxCancellableCallJoinsRemoteSettlement proves cancellation does not
// release a selected worker while the remote final writer still owns its work.
// Method: hold the cancellation receipt behind an explicit writer barrier.
func TestMuxCancellableCallJoinsRemoteSettlement(t *testing.T) {
	work, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	client, engine := net.Pipe()
	mux := NewMuxConn(NewConn(client, client))
	t.Cleanup(func() { closePipe(t, "client", client); closePipe(t, "engine", engine) })
	server := NewConn(engine, engine)
	started, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	serverDone := make(chan error, 1)
	go func() {
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
		if cancelRequest.Method != MethodCancelRequest {
			serverDone <- errors.New("cancellation did not use its registered operation")
			return
		}
		var input CancelRequestInput
		if err := json.Unmarshal(cancelRequest.Params, &input); err != nil {
			serverDone <- err
			return
		}
		if input.RequestID != request.ID {
			serverDone <- errors.New("cancel targeted another request")
			return
		}
		close(canceled)
		select {
		case <-release:
		case <-work.Done():
			serverDone <- work.Err()
			return
		}
		serverDone <- server.SendResult(work, cancelRequest.ID, CancelRequestOutput{
			RequestID: request.ID, Outcome: CancelRequestSettled})
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
	case <-canceled:
	case <-work.Done():
		t.Fatal(work.Err())
	}
	select {
	case err := <-finished:
		t.Fatalf("cancellation returned before remote settlement: %v", err)
	default:
	}
	close(release)
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation result: %v", err)
		}
	case <-work.Done():
		t.Fatal(work.Err())
	}
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
}
