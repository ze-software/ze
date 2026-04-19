// Design: docs/architecture/core-design.md -- redistribute plugin registration

package redistribute

import (
	"net"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// This plugin is callback-only. The `IngressFilter` callback below is
// registered at init() time via the plugin registry; the filter chain
// (`registry.IngressFilters()`) reads from the registry directly, so the
// callback fires regardless of whether the plugin's `RunEngine` ever runs.
//
// Auto-load on the `redistribute {}` config root is owned by
// `bgp-redistribute-egress` (the cross-protocol egress consumer), not by
// this wrapper. That separation lets the wrapper be a pure registration
// shim with a no-op `RunEngine` -- nothing auto-loads it, nothing tries to
// drive it through the 5-stage handshake, and operators who want only the
// ingress ACL get zero plugin spin-up cost.
//
// DO NOT invoke this plugin via `plugin { external bgp-redistribute { use
// bgp-redistribute } }`. The no-op `RunEngine` returns 0 immediately,
// which the engine sees as "rpc startup: read registration failed" and
// will spam stage-timeout warnings into every other plugin sharing the
// startup phase. The IngressFilter callback works without an explicit
// load -- registration alone is sufficient.
func init() {
	if err := registry.Register(registry.Registration{
		Name:          "bgp-redistribute",
		Description:   "Route redistribution ingress filter with loop prevention and family filtering",
		Features:      "filter",
		Dependencies:  []string{"bgp"},
		RunEngine:     func(_ net.Conn) int { return 0 },
		CLIHandler:    func(_ []string) int { return 0 },
		IngressFilter: IngressFilter,
		FilterStage:   registry.FilterStagePolicy,
	}); err != nil {
		panic("BUG: bgp-redistribute registration failed: " + err.Error())
	}
}
