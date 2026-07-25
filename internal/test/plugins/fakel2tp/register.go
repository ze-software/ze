package fakel2tp

import (
	"net"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
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
			{Name: "request fakel2tp emit"},
			{Name: "show fakel2tp help"},
		},
	}); err != nil {
		logger().Error(Name+" plugin failed", "error", err)
		return 1
	}
	return 0
}
