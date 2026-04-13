// Design: docs/architecture/core-design.md -- redistribute plugin registration

package redistribute

import (
	"net"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

func init() {
	if err := registry.Register(registry.Registration{
		Name:          "bgp-redistribute",
		Description:   "Route redistribution ingress filter with loop prevention and family filtering",
		Features:      "filter",
		ConfigRoots:   []string{"redistribute"},
		Dependencies:  []string{"bgp"},
		RunEngine:     func(_ net.Conn) int { return 0 },
		CLIHandler:    func(_ []string) int { return 0 },
		IngressFilter: IngressFilter,
		FilterStage:   registry.FilterStagePolicy,
	}); err != nil {
		panic("BUG: bgp-redistribute registration failed: " + err.Error())
	}
}
