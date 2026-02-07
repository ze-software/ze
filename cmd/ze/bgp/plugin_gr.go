package bgp

import (
	"codeberg.org/thomas-mangin/ze/internal/plugin/gr"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// cmdPluginGR runs the Graceful Restart capability plugin.
// It receives per-peer restart-time config and registers GR capabilities.
//
// CLI Mode: Direct hex input for human use.
//
//	ze bgp plugin gr --capa 0078000101...            # JSON output (default)
//	ze bgp plugin gr --capa 0078000101... --text     # text output
//	ze bgp plugin gr --capa -                        # read hex from stdin
//	ze bgp plugin gr --features                      # list supported features
//
// Engine Mode (no flags, no args): Full plugin with startup protocol.
func cmdPluginGR(args []string) int {
	return RunPlugin(PluginConfig{
		Name:         "gr",
		Features:     "capa yang",
		SupportsNLRI: false,
		SupportsCapa: true,
		GetYANG:      gr.GetYANG,
		ConfigLogger: func(level string) {
			gr.SetLogger(slogutil.PluginLogger("gr", level))
		},
		RunCLIDecode: gr.RunCLIDecode,
		RunEngine:    gr.RunGRPlugin,
	}, args)
}
