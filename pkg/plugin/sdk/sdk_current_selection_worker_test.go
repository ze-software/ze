// Design: docs/architecture/api/process-protocol.md -- independent selection lookup lifecycle.
package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestCurrentSelectionWorkerQueueIsBounded(t *testing.T) {
	w := &currentSelectionWorker{requests: make(chan currentSelectionJob, currentSelectionQueueDepth), done: make(chan struct{})}
	request := &rpc.Request{Method: rpc.MethodCurrentSelection}
	for range currentSelectionQueueDepth {
		require.NoError(t, w.admitWire(request))
	}
	require.ErrorIs(t, w.admitWire(request), errCurrentSelectionFull)
	require.NotNil(t, w.failure.Load(), "wire saturation must become Run's joined worker failure")
	require.ErrorIs(t, *w.failure.Load(), errCurrentSelectionFull)
	w.stopAdmission()
	require.ErrorIs(t, w.admitWire(request), errCurrentSelectionStopped)
}

func TestCurrentSelectionWireQueueFullFailsRun(t *testing.T) {
	pluginEnd, engineEnd := net.Pipe()
	defer pluginEnd.Close() //nolint:errcheck // test transport cleanup
	defer engineEnd.Close() //nolint:errcheck // test transport cleanup
	p := NewWithConn("current-full", pluginEnd)
	wire := rpc.NewConn(engineEnd, engineEnd)
	engine := &engineSide{mux: rpc.NewMuxConn(wire)}
	defer engine.mux.Close() //nolint:errcheck // test transport cleanup
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	entered := make(chan struct{})
	p.OnCurrentSelection(func(ctx context.Context, _ rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
		close(entered)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx, &Registration{}) }()
	completeStartup(t, ctx, engine)
	input := rpc.CurrentSelectionInput{Destination: "192.0.2.1", DestinationSession: 7,
		DeadlineMS: time.Now().Add(4 * time.Second).UnixMilli(),
		Keys:       []rpc.CurrentSelectionKey{{Family: "ipv4 unicast", KeyNLRI: []byte{0}, Revision: 3}}}
	body, err := json.Marshal(input)
	require.NoError(t, err)
	require.NoError(t, wire.WriteRawFrame(append(rpc.AppendRequest(nil, 100, rpc.MethodCurrentSelection, body), '\n')))
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("snapshot handler never entered")
	}
	for i := range currentSelectionQueueDepth + 1 {
		err := wire.WriteRawFrame(append(rpc.AppendRequest(nil, uint64(101+i), rpc.MethodCurrentSelection, body), '\n'))
		if err != nil {
			require.Equal(t, currentSelectionQueueDepth, i, "connection closed before queue filled")
		}
	}
	select {
	case err := <-done:
		require.ErrorIs(t, err, errCurrentSelectionFull)
	case <-ctx.Done():
		t.Fatal("Run did not join failed lookup ingress")
	}
}

func TestCurrentSelectionWorkerHonorsDeadline(t *testing.T) {
	p, _, bridge := newBridgedTestPair(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	left := make(chan struct{})
	p.OnCurrentSelection(func(ctx context.Context, _ rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
		<-ctx.Done()
		close(left)
		return nil, ctx.Err()
	})
	stop, err := p.startCurrentSelectionWorker(ctx)
	require.NoError(t, err)
	defer stop()
	bridge.SetReady()
	_, err = bridge.CurrentSelection(ctx, rpc.CurrentSelectionInput{Destination: "192.0.2.1", DestinationSession: 7,
		DeadlineMS: time.Now().Add(100 * time.Millisecond).UnixMilli(),
		Keys:       []rpc.CurrentSelectionKey{{Family: "ipv4 unicast", KeyNLRI: []byte{0}, Revision: 3}}})
	require.True(t, errors.Is(err, context.DeadlineExceeded), "got %v", err)
	select {
	case <-left:
	case <-ctx.Done():
		t.Fatal("deadline did not stop snapshot handler")
	}
}

func TestCurrentSelectionWorkerPanicFailsConnection(t *testing.T) {
	p, _, bridge := newBridgedTestPair(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	p.OnCurrentSelection(func(context.Context, rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
		panic("snapshot defect")
	})
	stop, err := p.startCurrentSelectionWorker(ctx)
	require.NoError(t, err)
	bridge.SetReady()
	_, err = bridge.CurrentSelection(ctx, rpc.CurrentSelectionInput{Destination: "192.0.2.1", DestinationSession: 7,
		Keys: []rpc.CurrentSelectionKey{{Family: "ipv4 unicast", KeyNLRI: []byte{0}, Revision: 3}}})
	require.ErrorContains(t, err, "snapshot defect")
	require.ErrorContains(t, stop(), "snapshot defect")
	require.False(t, bridge.Ready())
}
