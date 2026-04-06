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

// Engine->plugin callback method names.
const (
	callbackBye              = "ze-plugin-callback:bye"
	callbackDeliverEvent     = "ze-plugin-callback:deliver-event"
	callbackDeliverBatch     = "ze-plugin-callback:deliver-batch"
	callbackExecuteCommand   = "ze-plugin-callback:execute-command"
	callbackEncodeNLRI       = "ze-plugin-callback:encode-nlri"
	callbackDecodeNLRI       = "ze-plugin-callback:decode-nlri"
	callbackDecodeCapability = "ze-plugin-callback:decode-capability"
	callbackConfigVerify     = "ze-plugin-callback:config-verify"
	callbackConfigApply      = "ze-plugin-callback:config-apply"
	callbackConfigRollback   = "ze-plugin-callback:config-rollback"
	callbackValidateOpen     = "ze-plugin-callback:validate-open"
	callbackFilterUpdate     = "ze-plugin-callback:filter-update"
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

// bridgeEventLoop handles runtime callbacks for internal plugins after pipe shutdown.
// Reads from bridge.CallbackCh() instead of engineMux.Requests().
// Callbacks are processed serially (same guarantee as the pipe event loop).
func (p *Plugin) bridgeEventLoop(ctx context.Context) error {
	for {
		select {
		case cb, ok := <-p.bridge.CallbackCh():
			if !ok {
				return nil // Bridge closed (shutdown).
			}
			handler := p.getCallback(cb.Method)
			if handler == nil {
				cb.Result <- rpc.BridgeCallbackResult{Err: fmt.Errorf("unknown method: %s", cb.Method)}
				continue
			}
			result, err := handler(cb.Params)
			cb.Result <- rpc.BridgeCallbackResult{Data: result, Err: err}
			if cb.Method == callbackBye {
				return nil
			}

		case <-ctx.Done():
			return nil
		}
	}
}

// eventLoop handles runtime RPCs from the engine (external/pipe plugins).
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

		handler := p.getCallback(req.Method)
		if handler == nil {
			if sendErr := p.sendCallbackError(ctx, req.ID, "unknown method: "+req.Method); sendErr != nil {
				return sendErr
			}
			continue
		}

		// Bye: respond first (pipe protocol requires it), then invoke handler.
		if req.Method == callbackBye {
			if sendErr := p.sendCallbackOK(ctx, req.ID); sendErr != nil {
				return sendErr
			}
			handler(req.Params) //nolint:errcheck // bye handler is best-effort
			return nil
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
	}
}

// Startup-stage handlers (not in the callback map -- these run during
// the sequential startup protocol, before the event loop starts).

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
