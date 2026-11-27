// Design: docs/architecture/api/process-protocol.md — independent callback workers.
// Related: sdk_dispatch.go — both transports consume this registration.

package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"maps"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// asyncCallbackAnswer answers one admitted request. The callback MUST NOT wait
// on the worker: an overload refusal can answer from the ordinary event loop.
type asyncCallbackAnswer func(time.Time, json.RawMessage, error)

// asyncCallbackWorker is one connection-owned worker and its adapters. The
// event loop MUST call stop before returning; stop MUST cancel and join it.
type asyncCallbackWorker struct {
	admit   func(json.RawMessage, asyncCallbackAnswer) error
	failure func() error
	stop    func()
}

type asyncCallbackFactory func(context.Context, func(error)) asyncCallbackWorker

// setAsyncCallback changes registration before Run. Caller MUST hold p.mu.
// A nil factory removes support instead of leaving a no-op declaration.
func (p *Plugin) setAsyncCallback(method string, factory asyncCallbackFactory) {
	if factory == nil {
		delete(p.asyncCallbacks, method)
		return
	}
	if p.asyncCallbacks == nil {
		p.asyncCallbacks = make(map[string]asyncCallbackFactory)
	}
	p.asyncCallbacks[method] = factory
}

type asyncCallbackWorkers map[string]asyncCallbackWorker

// startAsyncCallbacks starts exactly the registered workers for this connection.
// Run's owner MUST defer stop. Registration MUST finish before Run starts.
func (p *Plugin) startAsyncCallbacks(ctx context.Context, abort func(error)) asyncCallbackWorkers {
	p.mu.Lock()
	factories := maps.Clone(p.asyncCallbacks)
	p.mu.Unlock()
	workers := make(asyncCallbackWorkers, len(factories))
	for method, factory := range factories {
		workers[method] = factory(ctx, abort)
	}
	return workers
}

// stop cancels and joins every worker started by startAsyncCallbacks. The set
// is bounded by the plugin's registrations and owns no per-request goroutines.
func (workers asyncCallbackWorkers) stop() {
	for _, worker := range workers {
		worker.stop()
	}
}

func (workers asyncCallbackWorkers) failure() error {
	for _, worker := range workers {
		if err := worker.failure(); err != nil {
			return err
		}
	}
	return nil
}

// answerAsyncCallback preserves a worker's own request deadline while permitting
// a bounded failure receipt after loop cancellation. A vanished reader cannot
// consume that receipt; a live-connection send failure is logged at WARN so the
// engine log records why the caller did not receive its callback receipt.
func (p *Plugin) answerAsyncCallback(loopCtx context.Context, id uint64, deadline time.Time, receipt json.RawMessage, answerErr error) {
	answerCtx, cancel := workerAnswerContext(loopCtx, deadline)
	defer cancel()
	var err error
	if answerErr != nil {
		err = p.sendCallbackError(answerCtx, id, answerErr.Error())
	} else {
		err = p.sendCallbackResult(answerCtx, id, receipt)
	}
	if err == nil {
		return
	}
	if loopCtx.Err() != nil {
		// refuse-by-default: guards Plugin.eventLoop -- worker callback completed after event loop cancellation; eventLoop stops workers and exits
		return
	}
	if isConnectionClosed(err) {
		// refuse-by-default: guards Plugin.eventLoop -- engine connection already closed; eventLoop observes connection end and exits
		return
	}
	if errors.Is(err, rpc.ErrMuxConnClosed) {
		// refuse-by-default: guards Plugin.eventLoop -- mux connection already closed; eventLoop observes connection end and exits
		return
	}
	if p.engineMux != nil {
		select {
		case <-p.engineMux.Done():
			// refuse-by-default: guards Plugin.eventLoop -- mux reader already exited; eventLoop observes connection end and exits
			return
		default:
		}
	}
	slog.Warn("could not send async callback answer to engine; the engine is left waiting for the callback receipt",
		"plugin", p.name, "request-id", id, "error", err,
		"fix", "ze show log recent component plugin.relay")
}
