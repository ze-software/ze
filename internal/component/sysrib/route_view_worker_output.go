// Design: docs/architecture/rib/unified-locrib.md -- routing-view output transport.
// Related: route_view_output.go -- one daemon evaluator for both transports.

package sysrib

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// publish emits one publication: every forwarding change one consumer step
// produced, in every family. It is the worker's only route to the daemon's
// output, so orderForwarding runs for every live, cascade, configuration and
// replay publication alike, and no caller can hand a FIB writer a route ahead
// of the route that makes its gateway reachable. A replayID of zero is live.
func (worker *routeViewWorker) publish(ctx context.Context, changes map[family.Family][]outgoingChange, replayID uint64) error {
	batches, err := orderForwarding(changes, replayID)
	if err != nil {
		return err
	}
	for _, batch := range batches {
		if err := worker.publishBatch(ctx, batch); err != nil {
			return err
		}
	}
	return nil
}

// publishBatch emits one ordered family batch in acknowledged chunks, in
// order. Only publish calls it.
func (worker *routeViewWorker) publishBatch(ctx context.Context, batch outgoingBatch) error {
	for start := 0; start < len(batch.Changes); start += rpc.RouteViewRecordsMax {
		end := min(start+rpc.RouteViewRecordsMax, len(batch.Changes))
		chunk := batch
		chunk.Changes = batch.Changes[start:end]
		if err := worker.publishChunk(ctx, chunk, 0); err != nil {
			return err
		}
	}
	return nil
}

func (worker *routeViewWorker) publishChunk(ctx context.Context, batch outgoingBatch, depth int) error {
	operation, cancel := worker.operation(ctx)
	defer cancel()
	if worker.outputOp == math.MaxUint64 {
		return routeViewRefusal("The system publication identity is exhausted", "A new routing-view session is required before more output", "ze request reload")
	}
	cursor := uint64(0)
	if worker.projection != nil {
		cursor = worker.projection.projection.Cursor()
	}
	input := routeViewPublication{Session: worker.client.view.Session, Cursor: cursor, Operation: worker.outputOp + 1, Batch: batch}
	if routeViewPublicationBytes(input) > rpc.RouteViewRequestBudget(rpc.MethodRouteViewPublish) {
		if len(batch.Changes) < 2 {
			return routeViewRefusal("A system route exceeds the publication frame", "The complete forwarding metadata cannot fit one routing-view request", "Reduce the route metadata and reload")
		}
		if depth == 12 {
			return routeViewRefusal("System publication splitting exceeded its bound", "A publication chunk cannot contain more than 4096 routes", "ze request reload")
		}
		left, right := batch, batch
		middle := len(batch.Changes) / 2
		left.Changes, right.Changes = batch.Changes[:middle], batch.Changes[middle:]
		if err := worker.publishChunk(ctx, left, depth+1); err != nil {
			return err
		}
		return worker.publishChunk(ctx, right, depth+1)
	}
	worker.outputOp = input.Operation
	var ack rpc.RouteViewPublishOutput
	if worker.client.plugin.IsInternal() {
		value, handled, err := worker.client.plugin.TryEngineNative(operation, rpc.MethodRouteViewPublish, input)
		if err != nil {
			return worker.publicationReset(operation, input, fmt.Errorf("publish native system routing output: %w", err))
		}
		if !handled {
			return routeViewRefusal("The internal system RIB has no native publisher", "Internal forwarding requires the registered value operation", "ze request reload")
		}
		var valid bool
		ack, valid = value.(rpc.RouteViewPublishOutput)
		if !valid {
			return routeViewRefusal("The native system publisher returned an invalid acknowledgment", "The publication outcome cannot be established", "ze request reload")
		}
	} else {
		encoded, err := json.Marshal(&batch)
		if err != nil {
			return err
		}
		answer, err := worker.client.plugin.RouteViewPublish(operation, &rpc.RouteViewPublishInput{Session: input.Session, Cursor: input.Cursor, Operation: input.Operation, Batch: encoded})
		if err != nil {
			return worker.publicationReset(operation, input, fmt.Errorf("publish system routing output: %w", err))
		}
		ack = *answer
	}
	if ack.Session != input.Session {
		return routeViewRefusal("The system publication acknowledgment belongs to another session", "The forwarding output was not acknowledged", "ze request reload")
	}
	if ack.Operation != input.Operation {
		return routeViewRefusal("The system publication acknowledgment has another operation", "The forwarding output was not acknowledged", "ze request reload")
	}
	if ack.Cursor != input.Cursor {
		return routeViewRefusal("The system publication acknowledgment has another cursor", "The forwarding output was not acknowledged", "ze request reload")
	}
	return nil
}

// publicationReset MUST keep the failed publication's exact lease and cursor.
// A refusal grants no recovery by itself: only a matching explicit Reset proves
// that this validated operation emitted nothing before a benign view fence.
func (worker *routeViewWorker) publicationReset(ctx context.Context, input routeViewPublication, refusal error) error {
	if worker.projection == nil {
		return refusal
	}
	page, err := worker.client.plugin.RouteViewRead(ctx, &rpc.RouteViewReadInput{
		Session: input.Session, Cursor: input.Cursor, Limit: rpc.RouteViewRecordsMax, WaitMillis: 0,
	})
	if err != nil {
		return fmt.Errorf("probe refused system publication: %w; %w", refusal, err)
	}
	if page.Phase != rpc.RouteViewPhaseReset {
		return refusal
	}
	if page.RejectedPublication != input.Operation {
		return refusal
	}
	worker.projection.mu.Lock()
	_, err = worker.projection.projection.Apply(page)
	worker.projection.mu.Unlock()
	if err != nil {
		return fmt.Errorf("validate refused system publication reset: %w; %w", refusal, err)
	}
	worker.client.health.reset()
	return errRouteViewReset
}
