package filter

import (
	"net"

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

	// Loop detection is engine code: the reactor runs LoopIngress as its own
	// protocol-stage ingress step and binds loop-detection chain members
	// through BindLoopDetection, so it registers no filter.
	_ = registry.Register(registry.Registration{
		Name:        LoopName,
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
