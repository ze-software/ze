package bgp_nlri_rtc

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
		Name:                 "bgp-rtc",
		Description:          "Route Target Constraint family plugin (RFC 4684)",
		RFCs:                 []string{"4684"},
		SupportsNLRI:         true,
		Features:             "nlri",
		Families:             []string{"ipv4/rtc"},
		RunEngine:            RunRTCPlugin,
		InProcessNLRIDecoder: DecodeNLRIHex,
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
			return RunCLIDecode(hex, "ipv4/rtc", text, out, errOut)
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error("rtc: registration failed", "error", err)
		os.Exit(1)
	}
}
