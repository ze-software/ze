// Design: docs/architecture/core-design.md -- redistribute plugin registration

package redistributeingress

import (
	"fmt"
	"net"
	"os"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func init() {
	// Route filter pipeline contribution (BGP-owned seam, not the generic registry).
	if err := filterapi.Register(filterapi.Filter{
		Name:    "bgp-redistribute",
		Stage:   filterapi.FilterStagePolicy,
		Ingress: IngressFilter,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "bgp-redistribute: filter registration failed: %v\n", err)
		os.Exit(1)
	}

	if err := registry.Register(registry.Registration{
		Name:         "bgp-redistribute",
		Description:  "Route redistribution ingress filter with loop prevention and family filtering",
		Features:     "filter",
		Dependencies: []string{"bgp"},
		RunEngine:    func(_ net.Conn) int { return 0 },
		CLIHandler:   func(_ []string) int { return 0 },
	}); err != nil {
		fmt.Fprintf(os.Stderr, "bgp-redistribute: registration failed: %v\n", err)
		os.Exit(1)
	}
}
