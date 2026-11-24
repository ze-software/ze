package rpki_decorator

import (
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func rpkiDecoratorSends() []rpc.MessageSendDecl {
	return []rpc.MessageSendDecl{{
		Message: bgpevents.Namespace + ":" + eventTypeUpdateRPKI,
		Tags: []rpc.MessageTagDecl{{
			Name:   events.TagDirection,
			Values: []string{events.DirectionReceived},
		}},
	}}
}

func rpkiDecoratorReceives() []rpc.MessageReceiveDecl {
	return []rpc.MessageReceiveDecl{
		{
			Message: bgpevents.Namespace + ":" + bgpevents.EventUpdate,
			Tags: []rpc.MessageTagDecl{{
				Name:   events.TagDirection,
				Values: []string{events.DirectionReceived},
			}},
		},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventRPKI},
	}
}

func init() {
	if err := events.RegisterMessage(bgpevents.Namespace, eventTypeUpdateRPKI, events.TagDef{
		Name:   events.TagDirection,
		Values: []string{events.DirectionReceived},
	}); err != nil {
		panic("BUG: bgp-rpki-decorator: register message update-rpki: " + err.Error())
	}

	reg := registry.Registration{
		Name:         "bgp-rpki-decorator",
		Description:  "Correlates UPDATE + RPKI events into merged update-rpki events",
		Dependencies: []string{"bgp", "bgp-rpki"},
		Sends:        rpkiDecoratorSends(),
		Receives:     rpkiDecoratorReceives(),
		RunEngine:    runDecorator,
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
		panic("BUG: bgp-rpki-decorator: registration failed: " + err.Error())
	}
}
