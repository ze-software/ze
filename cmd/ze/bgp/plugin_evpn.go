//nolint:dupl // Plugin CLI files intentionally follow the same pattern
package bgp

import (
	"flag"
	"io"

	"codeberg.org/thomas-mangin/ze/internal/plugin/evpn"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// cmdPluginEVPN runs the EVPN family plugin.
// It handles decoding of EVPN NLRI (RFC 7432, 9136).
//
// CLI Mode: Direct hex input for human use.
//
//	ze bgp plugin evpn --nlri 02210001252C...              # JSON output (default)
//	ze bgp plugin evpn --nlri 02210001252C... --text       # text output
//	ze bgp plugin evpn --nlri - --family l2vpn/evpn        # read hex from stdin
//	ze bgp plugin evpn --features                          # list supported features
//
// Engine Decode Mode (--decode): Protocol commands on stdin.
//
//	ze bgp plugin evpn --decode                            # reads "decode nlri ..." from stdin
//
// Engine Mode (no flags, no args): Full plugin with startup protocol.
func cmdPluginEVPN(args []string) int {
	var family *string

	return RunPlugin(PluginConfig{
		Name:         "evpn",
		Features:     "nlri",
		SupportsNLRI: true,
		SupportsCapa: false,
		GetYANG:      evpn.GetEVPNYANG,
		ConfigLogger: func(level string) {
			evpn.SetEVPNLogger(slogutil.PluginLogger("evpn", level))
		},
		ExtraFlags: func(fs *flag.FlagSet) {
			family = fs.String("family", "l2vpn/evpn", "Address family (l2vpn/evpn)")
		},
		RunCLIWithCtx: func(hex string, text bool, out, errOut io.Writer, fs *flag.FlagSet) int {
			return evpn.RunCLIDecode(hex, *family, text, out, errOut)
		},
		RunDecode: evpn.RunEVPNDecode,
		RunEngine: evpn.RunEVPNPlugin,
	}, args)
}
