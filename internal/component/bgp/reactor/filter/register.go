package filter

import (
	"fmt"
	"net"
	"os"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
)

// loopFilterType is the ze:filter list loop detection owns under the policy
// root (yang/ze-loop-detection.yang).
const loopFilterType = "loop-detection"

func init() {
	// Loop detection reads the AS path and sets nothing (spec "Fields each
	// filter type reads and sets").
	policy.MustRegisterFilterType(loopFilterType, []policy.Field{policy.FieldASPath}, nil)

	// Route filter pipeline contribution (BGP-owned seam, not the generic registry).
	if err := filterapi.Register(filterapi.Filter{
		Name:       "loop",
		Stage:      filterapi.FilterStageProtocol,
		Ingress:    LoopIngress,
		PolicyType: loopFilterType,
		// Loop detection runs as an always-on protocol filter, not as a run
		// plugin, so its counters arrive through filterapi.SetMetricsRegistry.
		ConfigureMetrics: SetMetricsRegistry,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "loop: filter registration failed: %v\n", err)
		os.Exit(1)
	}

	_ = registry.Register(registry.Registration{
		Name:        "loop",
		Description: "Route loop detection (RFC 4271 S9, RFC 4456 S8)",
		RFCs:        []string{"4271", "4456"},
		// Loop detection registers the loop-detection filter type into the
		// policy library above, so it names policy the way every filter type
		// plugin does (policy.PluginName).
		Dependencies: []string{policy.PluginName},
		RunEngine:    func(_ net.Conn) int { return 0 },
		CLIHandler:   func(_ []string) int { return 0 },
	})
}
