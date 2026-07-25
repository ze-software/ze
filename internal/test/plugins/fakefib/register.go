package fakefib

import (
	"context"
	"net"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

func init() {
	reg := registry.Registration{
		Name:        Name,
		Description: "Test-only sysrib event emitter for FIB functional tests (use ze.fakefib)",
		RunEngine:   runPlugin,
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
		panic("BUG: " + Name + " registration failed: " + err.Error())
	}
}

func runPlugin(conn net.Conn) int {
	var tb textbuf.Buffer
	logger().Debug(tb.Str(Name).Str(" plugin starting (RPC)").Slice())

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
