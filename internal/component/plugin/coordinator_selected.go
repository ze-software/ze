// Design: docs/architecture/api/process-protocol.md -- selected-route final receipts.
// Related: coordinator.go -- optional protocol reactor delegation.
package plugin

import (
	"context"
	"errors"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// SelectRoutes reaches the selected writer of the currently attached reactor.
// An unsupported reactor is an error, never a successful empty result.
func (c *Coordinator) SelectRoutes(ctx context.Context, batch *SelectedBatch, owner SelectedOwner) (*rpc.SelectRoutesOutput, error) {
	reactor := c.getReactor()
	if reactor == nil {
		return nil, ErrNoReactor
	}
	selected, supported := reactor.(ReactorSelectedCoordinator)
	if !supported {
		return nil, errors.New("select-routes: reactor does not support selected output")
	}
	return selected.SelectRoutes(ctx, batch, owner)
}
