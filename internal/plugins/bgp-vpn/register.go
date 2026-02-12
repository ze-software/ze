package bgp_vpn

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:         "bgp-vpn",
		Description:  "VPN family plugin",
		RFCs:         []string{"4364", "4659"},
		SupportsNLRI: true,
		Features:     "nlri",
		Families:     []string{"ipv4/vpn", "ipv6/vpn"},
		RunEngine:    RunVPNPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			SetVPNLogger(slogutil.Logger(loggerName))
		},
		InProcessDecoder: func(input, output *bytes.Buffer) int {
			return RunVPNDecode(input, output)
		},
		InProcessNLRIDecoder: DecodeNLRIHex,
		InProcessNLRIEncoder: EncodeNLRIHex,
	}
	reg.CLIHandler = func(args []string) int {
		var family *string
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = GetVPNYANG
		cfg.ConfigLogger = func(level string) {
			SetVPNLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.ExtraFlags = func(fs *flag.FlagSet) {
			family = fs.String("family", "ipv4/vpn", "Address family (ipv4/vpn, ipv6/vpn)")
		}
		cfg.RunCLIWithCtx = func(hex string, text bool, out, errOut io.Writer, fs *flag.FlagSet) int {
			return RunCLIDecode(hex, *family, text, out, errOut)
		}
		cfg.RunDecode = RunVPNDecode
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "vpn: registration failed: %v\n", err)
		os.Exit(1)
	}
}
