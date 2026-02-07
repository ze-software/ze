//nolint:dupl // Plugin CLI files intentionally follow the same pattern
package bgp

import (
	"flag"
	"io"

	"codeberg.org/thomas-mangin/ze/internal/plugin/flowspec"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// cmdPluginFlowSpec runs the FlowSpec family plugin.
// It handles decoding of FlowSpec NLRI (RFC 8955, 8956).
//
// CLI Mode: Direct hex input for human use.
//
//	ze bgp plugin flowspec --nlri 0718...              # JSON output (default)
//	ze bgp plugin flowspec --nlri 0718... --text       # text output
//	ze bgp plugin flowspec --nlri - --family ipv4/flow # read hex from stdin
//	ze bgp plugin flowspec --features                  # list supported features
//
// Engine Decode Mode (--decode): Protocol commands on stdin.
//
//	ze bgp plugin flowspec --decode                    # reads "decode nlri ..." from stdin
//
// Engine Mode (no flags, no args): Full plugin with startup protocol.
func cmdPluginFlowSpec(args []string) int {
	var family *string

	return RunPlugin(PluginConfig{
		Name:         "flowspec",
		Features:     "nlri",
		SupportsNLRI: true,
		SupportsCapa: false,
		GetYANG:      flowspec.GetFlowSpecYANG,
		ConfigLogger: func(level string) {
			flowspec.SetFlowSpecLogger(slogutil.PluginLogger("flowspec", level))
		},
		ExtraFlags: func(fs *flag.FlagSet) {
			family = fs.String("family", "ipv4/flow", "Address family (ipv4/flow, ipv6/flow, ipv4/flow-vpn, ipv6/flow-vpn)")
		},
		RunCLIWithCtx: func(hex string, text bool, out, errOut io.Writer, fs *flag.FlagSet) int {
			return flowspec.RunCLIDecode(hex, *family, text, out, errOut)
		},
		RunDecode: flowspec.RunFlowSpecDecode,
		RunEngine: flowspec.RunFlowSpecPlugin,
	}, args)
}
