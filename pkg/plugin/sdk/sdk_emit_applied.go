// Design: docs/architecture/api/process-protocol.md -- captured event settlement.
// Related: sdk_engine.go -- ordinary EmitEvent retains enqueue-only behavior.
package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// EmitEventApplied waits for the exact matching subscribers to apply this event
// and finish their declared deferred work. An actual self subscription refuses
// before publication. Callers MUST NOT hold recipient locks or create cyclic
// synchronous event dependencies. Cancellation can leave partial application;
// an error never confirms cleanup or output completion and MUST be reported.
// Both placements use the common cancellable RPC; this cold operation does not
// borrow the ordinary DirectBridge slot whose contract is enqueue-only.
func (p *Plugin) EmitEventApplied(ctx context.Context, namespace, eventType, direction, peerAddress, event string) (int, error) {
	work, cancel := context.WithTimeout(ctx, rpc.CancellableRequestTimeout)
	defer cancel()
	deadline, hasDeadline := work.Deadline()
	if !hasDeadline {
		return 0, errors.New("applied event has no work deadline; supply a bounded context")
	}
	input := &rpc.EmitEventInput{AwaitApplied: true, DeadlineMS: deadline.UnixMilli(), Namespace: namespace,
		EventType: eventType, Direction: direction, PeerAddress: peerAddress, Event: event}
	var result json.RawMessage
	var err error
	if p.engineBridgeActive() {
		result, err = p.callEngineWithResult(work, rpc.MethodEmitEvent, input)
	} else {
		result, err = p.engineMux.CallRPCCancellable(work, rpc.MethodEmitEvent, input)
	}
	if err != nil {
		if work.Err() != nil {
			if _, unsettled := errors.AsType[*rpc.CancellationUnsettledError](err); !unsettled {
				return 0, fmt.Errorf("applied event did not settle; cleanup/output may still be owed: %w", err)
			}
		}
		return 0, err
	}
	var output *rpc.EmitEventOutput
	if err := json.Unmarshal(result, &output); err != nil {
		return 0, fmt.Errorf("unmarshal applied event result: %w", err)
	}
	if output == nil {
		return 0, errors.New("applied event returned no completion receipt")
	}
	if !output.Applied {
		return 0, errors.New("engine returned no applied-event receipt; rebuild the host and plugin from the same source")
	}
	if output.DeadlineMS != input.DeadlineMS {
		return 0, errors.New("engine returned a different applied-event deadline; rebuild the host and plugin from the same source")
	}
	if err := ctx.Err(); err != nil {
		return output.Delivered, err
	}
	if !time.Now().Before(deadline) {
		return output.Delivered, context.DeadlineExceeded
	}
	return output.Delivered, nil
}
