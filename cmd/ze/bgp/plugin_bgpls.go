//nolint:dupl // Plugin CLI files intentionally follow the same pattern
package bgp

import (
	"flag"
	"io"

	"codeberg.org/thomas-mangin/ze/internal/plugin/bgpls"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// cmdPluginBGPLS runs the BGP-LS family plugin.
// It handles decoding of BGP-LS NLRI (RFC 7752, 9085, 9514).
//
// CLI Mode: Direct hex input for human use.
//
//	ze bgp plugin bgpls --nlri 0001000000000001...              # JSON output (default)
//	ze bgp plugin bgpls --nlri 0001000000000001... --text       # text output
//	ze bgp plugin bgpls --nlri - --family bgp-ls/bgp-ls         # read hex from stdin
//	ze bgp plugin bgpls --features                               # list supported features
//
// Engine Decode Mode (--decode): Protocol commands on stdin.
//
//	ze bgp plugin bgpls --decode                                 # reads "decode nlri ..." from stdin
//
// Engine Mode (no flags, no args): Full plugin with startup protocol.
func cmdPluginBGPLS(args []string) int {
	var family *string

	return RunPlugin(PluginConfig{
		Name:         "bgpls",
		Features:     "nlri",
		SupportsNLRI: true,
		SupportsCapa: false,
		GetYANG:      bgpls.GetBGPLSYANG,
		ConfigLogger: func(level string) {
			bgpls.SetBGPLSLogger(slogutil.PluginLogger("bgpls", level))
		},
		ExtraFlags: func(fs *flag.FlagSet) {
			family = fs.String("family", "bgp-ls/bgp-ls", "Address family (bgp-ls/bgp-ls, bgp-ls/bgp-ls-vpn)")
		},
		RunCLIWithCtx: func(hex string, text bool, out, errOut io.Writer, fs *flag.FlagSet) int {
			return bgpls.RunBGPLSCLIDecode(hex, *family, text, out, errOut)
		},
		RunDecode: bgpls.RunBGPLSDecode,
		RunEngine: bgpls.RunBGPLSPlugin,
	}, args)
}
