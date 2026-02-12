package evpn

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
		Name:         "evpn",
		Description:  "EVPN family plugin",
		RFCs:         []string{"7432", "9136"},
		SupportsNLRI: true,
		Features:     "nlri",
		Families:     []string{"l2vpn/evpn"},
		RunEngine:    RunEVPNPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			SetEVPNLogger(slogutil.Logger(loggerName))
		},
		InProcessDecoder: func(input, output *bytes.Buffer) int {
			return RunEVPNDecode(input, output)
		},
	}
	reg.CLIHandler = func(args []string) int {
		var family *string
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = GetEVPNYANG
		cfg.ConfigLogger = func(level string) {
			SetEVPNLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.ExtraFlags = func(fs *flag.FlagSet) {
			family = fs.String("family", "l2vpn/evpn", "Address family (l2vpn/evpn)")
		}
		cfg.RunCLIWithCtx = func(hex string, text bool, out, errOut io.Writer, fs *flag.FlagSet) int {
			return RunCLIDecode(hex, *family, text, out, errOut)
		}
		cfg.RunDecode = RunEVPNDecode
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "evpn: registration failed: %v\n", err)
		os.Exit(1)
	}
}
