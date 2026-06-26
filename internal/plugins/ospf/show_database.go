// Design: plan/learned/967-ospf-13-cli-diag-interop.md -- `show ospf database <type>` subviews.
// Each subview filters the LSDB snapshot to one LS Type (RFC 2328 / RFC 3101 Type 7).
// RFC: rfc/short/rfc2328.md (LSA types 1-5), rfc/short/rfc3101.md (Type 7 NSSA)

package ospf

import ospflsdb "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/lsdb"

// dbSubviewType maps a `show ospf database <type>` command to the LSASnapshot.Type
// string it filters to (types.LSType.String()).
var dbSubviewType = map[string]string{
	"show ospf database router":        "router",          // Type 1
	"show ospf database network":       "network",         // Type 2
	"show ospf database summary":       "summary-network", // Type 3
	"show ospf database asbr-summary":  "summary-asbr",    // Type 4
	"show ospf database external":      "as-external",     // Type 5
	"show ospf database nssa-external": "nssa",            // Type 7
}

// databaseSnapshotByType renders the LSDB snapshot filtered to a single LS Type. The
// per-area LSAs (router/network/summary/asbr-summary/nssa) and the AS-wide store
// (as-external) are both filtered, mirroring databaseSnapshot's single-element wrapping.
func (e *engine) databaseSnapshotByType(lsType string) []any {
	if e.lsdb == nil {
		return nil
	}
	full := e.lsdb.Snapshot()
	areas := make([]ospflsdb.AreaSnapshot, 0, len(full.Areas))
	for _, area := range full.Areas {
		if kept := filterLSAsByType(area.LSAs, lsType); len(kept) > 0 {
			areas = append(areas, ospflsdb.AreaSnapshot{Area: area.Area, LSAs: kept})
		}
	}
	return []any{ospflsdb.Snapshot{Areas: areas, ASExternal: filterLSAsByType(full.ASExternal, lsType)}}
}

func filterLSAsByType(lsas []ospflsdb.LSASnapshot, lsType string) []ospflsdb.LSASnapshot {
	out := make([]ospflsdb.LSASnapshot, 0, len(lsas))
	for i := range lsas {
		if lsas[i].Type == lsType {
			out = append(out, lsas[i])
		}
	}
	return out
}
