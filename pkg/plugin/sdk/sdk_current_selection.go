// Design: docs/architecture/api/process-protocol.md -- independent current selection validation.
// Related: sdk.go -- Run owns this worker's connection lifetime.
package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// CurrentSelectionHandler reads only a published desired-selection snapshot.
// It runs concurrently with ordinary callbacks and source-policy handlers and
// MUST NOT call engine RPC, perform I/O or acquire peer/election locks. Honor
// ctx. Return exact scope/key/revision echoes, presence and cut, no candidates.
// Output ownership passes to the SDK; keep no mutable alias to its bytes.
type CurrentSelectionHandler = rpc.CurrentSelectionHandler

// OnCurrentSelection registers current desired-state lookup before Run. Stage 1
// derives support from this registration. It grants no forwarding authority.
func (p *Plugin) OnCurrentSelection(handler CurrentSelectionHandler) {
	p.mu.Lock()
	p.onCurrentSelection = handler
	p.mu.Unlock()
}

// Sixteen maximum-size callback frames may wait behind the single active
// request, matching existing callback queue bounds. Saturation explicitly fails
// admission; the Mux reader never waits for the ordinary loop or a response.
const currentSelectionQueueDepth = 16

var errCurrentSelectionStopped = errors.New("current-selection connection stopped; retry on the live selecting connection")
var errCurrentSelectionFull = errors.New("current-selection queue is full; wait for an earlier lookup before retrying")

// currentSelectionQuery owns the validated raw input, including every native
// key. Its zero is not a query; only newCurrentSelectionQuery constructs it.
type currentSelectionQuery struct{ input rpc.CurrentSelectionInput }

func newCurrentSelectionQuery(input rpc.CurrentSelectionInput) (currentSelectionQuery, error) {
	if err := input.Validate(time.Now()); err != nil {
		return currentSelectionQuery{}, err
	}
	input.Keys = slices.Clone(input.Keys)
	for i := range input.Keys {
		input.Keys[i].KeyNLRI = slices.Clone(input.Keys[i].KeyNLRI)
	}
	return currentSelectionQuery{input: input}, nil
}

type currentSelectionAnswer struct {
	output *rpc.CurrentSelectionOutput
	err    error
}

// A job is either raw wire input (decoded by the worker), or an owned validated
// typed query. reply has capacity one, so caller cancellation never stalls stop.
type currentSelectionJob struct {
	wire  *rpc.Request
	query *currentSelectionQuery
	reply chan currentSelectionAnswer
}

// currentSelectionWorker is one long-lived connection worker. Independent Mux
// ingress and the typed slot enqueue directly, before ordinary callback queues.
// Cancellation and stop join the handler; there is no per-request goroutine.
type currentSelectionWorker struct {
	plugin    *Plugin
	handler   CurrentSelectionHandler
	requests  chan currentSelectionJob
	done      chan struct{}
	failure   atomic.Pointer[error]
	admission sync.Mutex
	stopped   bool
}

func (w *currentSelectionWorker) admitWire(request *rpc.Request) error {
	w.admission.Lock()
	defer w.admission.Unlock()
	if w.stopped {
		return errCurrentSelectionStopped
	}
	select {
	case w.requests <- currentSelectionJob{wire: request}:
		return nil
	default:
		w.failure.CompareAndSwap(nil, &errCurrentSelectionFull)
		return errCurrentSelectionFull
	}
}

func (w *currentSelectionWorker) lookup(ctx context.Context, input rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
	query, err := newCurrentSelectionQuery(input)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	reply := make(chan currentSelectionAnswer, 1)
	w.admission.Lock()
	if w.stopped {
		w.admission.Unlock()
		return nil, errCurrentSelectionStopped
	}
	select {
	case w.requests <- currentSelectionJob{query: &query, reply: reply}:
		w.admission.Unlock()
	default:
		w.admission.Unlock()
		return nil, errCurrentSelectionFull
	}
	select {
	case answer := <-reply:
		return answer.output, answer.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-w.done:
		select {
		case answer := <-reply:
			return answer.output, answer.err
		default:
			return nil, errCurrentSelectionStopped
		}
	}
}

func (w *currentSelectionWorker) run(ctx context.Context, transportDone <-chan struct{}) {
	defer close(w.done)
	defer w.refuseQueued()
	defer w.stopAdmission()
	for {
		select {
		case <-ctx.Done():
			return
		case <-transportDone:
			return
		case job := <-w.requests:
			if err := ctx.Err(); err != nil {
				w.refuse(job)
				return
			}
			query, err := w.decode(job)
			if err != nil {
				w.answer(ctx, job, nil, err)
				continue
			}
			output, err, panicked := w.serve(ctx, query)
			w.answer(ctx, job, output, err)
			if panicked {
				w.failure.Store(&err)
				if w.plugin.bridge != nil {
					w.plugin.bridge.FailCallbacks(err)
				}
				w.plugin.engineMux.Close() //nolint:errcheck // panic ends the connection; stop returns the cause
				return
			}
		}
	}
}

func (w *currentSelectionWorker) decode(job currentSelectionJob) (currentSelectionQuery, error) {
	if job.query != nil {
		return *job.query, nil
	}
	var input rpc.CurrentSelectionInput
	if err := json.Unmarshal(job.wire.Params, &input); err != nil {
		return currentSelectionQuery{}, fmt.Errorf("current-selection input: %w", err)
	}
	return newCurrentSelectionQuery(input)
}

func (w *currentSelectionWorker) serve(ctx context.Context, query currentSelectionQuery) (output *rpc.CurrentSelectionOutput, err error, panicked bool) {
	ctx, cancel := context.WithDeadline(ctx, time.UnixMilli(query.input.DeadlineMS))
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err, false
	}
	defer func() {
		if value := recover(); value != nil {
			output, err, panicked = nil, fmt.Errorf("plugin %s current-selection handler panicked: %v; restart the selecting plugin", w.plugin.name, value), true
		}
	}()
	output, err = w.handler(ctx, query.input)
	if err != nil {
		return nil, err, false
	}
	if err := ctx.Err(); err != nil {
		return nil, err, false
	}
	if err := output.Validate(query.input); err != nil {
		return nil, err, false
	}
	return output, nil, false
}

func (w *currentSelectionWorker) answer(ctx context.Context, job currentSelectionJob, output *rpc.CurrentSelectionOutput, answerErr error) {
	if job.reply != nil {
		job.reply <- currentSelectionAnswer{output: output, err: answerErr}
		return
	}
	// Malformed input must also get a bounded error response. A valid request's
	// own deadline further shortens the connection's response budget.
	ctx, cancel := context.WithTimeout(ctx, rpc.CancellableRequestTimeout)
	defer cancel()
	var header struct {
		DeadlineMS int64 `json:"deadline-ms"`
	}
	if err := json.Unmarshal(job.wire.Params, &header); err == nil {
		if header.DeadlineMS > 0 {
			var deadlineCancel context.CancelFunc
			ctx, deadlineCancel = context.WithDeadline(ctx, time.UnixMilli(header.DeadlineMS))
			defer deadlineCancel()
		}
	}
	var err error
	if answerErr != nil {
		err = w.plugin.engineMux.SendError(ctx, job.wire.ID, answerErr.Error())
	} else {
		err = w.plugin.engineMux.SendResult(ctx, job.wire.ID, output)
	}
	if err != nil {
		if ctx.Err() == nil {
			w.failure.CompareAndSwap(nil, &err)
		}
		w.plugin.engineMux.Close() //nolint:errcheck // failed reply means this transport cannot continue
	}
}

func (w *currentSelectionWorker) refuse(job currentSelectionJob) {
	if job.reply != nil {
		job.reply <- currentSelectionAnswer{err: errCurrentSelectionStopped}
	}
	// Wire callers observe connection closure; stop must not wait on I/O for
	// queued requests that have never read the snapshot.
	if job.wire != nil {
		w.plugin.engineMux.Close()
	} //nolint:errcheck // queued wire lookups must observe stop
}

func (w *currentSelectionWorker) stopAdmission() {
	w.admission.Lock()
	w.stopped = true
	w.admission.Unlock()
}

func (w *currentSelectionWorker) refuseQueued() {
	for {
		select {
		case job := <-w.requests:
			w.refuse(job)
		default:
			return
		}
	}
}

// startCurrentSelectionWorker installs independent ingress before ready and
// OnStarted. The returned stop MUST run; it fences admission, cancels and joins
// the one worker, then reports any transport or handler failure.
func (p *Plugin) startCurrentSelectionWorker(ctx context.Context) (func() error, error) {
	p.mu.Lock()
	handler := p.onCurrentSelection
	p.mu.Unlock()
	if handler == nil {
		return func() error { return nil }, nil
	}
	w := &currentSelectionWorker{plugin: p, handler: handler,
		requests: make(chan currentSelectionJob, currentSelectionQueueDepth), done: make(chan struct{})}
	workerCtx, cancel := context.WithCancel(ctx)
	remove, err := p.engineMux.RegisterIndependentRequestHandler(rpc.MethodCurrentSelection, w.admitWire)
	if err != nil {
		cancel()
		return nil, err
	}
	var transportDone <-chan struct{}
	if p.bridge != nil {
		p.bridge.SetCurrentSelection(w.lookup)
	} else {
		transportDone = p.engineMux.Done()
	}
	go w.run(workerCtx, transportDone)
	var once sync.Once
	return func() error {
		once.Do(func() { remove(); w.stopAdmission(); cancel(); <-w.done; w.refuseQueued() })
		if failure := w.failure.Load(); failure != nil {
			return *failure
		}
		return nil
	}, nil
}
