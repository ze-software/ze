// Design: docs/architecture/core-design.md -- test-only in-process plugin registration
package fakeflow

import (
	"net"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

func init() {
	reg := registry.Registration{
		Name:        Name,
		Description: "Test-only synthetic flow-observation injector (use ze.fakeflow; publishes only when invoked)",
		RunEngine:   runPlugin,
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
		panic("BUG: fakeflow registration failed")
	}
}

// runPlugin is the engine-mode entry point (in-process goroutine).
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
			{Name: "request fakeflow inject"},
			{Name: "show fakeflow selfcheck"},
			{Name: "show fakeflow help"},
		},
	}); err != nil {
		logger().Error("fakeflow plugin failed", "error", err)
		return 1
	}
	return 0
}
