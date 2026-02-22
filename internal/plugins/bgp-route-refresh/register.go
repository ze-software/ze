package bgp_route_refresh

import (
	"bytes"
	"log/slog"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/plugin/registry"
	rrschema "codeberg.org/thomas-mangin/ze/internal/plugins/bgp-route-refresh/schema"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:            "bgp-route-refresh",
		Description:     "Route Refresh capability decoding",
		RFCs:            []string{"2918", "7313"},
		SupportsCapa:    true,
		Features:        "capa yang",
		ConfigRoots:     []string{"bgp"},
		YANG:            rrschema.ZeRouteRefreshYANG,
		CapabilityCodes: []uint8{2, 70},
		RunEngine:       RunRouteRefreshPlugin,
		InProcessDecoder: func(input, output *bytes.Buffer) int {
			return RunDecodeMode(input, output)
		},
		ConfigureEngineLogger: func(loggerName string) {
			SetLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = GetYANG
		cfg.ConfigLogger = func(level string) {
			SetLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.RunCLIDecode = RunCLIDecode
		cfg.RunDecode = RunDecodeMode
		cfg.RunEngine = RunRouteRefreshPlugin
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error("route-refresh: registration failed", "error", err)
		os.Exit(1)
	}
}
