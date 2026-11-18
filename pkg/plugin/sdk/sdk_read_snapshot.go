// Design: docs/architecture/api/process-protocol.md -- independent snapshot callback workers.
// Related: sdk_current_selection.go, sdk_origin_admission_lookup.go -- protocol-owned queues and replies.
package sdk

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Both existing workers keep their concrete jobs, validation and answer paths.
// This interface shares only execution, without storing another worker or queue.
// Implementations MUST seal admission before draining their fixed queue.
// Callers MUST supply a constructed non-nil concrete worker; nil has no use.
type readSnapshotWork[Job, Query, Output any] interface {
	stopAdmission()
	refuseQueued()
	refuse(Job)
	decode(Job) (Query, error)
	serve(context.Context, Query) (*Output, error, bool)
	answer(context.Context, Job, *Output, error)
}

// runReadSnapshotWorker MUST own the caller's done channel until every handler
// has returned. The returned stop from startReadSnapshotWorker MUST cancel and
// join it. Work is bounded by the connection lifetime and each query deadline.
func runReadSnapshotWorker[Job, Query, Output any](ctx context.Context, transportDone <-chan struct{},
	requests <-chan Job, done chan struct{}, worker readSnapshotWork[Job, Query, Output], fail func(error)) {
	defer close(done)
	defer worker.refuseQueued()
	defer worker.stopAdmission()
	// One long-lived worker ends at cancellation, transport loss or a handler
	// defect. Each concrete queue has sixteen slots and never closes early.
	for {
		select {
		case <-ctx.Done():
			return
		case <-transportDone:
			return
		case job := <-requests:
			if err := ctx.Err(); err != nil {
				worker.refuse(job)
				return
			}
			query, err := worker.decode(job)
			if err != nil {
				worker.answer(ctx, job, nil, err)
				continue
			}
			output, err, panicked := worker.serve(ctx, query)
			worker.answer(ctx, job, output, err)
			if panicked {
				fail(err)
				return
			}
		}
	}
}

func failReadSnapshotWorker(plugin *Plugin, failure *atomic.Pointer[error], err error) {
	failure.Store(&err)
	if plugin.bridge != nil {
		plugin.bridge.FailCallbacks(err)
	}
	plugin.engineMux.Close() //nolint:errcheck // A handler defect ends the connection; stop reports its cause.
}

// serveReadSnapshot shares bounded invocation, panic handling and result
// validation. It never chooses a reply deadline; each protocol's answer method
// retains that separate contract. Input ownership stays with its typed query.
func serveReadSnapshot[Input, Output any](ctx context.Context, input Input, deadlineMS int64,
	handler func(context.Context, Input) (*Output, error), validate func(*Output, Input) error,
	panicError func(any) error) (output *Output, err error, panicked bool) {
	ctx, cancel := context.WithDeadline(ctx, time.UnixMilli(deadlineMS))
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err, false
	}
	defer func() {
		if value := recover(); value != nil {
			output, err, panicked = nil, panicError(value), true
		}
	}()
	output, err = handler(ctx, input)
	if err != nil {
		return nil, err, false
	}
	if err := ctx.Err(); err != nil {
		return nil, err, false
	}
	if err := validate(output, input); err != nil {
		return nil, err, false
	}
	return output, nil, false
}

// readSnapshotConnection is implemented only by the two constructed non-nil
// snapshot workers; a nil interface or typed nil is never a valid owner.
type readSnapshotConnection interface {
	admitWire(*rpc.Request) error
	stopAdmission()
	refuseQueued()
	run(context.Context, <-chan struct{})
}

// startReadSnapshotWorker installs only this worker's independent ingress.
// The caller MUST run the returned stop: it removes ingress, seals admission,
// cancels and joins runReadSnapshotWorker, then drains the bounded queue.
// The worker, queue, failure and bridge slot remain owned by its concrete caller.
func (p *Plugin) startReadSnapshotWorker(ctx context.Context, method string, worker readSnapshotConnection,
	done <-chan struct{}, failure *atomic.Pointer[error], publish func()) (func() error, error) {
	workerContext, cancel := context.WithCancel(ctx)
	remove, err := p.engineMux.RegisterIndependentRequestHandler(method, worker.admitWire)
	if err != nil {
		cancel()
		return nil, err
	}
	var transportDone <-chan struct{}
	if p.bridge != nil {
		publish()
	} else {
		transportDone = p.engineMux.Done()
	}
	go worker.run(workerContext, transportDone)
	var once sync.Once
	return func() error {
		once.Do(func() { remove(); worker.stopAdmission(); cancel(); <-done; worker.refuseQueued() })
		if failure := failure.Load(); failure != nil {
			return *failure
		}
		return nil
	}, nil
}
