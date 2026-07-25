// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing doctor check.
// checkOSPFSegmentRouting is a pre-runtime readiness check (spec-ospf-ext-5 AC-21):
// it resolves the SR config the same way the engine does and warns when the SRGB/SRLB
// ranges are unsound (overlap, out of the 20-bit label space, zero Range Size, or a
// Prefix-SID index beyond the SRGB) before those ranges reach the MPLS data plane.
// RFC: rfc/short/rfc8665.md (§3.2/§3.3 non-overlapping ranges); rfc/short/rfc8666.md

package ospf

import (
	"encoding/json"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
)

// codeOSPFSegmentRoutingOverlap fires when an enabled SR block has overlapping or
// otherwise unsound SRGB/SRLB ranges (registered in internal/core/diagnostic/codes.go).
const codeOSPFSegmentRoutingOverlap = "doctor-ospf-segment-routing-overlap"

// srConfigDiagnostics is the pure decision: an enabled SR config whose ranges fail
// validation yields one warning carrying the validation error.
func srConfigDiagnostics(cfg *sr.SRConfig) []diagnostic.Diagnostic {
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	err := cfg.Validate(nil)
	if err == nil {
		return nil
	}
	var tb textbuf.Buffer
	tb.Str("ospf segment-routing label ranges are unsound: ").Err(err)
	return []diagnostic.Diagnostic{{
		Code:     codeOSPFSegmentRoutingOverlap,
		Severity: diagnostic.SeverityWarning,
		Message:  tb.String(),
	}}
}

// checkOSPFSegmentRouting is the registered SR readiness doctor check. It resolves
// the SR blocks from the config tree and reports unsound ranges for either family.
func checkOSPFSegmentRouting(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	ospfTree := tree.GetContainer("ospf")
	if ospfTree == nil {
		return nil
	}
	data, err := json.Marshal(map[string]any{"ospf": ospfTree.ToMap()})
	if err != nil {
		return nil
	}
	v4, v6 := extractSRConfigs([]configSection{{Root: "ospf", Data: string(data)}})
	diags := make([]diagnostic.Diagnostic, 0, 2)
	diags = append(diags, srConfigDiagnostics(v4)...)
	diags = append(diags, srConfigDiagnostics(v6)...)
	return diags
}
