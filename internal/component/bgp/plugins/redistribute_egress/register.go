package redistributeegress

import (
	"context"
	"net"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

func init() {
	reg := registry.Registration{
		Name:         Name,
		Description:  "Egress redistribute: turns non-BGP protocol route events into BGP UPDATEs",
		ConfigRoots:  []string{"redistribute"},
		Dependencies: []string{"bgp"},
		RunEngine:    runPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(r any) {
			if m, ok := r.(metrics.Registry); ok {
				setMetricsRegistry(m)
			}
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
		panic("BUG: " + Name + " registration failed: " + err.Error())
	}
}

func runPlugin(conn net.Conn) int {
	logger().Debug(Name + " plugin starting (RPC)")

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	p.OnStarted(func(ctx context.Context) error {
		go run(ctx, p)
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{}); err != nil {
		logger().Error(Name+" plugin failed", "error", err)
		return 1
	}
	return 0
}
