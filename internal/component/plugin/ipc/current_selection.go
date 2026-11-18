// Design: docs/architecture/api/process-protocol.md -- current selection validation.
// Related: rpc.go -- PluginConn owns the exact authenticated selecting connection.
package ipc

import (
	"context"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// SendCurrentSelection queries the current desired snapshot on this connection,
// never a daemon-global provider or ordinary execute-command route. A published
// bridge commits cutover: a stopped or unready slot fails without socket fallback.
// The caller MUST retain this exact selecting process lifetime until return and
// separately verify its role, grants, destination session and returned revisions.
func (pc *PluginConn) SendCurrentSelection(ctx context.Context, input rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
	if bridge := pc.bridge.Load(); bridge != nil {
		return bridge.CurrentSelection(ctx, input)
	}
	if pc.mux != nil {
		return pc.mux.CurrentSelection(ctx, input)
	}
	return callReadSnapshot(ctx, pc.Conn, input, rpc.CurrentSelectionCallContext,
		rpc.MethodCurrentSelection, (*rpc.CurrentSelectionOutput).Validate)
}
