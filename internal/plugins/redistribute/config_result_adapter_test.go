// Design: docs/architecture/api/process-protocol.md -- route result adapter.
package redistribute

import (
	"context"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func (n *nopUpdater) UpdateRouteResult(ctx context.Context, peer, command string) (*rpc.UpdateRouteOutput, error) {
	a, w, err := n.UpdateRoute(ctx, peer, command)
	return &rpc.UpdateRouteOutput{Announced: a, Withdrawn: w}, err
}
