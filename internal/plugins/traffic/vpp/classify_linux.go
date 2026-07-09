// Design: plan/spec-followup-vpp-traffic.md -- classify + policer-classify
// pipeline for protocol filters. Reuses the proven firewall/vpp classify
// shape (table -> session -> interface bind) behind trafficvpp's ops seam.

//go:build linux

package trafficvpp

import (
	"fmt"

	"go.fd.io/govpp/binapi/interface_types"

	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
)

// noTable is VPP's sentinel for "no classify table" in a
// PolicerClassifySetInterface family slot.
const noTable = ^uint32(0)

// classifyBinding records the ip4/ip6 classify tables bound to one
// interface's policer-classify feature. VPP classify tables are anonymous
// (unlike named policers), so this in-memory tracker is the ONLY handle the
// backend has to unbind + delete them on a later Apply. That is also why
// startup orphan cleanup cannot reclaim classify tables by name the way it
// reclaims policers (documented in cleanupStartupOrphans); a same-process
// reconcile is the reclaim path.
type classifyBinding struct {
	ip4TableIdx uint32 // noTable if the class had no IPv4-matching filter
	ip6TableIdx uint32 // noTable if the class had no IPv6-matching filter
	policerIdx  uint32 // policer steered to; deleted on teardown
	policerName string // for logging + startup-orphan reconciliation
}

// hasProtocolFilter reports whether a class carries any protocol filter,
// which switches its policer from the egress policer-output binding to the
// ingress policer-classify pipeline (only matching traffic is policed).
func hasProtocolFilter(cls traffic.TrafficClass) bool {
	for _, f := range cls.Filters {
		if f.Type == traffic.FilterProtocol {
			return true
		}
	}
	return false
}

// applyClassProtocolFilters programs the classify + policer-classify pipeline
// so that only packets matching the class's protocol filters are steered to
// its policer (policerIdx). A protocol filter matches its protocol in BOTH
// address families (netlink parity: each `filter protocol` produces an IPv4
// and an IPv6 selector), so one ip4 table and one ip6 table are created, each
// with one session per protocol value, and both are bound in a single
// PolicerClassifySetInterface call.
//
// On any step failure the caller's undo list already holds the reversals for
// steps that succeeded; this function appends an undo closure immediately
// after each successful side effect and returns the error so applyInterface
// aborts. The returned binding is recorded by the caller for later reconcile.
func applyClassProtocolFilters(
	ops vppOps,
	swIfIndex interface_types.InterfaceIndex,
	policerIdx uint32,
	policerName string,
	cls traffic.TrafficClass,
	undo *[]func(),
) (classifyBinding, error) {
	binding := classifyBinding{
		ip4TableIdx: noTable,
		ip6TableIdx: noTable,
		policerIdx:  policerIdx,
		policerName: policerName,
	}

	var protos []uint8
	for _, f := range cls.Filters {
		if f.Type != traffic.FilterProtocol {
			continue
		}
		if f.Value > maxProtocolValue {
			return classifyBinding{}, fmt.Errorf("protocol %d out of range (0-%d)", f.Value, maxProtocolValue)
		}
		protos = append(protos, uint8(f.Value))
	}
	if len(protos) == 0 {
		return classifyBinding{}, fmt.Errorf("class %q: no protocol filters to program", cls.Name)
	}

	ip4Idx, err := buildProtocolTable(ops, classifyIPv4, policerIdx, protos, undo)
	if err != nil {
		return classifyBinding{}, err
	}
	binding.ip4TableIdx = ip4Idx

	ip6Idx, err := buildProtocolTable(ops, classifyIPv6, policerIdx, protos, undo)
	if err != nil {
		return classifyBinding{}, err
	}
	binding.ip6TableIdx = ip6Idx

	if err := ops.policerClassifySetInterface(swIfIndex, binding.ip4TableIdx, binding.ip6TableIdx, true); err != nil {
		return classifyBinding{}, fmt.Errorf("policer classify bind: %w", err)
	}
	boundIf, b4, b6 := swIfIndex, binding.ip4TableIdx, binding.ip6TableIdx
	*undo = append(*undo, func() {
		_ = ops.policerClassifySetInterface(boundIf, b4, b6, false)
	})

	return binding, nil
}

// buildProtocolTable creates one classify table for a family and adds a
// session per protocol value, each steering to policerIdx. It appends an undo
// closure for the table and every session so a later failure unwinds cleanly.
func buildProtocolTable(
	ops vppOps,
	fam classifyFamily,
	policerIdx uint32,
	protos []uint8,
	undo *[]func(),
) (uint32, error) {
	mask, _ := protocolClassifyVectors(fam, 0)
	tableIdx, err := ops.classifyAddDelTable(noTable, mask, classifySkipVectors, true)
	if err != nil {
		return 0, fmt.Errorf("classify table (%s): %w", familyName(fam), err)
	}
	*undo = append(*undo, func() { _, _ = ops.classifyAddDelTable(tableIdx, mask, classifySkipVectors, false) })

	for _, proto := range protos {
		_, match := protocolClassifyVectors(fam, proto)
		if err := ops.classifyAddDelSession(tableIdx, policerIdx, match, true); err != nil {
			return 0, fmt.Errorf("classify session (%s proto %d): %w", familyName(fam), proto, err)
		}
	}
	return tableIdx, nil
}

// reconcileClassifyRemovals tears down classify tables from the previous
// Apply that the new desired state no longer keeps. Because a filtered class
// always creates FRESH tables (they are anonymous, no upsert), every previous
// binding is stale after a successful re-apply:
//
//   - Interface still has a (new) classify binding: policerClassifySetInterface
//     already repointed the interface at the new tables, so the previous
//     tables are orphaned. Delete them (never unbind -- that would drop the
//     live new binding). A table index still present in the new binding
//     (VPP index reuse) is skipped so a live table is never deleted.
//   - Interface no longer classified (class lost its filter, or interface
//     removed): unbind (if the interface is still in VPP) then delete.
//
// Delete failures are logged and swallowed (VPP-side staleness after a restart
// must not fail the whole Apply), matching reconcileRemovals' policer path.
//
// Called with b.mu held.
func (b *backend) reconcileClassifyRemovals(
	ops vppOps,
	nameIndex map[string]interface_types.InterfaceIndex,
	newClassifyBindings map[string]classifyBinding,
) {
	lg := logger()
	for ifaceName, prev := range b.interfaceClassifyBindings {
		newB, stillBound := newClassifyBindings[ifaceName]
		swIfIndex, ifacePresent := nameIndex[ifaceName]

		if !stillBound {
			if ifacePresent {
				if err := ops.policerClassifySetInterface(swIfIndex, prev.ip4TableIdx, prev.ip6TableIdx, false); err != nil {
					lg.Warn("traffic-vpp: unbind stale classify failed (treating as already gone)",
						"iface", ifaceName, "err", err)
				}
			}
			deleteClassifyTables(ops, prev, classifyBinding{ip4TableIdx: noTable, ip6TableIdx: noTable}, lg, ifaceName)
			// The classify-bound policer is tracked here (not in
			// interfaceOutputPolicers), so delete it here too. Warn-and-continue
			// on failure like the policer reconcile path.
			if err := ops.policerDel(prev.policerIdx); err != nil {
				lg.Warn("traffic-vpp: delete stale classify policer failed (treating as already gone)",
					"iface", ifaceName, "policer", prev.policerName, "idx", prev.policerIdx, "err", err)
			}
			continue
		}
		// Replaced binding: delete only the previous tables that the new
		// binding does not reuse; the interface already points at the new ones.
		deleteClassifyTables(ops, prev, newB, lg, ifaceName)
	}
}

// deleteClassifyTables deletes prev's ip4/ip6 tables, skipping any index that
// keep reuses (so a live table is never deleted). Failures are logged.
func deleteClassifyTables(ops vppOps, prev, keep classifyBinding, lg logWarner, ifaceName string) {
	if prev.ip4TableIdx != noTable && prev.ip4TableIdx != keep.ip4TableIdx {
		if _, err := ops.classifyAddDelTable(prev.ip4TableIdx, nil, classifySkipVectors, false); err != nil {
			lg.Warn("traffic-vpp: delete stale classify table failed (treating as already gone)",
				"iface", ifaceName, "table", prev.ip4TableIdx, "family", "ip4", "err", err)
		}
	}
	if prev.ip6TableIdx != noTable && prev.ip6TableIdx != keep.ip6TableIdx {
		if _, err := ops.classifyAddDelTable(prev.ip6TableIdx, nil, classifySkipVectors, false); err != nil {
			lg.Warn("traffic-vpp: delete stale classify table failed (treating as already gone)",
				"iface", ifaceName, "table", prev.ip6TableIdx, "family", "ip6", "err", err)
		}
	}
}

// logWarner is the tiny slice of *slog.Logger the reconcile path uses, kept as
// an interface so the free function stays testable without a real logger.
type logWarner interface {
	Warn(msg string, args ...any)
}

func familyName(fam classifyFamily) string {
	if fam == classifyIPv6 {
		return "ip6"
	}
	return "ip4"
}
