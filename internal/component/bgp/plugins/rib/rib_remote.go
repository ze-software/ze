// Design: docs/architecture/route-selection.md -- forked RIB metric selection
package rib

import (
	"context"
	"net/netip"
	"slices"
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
//
// installed holds the last path sent for each route, so a re-mirror of an
// unchanged best path costs no engine round trip. The in-process Loc-RIB drops
// that no-op through Path.Equal, and the forked mirror MUST drop it the same
// way. Its size is bounded by the selected routes installed in the engine.
type remoteRIB struct {
	mu        sync.Mutex
	client    *sdk.Plugin
	sink      *routeinstall.Sink
	revision  uint64
	costs     map[netip.Addr]igpcost.Distance
	installed map[remoteRoute]locrib.Path
}

// remoteUnchanged reports a re-mirror the engine would treat as a no-op. It
// is locrib's own test, Path.Equal plus the equal-cost set, and it adds the
// eBGP class that Equal leaves out: sysrib keys its distance override on that
// class, and a new best from the other class can carry the same distance.
func remoteUnchanged(sent, path *locrib.Path) bool {
	return sent.Equal(*path) && sent.IsEBGP == path.IsEBGP && slices.Equal(sent.ECMP, path.ECMP)
}

// remoteRoute names one engine Loc-RIB entry this plugin owns: the source is
// always bgpProtocolID, so the family, the prefix and the instance identify it.
type remoteRoute struct {
	fam      family.Family
	prefix   netip.Prefix
	instance uint32
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
		client:    r.plugin,
		sink:      routeinstall.New(context.Background(), r.plugin),
		costs:     make(map[netip.Addr]igpcost.Distance),
		installed: make(map[remoteRoute]locrib.Path),
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
		delete(remote.installed, remoteRoute{fam: fam, prefix: prefix, instance: pathID})
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
		key := remoteRoute{fam: fam, prefix: prefix, instance: path.Instance}
		if sent, ok := remote.installed[key]; ok && remoteUnchanged(&sent, &path) {
			return
		}
		// A failed flush closes the client and the engine withdraws every
		// route this plugin owns, so recording before the flush cannot leave a
		// route the engine lacks marked as sent.
		remote.installed[key] = path
		remote.sink.InsertForward(fam, prefix, path)
		remote.sink.Flush()
	}
}
