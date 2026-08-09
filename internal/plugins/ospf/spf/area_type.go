// Design: docs/architecture/ospf/ospf-11-stub-nssa.md -- stub/NSSA Summary-LSA origination policy.
// RFC: rfc/short/rfc2328.md -- sec 3.6 stub areas (Type 3 default, no Type 4/5)

package spf

import "github.com/ze-software/ze/internal/plugins/ospf/types"

// Area-type policy strings (mirrors the lsdb AreaType* constants and the config enum).
const (
	AreaTypeNormal = "normal"
	AreaTypeStub   = "stub"
	AreaTypeNSSA   = "nssa"
)

// AreaSummaryPolicy carries the per-destination-area stub/NSSA origination policy the
// ABR applies on top of the normal Type 3/4 desired set.
type AreaSummaryPolicy struct {
	Type        string // normal | stub | nssa
	NoSummary   bool   // totally-stubby / totally-NSSA: suppress Type 3 except the default
	DefaultCost uint32 // metric for the injected Type 3 default
}

func (p AreaSummaryPolicy) isStubOrNSSA() bool {
	return p.Type == AreaTypeStub || p.Type == AreaTypeNSSA
}

// applyAreaTypePolicy rewrites the desired Type-3/4 set for a stub or NSSA
// destination area. Type-4 ASBR summaries never enter these areas. A no-summary
// area suppresses every Type-3 except its default. Stub areas and no-summary
// NSSAs get one Type-3 default at default-cost. A regular NSSA gets its default
// from the Type-7 originator.
func applyAreaTypePolicy(desired []summaryDesired, p AreaSummaryPolicy) []summaryDesired {
	if !p.isStubOrNSSA() {
		return desired
	}
	out := make([]summaryDesired, 0, len(desired)+1)
	for _, d := range desired {
		if d.Type == types.LSTypeSummaryASBR {
			continue // stub/NSSA: no Type 4
		}
		if p.NoSummary && d.Type == types.LSTypeSummaryNetwork {
			continue // totally-stubby/NSSA: suppress inter-area Type 3
		}
		out = append(out, d)
	}
	if p.Type == AreaTypeStub || p.Type == AreaTypeNSSA && p.NoSummary {
		out = append(out, summaryDesired{
			Type:   types.LSTypeSummaryNetwork,
			LSID:   types.LinkStateID([4]byte{}),
			Mask:   [4]byte{},
			Metric: p.DefaultCost,
		})
	}
	return out
}
