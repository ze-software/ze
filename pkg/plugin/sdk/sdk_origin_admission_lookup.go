// Design: docs/architecture/api/process-protocol.md -- independent original admission lookup.
// Related: sdk_current_selection.go -- same bounded independent Mux ingress.
package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// OriginAdmissionLookupHandler reads an immutable published accepted-record
// snapshot. It MUST NOT mutate candidates, call the engine, perform I/O or take
// election locks. It MUST honor ctx and transfer output/Community ownership to
// the SDK. Missing or stale IDs and retiring announce requests return an error.
type OriginAdmissionLookupHandler = rpc.OriginAdmissionLookupHandler

// OnOriginAdmissionLookup registers the read-only handler before Run. Stage 1
// derives support; the declaration confers no publisher or peer authority.
func (p *Plugin) OnOriginAdmissionLookup(handler OriginAdmissionLookupHandler) {
	p.mu.Lock()
	p.onOriginAdmissionLookup = handler
	p.mu.Unlock()
}

// One active request and sixteen maximum-size waiting frames bound this lane.
// The sole Mux reader never waits for a lookup handler or ordinary callbacks.
const originAdmissionLookupQueueDepth = 16

var errOriginAdmissionLookupStopped = errors.New("origin-admission-lookup connection stopped; retry on the original live accepting connection")
var errOriginAdmissionLookupFull = errors.New("origin-admission-lookup queue is full; wait for an earlier lookup before retrying")

// Only this validating constructor produces a usable query. Zero is not valid;
// the worker receives either one owned query or raw wire input, never both.
type originAdmissionLookupQuery struct {
	input rpc.OriginAdmissionLookupInput
}

func newOriginAdmissionLookupQuery(input rpc.OriginAdmissionLookupInput) (originAdmissionLookupQuery, error) {
	if err := input.Validate(time.Now()); err != nil {
		return originAdmissionLookupQuery{}, err
	}
	return originAdmissionLookupQuery{input: input}, nil
}

type originAdmissionLookupAnswer struct {
	output *rpc.OriginAdmissionLookupOutput
	err    error
}

// These sealed value variants have no typed-nil case. Only admitWire and lookup
// enqueue jobs. A typed reply's capacity one lets cancellation leave safely.
type originAdmissionLookupJob interface{ originAdmissionLookupJob() }
type originAdmissionLookupWire struct{ request *rpc.Request }
type originAdmissionLookupTyped struct {
	query originAdmissionLookupQuery
	reply chan originAdmissionLookupAnswer
}

func (originAdmissionLookupWire) originAdmissionLookupJob()  {}
func (originAdmissionLookupTyped) originAdmissionLookupJob() {}

// The connection owns this one long-lived worker. startOriginAdmissionLookupWorker
// creates its queue and starts run; its returned stop MUST fence admission,
// cancel and join the worker. No per-request goroutine or unbounded queue exists.
type originAdmissionLookupWorker struct {
	plugin    *Plugin
	handler   OriginAdmissionLookupHandler
	requests  chan originAdmissionLookupJob
	done      chan struct{}
	failure   atomic.Pointer[error]
	admission sync.Mutex
	stopped   bool
}

func (worker *originAdmissionLookupWorker) admitWire(request *rpc.Request) error {
	if request == nil {
		return errors.New("origin-admission-lookup request is missing; send a complete callback request")
	}
	worker.admission.Lock()
	defer worker.admission.Unlock()
	if worker.stopped {
		return errOriginAdmissionLookupStopped
	}
	select {
	case worker.requests <- originAdmissionLookupWire{request: request}:
		return nil
	default:
		worker.failure.CompareAndSwap(nil, &errOriginAdmissionLookupFull)
		return errOriginAdmissionLookupFull
	}
}

func (worker *originAdmissionLookupWorker) lookup(ctx context.Context, input rpc.OriginAdmissionLookupInput) (*rpc.OriginAdmissionLookupOutput, error) {
	query, err := newOriginAdmissionLookupQuery(input)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	reply := make(chan originAdmissionLookupAnswer, 1)
	worker.admission.Lock()
	if worker.stopped {
		worker.admission.Unlock()
		return nil, errOriginAdmissionLookupStopped
	}
	select {
	case worker.requests <- originAdmissionLookupTyped{query: query, reply: reply}:
		worker.admission.Unlock()
	default:
		worker.admission.Unlock()
		return nil, errOriginAdmissionLookupFull
	}
	select {
	case answer := <-reply:
		return answer.output, answer.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-worker.done:
		select {
		case answer := <-reply:
			return answer.output, answer.err
		default:
			return nil, errOriginAdmissionLookupStopped
		}
	}
}

func (worker *originAdmissionLookupWorker) run(ctx context.Context, transportDone <-chan struct{}) {
	runReadSnapshotWorker[originAdmissionLookupJob, originAdmissionLookupQuery, rpc.OriginAdmissionLookupOutput](ctx, transportDone, worker.requests, worker.done, worker, func(err error) {
		failReadSnapshotWorker(worker.plugin, &worker.failure, err)
	})
}

func (worker *originAdmissionLookupWorker) decode(job originAdmissionLookupJob) (originAdmissionLookupQuery, error) {
	switch value := job.(type) {
	case originAdmissionLookupTyped:
		return value.query, nil
	case originAdmissionLookupWire:
		var input rpc.OriginAdmissionLookupInput
		if err := json.Unmarshal(value.request.Params, &input); err != nil {
			return originAdmissionLookupQuery{}, fmt.Errorf("origin-admission-lookup input: %w", err)
		}
		return newOriginAdmissionLookupQuery(input)
	}
	return originAdmissionLookupQuery{}, errors.New("origin-admission-lookup internal job is missing; restart the accepting plugin")
}

func (worker *originAdmissionLookupWorker) serve(ctx context.Context, query originAdmissionLookupQuery) (*rpc.OriginAdmissionLookupOutput, error, bool) {
	return serveReadSnapshot(ctx, query.input, query.input.DeadlineMS, worker.handler,
		(*rpc.OriginAdmissionLookupOutput).Validate, func(value any) error {
			return fmt.Errorf("plugin %s origin-admission-lookup handler panicked: %v; restart the accepting plugin", worker.plugin.name, value)
		})
}

func (worker *originAdmissionLookupWorker) answer(ctx context.Context, job originAdmissionLookupJob, output *rpc.OriginAdmissionLookupOutput, answerErr error) {
	switch value := job.(type) {
	case originAdmissionLookupTyped:
		value.reply <- originAdmissionLookupAnswer{output: output, err: answerErr}
	case originAdmissionLookupWire:
		// Work expiry is a refused read, not a broken stream. Reply on a
		// fresh bounded budget derived from this worker's process lifetime,
		// never the expired request deadline. Process stop still cancels I/O.
		ctx, cancel := context.WithTimeout(ctx, rpc.CancellableRequestTimeout)
		defer cancel()
		var err error
		if answerErr != nil {
			err = worker.plugin.engineMux.SendError(ctx, value.request.ID, answerErr.Error())
		} else {
			err = worker.plugin.engineMux.SendResult(ctx, value.request.ID, output)
		}
		if err != nil {
			if ctx.Err() == nil {
				worker.failure.CompareAndSwap(nil, &err)
			}
			worker.plugin.engineMux.Close() //nolint:errcheck // A failed reply closes this unusable transport.
		}
	}
}

func (worker *originAdmissionLookupWorker) refuse(job originAdmissionLookupJob) {
	switch value := job.(type) {
	case originAdmissionLookupTyped:
		value.reply <- originAdmissionLookupAnswer{err: errOriginAdmissionLookupStopped}
	case originAdmissionLookupWire:
		worker.plugin.engineMux.Close() //nolint:errcheck // Never-entered wire callers observe closure without stop waiting on I/O.
	}
}

func (worker *originAdmissionLookupWorker) stopAdmission() {
	worker.admission.Lock()
	worker.stopped = true
	worker.admission.Unlock()
}

func (worker *originAdmissionLookupWorker) refuseQueued() {
	// Only the fixed queue population is drained after admission is sealed.
	for {
		select {
		case job := <-worker.requests:
			worker.refuse(job)
		default:
			return
		}
	}
}

// startOriginAdmissionLookupWorker installs independent ingress before ready.
// The caller MUST run the returned stop, which fences, cancels and joins this
// worker and reports its first handler or transport failure.
func (p *Plugin) startOriginAdmissionLookupWorker(ctx context.Context) (func() error, error) {
	p.mu.Lock()
	handler := p.onOriginAdmissionLookup
	p.mu.Unlock()
	if handler == nil {
		return func() error { return nil }, nil
	}
	worker := &originAdmissionLookupWorker{plugin: p, handler: handler,
		requests: make(chan originAdmissionLookupJob, originAdmissionLookupQueueDepth), done: make(chan struct{})}
	return p.startReadSnapshotWorker(ctx, rpc.MethodOriginAdmissionLookup, worker, worker.done, &worker.failure,
		func() { p.bridge.SetOriginAdmissionLookup(worker.lookup) })
}
