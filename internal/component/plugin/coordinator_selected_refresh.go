// Design: docs/architecture/api/process-protocol.md -- selected refresh delegation.
package plugin

import (
	"context"
	"errors"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func (c *Coordinator) SelectRefresh(ctx context.Context, request *SelectedRefresh, owner SelectedOwner) (*rpc.SelectRefreshOutput, error) {
	reactor := c.getReactor()
	if reactor == nil {
		return nil, ErrNoReactor
	}
	selected, supported := reactor.(ReactorSelectedRefresher)
	if !supported {
		return nil, errors.New("select-refresh reactor is unavailable; start the BGP subsystem before replay")
	}
	return selected.SelectRefresh(ctx, request, owner)
}
