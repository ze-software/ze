// Design: docs/architecture/api/process-protocol.md -- route result adapter.
package redistribute

import (
	"context"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func (f *fakeDispatcher) UpdateRouteResult(ctx context.Context, peer, command string) (*rpc.UpdateRouteOutput, error) {
	a, w, err := f.UpdateRoute(ctx, peer, command)
	return &rpc.UpdateRouteOutput{Announced: a, Withdrawn: w}, err
}
