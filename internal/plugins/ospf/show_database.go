// Design: plan/learned/967-ospf-13-cli-diag-interop.md -- `show ospf database <type>` subviews.
// Each subview filters the LSDB snapshot to one LS Type (RFC 2328 / RFC 3101 Type 7).
// RFC: rfc/short/rfc2328.md (LSA types 1-5), rfc/short/rfc3101.md (Type 7 NSSA)

package ospf

import ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"

// dbSubviewType maps a `show ospf database <type>` command to the LSASnapshot.Type
// string it filters to (types.LSType.String()).
var dbSubviewType = map[string]string{
	"show ospf database router":        "router",          // Type 1
	"show ospf database network":       "network",         // Type 2
	"show ospf database summary":       "summary-network", // Type 3
	"show ospf database asbr-summary":  "summary-asbr",    // Type 4
	"show ospf database external":      "as-external",     // Type 5
	"show ospf database nssa-external": "nssa",            // Type 7
	"show ospf database opaque-link":   "opaque-link",     // Type 9 (RFC 5250)
	"show ospf database opaque-area":   "opaque-area",     // Type 10 (RFC 5250)
	"show ospf database opaque-as":     "opaque-as",       // Type 11 (RFC 5250)
}

// databaseSnapshotByType renders the LSDB snapshot filtered to a single LS Type. It
// filters the per-area store (router/network/summary/asbr-summary/nssa and the Type-10
// opaque-area LSAs), the AS-wide Type-5 store (as-external), the AS-wide Type-11 opaque
// store (opaque-as), and the per-interface link store (Type-9 opaque-link), mirroring
// databaseSnapshot's single-element wrapping.
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
	links := make([]ospflsdb.LinkSnapshot, 0, len(full.Links))
	for _, link := range full.Links {
		if kept := filterLSAsByType(link.LSAs, lsType); len(kept) > 0 {
			links = append(links, ospflsdb.LinkSnapshot{Interface: link.Interface, LSAs: kept})
		}
	}
	return []any{ospflsdb.Snapshot{
		Areas:      areas,
		ASExternal: filterLSAsByType(full.ASExternal, lsType),
		ASOpaque:   filterLSAsByType(full.ASOpaque, lsType),
		Links:      links,
	}}
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
