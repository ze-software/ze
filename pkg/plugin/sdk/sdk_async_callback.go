// Design: docs/architecture/api/process-protocol.md — independent callback workers.
// Related: sdk_dispatch.go — both transports consume this registration.

package sdk

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"time"
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
// consume that receipt; its send error is recorded and never treated as success.
func (p *Plugin) answerAsyncCallback(ctx context.Context, id uint64, deadline time.Time, receipt json.RawMessage, answerErr error) {
	ctx, cancel := workerAnswerContext(ctx, deadline)
	defer cancel()
	var err error
	if answerErr != nil {
		err = p.sendCallbackError(ctx, id, answerErr.Error())
	} else {
		err = p.sendCallbackResult(ctx, id, receipt)
	}
	if err != nil {
		// A completed callback can outlive its caller at cancellation or stop.
		slog.Debug("worker callback answer not sent", "plugin", p.name, "error", err)
	}
}
