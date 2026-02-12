package bgp_ls

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
		Name:         "bgp-ls",
		Description:  "BGP-LS family plugin",
		RFCs:         []string{"7752", "9085", "9514"},
		SupportsNLRI: true,
		Features:     "nlri",
		Families:     []string{"bgp-ls/bgp-ls", "bgp-ls/bgp-ls-vpn"},
		RunEngine:    RunBGPLSPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			SetBGPLSLogger(slogutil.Logger(loggerName))
		},
		InProcessDecoder: func(input, output *bytes.Buffer) int {
			return RunBGPLSDecode(input, output)
		},
	}
	reg.CLIHandler = func(args []string) int {
		var family *string
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = GetBGPLSYANG
		cfg.ConfigLogger = func(level string) {
			SetBGPLSLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.ExtraFlags = func(fs *flag.FlagSet) {
			family = fs.String("family", "bgp-ls/bgp-ls", "Address family (bgp-ls/bgp-ls, bgp-ls/bgp-ls-vpn)")
		}
		cfg.RunCLIWithCtx = func(hex string, text bool, out, errOut io.Writer, fs *flag.FlagSet) int {
			return RunBGPLSCLIDecode(hex, *family, text, out, errOut)
		}
		cfg.RunDecode = RunBGPLSDecode
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "bgpls: registration failed: %v\n", err)
		os.Exit(1)
	}
}
