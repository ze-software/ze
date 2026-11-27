// Design: docs/architecture/api/process-protocol.md -- independent runtime inspection callback.
// Related: rpc.go -- PluginConn owns the exact authenticated plugin connection.
// Related: read_snapshot.go -- shared framed snapshot fallback for raw rpc.Conn.
package ipc

import (
	"context"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// SendRuntimeInspect queries Go runtime diagnostics on this exact plugin
// connection through its independent callback lane, bypassing the plugin's
// ordinary event loop. A published bridge commits cutover: a stopped or unready
// bridge fails without socket fallback.
func (pc *PluginConn) SendRuntimeInspect(ctx context.Context, input rpc.RuntimeInspectInput) (*rpc.RuntimeInspectOutput, error) {
	if bridge := pc.bridge.Load(); bridge != nil {
		return bridge.RuntimeInspect(ctx, input)
	}
	if pc.mux != nil {
		return pc.mux.RuntimeInspect(ctx, input)
	}
	return callReadSnapshot(ctx, pc.Conn, input, rpc.RuntimeInspectCallContext,
		rpc.MethodRuntimeInspect, (*rpc.RuntimeInspectOutput).Validate)
}
