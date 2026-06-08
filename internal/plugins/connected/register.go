// Design: docs/architecture/core-design.md -- plugin registration

package connected

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	connectedyang "codeberg.org/thomas-mangin/ze/internal/plugins/connected/yang"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

func init() {
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
		ConfigureEventBus: func(eb any) {
			if e, ok := eb.(ze.EventBus); ok {
				setEventBus(e)
			}
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
