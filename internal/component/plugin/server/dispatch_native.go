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
	handled = true
	defer engineOpPanics().request.Contain(
		func() taskguard.Context {
			return taskguard.Context{Unit: "request from plugin " + proc.Name(), MessageType: method}
		},
		func(p *taskguard.Panic) {
			result = nil
			err = &rpc.RPCCallError{Message: engineOpPanicMessage(method, p)}
		},
	)
	result, err = op.native(s, ctx, proc, input)
	if err != nil {
		return nil, true, engineOpCallError(err)
	}
	return result, true, nil
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
