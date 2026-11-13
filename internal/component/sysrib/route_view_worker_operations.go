// Design: docs/architecture/rib/unified-locrib.md -- serialized routing-view operations.
// Related: route_view_worker.go -- the joined worker owns all lease mutations.

package sysrib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const routeViewTasksMax = 16

// A task owns its inputs and one buffered result. Only the worker admits its
// mutation; callers MUST keep waiting for an admitted install's final verdict.
type routeViewTask interface {
	perform(*routeViewWorker) error
}

type routeViewConfigurationTask struct {
	ctx           context.Context
	configuration routeViewConfiguration
	done          chan error
}

type routeViewCandidatesTask struct {
	ctx   context.Context
	input rpc.RouteViewCandidatesInput
	done  chan routeViewCandidatesResult
}

type routeViewCandidatesResult struct {
	rows []json.RawMessage
	err  error
}

func (client *routeViewClient) ApplyConfiguration(ctx context.Context, distances map[string]int, permitted map[string]bool) error {
	ctx, cancel := context.WithTimeout(ctx, routeViewConfigurationTimeout)
	defer cancel()
	if _, err := routeViewClientDistances(distances); err != nil {
		return err
	}
	task := &routeViewConfigurationTask{ctx: ctx, configuration: routeViewConfiguration{distances: maps.Clone(distances), permitted: maps.Clone(permitted)}, done: make(chan error, 1)}
	worker, err := client.enqueue(ctx, task)
	if err != nil {
		return err
	}
	// Cancellation after admission still owes the compensation verdict. The
	// worker context also cancels every RPC, so Close can always join it.
	select {
	case err := <-task.done:
		return err
	case <-worker.done:
		select {
		case err := <-task.done:
			return err
		default:
			return client.stoppedError()
		}
	}
}

func (client *routeViewClient) Candidates(ctx context.Context, args []string) (any, error) {
	ctx, cancel := context.WithTimeout(ctx, routeViewSnapshotLifetime)
	defer cancel()
	prefix, filtered, err := candidatesPrefix(args)
	if err != nil {
		return nil, err
	}
	input := rpc.RouteViewCandidatesInput{Limit: rpc.RouteViewRecordsMax}
	if filtered {
		input.Prefix = prefix
	}
	task := &routeViewCandidatesTask{ctx: ctx, input: input, done: make(chan routeViewCandidatesResult, 1)}
	worker, err := client.enqueue(ctx, task)
	if err != nil {
		return nil, err
	}
	select {
	case answer := <-task.done:
		return answer.rows, answer.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-worker.done:
		select {
		case answer := <-task.done:
			return answer.rows, answer.err
		default:
			return nil, client.stoppedError()
		}
	}
}

func (client *routeViewClient) enqueue(ctx context.Context, task routeViewTask) (*routeViewWorker, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.failure != nil {
		return nil, client.failure
	}
	worker := client.worker
	if worker == nil {
		return nil, routeViewRefusal("The system routing-view worker has not started", "Runtime commands require an active consumer", "Retry after the system RIB starts")
	}
	if err := worker.ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case worker.tasks <- task:
		return worker, nil
	default:
		return nil, routeViewRefusal("The system routing-view operation queue is full", "At most sixteen commands or configuration requests can wait for the worker", "Retry after the pending operations complete")
	}
}

func (client *routeViewClient) stoppedError() error {
	if err := client.Failure(); err != nil {
		return err
	}
	return routeViewRefusal("The system routing-view worker stopped", "No runtime operation can be admitted after shutdown", "ze request reload")
}

func (worker *routeViewWorker) serveTask() error {
	select {
	case task := <-worker.tasks:
		return task.perform(worker)
	default:
		return nil
	}
}

func (task *routeViewCandidatesTask) perform(worker *routeViewWorker) error {
	operation, cancel := worker.operation(task.ctx)
	defer cancel()
	rows, err := worker.candidates(operation, task.input)
	task.done <- routeViewCandidatesResult{rows: rows, err: err}
	return nil
}

func (task *routeViewConfigurationTask) perform(worker *routeViewWorker) error {
	if err := task.ctx.Err(); err != nil {
		task.done <- err
		return nil
	}
	operation, cancel := worker.operation(task.ctx)
	previous := worker.client.configuration
	err := worker.configure(operation, task.configuration)
	if errors.Is(err, errRouteViewReset) {
		err = worker.reopen(operation, task.configuration)
	}
	cancel()
	if err != nil {
		// An RPC refusal can follow uncertain publication. Rebuilding a fresh
		// old-configuration lease also replaces a possibly fenced output lease.
		restore, stop := context.WithTimeout(worker.ctx, routeViewConfigurationCompensationTimeout)
		restoreErr := worker.reopen(restore, previous)
		stop()
		if restoreErr != nil {
			err = sdk.RunningConfigLost(errors.Join(err, restoreErr), "The system RIB could not restore its running configuration", "")
			worker.fail(err)
			task.done <- err
			return err
		}
	}
	task.done <- err
	return nil
}

// operation ties a finite request to both its caller and worker stop. A fresh
// restoration uses the worker context, independently of caller cancellation.
func (worker *routeViewWorker) operation(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(parent, routeViewConfigurationTimeout)
	stop := context.AfterFunc(worker.ctx, cancel)
	return ctx, func() { stop(); cancel() }
}

func (worker *routeViewWorker) candidates(ctx context.Context, input rpc.RouteViewCandidatesInput) ([]json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	input.Session = worker.client.view.Session
	rows := make([]json.RawMessage, 0)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		page, err := worker.client.plugin.RouteViewCandidates(ctx, &input)
		if err != nil {
			return nil, err
		}
		if page.Session != input.Session {
			return nil, routeViewRefusal("The candidate page belongs to another session", "The scan cannot mix authenticated consumer leases", "Retry show rib candidates")
		}
		if page.Epoch != worker.client.view.Epoch {
			return nil, routeViewRefusal("The candidate page belongs to another daemon epoch", "The scan cannot mix authority generations", "Retry show rib candidates")
		}
		if page.Snapshot == "" {
			return nil, routeViewRefusal("The candidate page has no snapshot identity", "The scan cannot prove its immutable pin", "Retry show rib candidates")
		}
		if input.Snapshot != "" {
			if page.Snapshot != input.Snapshot {
				return nil, routeViewRefusal("The candidate snapshot changed during its scan", "The scan cannot combine rows from different pins", "Retry show rib candidates")
			}
		}
		if len(page.Entries) > rpc.RouteViewRecordsMax {
			return nil, routeViewRefusal("The candidate page exceeds its record bound", "One reply can contain at most 4096 prefixes", "Retry show rib candidates")
		}
		if uint64(len(page.Entries)) > math.MaxUint64-input.Cursor {
			return nil, routeViewRefusal("The candidate cursor is exhausted", "The scan cannot advance safely", "Retry show rib candidates")
		}
		if page.Cursor != input.Cursor+uint64(len(page.Entries)) {
			return nil, routeViewRefusal("The candidate page has an invalid cursor", "The cursor must advance by the exact number of complete prefix rows", "Retry show rib candidates")
		}
		if !page.Complete {
			if len(page.Entries) == 0 {
				return nil, routeViewRefusal("The candidate scan made no progress", "An incomplete page must advance its immutable pin", "Retry show rib candidates")
			}
		}
		for _, row := range page.Entries {
			if !json.Valid(row) {
				return nil, routeViewRefusal("The candidate page contains an invalid row", "The rendered prefix must remain valid owned JSON", "Retry show rib candidates")
			}
			rows = append(rows, slices.Clone(row))
		}
		if page.Complete {
			return rows, nil
		}
		input.Snapshot, input.Cursor = page.Snapshot, page.Cursor
	}
}

func (worker *routeViewWorker) configure(ctx context.Context, configuration routeViewConfiguration) error {
	worker.desired = configuration
	worker.pending = make(map[prefixKey]outgoingChange)
	worker.client.health.setPending(0)
	defer func() { worker.pending = nil }()
	rows, err := routeViewClientDistances(configuration.distances)
	if err != nil {
		return err
	}
	if worker.client.distanceOp == math.MaxUint64 {
		return routeViewRefusal("The distance operation identity is exhausted", "A fresh system routing-view session is required", "ze request reload")
	}
	// Reserve before sending: even an uncertain failed call consumes its identity.
	worker.client.distanceOp++
	operation := worker.client.distanceOp
	ack, err := worker.client.plugin.RouteViewDistances(ctx, &rpc.RouteViewDistancesInput{Session: worker.client.view.Session, Operation: operation, Distances: rows})
	if err != nil {
		return fmt.Errorf("apply authoritative system routing distances: %w", err)
	}
	if ack.Session != worker.client.view.Session {
		return routeViewRefusal("The distance acknowledgment belongs to another session", "The applied declaration is uncertain", "ze request reload")
	}
	if ack.Operation != operation {
		return routeViewRefusal("The distance acknowledgment has another operation", "The applied declaration is uncertain", "ze request reload")
	}
	if worker.projection != nil {
		for !worker.projection.projection.Synchronized() || worker.projection.projection.Cursor() < ack.Cursor {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := worker.readSelected(ctx); err != nil {
				return err
			}
		}
		if !worker.projection.projection.Synchronized() {
			return routeViewRefusal("The selected view is incomplete after distance application", "Forwarding cannot acknowledge a partial authoritative snapshot", "ze request reload")
		}
	} else if ack.Cursor != 0 {
		return routeViewRefusal("The local distance acknowledgment has a stream cursor", "The local authority is synchronized at cursor zero", "ze request reload")
	} else {
		changes, err := worker.local.scan(ctx, worker.client.rib)
		if err != nil {
			return err
		}
		if err := worker.apply(ctx, changes, nil); err != nil {
			return err
		}
	}
	for fam, entries := range worker.client.rib.applyFIBImport(configuration.permitted) {
		worker.collect(fam, entries)
	}
	if err := worker.flush(ctx); err != nil {
		return err
	}
	worker.client.rib.mu.Lock()
	worker.client.rib.adminDist = configuration.distances
	worker.client.rib.mu.Unlock()
	worker.client.configuration = configuration
	worker.client.health.markSynchronized()
	return nil
}

// reopen MUST discard the fenced lease's consumer state before reconstructing
// authority. Its caller MUST provide the desired declaration for this attempt,
// which can differ from the last successfully committed configuration.
func (worker *routeViewWorker) reopen(ctx context.Context, configuration routeViewConfiguration) error {
	for {
		worker.client.health.beginResync()
		err := worker.reopenOnce(ctx, configuration)
		if !errors.Is(err, errRouteViewReset) {
			return err
		}
	}
}

func (worker *routeViewWorker) reopenOnce(ctx context.Context, configuration routeViewConfiguration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	old := worker.client.view.Session
	ack, err := worker.client.plugin.RouteViewClose(ctx, &rpc.RouteViewCloseInput{Session: old})
	if err != nil {
		return fmt.Errorf("close fenced system routing view: %w", err)
	}
	if ack.Session != old {
		return routeViewRefusal("The fenced routing-view cleanup belongs to another session", "Reconstruction cannot prove that the old output was withdrawn", "ze request reload")
	}
	input := rpc.RouteViewOpenInput{}
	if worker.local != nil {
		input.Reader = rpc.RouteViewReaderLocalAuthority
	}
	opened, err := worker.client.plugin.RouteViewOpen(ctx, &input)
	if err != nil {
		return fmt.Errorf("reopen authoritative system routing view: %w", err)
	}
	projection, err := newSelectedProjection(*opened)
	if err != nil {
		return err
	}
	// The daemon has withdrawn this lease's complete ledger. Clearing the
	// caches forces the replacement snapshot to emit owned initial installs.
	worker.client.rib.resolver.Store(nil)
	worker.client.rib.mu.Lock()
	clear(worker.client.rib.routes)
	clear(worker.client.rib.best)
	clear(worker.client.rib.installed)
	clear(worker.client.rib.lastECMP)
	worker.client.rib.mu.Unlock()
	worker.client.view = *opened
	worker.client.distanceOp, worker.outputOp = 0, 0
	if worker.pending != nil {
		clear(worker.pending)
	}
	if worker.local != nil {
		worker.client.rib.resolver.Store(newNHResolver(worker.local.rib))
	} else {
		worker.projection = &routeViewProjectionReader{projection: projection}
	}
	return worker.configure(ctx, configuration)
}

func (worker *routeViewWorker) collect(fam family.Family, entries []outgoingChange) {
	for _, entry := range entries {
		worker.pending[prefixKey{family: fam, prefix: entry.Prefix}] = entry
	}
	worker.client.health.setPending(len(worker.pending))
}

func (worker *routeViewWorker) flush(ctx context.Context) error {
	batches := make(map[family.Family][]outgoingChange)
	for key, entry := range worker.pending {
		batches[key.family] = append(batches[key.family], entry)
	}
	return worker.publish(ctx, batches, 0)
}
