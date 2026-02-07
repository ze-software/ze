package bgp

import (
	"codeberg.org/thomas-mangin/ze/internal/plugin/rr"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// cmdPluginRR runs the Route Server plugin.
//
// CLI Mode:
//
//	ze bgp plugin rr --features                    # list supported features
//
// Engine Mode (no flags, no args): Full plugin with startup protocol.
func cmdPluginRR(args []string) int {
	return RunPlugin(PluginConfig{
		Name:         "rr",
		Features:     "",
		SupportsNLRI: false,
		SupportsCapa: false,
		GetYANG:      func() string { return rrYANG },
		ConfigLogger: func(level string) {
			rr.SetLogger(slogutil.PluginLogger("rr", level))
		},
		RunEngine: rr.RunRouteServer,
	}, args)
}

// rrYANG is the YANG schema for the RR plugin.
// TODO: Add actual YANG schema when plugin config schema is defined.
const rrYANG = ""
