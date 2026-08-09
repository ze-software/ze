// Design: docs/architecture/ospf/ospf-ext-14-debug-introspection.md -- the AF-tagged SPF-explain view.
// RFC: rfc/short/rfc2328.md (Section 16.4 path preference), rfc/short/rfc5838.md (Section 2:
// AF identity of the OSPFv3 result).
//
// `show ospf spf detail` / `show ospf ipv6 spf detail` wrap the shared, AF-agnostic SPF
// explanation (spf/explain.go) with this engine's address family / Instance ID. Read-only:
// it reads the last computed result without a recompute (R-3).

package ospf

import ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"

// spfExplainView is the AF-tagged SPF-explain payload.
type spfExplainView struct {
	AddressFamily string                 `json:"address-family"`
	InstanceID    uint8                  `json:"instance-id"`
	Prefixes      []ospfspf.ExplainEntry `json:"prefixes"`
}

// spfExplainSnapshot renders the read-only per-prefix SPF explanation, tagged with this
// engine's address family and Instance ID.
func (e *engine) spfExplainSnapshot() []any {
	if e.spf == nil {
		return []any{}
	}
	view := spfExplainView{AddressFamily: e.af.String(), Prefixes: e.spf.ExplainSnapshot()}
	if e.dispatch != nil {
		view.InstanceID = e.dispatch.currentInstanceID()
	}
	return []any{view}
}
