// Design: docs/architecture/rib/unified-locrib.md -- routing-view consumer worker.
// Related: route_view_worker_local.go -- bounded authoritative local capture.

package sysrib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"sync"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var errRouteViewReset = errors.New("selected routing view requires a new consumer lease")

// routeViewWorker owns one consumer generation. Start MUST create it once;
// Close MUST cancel it and join done before releasing its SDK connection.
// Only this worker reads or advances the selected stream and output identity.
type routeViewWorker struct {
	client     *routeViewClient
	ctx        context.Context
	cancel     context.CancelFunc
	done       chan struct{}
	tasks      chan routeViewTask
	pending    map[prefixKey]outgoingChange
	desired    routeViewConfiguration
	projection *routeViewProjectionReader
	local      *routeViewLocalCapture
	outputOp   uint64
}

// routeViewProjectionReader serializes publication of complete pages with
// concurrent resolver/command LPM reads. Stored path values are immutable.
type routeViewProjectionReader struct {
	mu         sync.RWMutex
	projection *selectedProjection
}

func (reader *routeViewProjectionReader) LPM(fam family.Family, addr netip.Addr) (locrib.Path, netip.Prefix, bool) {
	reader.mu.RLock()
	defer reader.mu.RUnlock()
	return reader.projection.LPM(fam, addr)
}

// Start MUST return without waiting for runtime RPC admission. Its caller MUST
// eventually call Close, including after cancellation of the supplied context.
func (client *routeViewClient) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.worker != nil {
		return routeViewRefusal("The system routing-view worker already started", "A consumer generation has exactly one worker", "ze request reload")
	}
	run, cancel := context.WithCancel(ctx)
	worker := &routeViewWorker{client: client, ctx: run, cancel: cancel, done: make(chan struct{}), tasks: make(chan routeViewTask, routeViewTasksMax), desired: client.configuration}
	client.rib.resolver.Store(nil)
	client.worker = worker
	go worker.run(run)
	return nil
}

func (worker *routeViewWorker) run(ctx context.Context) {
	defer close(worker.done)
	defer worker.client.health.stop()
	if err := worker.consume(ctx); err != nil {
		// The engine's stop fences native dispatch before the owner context
		// ends, so the fence itself is a normal stop. A failed callback loop
		// reports its own cause, and a live owner's refusal is an RPC error.
		if errors.Is(err, rpc.ErrBridgeClosed) {
			return
		}
		if ctx.Err() != nil {
			if errors.Is(err, ctx.Err()) {
				return
			}
			if routeViewTransportStopped(err) {
				return
			}
		}
		worker.fail(err)
	}
}

func (worker *routeViewWorker) fail(err error) {
	worker.client.health.fail(err)
	worker.client.mu.Lock()
	if worker.client.failure != nil {
		worker.client.mu.Unlock()
		return
	}
	worker.client.failure = err
	worker.client.mu.Unlock()
	// Closing both SDK transports ends Run in either placement, allowing
	// exact-generation cleanup and the declared failure policy to run.
	if closeErr := worker.client.plugin.Close(); closeErr != nil {
		worker.client.mu.Lock()
		worker.client.failure = fmt.Errorf("routing-view worker failed: %w; close SDK: %w", err, closeErr)
		worker.client.health.fail(worker.client.failure)
		worker.client.mu.Unlock()
	}
}

func (worker *routeViewWorker) consume(ctx context.Context) error {
	switch reader := worker.client.reader.(type) {
	case routeViewClientLocal:
		worker.local = newRouteViewLocalCapture(reader.rib)
		defer worker.local.unsubscribe()
		worker.client.rib.resolver.Store(newNHResolver(reader.rib))
		changes, err := worker.local.scan(ctx, worker.client.rib)
		if err != nil {
			return err
		}
		worker.client.health.setPending(len(changes))
		if err := worker.apply(ctx, changes, nil); err != nil {
			return err
		}
		return worker.consumeLocal(ctx)
	case routeViewClientSelected:
		projection, err := newSelectedProjection(worker.client.view)
		if err != nil {
			return err
		}
		worker.projection = &routeViewProjectionReader{projection: projection}
		initial, cancel := worker.operation(ctx)
		err = worker.synchronize(initial)
		cancel()
		if err != nil {
			return err
		}
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := worker.readSelected(ctx); err != nil {
				if !errors.Is(err, errRouteViewReset) {
					return err
				}
				rebuild, cancel := worker.operation(ctx)
				err = worker.reopen(rebuild, worker.desired)
				cancel()
				if err != nil {
					return err
				}
			}
			if err := worker.replay(ctx); err != nil {
				if !errors.Is(err, errRouteViewReset) {
					return err
				}
				rebuild, cancel := worker.operation(ctx)
				err = worker.reopen(rebuild, worker.desired)
				cancel()
				if err != nil {
					return err
				}
			}
			if err := worker.serveTask(); err != nil {
				return err
			}
		}
	default:
		return routeViewRefusal("The system routing-view reader is unavailable", "Forwarding requires an initialized authoritative reader", "ze request reload")
	}
}

func (worker *routeViewWorker) readSelected(ctx context.Context) error {
	operation, cancel := worker.operation(ctx)
	defer cancel()
	p := worker.projection.projection
	page, err := worker.client.plugin.RouteViewRead(operation, &rpc.RouteViewReadInput{
		Session: worker.client.view.Session, Cursor: p.Cursor(), Limit: rpc.RouteViewRecordsMax, WaitMillis: rpc.RouteViewWaitMillisMax,
	})
	if err != nil {
		return fmt.Errorf("read selected routing view: %w", err)
	}
	var affected []netip.Addr
	if resolver := worker.client.rib.nextHopResolver(); resolver != nil {
		for i := range page.Entries {
			affected = append(affected, resolver.CoveredNHs(page.Entries[i].Prefix)...)
		}
	}
	worker.projection.mu.Lock()
	keys, err := p.Apply(page)
	worker.projection.mu.Unlock()
	if err != nil {
		return err
	}
	if page.Phase == rpc.RouteViewPhaseReset {
		worker.client.health.reset()
		return errRouteViewReset
	}
	if !p.Synchronized() {
		pending := 0
		for _, table := range p.staging {
			pending += table.Len()
		}
		worker.client.health.setPending(pending)
		return nil
	}
	if worker.client.rib.nextHopResolver() == nil {
		worker.client.rib.resolver.Store(newNHResolver(worker.projection))
	}
	changes := make([]routeViewSelectedChange, 0, len(keys))
	for _, key := range keys {
		var prepared routeViewSelectedChange = routeViewSelectedRemove{key: prefixKey{family: key.family, prefix: key.prefix}}
		var prepareErr error
		p.Inspect(key.family, key.prefix, func(value *selectedProjectionPath) {
			prepared, prepareErr = prepareRouteViewSelectedMetadata(locrib.Change{Kind: locrib.ChangeAdd, Family: key.family, Prefix: key.prefix, Best: value.path, ECMP: value.path.ECMP}, value.members)
		})
		if prepareErr != nil {
			return prepareErr
		}
		changes = append(changes, prepared)
	}
	return worker.apply(ctx, changes, affected)
}

func (worker *routeViewWorker) synchronize(ctx context.Context) error {
	for !worker.projection.projection.Synchronized() {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := worker.readSelected(ctx); err != nil {
			if !errors.Is(err, errRouteViewReset) {
				return err
			}
			return worker.reopen(ctx, worker.desired)
		}
	}
	return nil
}

func (worker *routeViewWorker) apply(ctx context.Context, changes []routeViewSelectedChange, affected []netip.Addr) error {
	resolver := worker.client.rib.nextHopResolver()
	for _, change := range changes {
		affected = append(affected, resolver.CoveredNHs(change.selectedKey().prefix)...)
	}
	batches, err := worker.client.rib.applySelectedBatch(changes)
	if err != nil {
		return err
	}
	for fam, entries := range worker.client.rib.cascadeChanges(affected) {
		batches[fam] = append(batches[fam], entries...)
	}
	for fam, entries := range batches {
		if worker.pending != nil {
			worker.collect(fam, entries)
			continue
		}
		if err := worker.publish(ctx, outgoingBatch{Family: fam, Changes: entries}); err != nil {
			return err
		}
	}
	if worker.pending == nil {
		worker.client.health.markSynchronized()
	}
	return nil
}

func (client *routeViewClient) HandleEvent(event string) error {
	envelope, err := rpc.ParseEventEnvelope(event)
	if err != nil {
		return err
	}
	if envelope.Namespace != sysribevents.Namespace {
		return routeViewRefusal("The system RIB received an unrelated event", "Only system replay requests are subscribed", "ze request reload")
	}
	if envelope.Event != sysribevents.EventReplayRequest {
		return routeViewRefusal("The system RIB received an unknown event", "Only replay requests can trigger a system replay", "ze request reload")
	}
	var request replay.Request
	if err := json.Unmarshal(envelope.Payload, &request); err != nil {
		return err
	}
	if request.ReplayID != replay.Broadcast {
		return routeViewRefusal("The system replay request is not a broadcast replay", "The system RIB only accepts the full-table broadcast identity", "ze request reload")
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.failure != nil {
		return client.failure
	}
	revision := client.replays[request.ReplayID]
	if revision == math.MaxUint64 {
		return routeViewRefusal("The system replay request identity is exhausted", "A new request cannot replace the last retained replay safely", "ze request reload")
	}
	client.replays[request.ReplayID] = revision + 1
	select {
	case client.wake <- struct{}{}:
	default:
	}
	return nil
}

// routeViewReplayRequest is a bounded snapshot of one pending arrival revision.
// The worker MUST retire only that revision after all output is acknowledged.
type routeViewReplayRequest struct {
	identity uint64
	revision uint64
}

func (worker *routeViewWorker) replay(ctx context.Context) error {
	if worker.projection != nil {
		if !worker.projection.projection.Synchronized() {
			return nil
		}
	}
	worker.client.mu.Lock()
	requests := make([]routeViewReplayRequest, 0, len(worker.client.replays))
	for id, revision := range worker.client.replays {
		requests = append(requests, routeViewReplayRequest{identity: id, revision: revision})
	}
	worker.client.mu.Unlock()
	if len(requests) == 0 {
		return nil
	}
	if worker.local != nil {
		if err := worker.applyLocal(ctx, worker.local.takeKeys(nil)); err != nil {
			return err
		}
	}
	for _, request := range requests {
		for _, batch := range worker.client.rib.replayBatches(&replay.Request{ReplayID: request.identity}) {
			if err := worker.publish(ctx, batch); err != nil {
				return err
			}
		}
		worker.client.mu.Lock()
		if worker.client.replays[request.identity] == request.revision {
			delete(worker.client.replays, request.identity)
		}
		worker.client.mu.Unlock()
	}
	return nil
}
