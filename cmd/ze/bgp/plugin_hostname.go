package bgp

import (
	"io"

	"codeberg.org/thomas-mangin/ze/internal/plugin/hostname"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// cmdPluginHostname runs the hostname (FQDN) capability plugin.
// It receives per-peer hostname/domain config and registers FQDN capabilities.
//
// CLI Mode: Direct hex input for human use.
//
//	ze bgp plugin hostname --capa 07726f7574657231...        # JSON output (default)
//	ze bgp plugin hostname --capa 07726f7574657231... --text # text output
//	ze bgp plugin hostname --capa -                          # read hex from stdin
//	ze bgp plugin hostname --features                        # list supported features
//
// Engine Decode Mode (--decode): Protocol commands on stdin.
//
//	ze bgp plugin hostname --decode                          # reads "decode capability ..." from stdin
//
// Engine Mode (no flags, no args): Full plugin with startup protocol.
func cmdPluginHostname(args []string) int {
	return RunPlugin(PluginConfig{
		Name:         "hostname",
		Features:     "capa yang",
		SupportsNLRI: false,
		SupportsCapa: true,
		GetYANG:      hostname.GetYANG,
		ConfigLogger: func(level string) {
			hostname.ConfigureLogger(slogutil.PluginLogger("hostname", level))
		},
		RunCLIDecode: hostname.RunCLIDecode,
		RunDecode:    hostname.RunDecodeMode,
		RunEngine: func(in io.Reader, out io.Writer) int {
			return hostname.NewHostnamePlugin(in, out).Run()
		},
	}, args)
}
