// Design: docs/plugin-development/protocol.md -- final selected refresh receipts.
// Related: sdk_selected.go -- selected routes use the same destination lifetime.
package sdk

import (
	"context"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// SelectRefresh writes one marker for the captured selected-output destination.
// Callers MUST await Begin before replay and await every route's final receipt
// before End. A successful End MUST NOT be treated as settling unrelated work.
func (p *Plugin) SelectRefresh(ctx context.Context, input rpc.SelectRefreshInput) (*rpc.SelectRefreshOutput, error) {
	ctx, cancel := rpc.SelectRefreshCallContext(ctx, &input)
	defer cancel()
	if err := input.Validate(time.Now()); err != nil {
		return nil, err
	}
	output, err := cancellableEngineResult[rpc.SelectRefreshOutput](p, ctx, rpc.MethodSelectRefresh, input)
	if err != nil {
		return nil, err
	}
	if err := output.Validate(input); err != nil {
		return nil, err
	}
	return output, nil
}
