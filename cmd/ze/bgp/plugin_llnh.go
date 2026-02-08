package bgp

import (
	"codeberg.org/thomas-mangin/ze/internal/plugin/llnh"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// cmdPluginLLNH runs the link-local next-hop capability plugin.
// It receives per-peer config and registers capability 77
// (draft-ietf-idr-linklocal-capability) for peers that enable it.
//
// CLI Mode: Direct hex input for human use.
//
//	ze bgp plugin llnh --capa           # JSON output (default, empty payload)
//	ze bgp plugin llnh --capa --text    # text output
//	ze bgp plugin llnh --features       # list supported features
//
// Engine Decode Mode (--decode): Protocol commands on stdin.
//
//	ze bgp plugin llnh --decode         # reads "decode capability ..." from stdin
//
// Engine Mode (no flags, no args): Full plugin with startup protocol.
func cmdPluginLLNH(args []string) int {
	return RunPlugin(PluginConfig{
		Name:         "llnh",
		Features:     "capa yang",
		SupportsNLRI: false,
		SupportsCapa: true,
		GetYANG:      llnh.GetLLNHYANG,
		ConfigLogger: func(level string) {
			llnh.SetLLNHLogger(slogutil.PluginLogger("llnh", level))
		},
		RunCLIDecode: llnh.RunLLNHCLIDecode,
		RunDecode:    llnh.RunLLNHDecodeMode,
		RunEngine:    llnh.RunLLNHPlugin,
	}, args)
}
