// Design: docs/architecture/api/process-protocol.md -- response-finish mutation correlation.
package sdk

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Write has delivered the COMPLETE reply frame to the engine before this gate
// holds its return. The production SDK worker still owns reply completion and
// RequestAdmission.Finish, while the engine can issue the next framed request.
type originReplyFinishConn struct {
	net.Conn
	ctx       context.Context
	armed     atomic.Bool
	delivered chan struct{}
	release   chan struct{}
	once      sync.Once
}

func (c *originReplyFinishConn) Write(frame []byte) (int, error) {
	n, err := c.Conn.Write(frame)
	if err == nil && c.armed.CompareAndSwap(true, false) {
		close(c.delivered)
		select {
		case <-c.release:
		case <-c.ctx.Done():
			return n, c.ctx.Err()
		}
	}
	return n, err
}
func (c *originReplyFinishConn) unblock() { c.once.Do(func() { close(c.release) }) }

type originReplyFinishHarness struct {
	connection *ipc.PluginConn
	worker     *originIntakeWorker
	gate       *originReplyFinishConn
	admitted   chan uint64
	stop       func()
}

// Uses the production reader enrollment, mutating worker, cancellation worker
// and supervisor. The independent handler's wrapper observes admission AFTER
// admitWire, so the followup is deterministically queued before reply1 finishes.
// No production hook or registry alternative is introduced for observation.
func newOriginReplyFinishHarness(t *testing.T, ctx context.Context, handler OriginIntakeHandler) *originReplyFinishHarness {
	t.Helper()
	return newOriginReplyFinishHarnessWithClient(t, ctx, handler, nil)
}

func newOriginReplyFinishHarnessWithClient(t *testing.T, ctx context.Context, handler OriginIntakeHandler, wrapClient func(net.Conn) net.Conn) *originReplyFinishHarness {
	t.Helper()
	pluginEnd, engineEnd := net.Pipe()
	gate := &originReplyFinishConn{Conn: pluginEnd, ctx: ctx, delivered: make(chan struct{}), release: make(chan struct{})}
	gate.armed.Store(true)
	p := NewWithConn("origin-reply-finish-test", gate)
	p.OnOriginIntake(handler)
	p.OnCurrentSelection(func(_ context.Context, input rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
		results := make([]rpc.CurrentSelectionResult, len(input.Keys))
		for i, key := range input.Keys {
			results[i] = rpc.CurrentSelectionResult{Family: key.Family, KeyNLRI: key.KeyNLRI, Revision: key.Revision, Presence: rpc.CurrentSelectionAbsent}
		}
		return &rpc.CurrentSelectionOutput{Destination: input.Destination, DestinationSession: input.DestinationSession, Results: results}, nil
	})
	client := engineEnd
	if wrapClient != nil {
		client = wrapClient(engineEnd)
	}
	engine := rpc.NewMuxConn(rpc.NewConn(client, client))
	stopReadOnly, err := p.startCurrentSelectionWorker(ctx)
	require.NoError(t, err)
	work, cancel := context.WithCancel(ctx)
	w := &originIntakeWorker{plugin: p, handler: p.onOriginIntake, ctx: work, cancel: cancel,
		receiver: rpc.NewRequestReceiver(work, originIntakeRequestScope),
		requests: make(chan originIntakeJob, originIntakeQueueDepth), cancellations: make(chan originCancellationJob, originIntakeQueueDepth),
		done: make(chan struct{}), cancelDone: make(chan struct{}), supervisorDone: make(chan struct{})}
	h := &originReplyFinishHarness{connection: ipc.NewMuxPluginConn(engine), worker: w, gate: gate, admitted: make(chan uint64, 4)}
	remove, err := p.engineMux.RegisterIndependentRequestHandler(rpc.MethodOriginIntake, func(request *rpc.Request) error {
		err := w.admitWire(request)
		h.admitted <- request.ID
		return err
	})
	require.NoError(t, err)
	removeCancel, err := p.engineMux.RegisterIndependentRequestHandler(rpc.MethodCancelRequest, w.admitCancellation)
	require.NoError(t, err)
	go w.run()
	go w.runCancellations()
	go w.supervise(p.engineMux.Done())
	h.stop = sync.OnceFunc(func() {
		gate.unblock()
		remove()
		removeCancel()
		w.stopAdmission()
		cancel()
		if err := p.Close(); err != nil {
			t.Error(err)
		}
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
		if err := stopReadOnly(); err != nil {
			t.Error(err)
		}
		joined, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		for _, done := range []<-chan struct{}{w.supervisorDone, w.done, w.cancelDone} {
			select {
			case <-done:
			case <-joined.Done():
				t.Error("origin response-finish fixture worker did not join")
			}
		}
		w.receiver.CancelAll()
	})
	t.Cleanup(h.stop)
	return h
}

func runOriginReplyFinishFollowup(t *testing.T, cancelFollowup bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	entered := make(chan rpc.OriginIntakeInput, 2)
	canceled := make(chan struct{})
	release := make(chan struct{})
	releaseMutation := sync.OnceFunc(func() { close(release) })
	defer releaseMutation()
	var active atomic.Int32
	h := newOriginReplyFinishHarness(t, ctx, func(work context.Context, input rpc.OriginIntakeInput) (*rpc.UpdateRouteOutput, error) {
		if active.Add(1) != 1 {
			t.Error("origin callbacks overlapped despite the single mutation worker")
		}
		defer active.Add(-1)
		if input.Operation == rpc.OriginIntakeAnnounce {
			return originIntakeTestReply(input), nil
		}
		entered <- rpc.CloneOriginIntake(input)
		if cancelFollowup {
			<-work.Done()
			close(canceled)
		}
		<-release
		if cancelFollowup {
			return nil, work.Err()
		}
		return &rpc.UpdateRouteOutput{Withdrawn: 1}, nil
	})
	original := originIntakeTestInput()
	first, err := h.connection.SendOriginIntake(ctx, original)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.True(t, first.Retained())
	require.Zero(t, first.Announced)
	select {
	case <-h.gate.delivered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	var firstID uint64
	select {
	case firstID = <-h.admitted:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	// Request1 has answered over the wire, but its write/Finish remain held.
	followup := rpc.CloneOriginIntake(original)
	followup.Operation = rpc.OriginIntakeWithdraw
	next, stopNext := context.WithCancel(ctx)
	defer stopNext()
	type answer struct {
		output *rpc.UpdateRouteOutput
		err    error
	}
	finished := make(chan answer, 1)
	go func() { output, err := h.connection.SendOriginIntake(next, followup); finished <- answer{output, err} }()
	var nextID uint64
	select {
	case nextID = <-h.admitted:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	require.NotEqual(t, firstID, nextID, "same receipt still owns independent request correlations")
	select {
	case value := <-entered:
		t.Fatalf("followup mutated before prior response finished: %+v", value)
	default:
	}
	select {
	case value := <-finished:
		t.Fatalf("followup completed while prior reply remained held: %v", value.err)
	default:
	}
	h.gate.unblock()
	var seen rpc.OriginIntakeInput
	select {
	case seen = <-entered:
	case value := <-finished:
		t.Fatalf("same-receipt followup was refused before its mutation: %v", value.err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	require.Equal(t, original.PeerOwner, seen.PeerOwner)
	require.Equal(t, original.OriginReceipt, seen.OriginReceipt)
	require.Equal(t, original.Route, seen.Route)
	require.Equal(t, original.Source, seen.Source)
	require.Equal(t, original.Producer, seen.Producer)
	// Handler entry proves bytes were consumed, but the caller's request Write
	// can still own its cancellation hook. This existing read-only RPC crosses
	// the SAME client Conn.mu and proves that write/hook joined before cancel.
	marker := rpc.CurrentSelectionInput{Destination: original.Destination, DestinationSession: 1,
		Keys: []rpc.CurrentSelectionKey{{Family: original.Route.Family, KeyNLRI: original.Route.KeyNLRI, Revision: 1}}}
	snapshot, err := h.connection.SendCurrentSelection(ctx, marker)
	require.NoError(t, err, "independent read-only RPC must finish the prior client write while mutation remains held")
	require.NotNil(t, snapshot)
	require.Zero(t, snapshot.Cut)
	require.Len(t, snapshot.Results, 1)
	require.Equal(t, rpc.CurrentSelectionAbsent, snapshot.Results[0].Presence)
	if cancelFollowup {
		stopNext()
		select {
		case <-canceled:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	select {
	case value := <-finished:
		t.Fatalf("unfinished followup mutation acknowledged: %v", value.err)
	default:
	}
	releaseMutation()
	var result answer
	select {
	case result = <-finished:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if cancelFollowup {
		require.ErrorIs(t, result.err, context.Canceled)
	} else {
		require.NoError(t, result.err)
		require.NotNil(t, result.output)
		require.Equal(t, uint32(1), result.output.Withdrawn)
	}
	require.NoError(t, ctx.Err(), "settling one request must keep the connection lifetime live")
	original.OriginReceipt++
	healthy, err := h.connection.SendOriginIntake(ctx, original)
	require.NoError(t, err)
	require.NotNil(t, healthy)
	require.True(t, healthy.Retained())
	require.Zero(t, healthy.Announced)
	var healthyID uint64
	select {
	case healthyID = <-h.admitted:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	_, err = h.worker.receiver.CancelAndJoin(ctx, healthyID, rpc.MethodOriginIntake)
	require.NoError(t, err, "settle the observed final reply completion before checking retained bookkeeping")
	// A consumed cancellation reply can itself precede its admission Finish.
	// Stop joins BOTH production workers before asserting zero retained state.
	h.stop()
	activeRequests, resources := h.worker.receiver.Counts()
	require.Zero(t, activeRequests)
	require.Zero(t, resources)
}

func TestOriginIntakeFramedFollowupQueuesBeforeReplyFinish(t *testing.T) {
	runOriginReplyFinishFollowup(t, false)
}
func TestOriginIntakeFramedFollowupCancellationJoinsMutation(t *testing.T) {
	runOriginReplyFinishFollowup(t, true)
}

// Hold a successful client Write return after the complete frame was delivered.
// Cancellation while this local write still owns its hook legitimately fences
// the stream. Remote mutation completion must remain separate from that error.
type originActiveRequestWriteConn struct {
	net.Conn
	delivered   chan struct{}
	released    chan struct{}
	closed      chan struct{}
	first       atomic.Bool
	releaseOnce sync.Once
	closeOnce   sync.Once
}

func (c *originActiveRequestWriteConn) Write(frame []byte) (int, error) {
	n, err := c.Conn.Write(frame)
	if err == nil && c.first.CompareAndSwap(true, false) {
		close(c.delivered)
		select {
		case <-c.released:
		case <-c.closed:
		}
	}
	return n, err
}
func (c *originActiveRequestWriteConn) Close() error {
	err := c.Conn.Close()
	c.closeOnce.Do(func() { close(c.closed) })
	return err
}
func (c *originActiveRequestWriteConn) unblock() { c.releaseOnce.Do(func() { close(c.released) }) }

func TestOriginIntakeActiveClientWriteReturnsUnsettledAndJoinsSDKStop(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	entered, canceled, returned := make(chan struct{}), make(chan struct{}), make(chan struct{})
	release := make(chan struct{})
	releaseMutation := sync.OnceFunc(func() { close(release) })
	defer releaseMutation()
	var client *originActiveRequestWriteConn
	h := newOriginReplyFinishHarnessWithClient(t, ctx, func(work context.Context, _ rpc.OriginIntakeInput) (*rpc.UpdateRouteOutput, error) {
		defer close(returned)
		close(entered)
		<-work.Done()
		close(canceled)
		<-release
		return nil, work.Err()
	}, func(conn net.Conn) net.Conn {
		client = &originActiveRequestWriteConn{Conn: conn, delivered: make(chan struct{}), released: make(chan struct{}), closed: make(chan struct{})}
		client.first.Store(true)
		return client
	})
	t.Cleanup(client.unblock)
	call, stopCall := context.WithCancel(ctx)
	defer stopCall()
	type answer struct {
		output *rpc.UpdateRouteOutput
		err    error
	}
	finished := make(chan answer, 1)
	go func() {
		out, err := h.connection.SendOriginIntake(call, originIntakeTestInput())
		finished <- answer{out, err}
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	select {
	case <-client.delivered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	stopCall() // Deliberately BEFORE the successful client Write can return/disarm.
	select {
	case <-client.closed:
	case <-ctx.Done():
		t.Fatal("active-write cancellation did not fence the actual stream")
	}
	select {
	case <-canceled:
	case <-ctx.Done():
		t.Fatal("terminal transport did not cancel the actual SDK mutation")
	}
	var result answer
	select {
	case result = <-finished:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	var unsettled *rpc.CancellationUnsettledError
	require.ErrorAs(t, result.err, &unsettled, "terminal transport is uncertainty, never normal completed cancellation")
	require.Nil(t, result.output, "an unsettled stream supplies no completed mutation receipt")
	select {
	case <-returned:
		t.Fatal("held remote mutation was falsely declared complete")
	default:
	}
	select {
	case <-h.worker.done:
		t.Fatal("SDK mutation worker joined before its held handler returned")
	default:
	}
	stopEntered, stopped := make(chan struct{}), make(chan struct{})
	go func() { close(stopEntered); h.stop(); close(stopped) }()
	<-stopEntered
	select {
	case <-stopped:
		t.Fatal("SDK stop acknowledged while its mutation remained held")
	default:
	}
	require.NoError(t, ctx.Err(), "terminal child connection must not cancel its independent test owner")
	releaseMutation()
	select {
	case <-stopped:
	case <-ctx.Done():
		t.Fatal("released SDK stop did not join")
	}
	select {
	case <-returned:
	default:
		t.Fatal("SDK stop returned before actual handler completion")
	}
	activeRequests, resources := h.worker.receiver.Counts()
	require.Zero(t, activeRequests)
	require.Zero(t, resources)
}
