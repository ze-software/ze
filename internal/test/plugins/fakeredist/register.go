package fakeredist

import (
	"context"
	"net"

	configredist "codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

func init() {
	// Register fakeredist with both upstream registries so bgp-redistribute
	// finds it during its own startup enumeration.
	ProtocolID = redistevents.RegisterProtocol(ProtocolName)
	redistevents.RegisterProducer(ProtocolID)
	if err := configredist.RegisterSource(configredist.RouteSource{
		Name:        ProtocolName,
		Protocol:    ProtocolName,
		Description: "Test-only synthetic redistribution source",
	}); err != nil {
		panic("BUG: " + Name + " source registration failed: " + err.Error())
	}

	reg := registry.Registration{
		Name:        Name,
		Description: "Test-only synthetic route producer (use ze.fakeredist; harmless when not invoked)",
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

// runPlugin is the engine-mode entry point.
func runPlugin(conn net.Conn) int {
	logger().Debug(Name + " plugin starting (RPC)")

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	p.OnExecuteCommand(dispatchCommand)

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: "fakeredist emit"},
			{Name: "fakeredist emit-burst"},
			{Name: "fakeredist help"},
		},
	}); err != nil {
		logger().Error(Name+" plugin failed", "error", err)
		return 1
	}
	return 0
}

// _ context import asserted -- sdk.SignalContext returns (context.Context,
// CancelFunc); the unused-import linter would strip this otherwise.
var _ = context.Background
