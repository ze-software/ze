package route_refresh

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	rrschema "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/route_refresh/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:            "bgp-route-refresh",
		Description:     "Route Refresh capability decoding",
		RFCs:            []string{"2918", "7313"},
		SupportsCapa:    true,
		Features:        "capa yang",
		ConfigRoots:     []string{"bgp"},
		Dependencies:    []string{"bgp"},
		YANG:            rrschema.ZeRouteRefreshYANG,
		CapabilityCodes: []uint8{2, 70},
		SendTypes:       []string{"enhanced-refresh"},
		RunEngine:       RunRouteRefreshPlugin,
		InProcessDecoder: func(input, output *bytes.Buffer) int {
			return RunDecodeMode(input, output)
		},
		ConfigureEngineLogger: func(loggerName string) {
			SetLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		var capCode uint
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = GetYANG
		cfg.ConfigLogger = func(level string) {
			SetLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.RunCLIDecode = RunCLIDecode
		cfg.ExtraFlags = func(fs *flag.FlagSet) {
			fs.UintVar(&capCode, "code", 2, "Capability code to decode (2 route-refresh, 70 enhanced-route-refresh)")
		}
		cfg.RunCLIWithCtx = func(hex string, text bool, out, errW io.Writer, _ *flag.FlagSet) int {
			if capCode > 255 {
				if _, err := fmt.Fprintf(errW, "error: capability code out of range: %d\n", capCode); err != nil {
					logger().Debug("write failed", "err", err)
				}
				return 1
			}
			return RunCLIDecodeWithCode(uint8(capCode), hex, text, out, errW)
		}
		cfg.RunDecode = RunDecodeMode
		cfg.RunEngine = RunRouteRefreshPlugin
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error("route-refresh: registration failed", "error", err)
		os.Exit(1)
	}
}
