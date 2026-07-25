package fakeredist

import (
	"context"
	"net"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
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

// runPlugin is the engine-mode entry point.
func runPlugin(conn net.Conn) int {
	var tb textbuf.Buffer
	logger().Debug(tb.Str(Name).Str(" plugin starting (RPC)").Slice())

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	p.OnExecuteCommand(dispatchCommand)

	// Redistribute late-join replay: on a ReplayRequest re-emit the current
	// synthetic route set tagged with the echoed ReplayID so a peer that
	// established after injection receives them (spec-redistribute-late-join-replay).
	if bus := getEventBus(); bus != nil {
		unsub := redistevents.ReplayRequestEvent.Subscribe(bus, func(r *redistevents.ReplayRequest) {
			reemitAll(r.ReplayID)
		})
		defer unsub()
	}

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: "request fakeredist emit"},
			{Name: "request fakeredist emit-burst"},
			{Name: "show fakeredist help"},
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
