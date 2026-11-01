// Design: docs/architecture/api/process-protocol.md — the typed filter-update slot (Runtime hot path)
// Related: filter_batch.go — the batched filter-update exchange a route takes otherwise

package ipc

import (
	"context"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// CallFilter judges route by the plugin's filter on the transport the plugin
// runs on (D-12). A plugin running internal that installed a filter slot is
// called through it on the caller's goroutine, nothing encoded; once the
// slot existed the route never falls back to the queue, so a stopped plugin
// answers rpc.ErrBridgeClosed ("unavailable"). Any other plugin is sent the
// route, its fields encoded, in its next filter-update batch (SendFilterUpdate),
// which ctx bounds as it bounds SendFilterUpdate. Safe for concurrent use.
func (pc *PluginConn) CallFilter(ctx context.Context, route *rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
	if bridge := pc.bridge.Load(); bridge != nil && bridge.HasFilterSlot() {
		return bridge.CallFilterSlot(route)
	}
	input, err := route.Input()
	if err != nil {
		return nil, err
	}
	return pc.SendFilterUpdate(ctx, &input)
}
