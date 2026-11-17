// Design: docs/architecture/api/process-protocol.md — applied event settlement.
// Related: sdk_async_callback.go — registered off-loop callback dispatch.

package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// EventSettlementHandler finishes asynchronous work admitted by earlier events.
// The engine MUST await event application before calling it. The handler MUST
// await downstream completion before returning, honor ctx, and synchronize with
// event/current-selection handlers, which remain live while this worker waits.
type EventSettlementHandler func(context.Context) error

// OnEventSettlement registers an independent settlement worker before Run.
// Run derives the Stage-1 declaration from this registration. Nil removes it.
func (p *Plugin) OnEventSettlement(handler EventSettlementHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if handler == nil {
		p.setAsyncCallback(rpc.MethodEventSettlement, nil)
		return
	}
	p.setAsyncCallback(rpc.MethodEventSettlement, func(ctx context.Context, abort func(error)) asyncCallbackWorker {
		worker := newEventSettlementWorker(p.name, handler, abort)
		workerCtx, cancel := context.WithCancel(ctx)
		go worker.run(workerCtx)
		return asyncCallbackWorker{admit: worker.admit, failure: worker.failure, stop: func() {
			cancel()
			<-worker.done
			worker.refuseQueued(errEventSettlementStopped)
		}}
	})
}

const eventSettlementQueueDepth = 16

var errEventSettlementStopped = errors.New("plugin stopped before event settlement; retry on the live plugin")

// eventSettlementQuery is admitted only after its raw RPC input was validated.
// Its zero is not a request. newEventSettlementQuery is its sole constructor.
type eventSettlementQuery struct{ deadline time.Time }

func newEventSettlementQuery(params json.RawMessage) (eventSettlementQuery, error) {
	var input rpc.EventSettlementInput
	if err := json.Unmarshal(params, &input); err != nil {
		return eventSettlementQuery{}, err
	}
	if err := input.Validate(time.Now()); err != nil {
		return eventSettlementQuery{}, err
	}
	return eventSettlementQuery{deadline: time.UnixMilli(input.DeadlineMS)}, nil
}

type eventSettlementRequest struct {
	query  eventSettlementQuery
	answer asyncCallbackAnswer
}

// eventSettlementWorker owns one bounded FIFO and one long-lived goroutine.
// Its factory MUST start run and its stop adapter MUST cancel and join done.
// Requests are not coalesced: one admitted after later events covers a newer
// frontier. Admission is nonblocking and refuses overload. Safe for concurrent use.
type eventSettlementWorker struct {
	plugin   string
	handler  EventSettlementHandler
	abort    func(error)
	requests chan eventSettlementRequest
	done     chan struct{}
	failed   chan error
}

func newEventSettlementWorker(plugin string, handler EventSettlementHandler, abort func(error)) *eventSettlementWorker {
	return &eventSettlementWorker{plugin: plugin, handler: handler, abort: abort,
		requests: make(chan eventSettlementRequest, eventSettlementQueueDepth),
		done:     make(chan struct{}), failed: make(chan error, 1)}
}

func (worker *eventSettlementWorker) admit(params json.RawMessage, answer asyncCallbackAnswer) error {
	query, err := newEventSettlementQuery(params)
	if err != nil {
		return err
	}
	select {
	case worker.requests <- eventSettlementRequest{query: query, answer: answer}:
		return nil
	default:
		return fmt.Errorf("plugin %s event settlement queue is full; wait for an earlier receipt before retrying", worker.plugin)
	}
}

func (worker *eventSettlementWorker) failure() error {
	select {
	case err := <-worker.failed:
		return err
	default:
		return nil
	}
}

// run stops on cancellation or a handler panic, refuses every queued request,
// and closes done. Its factory's stop adapter MUST join done before returning.
func (worker *eventSettlementWorker) run(ctx context.Context) {
	defer close(worker.done)
	for {
		select {
		case <-ctx.Done():
			worker.refuseQueued(errEventSettlementStopped)
			return
		case request := <-worker.requests:
			if panicked := worker.serve(ctx, request); panicked != nil {
				worker.refuseQueued(panicked)
				worker.failed <- panicked
				worker.abort(panicked)
				return
			}
		}
	}
}

func (worker *eventSettlementWorker) serve(ctx context.Context, request eventSettlementRequest) error {
	ctx, cancel := context.WithDeadline(ctx, request.query.deadline)
	defer cancel()
	err := ctx.Err()
	if err == nil {
		var panicked error
		err, panicked = worker.call(ctx)
		if panicked != nil {
			request.answer(request.query.deadline, nil, panicked)
			return panicked
		}
		if err == nil {
			err = ctx.Err()
		}
	}
	worker.answer(request, err)
	return nil
}

func (worker *eventSettlementWorker) call(ctx context.Context) (err, panicked error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			panicked = fmt.Errorf("plugin %s event settlement handler panic: %v", worker.plugin, recovered)
		}
	}()
	return worker.handler(ctx), nil
}

func (worker *eventSettlementWorker) answer(request eventSettlementRequest, cause error) {
	output := rpc.EventSettlementOutput{DeadlineMS: request.query.deadline.UnixMilli(), Status: rpc.StatusOK}
	if cause != nil {
		output.Status, output.Error = rpc.StatusError, cause.Error()
		if len(output.Error) > rpc.EventSettlementErrorBytesMax {
			request.answer(request.query.deadline, nil, errors.New("event settlement failure exceeds 4096 bytes"))
			return
		}
	}
	body, err := json.Marshal(output)
	request.answer(request.query.deadline, body, err)
}

// refuseQueued consumes at most eventSettlementQueueDepth requests. It runs
// after serving stops; the stop adapter repeats it after joining for admissions
// that raced a panic or cancellation, so no admitted caller is left unanswered.
func (worker *eventSettlementWorker) refuseQueued(cause error) {
	for {
		select {
		case request := <-worker.requests:
			worker.answer(request, cause)
		default:
			return
		}
	}
}
