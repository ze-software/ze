// Design: docs/architecture/api/process-protocol.md — source-policy lifecycle
// Overview: sdk_dispatch.go — event loop and callback dispatch
// Related: sdk_callbacks.go — OnSourcePolicy registers the handler this file runs
// Related: sdk_drain.go — the stop drain worker, a separate one-shot lifecycle

package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// SourcePolicyHandler settles one source-policy operation for a retained
// source. It runs on the connection's source-policy worker, never on the
// callback loop, so events and other callbacks keep being applied while it
// waits for its own workers and downstream writes. It therefore runs
// concurrently with OnEvent and every other callback handler, and MUST
// synchronize the state it shares with them. ctx ends at the request's
// absolute deadline or when the connection's loop stops; the handler MUST
// return when it ends.
type SourcePolicyHandler func(ctx context.Context, input rpc.SourcePolicyInput) error

// sourcePolicyQueueDepth is the number of admitted source-policy requests
// that may wait behind the one the worker is running. The coordinator awaits
// each owner's receipt before it sends that source's next operation, so the
// queue holds at most one request for each source being changed at once. A
// request that finds the queue full is answered at once with an error receipt
// and its handler never runs: the loop never blocks on the queue.
const sourcePolicyQueueDepth = 16

// sourcePolicyErrorBytesMax bounds the handler error a receipt may carry.
const sourcePolicyErrorBytesMax = 4096

// errSourcePolicyStopped answers a request still queued when the loop stops.
var errSourcePolicyStopped = errors.New("plugin connection stopped before the request ran")

// sourcePolicyAnswer sends one request's answer for its own request ID: the
// receipt, or err when the request has no receipt (an oversized handler error,
// a handler panic). deadline is the request's own deadline-ms, which bounds
// the send (workerAnswerContext): the answer may be written after the loop's context has ended. It
// MUST NOT wait on the worker, because admission calls it from the callback
// loop for a queue-full refusal.
type sourcePolicyAnswer func(deadline time.Time, receipt json.RawMessage, err error)

// sourcePolicyRequest is one admitted request: its validated input and the
// answer the worker sends once the handler has returned.
type sourcePolicyRequest struct {
	input  rpc.SourcePolicyInput
	answer sourcePolicyAnswer
}

// deadline is the request's absolute deadline-ms.
func (r sourcePolicyRequest) deadline() time.Time { return time.UnixMilli(r.input.DeadlineMS) }

// sourcePolicyWorker runs the plugin's source-policy handler off the callback
// loop. It is one long-lived goroutine per connection: startSourcePolicyWorker
// builds it and starts run once, the callback loop queues each request with
// admit in arrival order, and when the loop ends it cancels run's context and
// waits for done. run serves one request at a time, to completion, in the
// order admitted, so handler calls never overlap and each source's operations
// reach the owner in the order the engine sent them.
//
// Admission runs on the callback loop after every callback that arrived
// earlier has been applied, so a handler never sees state older than an event
// delivered before its request. Events that arrive while it runs are applied
// by the loop in their own order. Unlike the stop drain, this worker has no
// one-shot state: it serves every operation of every transaction for the
// connection's whole life.
//
// A handler panic leaves the owner's state unknown, so the worker answers the
// request with an error, records the failure, and calls abort, which ends the
// connection; the loop then returns that failure. Safe for concurrent use.
type sourcePolicyWorker struct {
	plugin   string
	handler  SourcePolicyHandler
	abort    func(err error)
	requests chan sourcePolicyRequest
	failed   chan error // capacity 1: the panic that ended the connection
	done     chan struct{}
}

func newSourcePolicyWorker(plugin string, handler SourcePolicyHandler, abort func(error)) *sourcePolicyWorker {
	return &sourcePolicyWorker{
		plugin:   plugin,
		handler:  handler,
		abort:    abort,
		requests: make(chan sourcePolicyRequest, sourcePolicyQueueDepth),
		failed:   make(chan error, 1),
		done:     make(chan struct{}),
	}
}

// admit validates a request and queues it for the worker. A malformed,
// unknown or already expired request returns its refusal for the loop to
// answer as a callback error, before any owner state changes. A request that
// finds the queue full is answered here with an error receipt. It runs on the
// callback loop and never blocks.
func (w *sourcePolicyWorker) admit(params json.RawMessage, answer sourcePolicyAnswer) error {
	var input rpc.SourcePolicyInput
	if err := json.Unmarshal(params, &input); err != nil {
		return fmt.Errorf("unmarshal source-policy: %w", err)
	}
	if err := input.Validate(time.Now()); err != nil {
		return err
	}
	select {
	case w.requests <- sourcePolicyRequest{input: input, answer: answer}:
		return nil
	default:
		refusal := errors.New("plugin " + w.plugin + ": source-policy queue full (" +
			strconv.Itoa(sourcePolicyQueueDepth) + " requests waiting); " +
			"send the next request after the owner answers an earlier one")
		receipt, err := sourcePolicyReceipt(input, refusal)
		answer(time.UnixMilli(input.DeadlineMS), receipt, err)
		return nil
	}
}

// failure returns the handler panic that ended the connection, or nil. The
// loop MUST return it when its transport closes, so the panic is not read as
// a clean shutdown. A nil worker has no failure.
func (w *sourcePolicyWorker) failure() error {
	if w == nil {
		return nil
	}
	select {
	case err := <-w.failed:
		return err
	default:
		return nil
	}
}

// run serves admitted requests until ctx ends or a handler panics, then
// answers every request still queued with a failed receipt. After a panic it
// runs nothing more: the owner's state is unknown, so the queued requests are
// refused, the failure is recorded, and abort ends the connection. It MUST be
// started once per connection, and the loop MUST cancel ctx and wait for done
// before it returns.
func (w *sourcePolicyWorker) run(ctx context.Context) {
	defer close(w.done)
	for {
		select {
		case request := <-w.requests:
			panicked := w.serve(ctx, request)
			if panicked == nil {
				continue
			}
			w.refuseQueued(panicked)
			w.failed <- panicked // capacity 1, and run stops after the first panic
			w.abort(panicked)
			return
		case <-ctx.Done():
			w.refuseQueued(errSourcePolicyStopped)
			return
		}
	}
}

// refuseQueued answers each request left in the queue with a failed receipt
// naming cause. Bounded by sourcePolicyQueueDepth: it runs only once the
// worker has stopped serving, or after run has returned.
func (w *sourcePolicyWorker) refuseQueued(cause error) {
	refusal := fmt.Errorf("request not run: %w", cause)
	for {
		select {
		case request := <-w.requests:
			receipt, err := sourcePolicyReceipt(request.input, refusal)
			request.answer(request.deadline(), receipt, err)
		default:
			return
		}
	}
}

// serve runs the handler for one request under the request's absolute
// deadline and answers only after the handler has returned. A request whose
// deadline passed while it waited is failed without running the handler. A
// handler panic is answered as an error and returned, so run stops.
func (w *sourcePolicyWorker) serve(ctx context.Context, request sourcePolicyRequest) error {
	deadline := request.deadline()
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	err := ctx.Err()
	if err == nil {
		var panicked error
		err, panicked = w.call(ctx, request.input)
		if panicked != nil {
			request.answer(deadline, nil, panicked)
			return panicked
		}
		if err == nil {
			err = ctx.Err()
		}
	}
	receipt, answerErr := sourcePolicyReceipt(request.input, err)
	request.answer(deadline, receipt, answerErr)
	return nil
}

// call runs the handler, turning a panic into panicked. The recover covers the
// handler alone, so a fault while answering can never send a second answer.
func (w *sourcePolicyWorker) call(ctx context.Context, input rpc.SourcePolicyInput) (handlerErr, panicked error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			panicked = fmt.Errorf("plugin %s: source-policy handler panic: %v", w.plugin, recovered)
		}
	}()
	return w.handler(ctx, input), nil
}

// sourcePolicyReceipt encodes the receipt that echoes the request's identity
// and target, with the failure when err is set. An error too long for a
// receipt is returned as the answer error instead.
func sourcePolicyReceipt(input rpc.SourcePolicyInput, err error) (json.RawMessage, error) {
	out := rpc.SourcePolicyOutput{
		TransactionID: input.TransactionID, Operation: input.Operation,
		Source: input.Source, NextPolicyEpoch: input.NextPolicyEpoch, Status: rpc.StatusOK,
	}
	if err != nil {
		out.Status, out.Error = rpc.StatusError, err.Error()
		if len(out.Error) > sourcePolicyErrorBytesMax {
			return nil, errors.New("source-policy handler error exceeds 4096 bytes")
		}
	}
	return json.Marshal(out)
}

// startSourcePolicyWorker starts the connection's source-policy worker when
// the plugin registered OnSourcePolicy, and returns it with the stop function
// the loop MUST defer. abort ends the connection after a handler panic. With
// no handler it returns a nil worker and a no-op stop, and a source-policy
// request is then answered as an unknown method.
func (p *Plugin) startSourcePolicyWorker(ctx context.Context, abort func(error)) (*sourcePolicyWorker, func()) {
	p.mu.Lock()
	handler := p.onSourcePolicy
	p.mu.Unlock()
	if handler == nil {
		return nil, func() {}
	}

	worker := newSourcePolicyWorker(p.name, handler, abort)
	workerCtx, cancel := context.WithCancel(ctx)
	go worker.run(workerCtx)

	// After done, nothing else reads the queue: a request the loop admitted
	// after run stopped on a panic is refused here.
	return worker, func() {
		cancel()
		<-worker.done
		worker.refuseQueued(errSourcePolicyStopped)
	}
}

// answerSourcePolicy sends the worker's answer for one wire request. The send
// outlives the loop's context, because a receipt at stop is written after the
// loop has ended, and is bounded by the request's own deadline instead
// (workerAnswerContext). A send
// that fails means the connection is closing or the engine stopped waiting, so
// the failure is logged and not returned.
func (p *Plugin) answerSourcePolicy(ctx context.Context, id uint64, deadline time.Time, receipt json.RawMessage, answerErr error) {
	ctx, cancel := workerAnswerContext(ctx, deadline)
	defer cancel()
	var err error
	if answerErr != nil {
		err = p.sendCallbackError(ctx, id, answerErr.Error())
	} else {
		err = p.sendCallbackResult(ctx, id, receipt)
	}
	if err != nil {
		slog.Debug("source-policy answer not sent", "plugin", p.name, "error", err)
	}
}
