// Design: docs/architecture/api/process-protocol.md -- independent runtime inspection callback.
// Related: bridge.go -- delivery lifetime fencing for typed callbacks.
// Related: runtime_inspect.go -- wire types and MuxConn transport for runtime-inspect.
package rpc

import "context"

// RuntimeInspectHandler collects one bounded read-only runtime diagnostic
// snapshot from the plugin process. It MUST honor ctx and MUST NOT close the
// production bridge or mux on diagnostic failure. Input is borrowed until
// return; output ownership passes to the caller.
type RuntimeInspectHandler func(context.Context, RuntimeInspectInput) (*RuntimeInspectOutput, error)

// SetRuntimeInspect installs the independent typed callback before SetReady.
func (b *DirectBridge) SetRuntimeInspect(handler RuntimeInspectHandler) {
	b.runtimeInspect = handler
	b.hasRuntimeInspect.Store(handler != nil)
}

// RuntimeInspect bypasses the ordinary callback queue. The SDK slot enqueues on
// its independent worker so a blocked execute-command cannot prevent inspection.
func (b *DirectBridge) RuntimeInspect(ctx context.Context, input RuntimeInspectInput) (*RuntimeInspectOutput, error) {
	return bridgeReadSnapshot(b, ctx, input, RuntimeInspectCallContext,
		&b.hasRuntimeInspect, &b.runtimeInspect,
		"runtime-inspect handler is not installed; start the SDK runtime inspection worker before Run",
		(*RuntimeInspectOutput).Validate)
}
