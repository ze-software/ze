package rpki

import (
	rpkiyang "github.com/ze-software/ze/internal/component/bgp/plugins/rpki/yang"
	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func rpkiBusSends() []rpc.MessageSendDecl {
	return []rpc.MessageSendDecl{
		{Message: events.NamespaceCLI + ":request bgp adj-rib-in disable-validation"},
		{Message: events.NamespaceCLI + ":request bgp adj-rib-in enable-validation"},
		{Message: events.NamespaceCLI + ":request bgp adj-rib-in batch-validate"},
		{
			Message: bgpevents.Namespace + ":" + bgpevents.EventRPKI,
			Tags: []rpc.MessageTagDecl{{
				Name:   events.TagDirection,
				Values: []string{events.DirectionReceived},
			}},
		},
	}
}

func rpkiBusReceives() []rpc.MessageReceiveDecl {
	return []rpc.MessageReceiveDecl{
		{
			Message: bgpevents.Namespace + ":" + bgpevents.EventUpdate,
			Tags: []rpc.MessageTagDecl{{
				Name:   events.TagDirection,
				Values: []string{events.DirectionReceived},
			}},
		},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventState},
		{Message: retention.Namespace + ":" + retention.EventType},
		{Message: retention.Namespace + ":" + retention.CleanupEventType},
	}
}

func init() {
	reg := registry.Registration{
		Name:        "bgp-rpki",
		Description: "RPKI origin validation via RTR protocol",
		RFCs:        []string{"6811", "8210"},
		Features:    "yang",
		YANG:        rpkiyang.ZeRPKIYANG,
		ConfigRoots: []string{configRootBGP},
		// RTR over TLS names certificates from the pki store, so the plugin
		// reads `pki`; a config with `pki` and no `bgp` does not need it.
		ConfigReads:             []string{configRootPKI},
		Dependencies:            []string{configRootBGP, "bgp-adj-rib-in"},
		RunEngine:               runRPKIPlugin,
		InProcessConfigVerifier: validateRPKISections,
		FatalOnConfigError:      true,
		Commands:                commandDecls(),
		Pipes:                   pipeDecls(),
		Sends:                   rpkiBusSends(),
		Receives:                rpkiBusReceives(),
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			SetMetricsRegistry(reg)
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = func() string { return rpkiyang.ZeRPKIYANG }
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		panic("BUG: bgp-rpki: registration failed: " + err.Error())
	}

	// This plugin connects to the caches the config names, so it owns the
	// readiness check that asks whether any of them can be reached (doctor.go).
	if err := diagnostic.RegisterDoctorCheck(rpkiDoctorCheck); err != nil {
		panic("BUG: bgp-rpki: doctor check registration failed: " + err.Error())
	}
	if err := config.RegisterServiceBlock("bgp/rpki/cache-server[*]", "RPKI cache server"); err != nil {
		panic("BUG: bgp-rpki: service block registration: " + err.Error())
	}
}
