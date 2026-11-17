// Design: docs/architecture/api/process-protocol.md -- route result adapter.
package ls_export

import (
	"context"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func (d *exportDelivery) UpdateRouteWithMetaResult(ctx context.Context, peer, command string, meta map[string]any) (*rpc.UpdateRouteOutput, error) {
	a, w, err := d.UpdateRouteWithMeta(ctx, peer, command, meta)
	return &rpc.UpdateRouteOutput{Announced: a, Withdrawn: w}, err
}
func (d *blockedExportDelivery) UpdateRouteWithMetaResult(ctx context.Context, peer, command string, meta map[string]any) (*rpc.UpdateRouteOutput, error) {
	a, w, err := d.UpdateRouteWithMeta(ctx, peer, command, meta)
	return &rpc.UpdateRouteOutput{Announced: a, Withdrawn: w}, err
}
func (c *exportCapture) UpdateRouteWithMetaResult(ctx context.Context, peer, command string, meta map[string]any) (*rpc.UpdateRouteOutput, error) {
	a, w, err := c.UpdateRouteWithMeta(ctx, peer, command, meta)
	return &rpc.UpdateRouteOutput{Announced: a, Withdrawn: w}, err
}
