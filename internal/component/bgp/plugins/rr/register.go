package rr

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:         "bgp-rr",
		Description:  "Route Reflector",
		RFCs:         []string{"4456"},
		Dependencies: []string{"bgp-adj-rib-in"},
		// The peer-up replay reflects the stored adj-rib-in into the client that
		// establishes, which is that client's initial routing update, and
		// signalSessionReady reports when it is out, after this plugin's own
		// End-of-RIB (rr.go).
		SignalsSessionReady: true,
		RunEngine:           runRouteReflector,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
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
		fmt.Fprintf(os.Stderr, "rr: registration failed: %v\n", err)
		os.Exit(1)
	}
}
