// Design: docs/architecture/api/process-protocol.md -- exact deciding connection.
// Related: rpc.go -- native cutover and authenticated connection lifetime.
package ipc

import (
	"context"
	"errors"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// SendOriginIntake uses the registered mutation slot or the same public framed
// callback. A legacy Conn has no joined receiver and cannot offer mutations.
func (pc *PluginConn) SendOriginIntake(ctx context.Context, input rpc.OriginIntakeInput) (*rpc.UpdateRouteOutput, error) {
	if bridge := pc.bridge.Load(); bridge != nil {
		return bridge.OriginIntake(ctx, input)
	}
	if pc.mux == nil {
		return nil, errors.New("origin-intake requires a multiplexed cancellable connection")
	}
	return pc.mux.OriginIntake(ctx, input)
}
