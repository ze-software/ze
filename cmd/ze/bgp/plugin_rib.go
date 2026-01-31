package bgp

import (
	"io"

	"codeberg.org/thomas-mangin/ze/internal/plugin/rib"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// cmdPluginRib runs the RIB (Routing Information Base) plugin.
//
// CLI Mode:
//
//	ze bgp plugin rib --features                   # list supported features
//
// Engine Mode (no flags, no args): Full plugin with startup protocol.
func cmdPluginRib(args []string) int {
	return RunPlugin(PluginConfig{
		Name:         "rib",
		Features:     "yang",
		SupportsNLRI: false,
		SupportsCapa: false,
		GetYANG:      func() string { return ribYANG },
		ConfigLogger: func(level string) {
			rib.SetLogger(slogutil.PluginLogger("rib", level))
		},
		RunEngine: func(in io.Reader, out io.Writer) int {
			return rib.NewRIBManager(in, out).Run()
		},
	}, args)
}

// ribYANG is the YANG schema for the RIB plugin.
// TODO: Add actual YANG schema when plugin config schema is defined.
const ribYANG = ""
