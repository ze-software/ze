// Design: docs/architecture/route-selection.md -- forked RIB metric selection
package rib

import (
	"context"
	"net/netip"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routeinstall"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// remoteRIB keeps metric reads and selected-route writes on the registered SDK
// transport. The bounded cache is cleared on every engine routing revision,
// including recursive-hop changes.
type remoteRIB struct {
	mu       sync.Mutex
	client   *sdk.Plugin
	sink     *routeinstall.Sink
	revision uint64
	costs    map[netip.Addr]igpcost.Distance
}

func (r *RIBManager) igpDistance(addr netip.Addr) igpcost.Distance {
	remote := r.forkRIB
	if remote == nil {
		return igpcost.Lookup(addr)
	}
	remote.mu.Lock()
	defer remote.mu.Unlock()
	if d, ok := remote.costs[addr]; ok {
		return d
	}
	out, err := remote.client.RouteMetrics(context.Background(), []string{addr.String()})
	if err != nil || len(out.Distances) != 1 {
		logger().Error("next-hop metric lookup failed", "next-hop", addr, "error", err)
		return igpcost.Distance{}
	}
	d := out.Distances[0]
	cost := igpcost.Distance{Cost: d.Cost, Resolved: d.Resolved, MissingAIGP: d.MissingAIGP}
	if len(remote.costs) >= 4096 {
		// Non-best candidates can churn next hops without changing the engine
		// revision. Bound their cache independently of retained route count.
		clear(remote.costs)
	}
	remote.costs[addr] = cost
	return cost
}

func (r *RIBManager) setupRemoteRIB() {
	r.forkRIB = &remoteRIB{
		client: r.plugin,
		sink:   routeinstall.New(context.Background(), r.plugin),
		costs:  make(map[netip.Addr]igpcost.Distance),
	}
}

func (r *RIBManager) runRemoteMetrics(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			out, err := r.plugin.RouteMetrics(ctx, nil)
			if err != nil {
				logger().Error("mandatory next-hop metric feed failed", "error", err)
				_ = r.plugin.Close()
				return
			}
			r.forkRIB.mu.Lock()
			changed := out.Revision != r.forkRIB.revision
			if changed {
				r.forkRIB.revision = out.Revision
				clear(r.forkRIB.costs)
			}
			r.forkRIB.mu.Unlock()
			if changed {
				r.reselectAIGPRoutes()
			}
		}
	}
}

func (r *RIBManager) removeLocRIB(fam family.Family, prefix netip.Prefix, pathID uint32) {
	if r.locRIB != nil {
		r.locRIB.Remove(fam, prefix, bgpProtocolID, pathID)
	} else if remote := r.forkRIB; remote != nil {
		remote.mu.Lock()
		defer remote.mu.Unlock()
		remote.sink.Remove(fam, prefix, bgpProtocolID, pathID)
		remote.sink.Flush()
	}
}

func (r *RIBManager) insertLocRIB(fam family.Family, prefix netip.Prefix, path locrib.Path, forward locrib.ForwardHandle) {
	if r.locRIB != nil {
		r.locRIB.InsertForward(fam, prefix, path, forward)
	} else if remote := r.forkRIB; remote != nil {
		remote.mu.Lock()
		defer remote.mu.Unlock()
		remote.sink.InsertForward(fam, prefix, path)
		remote.sink.Flush()
	}
}
