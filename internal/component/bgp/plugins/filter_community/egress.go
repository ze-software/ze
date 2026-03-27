// Design: docs/architecture/core-design.md — community filter egress path
// Overview: filter_community.go — plugin entry point
// Related: handler.go — AttrModHandlers for progressive build
// Related: filter.go — ingress filter (direct payload mutation)
// Related: config.go — config parsing

package filter_community

import (
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// applyEgressFilter accumulates community strip/tag ops into the ModAccumulator
// for the destination peer. The actual wire manipulation happens later in
// buildModifiedPayload via the registered AttrModHandlers.
// mods MUST be non-nil (caller guarantees fresh instance per peer).
func applyEgressFilter(defs communityDefs, fc filterConfig, mods *registry.ModAccumulator) {
	if mods == nil {
		return
	}
	// Strip first, then tag (same ordering as ingress).
	for _, name := range fc.egressStrip {
		def, ok := defs[name]
		if !ok {
			continue
		}
		code := byte(communityAttrCode(def.typ))
		for _, wire := range def.wireValues {
			mods.Op(code, registry.AttrModRemove, wire)
		}
	}

	for _, name := range fc.egressTag {
		def, ok := defs[name]
		if !ok {
			continue
		}
		code := byte(communityAttrCode(def.typ))
		for _, wire := range def.wireValues {
			mods.Op(code, registry.AttrModAdd, wire)
		}
	}
}
