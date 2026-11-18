// Design: docs/architecture/api/process-protocol.md -- work and response deadlines.
package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// An expired request refuses only that read. A real framed error reply and the
// typed negative both leave the original accepting connection useful.
func TestOriginAdmissionLookupExpiredRequestKeepsConnection(t *testing.T) {
	for _, typed := range []bool{false, true} {
		name := "framed"
		if typed {
			name = "typed"
		}
		t.Run(name, func(t *testing.T) {
			p, engine := newTestPair(t)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			bridge := rpc.NewDirectBridge()
			connection := ipc.NewMuxPluginConn(engine.mux)
			if typed {
				p.bridge = bridge
				connection.SetBridge(bridge)
			}
			var calls atomic.Int64
			p.OnOriginAdmissionLookup(func(_ context.Context, input rpc.OriginAdmissionLookupInput) (*rpc.OriginAdmissionLookupOutput, error) {
				calls.Add(1)
				return admissionLookupOutput(input), nil
			})
			stop, err := p.startOriginAdmissionLookupWorker(ctx)
			require.NoError(t, err)
			defer func() { require.NoError(t, stop()) }()
			if typed {
				bridge.SetReady()
			}
			input := admissionLookupInput()
			input.DeadlineMS = time.Now().Add(-time.Second).UnixMilli()
			if typed {
				_, err = bridge.OriginAdmissionLookup(ctx, input)
			} else {
				// CallRPC deliberately keeps its own live caller budget. The
				// expired work deadline must reach the actual framed reader.
				_, err = engine.mux.CallRPC(ctx, rpc.MethodOriginAdmissionLookup, input)
			}
			require.ErrorContains(t, err, "expired")
			require.Zero(t, calls.Load(), "expired input must not enter the snapshot handler")
			valid := admissionLookupInput()
			output, err := connection.SendOriginAdmissionLookup(ctx, valid)
			require.NoError(t, err)
			require.Equal(t, admissionLookupOutput(valid), output)
			require.Equal(t, int64(1), calls.Load())
			// Even the typed negative must leave the actual original socket
			// healthy, rather than hiding damage behind a live bridge.
			output, err = engine.mux.OriginAdmissionLookup(ctx, valid)
			require.NoError(t, err)
			require.Equal(t, admissionLookupOutput(valid), output)
			require.Equal(t, int64(2), calls.Load())
		})
	}
}

// A real valid framed request expires while queued behind a held read. The
// ingress witness observes the actual admission before its deadline; no test
// inserts a queue entry or substitutes a fake handler result for expiration.
func TestOriginAdmissionLookupQueuedExpiryKeepsConnection(t *testing.T) {
	p, engine := newTestPair(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	admitted := make(chan error, 1)
	var queuedCalls atomic.Int64
	handler := func(ctx context.Context, input rpc.OriginAdmissionLookupInput) (*rpc.OriginAdmissionLookupOutput, error) {
		if input.AdmissionID == 91 {
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		if input.AdmissionID == 92 {
			queuedCalls.Add(1)
		}
		return admissionLookupOutput(input), nil
	}
	worker := &originAdmissionLookupWorker{plugin: p, handler: handler,
		requests: make(chan originAdmissionLookupJob, originAdmissionLookupQueueDepth), done: make(chan struct{})}
	workerContext, stopWorker := context.WithCancel(ctx)
	remove, err := p.engineMux.RegisterIndependentRequestHandler(rpc.MethodOriginAdmissionLookup, func(request *rpc.Request) error {
		var input rpc.OriginAdmissionLookupInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return err
		}
		if input.AdmissionID == 92 {
			// This records actual valid ingress, before the production worker
			// later decodes the same immutable frame after its deadline.
			validation := input.Validate(time.Now())
			if validation != nil {
				admitted <- validation
				return validation
			}
			err := worker.admitWire(request)
			witness := err
			if witness == nil {
				witness = input.Validate(time.Now())
			}
			admitted <- witness
			return err
		}
		return worker.admitWire(request)
	})
	require.NoError(t, err)
	go worker.run(workerContext, p.engineMux.Done())
	defer func() {
		remove()
		worker.stopAdmission()
		stopWorker()
		<-worker.done
		worker.refuseQueued()
		if failure := worker.failure.Load(); failure != nil {
			require.NoError(t, *failure)
		}
	}()
	type result struct {
		output *rpc.OriginAdmissionLookupOutput
		err    error
	}
	first := make(chan result, 1)
	go func() {
		defer close(first)
		output, err := engine.mux.OriginAdmissionLookup(ctx, admissionLookupInput())
		first <- result{output: output, err: err}
	}()
	t.Cleanup(func() { admissionLookupJoin(t, cancel, first) })
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("blocking read never entered")
	}
	queuedInput := admissionLookupInput()
	queuedInput.AdmissionID = 92
	queuedInput.DeadlineMS = time.Now().Add(time.Second).UnixMilli()
	queued := make(chan error, 1)
	go func() {
		defer close(queued)
		_, err := engine.mux.CallRPC(ctx, rpc.MethodOriginAdmissionLookup, queuedInput)
		queued <- err
	}()
	t.Cleanup(func() { admissionLookupJoin(t, cancel, queued) })
	select {
	case err := <-admitted:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("queued request never reached real valid ingress")
	}
	require.Len(t, worker.requests, 1)
	expiry := time.NewTimer(time.Until(time.UnixMilli(queuedInput.DeadlineMS)))
	defer expiry.Stop()
	select {
	case <-expiry.C:
	case <-ctx.Done():
		t.Fatal("queued work deadline never arrived")
	}
	close(release)
	completed := <-first
	require.NoError(t, completed.err)
	require.Equal(t, admissionLookupOutput(admissionLookupInput()), completed.output)
	err = <-queued
	require.ErrorContains(t, err, "expired")
	require.Zero(t, queuedCalls.Load(), "expired queued work must never enter the handler")
	valid := admissionLookupInput()
	valid.AdmissionID = 93
	output, err := engine.mux.OriginAdmissionLookup(ctx, valid)
	require.NoError(t, err)
	require.Equal(t, admissionLookupOutput(valid), output)
}

type originAdmissionReplyFailureConn struct {
	net.Conn
	fail      atomic.Bool
	attempted chan struct{}
	once      sync.Once
	failure   error
}

func (connection *originAdmissionReplyFailureConn) Write(data []byte) (int, error) {
	if connection.fail.Load() {
		connection.once.Do(func() { close(connection.attempted) })
		return 0, connection.failure
	}
	return connection.Conn.Write(data)
}

// A genuine response writer failure still closes the unusable connection. The
// successful first read proves useful output before the exact writer is faulted.
func TestOriginAdmissionLookupResponseWriteFailureClosesConnection(t *testing.T) {
	pluginEnd, engineEnd := net.Pipe()
	failure := errors.New("test response writer failed")
	connection := &originAdmissionReplyFailureConn{Conn: pluginEnd, attempted: make(chan struct{}), failure: failure}
	p := NewWithConn("lookup-writer-failure", connection)
	engine := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	defer engine.Close() //nolint:errcheck // Test transport cleanup.
	defer p.Close()      //nolint:errcheck // Test transport cleanup.
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	p.OnOriginAdmissionLookup(func(_ context.Context, input rpc.OriginAdmissionLookupInput) (*rpc.OriginAdmissionLookupOutput, error) {
		return admissionLookupOutput(input), nil
	})
	stop, err := p.startOriginAdmissionLookupWorker(ctx)
	require.NoError(t, err)
	defer func() { require.ErrorIs(t, stop(), failure) }()
	input := admissionLookupInput()
	output, err := engine.OriginAdmissionLookup(ctx, input)
	require.NoError(t, err)
	require.Equal(t, admissionLookupOutput(input), output)
	connection.fail.Store(true)
	output, err = engine.OriginAdmissionLookup(ctx, input)
	require.Error(t, err)
	require.Nil(t, output)
	select {
	case <-connection.attempted:
	case <-ctx.Done():
		t.Fatal("faulted response writer never ran")
	}
	select {
	case <-p.engineMux.Done():
	case <-ctx.Done():
		t.Fatal("real response write failure left the connection open")
	}
	require.ErrorIs(t, stop(), failure)
}
