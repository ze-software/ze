// Design: docs/architecture/api/process-protocol.md -- full route result adapters.
package reactor

import (
	"context"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func (b *redistributionNextHopBoundary) UpdateRouteResult(ctx context.Context, peer, command string) (*rpc.UpdateRouteOutput, error) {
	a, w, err := b.UpdateRoute(ctx, peer, command)
	return &rpc.UpdateRouteOutput{Announced: a, Withdrawn: w}, err
}
func (b *serialRedistributionWireBoundary) UpdateRouteResult(ctx context.Context, peer, command string) (*rpc.UpdateRouteOutput, error) {
	a, w, err := b.UpdateRoute(ctx, peer, command)
	return &rpc.UpdateRouteOutput{Announced: a, Withdrawn: w}, err
}

func (b *redistributionNextHopBoundary) UpdateRouteRequestResult(ctx context.Context, input rpc.UpdateRouteInput) (*rpc.UpdateRouteOutput, error) {
	return b.UpdateRouteResult(ctx, input.PeerSelector, input.Command)
}
func (b *redistributionNextHopBoundary) OnOriginAdmissionLookup(rpc.OriginAdmissionLookupHandler) {}

func (b *serialRedistributionWireBoundary) UpdateRouteRequestResult(ctx context.Context, input rpc.UpdateRouteInput) (*rpc.UpdateRouteOutput, error) {
	return b.UpdateRouteResult(ctx, input.PeerSelector, input.Command)
}
func (b *serialRedistributionWireBoundary) OnOriginAdmissionLookup(rpc.OriginAdmissionLookupHandler) {
}
