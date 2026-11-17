// Design: docs/architecture/api/process-protocol.md -- original source leases.
// Related: sdk_native.go -- owner operations share native and JSON evaluators.
package sdk

import (
	"context"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// SourceLease acquires, retains or releases the authenticated process's lease.
// The caller MUST acquire before the original source's ordered DOWN completes,
// retain only while its deciding state keeps the source eligible, and release
// when its last candidate is purged. Release success joins final writes.
func (p *Plugin) SourceLease(ctx context.Context, input rpc.SourceLeaseInput) (*rpc.SourceLeaseOutput, error) {
	ctx, cancel := rpc.SourceLeaseCallContext(ctx, &input)
	defer cancel()
	if err := input.Validate(time.Now()); err != nil {
		return nil, err
	}
	output, err := cancellableEngineResult[rpc.SourceLeaseOutput](p, ctx, rpc.MethodSourceLease, input)
	if err != nil {
		return nil, err
	}
	if err := output.Validate(input); err != nil {
		return nil, err
	}
	return output, nil
}
