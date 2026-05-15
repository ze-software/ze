// Design: docs/architecture/core-design.md -- plugin registration

package kernel

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	kernelschema "codeberg.org/thomas-mangin/ze/internal/plugins/kernel/schema"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

const pluginName = "kernel"

var sourcesOnce sync.Once

func registerKernelSources() {
	sourcesOnce.Do(func() {
		_ = redistribute.RegisterSource(redistribute.RouteSource{
			Name:        pluginName,
			Protocol:    pluginName,
			Description: "externally-installed kernel routes (DHCP, PPP, manual)",
		})
	})
}

func init() {
	reg := registry.Registration{
		Name:        pluginName,
		Description: "Kernel routes: redistribute externally-installed kernel routes into BGP",
		Features:    "yang",
		YANG:        kernelschema.ZeKernelConfYANG,
		ConfigRoots: []string{pluginName},
		RunEngine:   runKernelPlugin,
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
		fmt.Fprintf(os.Stderr, "kernel: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func runKernelPlugin(conn net.Conn) int {
	p := sdk.NewWithConn(pluginName, conn)
	defer func() { _ = p.Close() }()

	registerKernelSources()

	bus := getEventBus()
	obs := newRouteObserver(bus)

	p.OnStarted(func(ctx context.Context) error {
		go obs.run(ctx)
		return nil
	})

	ctx := context.Background()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{pluginName},
	}); err != nil {
		logger().Error("kernel plugin failed", "error", err)
		return 1
	}
	return 0
}
