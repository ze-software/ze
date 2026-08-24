// Design: docs/architecture/firewall/table-ownership-and-shutdown-flush.md -- table ownership
// Related: registry.go -- RegisterTables refuses a name without the prefix
// Related: config.go -- tableNamePrefix, the prefix every owner now carries

package firewall

import (
	"slices"
	"sync/atomic"
)

// legacyTable is one kernel table name ze wrote before every owner carried
// tableNamePrefix, and the shape that identifies it as ours.
type legacyTable struct {
	// families are the address families the producer wrote this name in. A
	// table under the same name in any other family belongs to somebody else.
	families []TableFamily
	// chains are every chain name the producer wrote, and the only ones it
	// wrote. A kernel table under the same name holding any other chain
	// belongs to somebody else.
	chains []string
}

// legacyTables are kernel tables ze wrote before every owner carried
// tableNamePrefix. A backend decides which kernel tables ze owns by that
// prefix, so a table without it survived every reconcile and every withdraw:
// its rules kept enforcing after the config or the route that asked for them
// was gone, and each reconcile appended a second copy of every rule.
//
// Four producers shipped a bare name and each one is here: copp, ddos-local,
// the FlowSpec bridge and the anomaly-shape responder. The other three carried
// the prefix from their first commit, so this map is the whole population
// rather than a sample of it: policy-routes (ze_pr), firewall-irr
// (ze_irr_iface) and the firewall engine (tableNamePrefix).
//
// A router upgraded from such a build still holds the old table, and renaming
// the producer does not reach it. The nft backend's sweep deletes each entry
// once, on the first reconcile of the process, so the upgrade strands nothing
// and a table written later under one of these names is left alone. See
// legacySweepPending below.
//
// Remove an entry when no supported upgrade path starts from a build that
// wrote it, and remove this file when the map is empty.
var legacyTables = map[string]legacyTable{
	// internal/plugins/copp/translate.go, now ze_copp. translatePolicy wrote
	// one table with one chain, in one family.
	"copp": {families: []TableFamily{FamilyInet}, chains: []string{"input"}},
	// internal/plugins/ddos/local/responder.go, now ze_ddos-local. The family
	// follows the victim prefix (familyFromPrefix) and the chain follows the
	// hook (hookChainName), so an upgraded box can hold this name in either
	// family with either chain.
	"ddos-local": {families: []TableFamily{FamilyIP, FamilyIP6}, chains: []string{"forward", "ingress"}},
	// internal/plugins/flowspec-firewall/state.go, now ze_flowspec
	"flowspec": {families: []TableFamily{FamilyInet}, chains: []string{"flowspec-fwd", "flowspec-in"}},
	// internal/plugins/anomaly/shape/match.go, now ze_anomaly-shape
	"anomaly-shape": {families: []TableFamily{FamilyIP}, chains: []string{"ingress"}},
	// internal/plugins/anomaly/shape/match.go, now ze_anomaly-shape6
	"anomaly-shape6": {families: []TableFamily{FamilyIP6}, chains: []string{"ingress"}},
}

// IsLegacyTableName reports whether a kernel table's name and family match a
// producer ze used to have. It is a PRE-FILTER for IsLegacyTable, never the
// decision: it says nothing about who wrote the table.
//
// It exists because reading a table's chains costs a netlink round trip, and
// the sweep runs inside Backend.Apply with the process-wide reconcile lock
// held. This keeps that round trip off every kernel table and pays it only for
// the handful of names ze ever wrote bare.
func IsLegacyTableName(name string, family TableFamily) bool {
	want, ok := legacyTables[name]
	return ok && slices.Contains(want.families, family)
}

// IsLegacyTable reports whether a kernel table is one ze wrote under a name
// that carries no ownership prefix. A backend deletes such a table on the
// first reconcile of the process, and logs that it did.
//
// The name and the family are NOT enough to decide this. The names ze used are
// ordinary words, and another tool that programs nftables can use the same
// one; deleting that tool's table would be a worse failure than the stale rule
// this removal exists to clear. So every chain the kernel table holds must be
// one this producer wrote, and a table holding no chain at all is not ours
// either: each of these producers wrote its table with its chains in one
// atomic flush, so the empty shape is one none of them ever produced.
//
// The chain test reads NAMES and nothing else. It does not read the hook, the
// type or the priority, so a foreign table whose chains all happen to be named
// as ze named its own still matches. legacySweepPending is what bounds that
// residual: the removal looks once, on the first reconcile of the process, so
// only a table already in the kernel when ze started is ever a candidate.
func IsLegacyTable(name string, family TableFamily, chains []string) bool {
	want, ok := legacyTables[name]
	if !ok || !slices.Contains(want.families, family) || len(chains) == 0 {
		return false
	}
	for _, chain := range chains {
		if !slices.Contains(want.chains, chain) {
			return false
		}
	}
	return true
}

// legacySweepPending is true while the one-time removal above has not yet
// reached a backend in this process. It is what makes the removal a MIGRATION
// with an end rather than a standing deletion policy, and two places read it.
//
// Backend.Apply reads it before it looks for a legacy table at all, so the
// removal runs on the first reconcile of the process and on no later one. A
// table that appears under a legacy name after that was written while ze was
// running, and no current producer can write one: RegisterTables refuses a
// name without the prefix. So it belongs to somebody else, and ze leaves it.
//
// ApplyAll reads it for the ONE case where reconciling an EMPTY desired set is
// not a no-op. A box that holds a legacy table, has no firewall {} section,
// and has no owner registering anything never loads a backend at all: ApplyAll
// returns before it reaches one (registry.go), so nothing can ever delete that
// table and the rules an older build installed enforce for the life of the
// box. That is the upgrade this removal exists for, so the empty set has to
// reach a backend exactly once.
//
// It starts false when the map above is empty, so deleting the last entry
// restores the pre-migration behavior with no other edit.
//
// Safe for concurrent use.
var legacySweepPending atomic.Bool

func init() { //nolint:gochecknoinits // derived from the map above, which is a var
	legacySweepPending.Store(len(legacyTables) > 0)
}

// LegacySweepPending reports whether the one-time removal still has to reach a
// backend in this process. A backend reads it to decide whether to look for a
// legacy table at all, and a caller that has no table to register reads it to
// decide whether an empty reconcile is worth running.
func LegacySweepPending() bool { return legacySweepPending.Load() }

// legacySweepReached records that a reconcile got as far as a backend, which
// is where the removal happens. Called on every successful Apply, so both the
// empty-set exemption and the removal itself last for one reconcile rather
// than for the life of the process.
func legacySweepReached() { legacySweepPending.Store(false) }
