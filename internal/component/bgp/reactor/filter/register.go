package filter

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
		Name:    "loop",
		Stage:   filterapi.FilterStageProtocol,
		Ingress: LoopIngress,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "loop: filter registration failed: %v\n", err)
		os.Exit(1)
	}

	_ = registry.Register(registry.Registration{
		Name:        "loop",
		Description: "Route loop detection (RFC 4271 S9, RFC 4456 S8)",
		RFCs:        []string{"4271", "4456"},
		RunEngine:   func(_ net.Conn) int { return 0 },
		CLIHandler:  func(_ []string) int { return 0 },
	})
}
