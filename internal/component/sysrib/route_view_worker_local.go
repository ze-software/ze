// Design: docs/architecture/rib/unified-locrib.md -- local routing-view consumer.
// Related: route_view_worker.go -- one joined consumer generation.

package sysrib

import (
	"context"
	"net/netip"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// routeViewLocalCapture queues only keys. Overflow requests a full scan rather
// than retaining unbounded keys or borrowed change/forward-handle storage.
// Its owner MUST unsubscribe before releasing the capture, after worker exit.
type routeViewLocalCapture struct {
	rib         *locrib.RIB
	keys        chan prefixKey
	overflow    atomic.Bool
	unsubscribe func()
}

func newRouteViewLocalCapture(rib *locrib.RIB) *routeViewLocalCapture {
	capture := &routeViewLocalCapture{rib: rib, keys: make(chan prefixKey, rpc.RouteViewRecordsMax)}
	// Subscribe before the scan: a key touched during the scan is reread from
	// authority after the scan, so an older captured value cannot revive it.
	capture.unsubscribe = rib.OnChange(capture.offer)
	return capture
}

func (capture *routeViewLocalCapture) offer(change locrib.Change) { //nolint:gocritic // hugeParam: the signature is locrib.ChangeHandler's
	key := prefixKey{family: change.Family, prefix: change.Prefix}
	select {
	case capture.keys <- key:
	default:
		capture.recordOverflow(key)
	}
}

func (capture *routeViewLocalCapture) recordOverflow(key prefixKey) {
	capture.overflow.Store(true)
	// The consumer may have drained the queue and checked overflow since the
	// failed offer. Retry the same key after recording the scan request: either
	// this key or an already queued key MUST wake that consumer.
	select {
	case capture.keys <- key:
	default:
	}
}

func prepareRouteViewLocal(fam family.Family, prefix netip.Prefix, group *locrib.PathGroup) (routeViewSelectedChange, error) {
	if group.Best < 0 {
		return routeViewSelectedRemove{key: prefixKey{family: fam, prefix: prefix}}, nil
	}
	if group.Best >= len(group.Paths) {
		return routeViewSelectedRemove{key: prefixKey{family: fam, prefix: prefix}}, nil
	}
	best := group.Paths[group.Best]
	members := group.ForwardingMembers(best)
	hops := make([]locrib.NextHop, len(members))
	metadata := make([]projectionMemberMetadata, len(members))
	for i := range members {
		hops[i] = members[i].NextHop
		metadata[i] = projectionMemberMetadata{source: members[i].Source, labels: members[i].Labels, sid: members[i].SRv6SID}
	}
	return prepareRouteViewSelectedMetadata(&locrib.Change{Kind: locrib.ChangeAdd, Family: fam, Prefix: prefix, Best: best, ECMP: hops}, metadata)
}

func (capture *routeViewLocalCapture) inspect(key prefixKey) (routeViewSelectedChange, error) {
	var result routeViewSelectedChange = routeViewSelectedRemove{key: key}
	var err error
	capture.rib.Inspect(key.family, key.prefix, func(group locrib.PathGroup) {
		result, err = prepareRouteViewLocal(key.family, key.prefix, &group)
	})
	return result, err
}

// scan copies under authority locks, reconciles captured keys, and includes
// cached keys removed from authority. Forwarding MUST run after scan returns.
func (capture *routeViewLocalCapture) scan(ctx context.Context, state *sysRIB) ([]routeViewSelectedChange, error) {
	// Continuous overflow may require repeated complete scans. The scan owns
	// one finite lifetime across those attempts, rather than renewing per pass.
	ctx, cancel := context.WithTimeout(ctx, routeViewSnapshotLifetime)
	defer cancel()
	for {
		capture.overflow.Store(false)
		selected := make(map[prefixKey]routeViewSelectedChange)
		for _, fam := range capture.rib.Families() {
			var scanErr error
			capture.rib.Iterate(fam, func(prefix netip.Prefix, group locrib.PathGroup) bool {
				if scanErr = ctx.Err(); scanErr != nil {
					return false
				}
				var change routeViewSelectedChange
				change, scanErr = prepareRouteViewLocal(fam, prefix, &group)
				if scanErr != nil {
					return false
				}
				selected[change.selectedKey()] = change
				return true
			})
			if scanErr != nil {
				return nil, scanErr
			}
		}
		for _, key := range capture.takeKeys(nil) {
			change, err := capture.inspect(key)
			if err != nil {
				return nil, err
			}
			selected[key] = change
		}
		if capture.overflow.Load() {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			continue
		}
		state.mu.RLock()
		for key := range state.routes {
			if _, exists := selected[key]; !exists {
				selected[key] = routeViewSelectedRemove{key: key}
			}
		}
		state.mu.RUnlock()
		changes := make([]routeViewSelectedChange, 0, len(selected))
		for _, change := range selected {
			changes = append(changes, change)
		}
		return changes, ctx.Err()
	}
}

func (capture *routeViewLocalCapture) takeKeys(keys []prefixKey) []prefixKey {
	for len(keys) < rpc.RouteViewRecordsMax {
		select {
		case key := <-capture.keys:
			keys = append(keys, key)
		default:
			return keys
		}
	}
	return keys
}

func (worker *routeViewWorker) consumeLocal(ctx context.Context) error {
	for {
		if err := worker.serveTask(); err != nil {
			return err
		}
		if err := worker.replay(ctx); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-worker.client.wake:
			continue
		case task := <-worker.tasks:
			if err := task.perform(worker); err != nil {
				return err
			}
		case key := <-worker.local.keys:
			keys := worker.local.takeKeys([]prefixKey{key})
			if err := worker.applyLocal(ctx, keys); err != nil {
				return err
			}
		}
	}
}

// applyLocal consumes captured authority changes before live output or replay.
// Its caller MUST bound keys through takeKeys; overflow uses the bounded scan.
func (worker *routeViewWorker) applyLocal(ctx context.Context, keys []prefixKey) error {
	if worker.local.overflow.Load() {
		worker.client.health.beginResync()
		changes, err := worker.local.scan(ctx, worker.client.rib)
		if err != nil {
			return err
		}
		worker.client.health.setPending(len(changes))
		return worker.apply(ctx, changes, nil)
	}
	if len(keys) == 0 {
		return nil
	}
	changes := make([]routeViewSelectedChange, 0, len(keys))
	for _, key := range keys {
		change, err := worker.local.inspect(key)
		if err != nil {
			return err
		}
		changes = append(changes, change)
	}
	return worker.apply(ctx, changes, nil)
}
