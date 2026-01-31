package bgp

import (
	"io"

	"codeberg.org/thomas-mangin/ze/internal/plugin/evpn"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// cmdPluginEVPN runs the EVPN family plugin.
// It handles decoding of EVPN NLRI (RFC 7432, 9136).
//
// CLI Mode: Direct hex input for human use.
//
//	ze bgp plugin evpn --nlri 02210001252C...        # JSON output (default)
//	ze bgp plugin evpn --nlri 02210001252C... --text # text output
//	ze bgp plugin evpn --nlri -                      # read hex from stdin
//	ze bgp plugin evpn --features                    # list supported features
//
// Engine Decode Mode (--decode): Protocol commands on stdin.
//
//	ze bgp plugin evpn --decode                      # reads "decode nlri ..." from stdin
//
// Engine Mode (no flags, no args): Full plugin with startup protocol.
func cmdPluginEVPN(args []string) int {
	return RunPlugin(PluginConfig{
		Name:         "evpn",
		Features:     "nlri",
		SupportsNLRI: true,
		SupportsCapa: false,
		GetYANG:      evpn.GetEVPNYANG,
		ConfigLogger: func(level string) {
			evpn.SetEVPNLogger(slogutil.PluginLogger("evpn", level))
		},
		RunCLIDecode: evpn.RunCLIDecode,
		RunDecode:    evpn.RunEVPNDecode,
		RunEngine: func(in io.Reader, out io.Writer) int {
			return evpn.NewEVPNPlugin(in, out).Run()
		},
	}, args)
}
