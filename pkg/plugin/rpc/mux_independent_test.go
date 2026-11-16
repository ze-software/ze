// Design: docs/architecture/api/process-protocol.md -- independent callback ingress.
package rpc

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMuxIndependentIngressPreservesOrdinaryRequests(t *testing.T) {
	left, right := net.Pipe()
	defer closePipe(t, "left", left)
	defer closePipe(t, "right", right)
	conn := NewConn(right, right)
	mux := NewMuxConn(NewConn(left, left))
	defer mux.Close() //nolint:errcheck // test transport cleanup
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	admitted := make(chan *Request, 1)
	remove, err := mux.RegisterIndependentRequestHandler(MethodCurrentSelection, func(request *Request) error {
		admitted <- request
		return nil
	})
	require.NoError(t, err)
	defer remove()
	// Cancel requests remain on the engine runtime lane, never this callback.
	require.NoError(t, conn.writeAppended(ctx, func(buf []byte) []byte { return AppendRequest(buf, 1, MethodCancelRequest, nil) }))
	select {
	case request := <-mux.Requests():
		require.Equal(t, MethodCancelRequest, request.Method)
	case <-ctx.Done():
		t.Fatal("ordinary runtime request lost")
	}
	require.NoError(t, conn.writeAppended(ctx, func(buf []byte) []byte { return AppendRequest(buf, 2, MethodCurrentSelection, nil) }))
	select {
	case request := <-admitted:
		require.Equal(t, uint64(2), request.ID)
	case <-ctx.Done():
		t.Fatal("independent request was not admitted")
	}
}

func TestMuxIndependentIngressFailureClosesConnection(t *testing.T) {
	left, right := net.Pipe()
	defer closePipe(t, "left", left)
	defer closePipe(t, "right", right)
	mux := NewMuxConn(NewConn(left, left))
	defer mux.Close() //nolint:errcheck // test transport cleanup
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	remove, err := mux.RegisterIndependentRequestHandler(MethodCurrentSelection, func(*Request) error { return errors.New("queue full") })
	require.NoError(t, err)
	defer remove()
	conn := NewMuxConn(NewConn(right, right))
	defer conn.Close() //nolint:errcheck // test transport cleanup
	_, err = conn.CallRPC(ctx, MethodCurrentSelection, nil)
	require.Error(t, err)
	select {
	case <-mux.Done():
	case <-ctx.Done():
		t.Fatal("rejected ingress did not end connection")
	}
}
