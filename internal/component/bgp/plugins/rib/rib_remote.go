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
	"github.com/ze-software/ze/pkg/plugin/rpc"
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
	mu                  sync.Mutex
	client              *sdk.Plugin
	sink                *routeinstall.Sink
	revision            uint64
	costs               map[netip.Addr]igpcost.Distance
	prospective         map[prospectiveMetricKey]igpcost.Distance
	installed           map[remoteRoute]locrib.Path
	metricFailed        bool
	metricIssueRevision uint64
	metricIssueSent     uint64
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
	ctx, cancel := context.WithTimeout(context.Background(), nextHopMetricTimeout)
	defer cancel()
	out, err := remote.client.RouteMetrics(ctx, []string{addr.String()})
	if err != nil {
		remote.noteMetricFailure(true)
		logger().Error(remoteMetricProblem, "code", codeRemoteMetricFailure, "next-hop", addr, "error", err)
		return igpcost.Distance{}
	}
	remote.noteMetricFailure(false)
	d := out.Distances[0]
	cost := igpcost.Distance{Cost: d.Cost, Resolved: d.Resolved, Recursive: d.Recursive, MissingAIGP: d.MissingAIGP}
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
			r.reportRemoteMetricIssue(ctx)
			poll, cancel := context.WithTimeout(ctx, nextHopMetricTimeout)
			out, err := r.plugin.RouteMetrics(poll, nil)
			cancel()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				logger().Error(remoteMetricProblem, "code", codeRemoteMetricFailure, "error", err)
				// Preserve the mandatory feed's fatal lifecycle, but retain an error
				// event before Close removes this process's active warnings.
				r.reportRemoteMetricFailure(ctx)
				_ = r.plugin.Close()
				return
			}
			r.forkRIB.mu.Lock()
			changed := out.Revision != r.forkRIB.revision
			if r.forkRIB.metricFailed {
				// A failed admission was deliberately not cached. Retry retained
				// paths even when the routing table itself has not changed.
				changed = true
			}
			r.forkRIB.noteMetricFailure(false)
			if changed {
				r.forkRIB.revision = out.Revision
				clear(r.forkRIB.costs)
				clear(r.forkRIB.prospective)
			}
			r.forkRIB.mu.Unlock()
			if changed {
				r.reselectAIGPRoutes()
			}
			r.reportRemoteMetricIssue(ctx)
		}
	}
}

const codeRemoteMetricFailure = "doctor-bgp-next-hop-metrics"

const remoteMetricProblem = "The BGP RIB could not read next-hop reachability from the engine; affected paths cannot be selected. Inspect the plugin connection with ze show doctor."

// noteMetricFailure coalesces failures into one bounded warning. The caller
// MUST hold remote.mu; only the existing metric worker sends report RPCs.
func (remote *remoteRIB) noteMetricFailure(failed bool) {
	if remote.metricFailed == failed {
		return
	}
	remote.metricFailed = failed
	remote.metricIssueRevision++
}

// reportRemoteMetricIssue sends a snapshot outside peerMu and remote.mu. A
// concurrent transition remains pending for the worker's next tick.
func (r *RIBManager) reportRemoteMetricIssue(ctx context.Context) {
	remote := r.forkRIB
	remote.mu.Lock()
	revision := remote.metricIssueRevision
	failed := remote.metricFailed
	pending := revision != remote.metricIssueSent
	remote.mu.Unlock()
	if !pending {
		return
	}
	reportCtx, cancel := context.WithTimeout(ctx, nextHopMetricTimeout)
	defer cancel()
	var err error
	if failed {
		err = r.plugin.RaiseIssue(reportCtx, sdk.Issue{Severity: rpc.ReportWarning,
			Code: codeRemoteMetricFailure, Subject: "bgp-rib", Message: remoteMetricProblem, Fix: "ze show doctor"})
	} else {
		err = r.plugin.ClearIssue(reportCtx, codeRemoteMetricFailure, "bgp-rib")
	}
	if err != nil {
		logger().Error("The BGP RIB could not report its next-hop lookup problem; inspect the plugin connection with ze show doctor.",
			"code", codeRemoteMetricFailure, "error", err)
		return
	}
	remote.mu.Lock()
	remote.metricIssueSent = revision
	remote.mu.Unlock()
}

func (r *RIBManager) reportRemoteMetricFailure(ctx context.Context) {
	reportCtx, cancel := context.WithTimeout(ctx, nextHopMetricTimeout)
	defer cancel()
	if err := r.plugin.RaiseIssue(reportCtx, sdk.Issue{Severity: rpc.ReportError,
		Code: codeRemoteMetricFailure, Subject: "bgp-rib", Message: remoteMetricProblem, Fix: "ze show doctor"}); err != nil {
		logger().Error("The BGP RIB could not report its failed metric feed; inspect the plugin connection with ze show doctor.",
			"code", codeRemoteMetricFailure, "error", err)
	}
}

func (r *RIBManager) removeLocRIB(fam family.Family, prefix netip.Prefix) {
	if loc := r.locRIB.Load(); loc != nil {
		loc.Remove(fam, prefix, bgpProtocolID, bgpLocRIBInstance)
	} else if remote := r.forkRIB; remote != nil {
		remote.mu.Lock()
		defer remote.mu.Unlock()
		delete(remote.installed, remoteRoute{fam: fam, prefix: prefix, instance: bgpLocRIBInstance})
		remote.sink.Remove(fam, prefix, bgpProtocolID, bgpLocRIBInstance)
		remote.sink.Flush()
	}
}

func (r *RIBManager) insertLocRIB(fam family.Family, prefix netip.Prefix, path locrib.Path, forward locrib.ForwardHandle) {
	if loc := r.locRIB.Load(); loc != nil {
		loc.InsertForward(fam, prefix, path, forward)
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
