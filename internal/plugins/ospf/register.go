// Design: plan/learned/958-ospf-4-component-config.md -- OSPFv2 plugin registration
// Related: config.go -- typed config resolution
// Related: instance.go -- engine orchestration and packet dispatcher
package ospf

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"sync/atomic"

	configredist "codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"
	configyang "codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/diagnostic"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	ospfredistribute "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/redistribute"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/transport"
	ospfv3transport "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/v3/transport"
	ospfyang "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/yang"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

var (
	loggerPtr   atomic.Pointer[slog.Logger]
	eventBusPtr atomic.Pointer[ze.EventBus]
	metricsPtr  atomic.Pointer[metrics.Registry]
)

func init() { loggerPtr.Store(slogutil.DiscardLogger()) }

func logger() *slog.Logger { return loggerPtr.Load() }

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

func setEventBus(eb ze.EventBus) {
	if eb != nil {
		eventBusPtr.Store(&eb)
	}
}

func getEventBus() ze.EventBus {
	p := eventBusPtr.Load()
	if p == nil {
		return nil
	}
	return *p
}

func setMetricsRegistry(reg metrics.Registry) {
	if reg != nil {
		metricsPtr.Store(&reg)
	}
}

func getMetricsRegistry() metrics.Registry {
	p := metricsPtr.Load()
	if p == nil {
		return nil
	}
	return *p
}

func registerOSPF() {
	_ = events.RegisterNamespace(Namespace, EventNeighborUp, EventNeighborDown, EventSPFRun, EventLSDBChange, EventInterfaceState, EventDRChange, EventNeighborChange)

	// Register the single "ospf" redistribution source at init (not OnStarted) so
	// `ze config validate` and editor completion of `import ospf` see it without the
	// engine running. The redistevents PRODUCER is registered transitively by the
	// ospfredistribute package import (its events sub-package init).
	ospfredistribute.RegisterOSPFSources()

	registerOSPFDiagnosticCodes()
	registerOSPFDoctor()

	configyang.RegisterCompleteFn("ospf-router-id", routerIDCompletions)
	configyang.RegisterCompleteFn("ospf-area-id", areaIDCompletions)

	reg := registry.Registration{
		Name:                    "ospf",
		Description:             "Open Shortest Path First v2 (RFC 2328): native link-state IPv4 IGP",
		Features:                "yang",
		YANG:                    ospfyang.ZeOSPFConfYANG,
		ConfigRoots:             []string{"ospf"},
		Dependencies:            []string{"interface", "fib-kernel", "sysctl"},
		RFCs:                    []string{"2328", "5709", "7474", "9129"},
		RunEngine:               runOSPFEngine,
		InProcessConfigVerifier: verifyOSPFConfigSections,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
			transport.SetLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg any) {
			if r, ok := reg.(metrics.Registry); ok {
				setMetricsRegistry(r)
			}
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
			transport.SetLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "ospf: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func init() { registerOSPF() }

// ospfDiagnosticCodes is the explanation metadata for the two OSPF config-sanity codes
// this component OWNS (spec-ospf-13). Deliberately NOT in the central
// diagnostic.builtinCodes slice -- owning them here removes them with the component
// (ai/rules/plugin-self-containment.md). The doctor-ospf-raw-socket code is owned by
// ospf-3 and registered there.
var ospfDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeOSPFRouterIDMissing,
		Title:       "OSPF router-id missing",
		Description: "OSPF is configured but no router-id is set and none can be derived from an interface IPv4 address. The engine cannot originate LSAs or form adjacencies without a router-id; set `ospf { router-id <dotted-quad> }`.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-ospf-router-id-missing"},
	},
	{
		Code:        codeOSPFInterfaceAreaUnbound,
		Title:       "OSPF interface bound to undeclared area",
		Description: "An OSPF interface references an area that is not declared under `areas`. The interface forms no adjacency; declare the area or correct the interface `area` binding.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-ospf-interface-area-unbound"},
	},
}

func registerOSPFDiagnosticCodes() {
	for _, m := range ospfDiagnosticCodes {
		_ = diagnostic.Register(m)
	}
}

// registerOSPFDoctor registers the OSPF config-sanity doctor check (checkOSPFConfigSanity
// in doctor.go). The os.Exit on a registration failure is confined to this registration
// file (ai/patterns/registration.md).
func registerOSPFDoctor() {
	check := diagnostic.DoctorCheck{
		Name:         "ospf-config-sanity",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        736, // just after the ospf-3 raw-socket check (735)
		Component:    "ospf",
		Dependencies: []string{"config-tree"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{codeOSPFRouterIDMissing, codeOSPFInterfaceAreaUnbound},
		Check:        checkOSPFConfigSanity,
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		fmt.Fprintf(os.Stderr, "ospf: doctor config-sanity registration: %v\n", err)
		os.Exit(2)
	}
}

func routerIDCompletions() []string { return []string{"192.0.2.1"} }

func areaIDCompletions() []string { return []string{"0.0.0.0", "0"} }

func sdkConfigSections(sections []sdk.ConfigSection) []configSection {
	out := make([]configSection, len(sections))
	for i, s := range sections {
		out[i] = configSection{Root: s.Root, Data: s.Data}
	}
	return out
}

func rpcConfigSections(sections []rpc.ConfigSection) []configSection {
	out := make([]configSection, len(sections))
	for i, s := range sections {
		out[i] = configSection{Root: s.Root, Data: s.Data}
	}
	return out
}

func verifyOSPFConfigSections(sections []rpc.ConfigSection) error {
	cfg, err := parseOSPFConfig(rpcConfigSections(sections), systemRouterIDSource{})
	if err != nil {
		return err
	}
	return validateConfig(cfg)
}

func runOSPFEngine(conn net.Conn) int {
	log := logger()
	log.Debug("ospf engine starting")

	p := sdk.NewWithConn("ospf", conn)
	defer func() { _ = p.Close() }()

	eng := newEngine(transport.New(transport.NewBackend()))
	if reg := getMetricsRegistry(); reg != nil {
		eng.transport.SetMetrics(reg)
		eng.setMetrics(reg)
	}
	if eb := getEventBus(); eb != nil {
		eng.setEventSink(newEventSink(eb))
	}

	// The IPv6 (OSPFv3) address family runs as a second engine instance over the v6 codec and
	// the ospfv3 transport (RFC 5340). It is driven by `ospf { address-family ipv6 { ... } }`
	// (cfg.V6) and stays idle (no interfaces opened) when that section is absent. Construction
	// is cheap -- no sockets open until openInterfaces. Shared metrics/events and `show ipv6
	// ospf` observability are a later phase, so this instance is not wired to the registry yet.
	eng6 := newEngineWithCodec(ospfv3transport.New(ospfv3transport.NewBackend()), v6Codec{})

	var (
		cfgMu       sync.Mutex
		activeCfg   ospfConfig
		pendingCfg  ospfConfig
		havePending bool
	)

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		cfg, err := parseOSPFConfig(sdkConfigSections(sections), systemRouterIDSource{})
		if err != nil {
			return err
		}
		if err := validateConfig(cfg); err != nil {
			return err
		}
		cfgMu.Lock()
		pendingCfg = cfg
		havePending = true
		cfgMu.Unlock()
		return nil
	})

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseOSPFConfig(sdkConfigSections(sections), systemRouterIDSource{})
		if err != nil {
			return err
		}
		if err := validateConfig(cfg); err != nil {
			return err
		}
		cfgMu.Lock()
		activeCfg = cfg
		cfgMu.Unlock()
		eng.setConfig(cfg)
		if cfg.V6 != nil {
			eng6.setConfig(*cfg.V6)
		}
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		cfgMu.Lock()
		if havePending {
			activeCfg = pendingCfg
			havePending = false
		}
		cfg := activeCfg
		cfgMu.Unlock()
		eng.reconcile(cfg)
		if cfg.V6 != nil {
			eng6.reconcile(*cfg.V6)
		}
		return nil
	})

	p.OnStarted(func(_ context.Context) error {
		// Wire redistribution before the idle check so a `redistribute { destination
		// ospf { import <source> } }` rule has a consumer even when OSPF is idle.
		// ReregisterConsumer (not RegisterConsumer) because OnStarted re-fires on SDK
		// reconnect with a fresh engine instance.
		consumer := ospfredistribute.NewConsumer(eng)
		// IPv6 redistributed routes originate OSPFv3 AS-External-LSAs through the v6 engine.
		consumer.SetV6Injector(eng6)
		if reg := getMetricsRegistry(); reg != nil {
			consumer.SetMetrics(reg)
		}
		if replaced := configredist.ReregisterConsumer(consumer); replaced {
			log.Info("ospf: rewired redistribution consumer for new engine instance")
		}
		eng.wireRedistProducer(ospfredistribute.NewSource(getEventBus()))

		// Watch the Loc-RIB default route so a conditional `default-information
		// originate` reacts live when a non-OSPF default appears or disappears, even
		// while OSPF is otherwise idle. Started before the idle check; the worker stops
		// on engine shutdown.
		eng.watchDefaultRoute()

		cfgMu.Lock()
		cfg := activeCfg
		cfgMu.Unlock()
		if !cfg.Present() {
			log.Warn("ospf: no config present, engine idle")
			return nil
		}
		eng.setConfig(cfg)
		eng.subscribeIfaceEvents(getEventBus())
		if err := eng.openInterfaces(); err != nil {
			return fmt.Errorf("ospf: opening interfaces: %w", err)
		}
		log.Info("ospf: engine started", "router-id", cfg.RouterID.String(), "interfaces", eng.transport.OpenInterfaceCount())
		if cfg.V6 != nil && cfg.V6.Present() {
			eng6.setConfig(*cfg.V6)
			eng6.subscribeIfaceEvents(getEventBus())
			if err := eng6.openInterfaces(); err != nil {
				return fmt.Errorf("ospf: opening ipv6 interfaces: %w", err)
			}
			log.Info("ospf: ipv6 family started", "router-id", cfg.V6.RouterID.String(), "interfaces", eng6.transport.OpenInterfaceCount())
		}
		return nil
	})

	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, any, error) {
		const statusDone = "done"
		switch command {
		case "show ospf":
			return statusDone, eng.processSummary(), nil
		case "show ospf neighbor":
			return statusDone, eng.neighborSnapshot(), nil
		case "show ospf interface":
			return statusDone, eng.interfaceSnapshot(), nil
		case "show ospf database":
			return statusDone, eng.databaseSnapshot(), nil
		case "show ospf database router", "show ospf database network", "show ospf database summary",
			"show ospf database asbr-summary", "show ospf database external", "show ospf database nssa-external":
			return statusDone, eng.databaseSnapshotByType(dbSubviewType[command]), nil
		case "show ospf route":
			return statusDone, eng.routeSnapshot(), nil
		case "show ospf border-routers":
			return statusDone, eng.borderRouterSnapshot(), nil
		case "show ospf spf":
			return statusDone, eng.spfSnapshot(), nil
		case "clear ospf process":
			return statusDone, clearResult{Action: "clear ospf process", Cleared: eng.clearProcess()}, nil
		case "clear ospf neighbor":
			return statusDone, clearResult{Action: "clear ospf neighbor", Cleared: eng.clearNeighbors()}, nil
		case "clear ospf counters":
			eng.clearCounters()
			return statusDone, clearResult{Action: "clear ospf counters", Cleared: 0}, nil
		default:
			return "error", "", fmt.Errorf("unknown command: %s", command)
		}
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{"ospf"},
		VerifyBudget: 1,
		ApplyBudget:  1,
		Commands: []sdk.CommandDecl{
			{Name: "show ospf"},
			{Name: "show ospf neighbor"},
			{Name: "show ospf interface"},
			{Name: "show ospf database"},
			{Name: "show ospf database router"},
			{Name: "show ospf database network"},
			{Name: "show ospf database summary"},
			{Name: "show ospf database asbr-summary"},
			{Name: "show ospf database external"},
			{Name: "show ospf database nssa-external"},
			{Name: "show ospf route"},
			{Name: "show ospf border-routers"},
			{Name: "show ospf spf"},
			{Name: "clear ospf process"},
			{Name: "clear ospf neighbor"},
			{Name: "clear ospf counters"},
		},
	})
	if err != nil {
		log.Error("ospf engine failed", "error", err)
		eng.shutdown()
		eng6.shutdown()
		return 1
	}
	eng.shutdown()
	eng6.shutdown()
	return 0
}
