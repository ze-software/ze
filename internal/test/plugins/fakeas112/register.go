package fakeas112

import (
	"context"
	"net"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	as112events "github.com/ze-software/ze/internal/plugins/as112/events"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

func init() {
	// Register "as112" as a config-layer redistribute source so
	// `redistribute { import as112 { ... } }` resolves even without the real
	// AS112 DNS engine running. RegisterSource is idempotent for the same
	// name+protocol, so this is a harmless no-op when the real as112 plugin
	// (which also registers it) is present in the build; it keeps fakeas112
	// self-contained when it is not.
	if err := configredist.RegisterSource(configredist.RouteSource{
		Name:        as112events.Namespace,
		Protocol:    as112events.Namespace,
		Description: "AS112 covering prefixes (test producer)",
	}); err != nil {
		panic("BUG: " + Name + " source registration failed: " + err.Error())
	}
	// The (as112) ProtocolID + producer presence are registered by the
	// as112events package at its own import-time init (RegisterProtocol /
	// RegisterProducer); importing it (used above for Namespace, and in
	// fakeas112.go for ProtocolID / RouteChange) is enough.

	reg := registry.Registration{
		Name:        Name,
		Description: "Test-only synthetic AS112 route producer (use ze.fakeas112; harmless when not invoked)",
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
	// announced set tagged with the echoed ReplayID so a peer that established
	// after injection receives them (mirror fakeredist / the real producer).
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
			{Name: "request fakeas112 emit"},
			{Name: "show fakeas112 help"},
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
