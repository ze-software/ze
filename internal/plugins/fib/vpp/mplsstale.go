// Design: docs/architecture/mpls/mpls-kernel.md -- label-switching entries on the VPP data plane
// Related: fibvpp.go -- requestLabelReplay marks, run sweeps after labelSweepDelay
// Related: doctor_label.go -- the fib-vpp-label-entries check reports what the sweep could not remove
//
// VPP keeps its FIB when Ze stops, so the label routes a previous fib-vpp
// writer installed in MPLS table 0 keep forwarding after a Ze restart, and no
// source remembers them. A new writer reads table 0 when it opens, before it
// asks the sources for a replay, and once the replay window has passed it
// removes every leftover no source re-claimed. This is fib-kernel's
// stale-mark-then-sweep (startupSweep, sweepStale) applied to the label
// table. VPP carries no owner mark on a label route, so every unicast route
// with an unreserved in-label counts as Ze's: fib-vpp is the only writer of
// MPLS table 0 on the VPP Ze supervises.
package fibvpp

import (
	"slices"
	"time"
)

// labelSweepDelay is how long the sources have to answer the replay before a
// leftover label route is removed. It is fib-kernel's sweepDelay: a source
// that answers later re-installs the entry through its own retry path.
const labelSweepDelay = 30 * time.Second

// markStaleLabelsLocked records what MPLS table 0 holds that this writer did
// not install. A table that cannot be read is recorded for the doctor check:
// the leftovers it may hold are then neither removed nor known. Caller holds
// f.mu, and MPLS table 0 exists.
func (f *fibVPP) markStaleLabelsLocked() {
	positions, err := f.mplsBackend.dumpLabelRoutes()
	if err != nil {
		f.labelDumpErr = err
		logger().Error("fib-vpp: MPLS table 0 could not be read, so label routes a previous run left in VPP "+
			"are not removed; run 'ze show doctor' for the fix", "error", err, "code", codeStaleLabel)
		return
	}
	f.labelDumpErr = nil
	f.labelStale = f.labelStale[:0]
	for _, p := range positions {
		if _, owned := f.labelOwners[p.inLabel]; owned {
			continue
		}
		f.labelStale = append(f.labelStale, p)
	}
	logger().Info("fib-vpp: label routes left in MPLS table 0", "count", len(f.labelStale))
}

// sweepStaleLabels removes every leftover label route whose in-label no source
// re-claimed since markStaleLabelsLocked ran (reclaimStaleLabelLocked drops a
// re-claimed one; the owner check covers an in-label installed before the
// mark). A route VPP refuses to remove is kept for the doctor check. Safe for
// concurrent use: it takes f.mu.
func (f *fibVPP) sweepStaleLabels() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.retired {
		return
	}
	removed := 0
	for _, p := range f.labelStale {
		if _, owned := f.labelOwners[p.inLabel]; owned {
			continue
		}
		if err := f.mplsBackend.delLabelRoute(p); err != nil {
			if f.labelSweepFailures == nil {
				f.labelSweepFailures = make(map[labelPosition]error)
			}
			f.labelSweepFailures[p] = err
			logger().Error("fib-vpp: a label route a previous run left in VPP could not be removed and "+
				"still forwards; run 'ze show doctor' for the fix", "in-label", p.inLabel, "eos", p.eos,
				"error", err, "code", codeStaleLabel)
			continue
		}
		delete(f.labelSweepFailures, p)
		removed++
	}
	f.labelStale = nil
	logger().Info("fib-vpp: label routes left by a previous run removed", "count", removed)
}

// reclaimStaleLabelLocked forgets inLabel as a leftover once a source has
// installed it: the source's add rewrote both stack positions, so the route
// is the source's now, and a later removal by that source must not be
// followed by a sweep of a route that is already gone. Caller holds f.mu.
func (f *fibVPP) reclaimStaleLabelLocked(inLabel uint32) {
	f.labelStale = slices.DeleteFunc(f.labelStale, func(p labelPosition) bool { return p.inLabel == inLabel })
	for p := range f.labelSweepFailures {
		if p.inLabel == inLabel {
			delete(f.labelSweepFailures, p)
		}
	}
}

// staleLabelFailuresLocked lists, sorted, the leftover label routes VPP would
// not remove. Caller holds f.mu.
func (f *fibVPP) staleLabelFailuresLocked() []labelPosition {
	positions := make([]labelPosition, 0, len(f.labelSweepFailures))
	for p := range f.labelSweepFailures {
		positions = append(positions, p)
	}
	slices.SortFunc(positions, func(a, b labelPosition) int {
		if a.inLabel != b.inLabel {
			return int(a.inLabel) - int(b.inLabel)
		}
		return int(a.eos) - int(b.eos)
	})
	return positions
}
