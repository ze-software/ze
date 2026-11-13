// Design: docs/architecture/api/process-protocol.md -- native owner engine operations.
// Related: dispatch_registry.go -- one entry for public and native adapters.

package server

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/taskguard"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// dispatchPluginRPCNative resolves the public registry's optional native adapter.
// wireBridgeDispatch binds proc to the authenticated process generation. A miss
// leaves public RPC selection to the caller; an admitted failure never does.
func (s *Server) dispatchPluginRPCNative(ctx context.Context, proc *process.Process, method string, input any) (result any, handled bool, err error) {
	op := lookupEngineOp(method)
	if op == nil {
		return nil, false, nil
	}
	if op.native == nil {
		return nil, false, nil
	}
	defer engineOpPanics().request.Contain(
		func() taskguard.Context {
			return taskguard.Context{Unit: "request from plugin " + proc.Name(), MessageType: method}
		},
		func(p *taskguard.Panic) {
			// A recovered handler panic was still handled here: its refusal
			// must not send the caller on to public RPC selection.
			result = nil
			handled = true
			err = &rpc.RPCCallError{Message: engineOpPanicMessage(method, p)}
		},
	)
	result, err = op.native(s, ctx, proc, input)
	if err != nil {
		return nil, true, s.directCallError(proc, err)
	}
	return result, true, nil
}

// directCallError is the in-process answer to a failed runtime request, for
// the native and the JSON-shaped Direct paths alike. A failure that is the end
// of the plugin's engine session (engineSessionEnded) answers
// rpc.ErrBridgeClosed, the bridge's own answer once Process.Stop fences its
// dispatch, as the connection close answers it on the socket path. An admitted
// call can meet the stop's cancellation before that fence: a daemon stop
// cancels the server context before ProcessManager.Stop reaches Process.Stop.
// Every other failure is engineOpCallError's answer.
func (s *Server) directCallError(proc *process.Process, err error) error {
	if s.engineSessionEnded(proc, err) {
		return rpc.ErrBridgeClosed
	}
	return engineOpCallError(err)
}

// engineOpCallError gives native and JSON-shaped Direct calls the same refusal
// detail, while preserving cancellation and an already normalized RPC error.
func engineOpCallError(err error) error {
	if errors.Is(err, context.Canceled) {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if callErr, ok := errors.AsType[*rpc.RPCCallError](err); ok {
		return callErr
	}
	return &rpc.RPCCallError{Message: err.Error()}
}
