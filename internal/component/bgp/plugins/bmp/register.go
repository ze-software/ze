package bmp

import (
	"log/slog"
	"os"

	bmpyang "github.com/ze-software/ze/internal/component/bgp/plugins/bmp/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/ze"
)

func init() {
	reg := registry.Registration{
		Name:        "bgp-bmp",
		Description: "BMP receiver and sender (RFC 7854, 8671)",
		RFCs:        []string{"7854", "8671"},
		Features:    "yang",
		YANG:        bmpyang.ZeBMPConfYANG,
		ConfigRoots: []string{"bgp", "environment"},
		RunEngine:   runBMPPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		// RFC 9069 Loc-RIB monitoring subscribes to the RIB's best-change
		// events on the in-process EventBus (mirrors redistribute_egress).
		ConfigureEventBus: func(eb ze.EventBus) {
			setEventBus(eb)
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error("bgp-bmp: registration failed", "error", err)
		os.Exit(1)
	}
}
