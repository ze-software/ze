//nolint:dupl // Plugin CLI files intentionally follow the same pattern
package bgp

import (
	"flag"
	"io"

	"codeberg.org/thomas-mangin/ze/internal/plugin/vpn"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// cmdPluginVPN runs the VPN family plugin.
// It handles decoding of VPN NLRI (RFC 4364, 4659).
//
// CLI Mode: Direct hex input for human use.
//
//	ze bgp plugin vpn --nlri 70000641...              # JSON output (default)
//	ze bgp plugin vpn --nlri 70000641... --text       # text output
//	ze bgp plugin vpn --nlri - --family ipv4/vpn      # read hex from stdin
//	ze bgp plugin vpn --features                      # list supported features
//
// Engine Decode Mode (--decode): Protocol commands on stdin.
//
//	ze bgp plugin vpn --decode                        # reads "decode nlri ..." from stdin
//
// Engine Mode (no flags, no args): Full plugin with startup protocol.
func cmdPluginVPN(args []string) int {
	var family *string

	return RunPlugin(PluginConfig{
		Name:         "vpn",
		Features:     "nlri",
		SupportsNLRI: true,
		SupportsCapa: false,
		GetYANG:      vpn.GetVPNYANG,
		ConfigLogger: func(level string) {
			vpn.SetVPNLogger(slogutil.PluginLogger("vpn", level))
		},
		ExtraFlags: func(fs *flag.FlagSet) {
			family = fs.String("family", "ipv4/vpn", "Address family (ipv4/vpn, ipv6/vpn)")
		},
		RunCLIWithCtx: func(hex string, text bool, out, errOut io.Writer, fs *flag.FlagSet) int {
			return vpn.RunCLIDecode(hex, *family, text, out, errOut)
		},
		RunDecode: vpn.RunVPNDecode,
		RunEngine: func(in io.Reader, out io.Writer) int {
			return vpn.NewVPNPlugin(in, out).Run()
		},
	}, args)
}
