// Design: docs/architecture/traffic/followup-vpp-traffic.md -- classify + policer-classify
// pipeline for steering filters (protocol + dscp). Reuses the proven
// firewall/vpp classify shape (table -> session -> interface bind) behind
// trafficvpp's ops seam, extended to multi-class steering (phase 6): all
// filtered classes on one interface share ONE table per distinct field mask
// (each class contributing its own sessions -> its own policer), and
// distinct-mask tables are chained via ClassifyAddDelTable.NextTableIndex.
// Real VPP v25.10 validated the dscp offsets, multi-session steering, and the
// NextTableIndex chain fall-through.

//go:build linux

package trafficvpp

import (
	"fmt"
	"sort"

	"go.fd.io/govpp/binapi/interface_types"

	"github.com/ze-software/ze/internal/component/traffic"
)

// noTable is VPP's sentinel for "no classify table" in a
// PolicerClassifySetInterface family slot and for "no chain successor" in a
// table's NextTableIndex.
const noTable = ^uint32(0)

// classifyBinding records the classify state for ONE interface's
// policer-classify feature: the per-family table CHAINS (head first; a miss on
// the head falls through to the next via NextTableIndex) and the filtered
// policers steered to. VPP classify tables are anonymous (unlike named
// policers), so this in-memory tracker is the ONLY handle the backend has to
// unbind + delete the tables on a later Apply. That is also why startup orphan
// cleanup cannot reclaim classify tables by name the way it reclaims policers
// (documented in cleanupStartupOrphans); a same-process reconcile is the
// reclaim path.
type classifyBinding struct {
	ip4Tables []uint32          // chain, head first; empty if no IPv4 steering
	ip6Tables []uint32          // chain, head first; empty if no IPv6 steering
	policers  map[string]uint32 // filtered policer name -> VPP index (for teardown)
}

// classifySteer is one (family, field-match) -> policer steering requirement,
// expanded from a class's filters. A single `filter protocol`/`filter dscp`
// produces one steer per address family (netlink parity: each filter selects in
// both IPv4 and IPv6).
type classifySteer struct {
	fam        classifyFamily
	mask       []byte
	match      []byte
	policerIdx uint32
}

// collectClassSteerings expands one class's steering filters into per-family
// (mask, match) -> policer requirements. Value bounds are the verifier's
// responsibility; filterClassifyVectors returns ok=false for non-steering
// filter types (mark), which are skipped defensively.
func collectClassSteerings(cls traffic.TrafficClass, policerIdx uint32) []classifySteer {
	var out []classifySteer
	for _, f := range cls.Filters {
		for _, fam := range []classifyFamily{classifyIPv4, classifyIPv6} {
			mask, match, ok := filterClassifyVectors(fam, f)
			if !ok {
				continue
			}
			out = append(out, classifySteer{fam: fam, mask: mask, match: match, policerIdx: policerIdx})
		}
	}
	return out
}

// applyInterfaceClassify programs the classify + policer-classify pipeline for
// every filtered class on one interface. Steerings from all filtered classes
// are grouped per family by field mask: one table per distinct mask carries a
// session per (value -> policer), and distinct-mask tables are chained via
// NextTableIndex (head bound; a miss falls through). Both family heads are bound
// in one PolicerClassifySetInterface call.
//
// On any step failure the caller's undo list already holds the reversals for
// the policers created before this call; this function appends an undo closure
// immediately after each successful side effect and returns the error so
// applyInterface aborts. The returned binding is recorded by the caller for
// later reconcile.
func applyInterfaceClassify(
	ops vppOps,
	swIfIndex interface_types.InterfaceIndex,
	steerings []classifySteer,
	policers map[string]uint32,
	undo *[]func(),
) (classifyBinding, error) {
	binding := classifyBinding{policers: policers}

	ip4Chain, err := buildFamilyChain(ops, classifyIPv4, steerings, undo)
	if err != nil {
		return classifyBinding{}, err
	}
	binding.ip4Tables = ip4Chain

	ip6Chain, err := buildFamilyChain(ops, classifyIPv6, steerings, undo)
	if err != nil {
		return classifyBinding{}, err
	}
	binding.ip6Tables = ip6Chain

	head4, head6 := headOrNoTable(ip4Chain), headOrNoTable(ip6Chain)
	if err := ops.policerClassifySetInterface(swIfIndex, head4, head6, true); err != nil {
		return classifyBinding{}, fmt.Errorf("policer classify bind: %w", err)
	}
	boundIf := swIfIndex
	*undo = append(*undo, func() {
		_ = ops.policerClassifySetInterface(boundIf, head4, head6, false)
	})

	return binding, nil
}

// headOrNoTable returns the chain head (first table) or noTable for an empty
// chain (that family has no steering filter).
func headOrNoTable(chain []uint32) uint32 {
	if len(chain) == 0 {
		return noTable
	}
	return chain[0]
}

// maskGroup collects the sessions that share one classify table (same field
// mask). matches[i] steers to policers[i].
type maskGroup struct {
	mask     []byte
	matches  [][]byte
	policers []uint32
}

// buildFamilyChain creates the classify-table chain for one address family from
// the interface's steerings. Steerings are grouped by mask (deterministically
// ordered by mask bytes so the chain -- and tests -- are stable); each group
// becomes one table with a session per match. Tables are created in REVERSE so
// each one's NextTableIndex can point at the already-created successor. Returns
// the chain head-first ([0] is bound; a miss falls through to [1], ...). An
// empty steering set for the family yields a nil chain.
func buildFamilyChain(
	ops vppOps,
	fam classifyFamily,
	steerings []classifySteer,
	undo *[]func(),
) ([]uint32, error) {
	groups := groupSteeringsByMask(fam, steerings)
	if len(groups) == 0 {
		return nil, nil
	}
	chain := make([]uint32, len(groups))
	next := noTable
	for i := len(groups) - 1; i >= 0; i-- {
		g := groups[i]
		tableIdx, err := ops.classifyAddDelTable(noTable, g.mask, classifySkipVectors, next, true)
		if err != nil {
			return nil, fmt.Errorf("classify table (%s): %w", familyName(fam), err)
		}
		created := tableIdx
		*undo = append(*undo, func() {
			if _, derr := ops.classifyAddDelTable(created, nil, classifySkipVectors, noTable, false); derr != nil {
				logger().Warn("traffic-vpp: undo classify table delete failed",
					"table", created, "err", derr)
			}
		})
		for j, match := range g.matches {
			if err := ops.classifyAddDelSession(tableIdx, g.policers[j], match, true); err != nil {
				return nil, fmt.Errorf("classify session (%s): %w", familyName(fam), err)
			}
		}
		chain[i] = tableIdx
		next = tableIdx
	}
	return chain, nil
}

// groupSteeringsByMask partitions a family's steerings by field mask, preserving
// one table per distinct mask. Groups are ordered deterministically by mask
// bytes so the emitted chain is stable across applies (and unit-testable).
// Within a group, sessions keep their steering order.
func groupSteeringsByMask(fam classifyFamily, steerings []classifySteer) []maskGroup {
	order := make([]string, 0)
	byMask := make(map[string]*maskGroup)
	for _, s := range steerings {
		if s.fam != fam {
			continue
		}
		key := string(s.mask)
		g, ok := byMask[key]
		if !ok {
			g = &maskGroup{mask: s.mask}
			byMask[key] = g
			order = append(order, key)
		}
		g.matches = append(g.matches, s.match)
		g.policers = append(g.policers, s.policerIdx)
	}
	sort.Strings(order)
	groups := make([]maskGroup, 0, len(order))
	for _, key := range order {
		groups = append(groups, *byMask[key])
	}
	return groups
}

// reconcileClassifyRemovals tears down classify state from the previous Apply
// that the new desired state no longer keeps. Filtered classes always create
// FRESH tables (anonymous, no upsert), so every previous table is stale after a
// successful re-apply:
//
//   - Interface still classified: applyInterfaceClassify already repointed the
//     policer-classify feature at the new chain heads, so the previous tables
//     are orphaned. Delete them (never unbind -- that would drop the live new
//     binding). Any previous policer whose class is gone from the new binding
//     (and not adopted by the egress-output path) is deleted too.
//   - Interface no longer classified (all filtered classes dropped, or interface
//     removed): unbind (if the interface is still in VPP) then delete every
//     table and every filtered policer.
//
// Delete failures are logged and swallowed (VPP-side staleness after a restart
// must not fail the whole Apply), matching reconcileRemovals' policer path.
//
// Called with b.mu held.
func (b *backend) reconcileClassifyRemovals(
	ops vppOps,
	nameIndex map[string]interface_types.InterfaceIndex,
	newClassifyBindings map[string]classifyBinding,
	newOutputPolicers map[string]map[string]uint32,
) {
	lg := logger()
	for ifaceName, prev := range b.interfaceClassifyBindings {
		newB, stillBound := newClassifyBindings[ifaceName]
		swIfIndex, ifacePresent := nameIndex[ifaceName]

		if !stillBound {
			if ifacePresent {
				if err := ops.policerClassifySetInterface(swIfIndex, headOrNoTable(prev.ip4Tables), headOrNoTable(prev.ip6Tables), false); err != nil {
					lg.Warn("traffic-vpp: unbind stale classify failed (treating as already gone)",
						"iface", ifaceName, "err", err)
				}
			}
			deleteClassifyTables(ops, prev, classifyBinding{}, lg, ifaceName)
			deleteClassifyPolicers(ops, prev.policers, nil, newOutputPolicers[ifaceName], lg, ifaceName)
			continue
		}
		// Replaced binding: the interface already points at the new chain heads.
		// Delete only the previous tables the new binding does not reuse, and
		// only the previous policers the new binding (or the output path) no
		// longer keeps.
		deleteClassifyTables(ops, prev, newB, lg, ifaceName)
		deleteClassifyPolicers(ops, prev.policers, newB.policers, newOutputPolicers[ifaceName], lg, ifaceName)
	}
}

// deleteClassifyTables deletes every table in prev's ip4/ip6 chains, skipping
// any index the keep binding reuses (so a live table is never deleted).
// Failures are logged.
func deleteClassifyTables(ops vppOps, prev, keep classifyBinding, lg logWarner, ifaceName string) {
	keepIdx := make(map[uint32]bool)
	for _, idx := range keep.ip4Tables {
		keepIdx[idx] = true
	}
	for _, idx := range keep.ip6Tables {
		keepIdx[idx] = true
	}
	del := func(idx uint32, family string) {
		if keepIdx[idx] {
			return
		}
		if _, err := ops.classifyAddDelTable(idx, nil, classifySkipVectors, noTable, false); err != nil {
			lg.Warn("traffic-vpp: delete stale classify table failed (treating as already gone)",
				"iface", ifaceName, "table", idx, "family", family, "err", err)
		}
	}
	for _, idx := range prev.ip4Tables {
		del(idx, "ip4")
	}
	for _, idx := range prev.ip6Tables {
		del(idx, "ip6")
	}
}

// deleteClassifyPolicers deletes filtered policers present in prev but absent
// from both the new classify binding (keepClassify) and the new egress-output
// path (keepOutput -- a class that migrated from filtered to unfiltered keeps
// its policer, now output-bound). Failures are logged and swallowed like the
// output policer reconcile.
func deleteClassifyPolicers(ops vppOps, prev, keepClassify, keepOutput map[string]uint32, lg logWarner, ifaceName string) {
	for name, idx := range prev {
		if _, keep := keepClassify[name]; keep {
			continue
		}
		if _, keep := keepOutput[name]; keep {
			continue
		}
		if err := ops.policerDel(idx); err != nil {
			lg.Warn("traffic-vpp: delete stale classify policer failed (treating as already gone)",
				"iface", ifaceName, "policer", name, "idx", idx, "err", err)
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
