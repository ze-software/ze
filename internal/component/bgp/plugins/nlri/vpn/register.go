package vpn

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func init() {
	// RFC 5512: Register Tunnel Encapsulation attribute (type 23).
	attribute.RegisterName(23, "TUNNEL_ENCAPSULATION")

	reg := registry.Registration{
		Name:         "bgp-nlri-vpn",
		Description:  "VPN family plugin",
		RFCs:         []string{"4364", "4659"},
		SupportsNLRI: true,
		Features:     "nlri",
		Families:     []string{familyIPv4VPN, familyIPv6VPN},
		RunEngine:    runVPNPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setVPNLogger(slogutil.Logger(loggerName))
		},
		InProcessDecoder: func(input, output *bytes.Buffer) int {
			return runVPNDecode(input, output)
		},
		InProcessNLRIDecoder:  DecodeNLRIHex,
		InProcessNLRIEncoder:  EncodeNLRIHex,
		InProcessRouteEncoder: EncodeRoute,
	}
	reg.CLIHandler = func(args []string) int {
		var family *string
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = getVPNYANG
		cfg.ConfigLogger = func(level string) {
			setVPNLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.ExtraFlags = func(fs *flag.FlagSet) {
			family = fs.String("family", familyIPv4VPN, "Address family (ipv4/mpls-vpn, ipv6/mpls-vpn)")
		}
		cfg.RunCLIWithCtx = func(hex string, text bool, out, errOut io.Writer, fs *flag.FlagSet) int {
			return RunCLIDecode(hex, *family, text, out, errOut)
		}
		cfg.RunDecode = runVPNDecode
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "vpn: registration failed: %v\n", err)
		os.Exit(1)
	}
}
