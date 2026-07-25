// Design: docs/architecture/core-design.md -- plugin registration

package connected

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
	connectedyang "github.com/ze-software/ze/internal/plugins/connected/yang"
	"github.com/ze-software/ze/pkg/ze"
)

func init() {
	// Register the redistribute source at init (not only in runConnectedPlugin)
	// so `import connected` resolves during `ze config validate`, which imports
	// plugins but does not start their engines. sync.Once keeps the run-time call
	// idempotent.
	registerConnectedSources()

	reg := registry.Registration{
		Name:        pluginName,
		Description: "Connected routes: redistribute directly connected interface prefixes",
		Features:    "yang",
		YANG:        connectedyang.ZeConnectedConfYANG,
		ConfigRoots: []string{pluginName},
		RunEngine:   runConnectedPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
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
		fmt.Fprintf(os.Stderr, "connected: registration failed: %v\n", err)
		os.Exit(1)
	}
}
