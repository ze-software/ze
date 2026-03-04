package bgp_nlri_vpls

import (
	"flag"
	"io"
	"log/slog"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:                  "bgp-vpls",
		Description:           "VPLS family plugin (RFC 4761)",
		RFCs:                  []string{"4761", "4762"},
		SupportsNLRI:          true,
		Features:              "nlri",
		Families:              []string{"l2vpn/vpls"},
		RunEngine:             RunVPLSPlugin,
		InProcessNLRIDecoder:  DecodeNLRIHex,
		InProcessNLRIEncoder:  EncodeNLRIHex,
		InProcessRouteEncoder: EncodeRoute,
		ConfigureEngineLogger: func(loggerName string) {
			SetLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			SetLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.RunCLIWithCtx = func(hex string, text bool, out, errOut io.Writer, _ *flag.FlagSet) int {
			return RunCLIDecode(hex, "l2vpn/vpls", text, out, errOut)
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error("vpls: registration failed", "error", err)
		os.Exit(1)
	}
}
