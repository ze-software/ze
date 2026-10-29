// Design: docs/architecture/api/process-protocol.md — event loop and callback dispatch
// Overview: sdk.go — plugin SDK core

package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Engine->plugin callback method names.
const (
	callbackBye              = "ze-plugin-callback:bye"
	callbackConfigure        = "ze-plugin-callback:configure"
	callbackDeliverEvent     = "ze-plugin-callback:deliver-event"
	callbackDeliverBatch     = "ze-plugin-callback:deliver-batch"
	callbackExecuteCommand   = "ze-plugin-callback:execute-command"
	callbackEncodeNLRI       = "ze-plugin-callback:encode-nlri"
	callbackDecodeNLRI       = "ze-plugin-callback:decode-nlri"
	callbackDecodeCapability = "ze-plugin-callback:decode-capability"
	callbackConfigVerify     = "ze-plugin-callback:config-verify"
	callbackConfigApply      = "ze-plugin-callback:config-apply"
	callbackConfigRollback   = "ze-plugin-callback:config-rollback"
	callbackConfigCommitted  = "ze-plugin-callback:config-committed"
	callbackConfigComplete   = "ze-plugin-callback:config-complete"
	callbackSourcePolicy     = "ze-plugin-callback:source-policy"
	callbackOpDecompose      = "ze-plugin-callback:config-operation-decompose"
	callbackOpVerify         = "ze-plugin-callback:config-operation-verify"
	callbackOpApply          = "ze-plugin-callback:config-operation-apply"
	callbackOpRollback       = "ze-plugin-callback:config-operation-rollback"
	callbackOpCommit         = "ze-plugin-callback:config-operation-commit"
	callbackValidateOpen     = "ze-plugin-callback:validate-open"
	callbackFilterUpdate     = "ze-plugin-callback:filter-update"
	callbackDoctorCheck      = "ze-plugin-callback:doctor-check"
	callbackEnrichShow       = "ze-plugin-callback:enrich-show"
	// callbackPostStartup is sent by the engine after all startup phases
	// complete and both the plugin registry and dispatcher command registry
	// have been frozen. It is the only guaranteed-safe point for a plugin to
	// issue DispatchCommand calls targeting another plugin across the
	// multi-phase load order (config-path auto-load, explicit, family,
	// event-type, send-type). Handled via OnAllPluginsReady.
	callbackPostStartup = "ze-plugin-callback:post-startup"
)

// serveOne reads one request from the MuxConn, dispatches it, and sends the response.
func (p *Plugin) serveOne(ctx context.Context, expectedMethod string, handler func(json.RawMessage) error) error {
	req, err := p.readCallback(ctx)
	if err != nil {
		return fmt.Errorf("read request: %w", err)
	}

	if req.Method != expectedMethod {
		return fmt.Errorf("expected method %q, got %q", expectedMethod, req.Method)
	}

	if err := handler(req.Params); err != nil {
		return p.sendCallbackError(ctx, req.ID, err.Error())
	}

	return p.sendCallbackOK(ctx, req.ID)
}

// readCallback reads the next inbound request from the engine via MuxConn.
func (p *Plugin) readCallback(ctx context.Context) (*rpc.Request, error) {
	select {
	case req, ok := <-p.engineMux.Requests():
		if !ok {
			return nil, rpc.ErrMuxConnClosed
		}
		return req, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.engineMux.Done():
		return nil, rpc.ErrMuxConnClosed
	}
}

// sendCallbackOK sends a successful response to an engine-initiated request.
func (p *Plugin) sendCallbackOK(ctx context.Context, id uint64) error {
	return p.engineMux.SendOK(ctx, id)
}

// sendCallbackError sends an error response to an engine-initiated request.
func (p *Plugin) sendCallbackError(ctx context.Context, id uint64, message string) error {
	return p.engineMux.SendError(ctx, id, message)
}

// sendCallbackResult sends a result response to an engine-initiated request.
func (p *Plugin) sendCallbackResult(ctx context.Context, id uint64, data any) error {
	return p.engineMux.SendResult(ctx, id, data)
}

// isConnectionClosed reports whether err indicates a closed connection.
// During shutdown the engine closes the socket, producing EOF or
// "use of closed network connection" -- both are clean exit signals.
func isConnectionClosed(err error) bool {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) || errors.Is(err, net.ErrClosed) {
		return true
	}
	// Fallback: net.Pipe and Unix sockets may surface these as opaque strings
	// when error chains don't wrap the sentinel values.
	msg := err.Error()
	return strings.Contains(msg, "use of closed network connection") ||
		strings.Contains(msg, "read/write on closed pipe")
}

// getCallback returns the registered handler for a callback method name.
func (p *Plugin) getCallback(method string) callbackHandler {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.callbacks[method]
}

func bridgeCallbackPanicError(any) error {
	return fmt.Errorf("%w: bridge callback panic", rpc.ErrBridgeFailed)
}

func (p *Plugin) failBridgeCallbacks(err error) {
	if p.bridge == nil {
		return
	}
	p.bridge.FailCallbacks(err)
	callbackCh := p.bridge.CallbackCh()
	executeCommandCh := p.bridge.ExecuteCommandRequests()
	for callbackCh != nil || executeCommandCh != nil {
		select {
		case cb, ok := <-callbackCh:
			if !ok {
				callbackCh = nil
				continue
			}
			cb.Result <- rpc.BridgeCallbackResult{Err: err}
		case req, ok := <-executeCommandCh:
			if !ok {
				executeCommandCh = nil
				continue
			}
			req.Result <- rpc.ExecuteCommandResult{Err: err}
		default:
			return
		}
	}
}

func (p *Plugin) handleBridgeCallback(cb rpc.BridgeCallback) (stop bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = bridgeCallbackPanicError(r)
			cb.Result <- rpc.BridgeCallbackResult{Err: err}
			p.failBridgeCallbacks(err)
			stop = true
		}
	}()

	handler := p.getCallback(cb.Method)
	if handler == nil {
		cb.Result <- rpc.BridgeCallbackResult{Err: fmt.Errorf("unknown method: %s", cb.Method)}
		return false, nil
	}
	result, callErr := handler(cb.Params)
	cb.Result <- rpc.BridgeCallbackResult{Data: result, Err: callErr}
	return cb.Method == callbackBye && callErr == nil, nil
}

func (p *Plugin) handleBridgeExecuteCommand(req rpc.ExecuteCommandRequest) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = bridgeCallbackPanicError(r)
			req.Result <- rpc.ExecuteCommandResult{Err: err}
			p.failBridgeCallbacks(err)
		}
	}()
	output, callErr := p.bridge.RunExecuteCommand(req)
	req.Result <- rpc.ExecuteCommandResult{Output: output, Err: callErr}
	return nil
}

// bridgeEventLoop handles runtime callbacks for internal plugins after pipe shutdown.
// Reads from bridge callback channels instead of engineMux.Requests().
// Callbacks are processed serially (same guarantee as the pipe event loop).
func (p *Plugin) bridgeEventLoop(ctx context.Context) error {
	drain, stopDrain := p.startDrainWorker(ctx)
	defer stopDrain()
	// A source-policy handler panic fails every bridge callback, as a panic
	// on this loop does (handleBridgeCallback).
	sourcePolicy, stopSourcePolicy := p.startSourcePolicyWorker(ctx, func(err error) {
		p.failBridgeCallbacks(fmt.Errorf("%w: %w", rpc.ErrBridgeFailed, err))
	})
	defer stopSourcePolicy()

	for {
		select {
		case cb, ok := <-p.bridge.CallbackCh():
			if !ok {
				// Bridge closed: shutdown, or a source-policy handler panic.
				return sourcePolicy.failure()
			}
			// The source-policy handler runs on its worker; the result
			// channel has capacity 1, so neither a refusal here nor the
			// worker's answer blocks.
			if cb.Method == callbackSourcePolicy && sourcePolicy != nil {
				result := cb.Result
				if err := sourcePolicy.admit(cb.Params, func(_ time.Time, receipt json.RawMessage, err error) {
					result <- rpc.BridgeCallbackResult{Data: receipt, Err: err}
				}); err != nil {
					result <- rpc.BridgeCallbackResult{Err: err}
				}
				continue
			}
			// The drain runs on its worker; the result channel has
			// capacity 1, so the worker's answer never blocks.
			if cb.Method == rpc.MethodDrain && drain != nil {
				result := cb.Result
				if err := drain.admit(cb.Params, func(err error) {
					result <- rpc.BridgeCallbackResult{Err: err}
				}); err != nil {
					result <- rpc.BridgeCallbackResult{Err: err}
				}
				continue
			}
			stop, err := p.handleBridgeCallback(cb)
			if err != nil {
				return err
			}
			if stop {
				return nil
			}

		case req, ok := <-p.bridge.ExecuteCommandRequests():
			if !ok {
				// Bridge closed: shutdown, or a source-policy handler panic.
				return sourcePolicy.failure()
			}
			if err := p.handleBridgeExecuteCommand(req); err != nil {
				return err
			}

		case <-ctx.Done():
			return nil
		}
	}
}

// eventLoop handles runtime RPCs from the engine (external/pipe plugins).
// Reads from engineMux.Requests() for inbound requests.
func (p *Plugin) eventLoop(ctx context.Context) error {
	drain, stopDrain := p.startDrainWorker(ctx)
	defer stopDrain()
	// A source-policy handler panic closes the connection; the read below
	// then fails and the loop returns the panic rather than a clean stop.
	sourcePolicy, stopSourcePolicy := p.startSourcePolicyWorker(ctx, func(error) {
		p.engineMux.Close() //nolint:errcheck // the failure is returned by the loop, not by Close
	})
	defer stopSourcePolicy()

	for {
		req, err := p.readCallback(ctx)
		if err != nil {
			if failure := sourcePolicy.failure(); failure != nil {
				return failure
			}
			// Context canceled or connection closed = clean shutdown.
			// The engine closes the socket to signal internal plugins to exit;
			// this races with context cancellation, so check both.
			if ctx.Err() == nil && !isConnectionClosed(err) && !errors.Is(err, rpc.ErrMuxConnClosed) {
				return fmt.Errorf("event loop read: %w", err)
			}
			return nil //nolint:nilerr // EOF/context-cancel during shutdown is not an error
		}

		// The drain is answered by its worker, for this request's own ID,
		// after later requests may already have been answered (the mux
		// matches answers by ID). A refusal is answered here at once.
		if req.Method == rpc.MethodDrain && drain != nil {
			id := req.ID
			if err := drain.admit(req.Params, func(err error) { p.answerDrain(ctx, id, req.Params, err) }); err != nil {
				if sendErr := p.sendCallbackError(ctx, id, err.Error()); sendErr != nil {
					return sendErr
				}
			}
			continue
		}

		// The source-policy handler runs on its worker, which answers for
		// this request's own ID once the handler returns. A malformed request
		// is refused here at once, before any owner state changes.
		if req.Method == callbackSourcePolicy && sourcePolicy != nil {
			id := req.ID
			if err := sourcePolicy.admit(req.Params, func(deadline time.Time, receipt json.RawMessage, err error) {
				p.answerSourcePolicy(ctx, id, deadline, receipt, err)
			}); err != nil {
				if sendErr := p.sendCallbackError(ctx, id, err.Error()); sendErr != nil {
					return sendErr
				}
			}
			continue
		}

		handler := p.getCallback(req.Method)
		if handler == nil {
			if sendErr := p.sendCallbackError(ctx, req.ID, "unknown method: "+req.Method); sendErr != nil {
				return sendErr
			}
			continue
		}

		// Execute-command: the one callback whose answer can be a walk. A walk
		// is written as the answer sequence and nothing may follow it on the
		// wire, so this callback is answered where the connection is rather
		// than through the single result line below. The map entry serves the
		// transport that carries no line for a record (OnExecuteCommand,
		// sdk_callbacks.go), and both reach one implementation.
		if req.Method == callbackExecuteCommand {
			if sendErr := p.answerExecuteCommand(ctx, req); sendErr != nil {
				return sendErr
			}
			continue
		}

		result, err := handler(req.Params)
		if err != nil {
			if sendErr := p.sendCallbackError(ctx, req.ID, err.Error()); sendErr != nil {
				return sendErr
			}
			continue
		}
		if result != nil {
			if sendErr := p.sendCallbackResult(ctx, req.ID, result); sendErr != nil {
				return sendErr
			}
		} else {
			if sendErr := p.sendCallbackOK(ctx, req.ID); sendErr != nil {
				return sendErr
			}
		}
		// Removal is acknowledged only after cleanup succeeds. A failed callback
		// leaves this loop live for configuration compensation or another Bye.
		if req.Method == callbackBye {
			return nil
		}
	}
}

// workerAnswerWriteMax is the time a worker's answer may take to write once
// its request's deadline has passed. A failed answer for an expired request is
// written after that deadline by definition, so the bound is the later of the
// two.
const workerAnswerWriteMax = time.Second

// workerAnswerContext returns the context a drain or source-policy worker
// writes its answer with. It does not end with the loop's ctx, because an
// answer at stop is written after the loop has ended, and it ends at the later
// of deadline and workerAnswerWriteMax from now, so a send is never unbounded.
func workerAnswerContext(ctx context.Context, deadline time.Time) (context.Context, context.CancelFunc) {
	bound := time.Now().Add(workerAnswerWriteMax)
	if deadline.After(bound) {
		bound = deadline
	}
	return context.WithDeadline(context.WithoutCancel(ctx), bound)
}

// answerDrain sends the drain worker's answer for one drain request. The send
// outlives the loop's context, because a drain cut short by the loop's stop is
// answered after the loop has ended, and is bounded by the request's own
// deadline instead (workerAnswerContext), read from params, which admit already
// decoded. A send that
// fails means the connection is closing or the engine stopped waiting, so the
// failure is logged and not returned.
func (p *Plugin) answerDrain(ctx context.Context, id uint64, params json.RawMessage, handlerErr error) {
	var input rpc.DrainInput
	if err := json.Unmarshal(params, &input); err != nil {
		slog.Debug("drain answer not sent", "plugin", p.name, "error", err)
		return
	}
	ctx, cancel := workerAnswerContext(ctx, input.Deadline)
	defer cancel()
	var err error
	if handlerErr != nil {
		err = p.sendCallbackError(ctx, id, handlerErr.Error())
	} else {
		err = p.sendCallbackResult(ctx, id, &rpc.DrainOutput{})
	}
	if err != nil {
		slog.Debug("drain answer not sent", "plugin", p.name, "error", err)
	}
}

// answerExecuteCommand answers one execute-command request from the socket
// event loop: the answer sequence, or one error line when the handler failed.
//
// The answer writer is offered for every request. One answer has one encoding,
// so the engine reads a head, its records and a terminator, and a plugin that
// wrote a single-line result here would hand it a head line's tail for the
// whole result (R-1 of spec-record-answers-1-sdk-path,
// PluginConn.SendExecuteCommandAnswer, internal/component/plugin/ipc/rpc.go).
//
// An error AFTER the answer opened still travels as an error line: a mid-walk
// refusal, a row that disagrees with the declared columns among them, ends the
// answer at the consumer with the producer named, where a silent stop would
// read as a lost connection.
func (p *Plugin) answerExecuteCommand(ctx context.Context, req *rpc.Request) error {
	p.mu.Lock()
	fn := p.onExecuteCommand
	p.mu.Unlock()

	answer := p.engineMux.AnswerWriter(ctx)

	result, err := executeCommandAnswer(fn, req.Params, answer, req.ID)
	if err != nil {
		return p.sendCallbackError(ctx, req.ID, err.Error())
	}
	if result == nil {
		// The answer sequence is on the wire and it ended at its terminator.
		return nil
	}
	return p.sendCallbackResult(ctx, req.ID, result)
}

// Startup-stage handlers (not in the callback map -- these run during
// the sequential startup protocol, before the event loop starts).

func (p *Plugin) handleConfigure(params json.RawMessage) error {
	var input struct {
		Sections []ConfigSection `json:"sections"`
		Claims   []string        `json:"claims,omitempty"`
	}
	if params != nil {
		if err := json.Unmarshal(params, &input); err != nil {
			return fmt.Errorf("unmarshal configure: %w", err)
		}
	}

	// Record the claim set BEFORE running the plugin's configure handler, so a
	// handler that stands its own default behavior down for a claiming plugin
	// can read it here -- at Stage 2, which completes before Stage 5 ready and
	// therefore before this plugin can receive any runtime event.
	p.mu.Lock()
	if len(input.Claims) > 0 {
		p.claims = make(map[string]bool, len(input.Claims))
		for _, token := range input.Claims {
			p.claims[token] = true
		}
	} else {
		p.claims = nil
	}
	fn := p.onConfigure
	p.mu.Unlock()

	// An error out of here is this plugin REFUSING the configuration it was
	// sent, and serveOne turns it into the Stage 2 error RESPONSE the engine
	// recognizes as a refusal. A plugin whose registration asked for it stops
	// ze on one; a failure to DELIVER the configuration never does, because it
	// produces no response at all (isConfigRefusal,
	// internal/component/plugin/server/startup.go).
	if fn != nil {
		return fn(input.Sections)
	}

	return nil
}

func (p *Plugin) handleShareRegistry(params json.RawMessage) error {
	var input struct {
		Commands []RegistryCommand `json:"commands"`
	}
	if params != nil {
		if err := json.Unmarshal(params, &input); err != nil {
			return fmt.Errorf("unmarshal share-registry: %w", err)
		}
	}

	p.mu.Lock()
	fn := p.onShareRegistry
	p.mu.Unlock()

	if fn != nil {
		fn(input.Commands)
	}

	return nil
}
