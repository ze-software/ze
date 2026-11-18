// Design: docs/architecture/api/process-protocol.md -- original accepted redistribution lookup.
// Related: rpc.go -- PluginConn owns the exact authenticated accepting connection.
package ipc

import (
	"context"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// SendOriginAdmissionLookup queries the original accepted snapshot on this connection,
// never a daemon-global provider or ordinary execute-command route. A published
// bridge commits cutover: a stopped or unready slot fails without socket fallback.
// The caller MUST retain this exact accepting process lifetime until return and
// separately verify the publisher, accepted lifetime, actual grants and immutable
// route build. This lookup grants no process or peer authority.
func (pc *PluginConn) SendOriginAdmissionLookup(ctx context.Context, input rpc.OriginAdmissionLookupInput) (*rpc.OriginAdmissionLookupOutput, error) {
	if bridge := pc.bridge.Load(); bridge != nil {
		return bridge.OriginAdmissionLookup(ctx, input)
	}
	if pc.mux != nil {
		return pc.mux.OriginAdmissionLookup(ctx, input)
	}
	return callReadSnapshot(ctx, pc.Conn, input, rpc.OriginAdmissionLookupCallContext,
		rpc.MethodOriginAdmissionLookup, (*rpc.OriginAdmissionLookupOutput).Validate)
}
