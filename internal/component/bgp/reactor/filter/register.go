// Design: docs/architecture/core-design.md -- route loop detection ingress filter
// Related: ../loopdetect/loop.go -- the engine's loop step this registration names.
//
// Package filter registers the loop plugin: the loop-detection YANG (yang/),
// the loop-detection filter type, and the registry entry that names loop
// detection in the plugin catalog. Loop detection itself is engine code in
// reactor/loopdetect, which the reactor and config resolution call directly;
// this package depends on it, never the reverse.
package filter

import (
	"net"

	"github.com/ze-software/ze/internal/component/bgp/reactor/loopdetect"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
)

func init() {
	// Loop detection reads the AS path and sets nothing (spec "Fields each
	// filter type reads and sets").
	policy.MustRegisterFilterType(loopdetect.FilterType, []policy.Field{policy.FieldASPath}, nil)

	// Loop detection is engine code: the reactor runs loopdetect.LoopIngress
	// as its own protocol-stage ingress step and binds loop-detection chain
	// members through loopdetect.BindLoopDetection, so it registers no filter.
	_ = registry.Register(registry.Registration{
		Name:        loopdetect.LoopName,
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
