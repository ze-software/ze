// Design: docs/architecture/core-design.md -- engine-derived export kind.
// Related: reactor_api_selected.go -- authenticated selected authority.
package reactor

import "github.com/ze-software/ze/internal/component/bgp/filterapi"

// exportKind reads selected authority rather than source facts or metadata.
// The other forwarding entry points are the existing explicit relay/role
// requests; only an authenticated selected write carries the ordinary class.
func (source forwardSourceInfo) exportKind() filterapi.ExportKind {
	if source.selected == nil {
		return filterapi.ExportExplicit
	}
	if source.selected.write == nil {
		return filterapi.ExportUnknown
	}
	return source.selected.write.output.kind
}

// exportKind is the final writer's per-item authority. A nil item is its direct
// originated/raw entry point. Selected output keeps its already-derived kind,
// including Unknown, which the policy boundary MUST refuse. No missing source
// fact or plugin-supplied metadata can change this answer.
func (item *fwdItem) exportKind() filterapi.ExportKind {
	if item == nil {
		return filterapi.ExportExplicit
	}
	if item.selected != nil {
		return item.selected.output.kind
	}
	return filterapi.ExportExplicit
}
