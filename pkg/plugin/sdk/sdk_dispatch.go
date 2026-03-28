// Design: docs/architecture/api/process-protocol.md — event loop and callback dispatch
// Overview: sdk.go — plugin SDK core

package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
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
// "use of closed network connection" — both are clean exit signals.
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

// eventLoop handles runtime RPCs from the engine.
// Reads from engineMux.Requests() for inbound requests.
func (p *Plugin) eventLoop(ctx context.Context) error {
	for {
		req, err := p.readCallback(ctx)
		if err != nil {
			// Context canceled or connection closed = clean shutdown.
			// The engine closes the socket to signal internal plugins to exit;
			// this races with context cancellation, so check both.
			if ctx.Err() == nil && !isConnectionClosed(err) && !errors.Is(err, rpc.ErrMuxConnClosed) {
				return fmt.Errorf("event loop read: %w", err)
			}
			return nil //nolint:nilerr // EOF/context-cancel during shutdown is not an error
		}

		if err := p.dispatchCallback(ctx, req); err != nil {
			return err
		}

		// bye signals clean shutdown
		if req.Method == "ze-plugin-callback:bye" {
			return nil
		}
	}
}

// dispatchCallback routes a callback request to the appropriate handler and sends the response.
func (p *Plugin) dispatchCallback(ctx context.Context, req *rpc.Request) error {
	switch req.Method {
	case "ze-plugin-callback:deliver-event":
		if handleErr := p.handleDeliverEvent(req.Params); handleErr != nil {
			return p.sendCallbackError(ctx, req.ID, handleErr.Error())
		}
		return p.sendCallbackOK(ctx, req.ID)

	case "ze-plugin-callback:deliver-batch":
		if handleErr := p.handleDeliverBatch(req.Params); handleErr != nil {
			return p.sendCallbackError(ctx, req.ID, handleErr.Error())
		}
		return p.sendCallbackOK(ctx, req.ID)

	case "ze-plugin-callback:encode-nlri":
		return p.handleNLRICallback(ctx, req, p.encodeNLRIHandler())

	case "ze-plugin-callback:decode-nlri":
		return p.handleNLRICallback(ctx, req, p.decodeNLRIHandler())

	case "ze-plugin-callback:decode-capability":
		return p.handleNLRICallback(ctx, req, p.decodeCapabilityHandler())

	case "ze-plugin-callback:execute-command":
		return p.handleExecuteCommand(ctx, req)

	case "ze-plugin-callback:config-verify":
		return p.handleConfigVerify(ctx, req)

	case "ze-plugin-callback:config-apply":
		return p.handleConfigApply(ctx, req)

	case "ze-plugin-callback:validate-open":
		return p.handleValidateOpen(ctx, req)

	case "ze-plugin-callback:filter-update":
		return p.handleFilterUpdate(ctx, req)

	case "ze-plugin-callback:bye":
		return p.handleByeAndRespond(ctx, req)
	}

	// Ze's fail-on-unknown rule: reject unknown methods with an error response.
	errMsg := fmt.Sprintf("unknown method: %s", req.Method)
	return p.sendCallbackError(ctx, req.ID, errMsg)
}

// handleByeAndRespond handles bye by responding first, then invoking callback.
func (p *Plugin) handleByeAndRespond(ctx context.Context, req *rpc.Request) error {
	// Send response before invoking callback
	if err := p.sendCallbackOK(ctx, req.ID); err != nil {
		return err
	}

	var input struct {
		Reason string `json:"reason,omitempty"`
	}
	if req.Params != nil {
		// Non-fatal: bye still processed even if params fail to parse
		_ = json.Unmarshal(req.Params, &input) //nolint:errcheck // best-effort
	}

	p.mu.Lock()
	fn := p.onBye
	p.mu.Unlock()

	if fn != nil {
		fn(input.Reason)
	}

	return nil
}
func (p *Plugin) handleConfigure(params json.RawMessage) error {
	var input struct {
		Sections []ConfigSection `json:"sections"`
	}
	if params != nil {
		if err := json.Unmarshal(params, &input); err != nil {
			return fmt.Errorf("unmarshal configure: %w", err)
		}
	}

	p.mu.Lock()
	fn := p.onConfigure
	p.mu.Unlock()

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

func (p *Plugin) handleDeliverEvent(params json.RawMessage) error {
	var input struct {
		Event string `json:"event"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return fmt.Errorf("unmarshal deliver-event: %w", err)
	}

	p.mu.Lock()
	fn := p.onEvent
	p.mu.Unlock()

	if fn != nil {
		return fn(input.Event)
	}

	return nil
}

// handleDeliverBatch processes a batched event delivery by dispatching each
// event to the onEvent handler. Short-circuits on the first handler error.
func (p *Plugin) handleDeliverBatch(params json.RawMessage) error {
	events, err := rpc.ParseBatchEvents(params)
	if err != nil {
		return err
	}

	p.mu.Lock()
	fn := p.onEvent
	p.mu.Unlock()

	if fn == nil {
		return nil
	}

	for _, raw := range events {
		var eventStr string
		if err := json.Unmarshal(raw, &eventStr); err != nil {
			return fmt.Errorf("unmarshal batch event: %w", err)
		}
		if err := fn(eventStr); err != nil {
			return err
		}
	}

	return nil
}

// handleNLRICallback handles an NLRI-related RPC by unmarshalling params, invoking
// a handler function, and sending either the result or an error response.
func (p *Plugin) handleNLRICallback(ctx context.Context, req *rpc.Request, handler func(json.RawMessage) (any, error)) error {
	if handler == nil {
		return p.sendCallbackError(ctx, req.ID, req.Method+" not supported")
	}

	result, err := handler(req.Params)
	if err != nil {
		return p.sendCallbackError(ctx, req.ID, err.Error())
	}

	return p.sendCallbackResult(ctx, req.ID, result)
}

func (p *Plugin) encodeNLRIHandler() func(json.RawMessage) (any, error) {
	p.mu.Lock()
	fn := p.onEncodeNLRI
	p.mu.Unlock()

	if fn == nil {
		return nil
	}

	return func(params json.RawMessage) (any, error) {
		var input rpc.EncodeNLRIInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("unmarshal encode-nlri: %w", err)
		}
		hex, err := fn(input.Family, input.Args)
		if err != nil {
			return nil, err
		}
		return struct {
			Hex string `json:"hex"`
		}{Hex: hex}, nil
	}
}

func (p *Plugin) decodeNLRIHandler() func(json.RawMessage) (any, error) {
	p.mu.Lock()
	fn := p.onDecodeNLRI
	p.mu.Unlock()

	if fn == nil {
		return nil
	}

	return func(params json.RawMessage) (any, error) {
		var input rpc.DecodeNLRIInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("unmarshal decode-nlri: %w", err)
		}
		jsonResult, err := fn(input.Family, input.Hex)
		if err != nil {
			return nil, err
		}
		return struct {
			JSON string `json:"json"`
		}{JSON: jsonResult}, nil
	}
}

func (p *Plugin) decodeCapabilityHandler() func(json.RawMessage) (any, error) {
	p.mu.Lock()
	fn := p.onDecodeCapability
	p.mu.Unlock()

	if fn == nil {
		return nil
	}

	return func(params json.RawMessage) (any, error) {
		var input rpc.DecodeCapabilityInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("unmarshal decode-capability: %w", err)
		}
		jsonResult, err := fn(input.Code, input.Hex)
		if err != nil {
			return nil, err
		}
		return struct {
			JSON string `json:"json"`
		}{JSON: jsonResult}, nil
	}
}

func (p *Plugin) handleExecuteCommand(ctx context.Context, req *rpc.Request) error {
	p.mu.Lock()
	fn := p.onExecuteCommand
	p.mu.Unlock()

	if fn == nil {
		return p.sendCallbackError(ctx, req.ID, "execute-command not supported")
	}

	var input rpc.ExecuteCommandInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		return p.sendCallbackError(ctx, req.ID, fmt.Sprintf("unmarshal execute-command: %v", err))
	}

	status, data, err := fn(input.Serial, input.Command, input.Args, input.Peer)
	if err != nil {
		return p.sendCallbackError(ctx, req.ID, err.Error())
	}

	return p.sendCallbackResult(ctx, req.ID, &rpc.ExecuteCommandOutput{
		Status: status,
		Data:   data,
	})
}

func (p *Plugin) handleFilterUpdate(ctx context.Context, req *rpc.Request) error {
	p.mu.Lock()
	fn := p.onFilterUpdate
	p.mu.Unlock()

	if fn == nil {
		return p.sendCallbackError(ctx, req.ID, "filter-update not supported")
	}

	var input rpc.FilterUpdateInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		return p.sendCallbackError(ctx, req.ID, fmt.Sprintf("unmarshal filter-update: %v", err))
	}

	out, err := fn(&input)
	if err != nil {
		return p.sendCallbackError(ctx, req.ID, err.Error())
	}

	return p.sendCallbackResult(ctx, req.ID, out)
}

// handleConfigRPC is a shared handler for config-verify and config-apply RPCs.
// Both follow the same pattern: unmarshal params, call handler, return status/error result.
// The handler function receives raw params and returns an error (reject) or nil (accept).
func (p *Plugin) handleConfigRPC(ctx context.Context, req *rpc.Request, handler func(json.RawMessage) error) error {
	if handler == nil {
		// No handler = graceful no-op (not all plugins care about config).
		return p.sendCallbackResult(ctx, req.ID, &struct {
			Status string `json:"status"`
		}{Status: rpc.StatusOK})
	}

	if err := handler(req.Params); err != nil {
		return p.sendCallbackResult(ctx, req.ID, &struct {
			Status string `json:"status"`
			Error  string `json:"error,omitempty"`
		}{Status: rpc.StatusError, Error: err.Error()})
	}

	return p.sendCallbackResult(ctx, req.ID, &struct {
		Status string `json:"status"`
	}{Status: rpc.StatusOK})
}

func (p *Plugin) handleConfigVerify(ctx context.Context, req *rpc.Request) error {
	p.mu.Lock()
	fn := p.onConfigVerify
	p.mu.Unlock()

	var handler func(json.RawMessage) error
	if fn != nil {
		handler = func(params json.RawMessage) error {
			var input rpc.ConfigVerifyInput
			if err := json.Unmarshal(params, &input); err != nil {
				return fmt.Errorf("unmarshal config-verify: %w", err)
			}
			return fn(input.Sections)
		}
	}

	return p.handleConfigRPC(ctx, req, handler)
}

func (p *Plugin) handleConfigApply(ctx context.Context, req *rpc.Request) error {
	p.mu.Lock()
	fn := p.onConfigApply
	p.mu.Unlock()

	var handler func(json.RawMessage) error
	if fn != nil {
		handler = func(params json.RawMessage) error {
			var input rpc.ConfigApplyInput
			if err := json.Unmarshal(params, &input); err != nil {
				return fmt.Errorf("unmarshal config-apply: %w", err)
			}
			return fn(input.Sections)
		}
	}

	return p.handleConfigRPC(ctx, req, handler)
}

func (p *Plugin) handleValidateOpen(ctx context.Context, req *rpc.Request) error {
	p.mu.Lock()
	fn := p.onValidateOpen
	p.mu.Unlock()

	if fn == nil {
		// No handler = accept (no-op).
		return p.sendCallbackResult(ctx, req.ID, &rpc.ValidateOpenOutput{Accept: true})
	}

	var input rpc.ValidateOpenInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		return p.sendCallbackResult(ctx, req.ID, &rpc.ValidateOpenOutput{
			Accept: false, Reason: fmt.Sprintf("unmarshal validate-open: %v", err),
		})
	}

	output := fn(&input)
	return p.sendCallbackResult(ctx, req.ID, output)
}
