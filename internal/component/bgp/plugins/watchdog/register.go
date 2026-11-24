package watchdog

import (
	"bytes"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func watchdogBusSends() []rpc.MessageSendDecl {
	return []rpc.MessageSendDecl{
		{Message: events.NamespaceCLI + ":request peer"},
		{Message: bgpevents.Namespace + ":" + bgpevents.SendUpdate},
	}
}

func watchdogBusReceives() []rpc.MessageReceiveDecl {
	return []rpc.MessageReceiveDecl{
		{Message: bgpevents.Namespace + ":" + bgpevents.EventState},
	}
}

func init() {
	pluginSetup()
	// The request bgp watchdog commands YANG declares (cmd_request.go).
	pluginserver.RegisterRPCs(commandRPCs()...)
}

func pluginSetup() {
	reg := registry.Registration{
		Name:         "bgp-watchdog",
		Description:  "Watchdog route management plugin",
		ConfigRoots:  []string{configRootBGP},
		Dependencies: []string{configRootBGP},
		RunEngine:    runWatchdogPlugin,
		Commands:     commandDecls(),
		Sends:        watchdogBusSends(),
		Receives:     watchdogBusReceives(),
		// A peer-up resends this peer's announced pool, and the resend reports
		// when it is out (watchdog.go).
		SignalsSessionReady: true,
		InProcessDecoder: func(input, output *bytes.Buffer) int {
			return 0
		},
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			SetMetricsRegistry(reg)
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.RunEngine = runWatchdogPlugin
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		panic("BUG: bgp-watchdog: registration failed: " + err.Error())
	}
}
