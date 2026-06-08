package fakefib

import (
	"context"
	"net"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

func init() {
	reg := registry.Registration{
		Name:        Name,
		Description: "Test-only sysrib event emitter for FIB functional tests (use ze.fakefib)",
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
	var tb textbuf.Buffer
	logger().Debug(tb.Str(Name).Str(" plugin starting (RPC)").String())

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	p.OnExecuteCommand(dispatchCommand)

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: "request fakefib emit"},
			{Name: "show fakefib help"},
		},
	}); err != nil {
		logger().Error(Name+" plugin failed", "error", err)
		return 1
	}
	return 0
}

var _ = context.Background
