package fakel2tp

import (
	"net"

	configredist "codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

func init() {
	// Register "l2tp" as a config-layer redistribute source so
	// `redistribute { import l2tp { ... } }` is valid even without
	// the full L2TP subsystem running. RegisterSource is idempotent
	// for same name+protocol; if the real subsystem also registers,
	// both succeed.
	if err := configredist.RegisterSource(configredist.RouteSource{
		Name:        "l2tp",
		Protocol:    "l2tp",
		Description: "subscriber routes from L2TP tunnels (test producer)",
	}); err != nil {
		panic("BUG: " + Name + " source registration failed: " + err.Error())
	}

	reg := registry.Registration{
		Name:        Name,
		Description: "Test-only synthetic L2TP route producer (use ze.fakel2tp; harmless when not invoked)",
		RunEngine:   runPlugin,
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
		panic("BUG: " + Name + " registration failed: " + err.Error())
	}
}

func runPlugin(conn net.Conn) int {
	logger().Debug(Name + " plugin starting (RPC)")

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	p.OnExecuteCommand(dispatchCommand)

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: "fakel2tp emit"},
			{Name: "fakel2tp help"},
		},
	}); err != nil {
		logger().Error(Name+" plugin failed", "error", err)
		return 1
	}
	return 0
}
