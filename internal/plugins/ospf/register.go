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

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	ospfredistribute "github.com/ze-software/ze/internal/plugins/ospf/redistribute"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	ospfyang "github.com/ze-software/ze/internal/plugins/ospf/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

var (
	loggerPtr       atomic.Pointer[slog.Logger]
	eventBusPtr     atomic.Pointer[ze.EventBus]
	metricsPtr      atomic.Pointer[metrics.Registry]
	routeInstallPtr atomic.Pointer[sdk.Plugin]
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

// setRouteInstallClient records the SDK plugin handle SPF uses to ship computed
// routes to the engine when OSPF runs FORKED (locrib.Default() is nil in a
// subprocess). Set once at engine start, before the engines build their SPF
// installers. In-process this stays nil and installers write the local Loc-RIB.
func setRouteInstallClient(p *sdk.Plugin) {
	if p != nil {
		routeInstallPtr.Store(p)
	}
}

// routeInstallClient returns the forked route-install SDK handle, or nil in-process.
func routeInstallClient() *sdk.Plugin { return routeInstallPtr.Load() }

func registerOSPF() {
	_ = events.RegisterNamespace(Namespace, EventNeighborUp, EventNeighborDown, EventSPFRun, EventLSDBChange, EventInterfaceState, EventDRChange, EventNeighborChange)

	// Register the single "ospf" redistribution source at init (not OnStarted) so
	// `ze config validate` and editor completion of `import ospf` see it without the
	// engine running. The redistevents PRODUCER is registered transitively by the
	// ospfredistribute package import (its events sub-package init).
	ospfredistribute.RegisterOSPFSources()

	registerOSPFDiagnosticCodes()
	registerOSPFDoctor()
	registerOSPFIPsecDoctor()
	registerOSPFGracefulRestartDoctor()
	registerOSPFSegmentRoutingDoctor()
	registerOSPFDebugDoctor()
	// spec-ospf-ext-14: register the OSPFv3 base + extended LSA detail decoders (the codec's
	// own decoders) so `show ospf ipv6 database <type> detail` renders named bodies.
	registerV3BaseDecoders()

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
		ConfigureMetrics: func(reg metrics.Registry) {
			setMetricsRegistry(reg)
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			setEventBus(eb)
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
// (ai/rules/plugins.md). The doctor-ospf-raw-socket code is owned by
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
	{
		Code:        codeOSPFBFDPluginAbsent,
		Title:       "OSPF BFD enabled but BFD plugin not loaded",
		Description: "BFD (RFC 5880 / RFC 5881) is enabled on an OSPF interface but the BFD plugin is not loaded in this process. OSPF still forms adjacencies and detects loss on the Hello/Dead timers; sub-second BFD failure detection is unavailable until the `bfd` plugin runs.",
		Examples:    []string{"ze doctor --json", "ze explain doctor-ospf-bfd-plugin-absent"},
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
		Codes:        []string{codeOSPFRouterIDMissing, codeOSPFInterfaceAreaUnbound, codeOSPFBFDPluginAbsent},
		Check:        checkOSPFConfigSanity,
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		fmt.Fprintf(os.Stderr, "ospf: doctor config-sanity registration: %v\n", err)
		os.Exit(2)
	}
}

// registerOSPFGracefulRestartDoctor registers the RFC 3623 / RFC 5187 Graceful Restart NVS
// readiness check (doctor.go). It is a no-op unless the restarter is enabled and warns when
// the non-volatile restart-fact store is unwritable (spec-ospf-ext-9).
func registerOSPFGracefulRestartDoctor() {
	check := diagnostic.DoctorCheck{
		Name:         "ospf-graceful-restart-nvs",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        740, // just after the ospfv3 ipsec check (738)
		Component:    "ospf",
		Dependencies: []string{"config-tree", "blob-store"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{codeOSPFGracefulRestartNVS},
		Check:        checkOSPFGracefulRestartNVS,
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		fmt.Fprintf(os.Stderr, "ospf: doctor graceful-restart registration: %v\n", err)
		os.Exit(2)
	}
}

// registerOSPFSegmentRoutingDoctor registers the RFC 8665 / RFC 8666 Segment Routing
// readiness check (sr_doctor.go). It is a no-op unless SR is enabled and warns when the
// SRGB/SRLB label ranges are unsound (spec-ospf-ext-5 AC-21).
func registerOSPFSegmentRoutingDoctor() {
	check := diagnostic.DoctorCheck{
		Name:         "ospf-segment-routing",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        742, // just after the graceful-restart NVS check (740)
		Component:    "ospf",
		Dependencies: []string{"config-tree"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{codeOSPFSegmentRoutingOverlap},
		Check:        checkOSPFSegmentRouting,
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		fmt.Fprintf(os.Stderr, "ospf: doctor segment-routing registration: %v\n", err)
		os.Exit(2)
	}
}

// registerOSPFDebugDoctor registers the ext-14 debug-enabled sanity check (a Warning when
// debug LSA injection is left on, AC-25). It adds no new runtime dependency.
func registerOSPFDebugDoctor() {
	_ = diagnostic.Register(ospfDebugDoctorCode)
	check := diagnostic.DoctorCheck{
		Name:         "ospf-debug-enabled",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        744, // just after the segment-routing check (742)
		Component:    "ospf",
		Dependencies: []string{"config-tree"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{codeOSPFDebugEnabled},
		Check:        checkOSPFDebugEnabled,
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		fmt.Fprintf(os.Stderr, "ospf: doctor debug-enabled registration: %v\n", err)
		os.Exit(2)
	}
}

// registerOSPFIPsecDoctor registers the RFC 4552 IPsec readiness check (doctor_ipsec.go).
// It is a no-op unless an IPv6-family interface configures IPsec and warns when the kernel
// XFRM dataplane is unavailable (spec-ospf-ext-16 AC-12).
func registerOSPFIPsecDoctor() {
	check := diagnostic.DoctorCheck{
		Name:         "ospfv3-ipsec",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        738, // just after the ospfv3 raw-socket check (737)
		Component:    "ospf",
		Dependencies: []string{"config-tree", "netlink"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{codeOSPFv3IPsec},
		Check:        checkOSPFv3IPsec,
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		fmt.Fprintf(os.Stderr, "ospf: doctor ipsec registration: %v\n", err)
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

	// Ship SPF routes to the engine over RPC when OSPF runs FORKED (locrib.Default()
	// is nil in a subprocess). No-op in-process, where the installer writes the local
	// Loc-RIB directly. Set before wireV4Engine builds the SPF installer. (spec-forked-route-install)
	setRouteInstallClient(p)

	// wireV4Engine builds a fresh OSPFv2 engine over its own raw transport and applies the
	// shared metrics/event-bus and opaque-consumer wiring. RFC 6549 multi-instance stands up
	// one such engine per configured Instance ID; each demuxes its own Instance ID on its own
	// socket, so a multicast datagram reaches every instance and only the matching one
	// processes it.
	wireV4Engine := func() *engine {
		e := newEngine(transport.New(transport.NewBackend()))
		// Register the RFC 3630 / RFC 5392 TE opaque consumers (Opaque type 1 + 6) for the IPv4
		// engine (spec-ospf-ext-2). OSPFv3 TE is out of scope, so only the v4 engine registers.
		if err := registerTEConsumer(e); err != nil {
			log.Warn("ospf: TE opaque consumer registration", "error", err)
		}
		// Register the RFC 7770 Router Information opaque consumer (Opaque type 4) for the IPv4
		// engine (spec-ospf-ext-3). The OSPFv3 RI LSA is native (function code 12) and originated
		// by the v6 engine's self-LSA pass, so only the v4 engine registers the opaque consumer.
		if err := registerRIConsumer(e); err != nil {
			log.Warn("ospf: Router Information opaque consumer registration", "error", err)
		}
		// Register the RFC 7684 Extended Prefix (Opaque type 7) and Extended Link (Opaque type
		// 8) opaque consumers for the IPv4 engine (spec-ospf-ext-4). OSPFv3 uses RFC 8362
		// TLV-based LSAs, not opaque LSAs, so only the v4 engine registers.
		if err := registerExtConsumers(e); err != nil {
			log.Warn("ospf: Extended Prefix/Link opaque consumer registration", "error", err)
		}
		// Register the RFC 3623 Grace-LSA opaque consumer (Opaque type 3, link scope) for the
		// IPv4 engine (spec-ospf-ext-9). The OSPFv3 Grace-LSA is a native link-scope LSA (LS
		// Type 0x000B) delivered via the v6 LSUpdate scan, so only the v4 engine registers the
		// opaque consumer; the shared helper reacts in graceOnReceive.
		if err := registerGraceConsumer(e); err != nil {
			log.Warn("ospf: Graceful Restart opaque consumer registration", "error", err)
		}
		// Register the RFC 8665 Segment Routing TLV builders into the RI (Opaque type
		// 4) and Extended Prefix/Link (Opaque types 7/8) carriers (spec-ospf-ext-5).
		// The builders are process-global and shared with the OSPFv3 RI LSA, so
		// registration is idempotent-tolerant across the per-instance engine factory.
		if err := registerSRConsumer(e); err != nil {
			log.Warn("ospf: Segment Routing TLV registration", "error", err)
		}
		if reg := getMetricsRegistry(); reg != nil {
			e.transport.SetMetrics(reg)
			e.setMetrics(reg)
			e.setBFDMetrics(reg)
			// spec-ospf-ext-5: the ze_ospf_sr_* series are process-global (af-labeled),
			// registered once across all engine instances.
			setSRMetrics(reg)
			// spec-ospf-ext-14: the six ze_ospf_debug_* / ze_ospfv3_debug_* series are
			// process-global, registered once (NOT via setMetrics: the v6 series would trip
			// the ze_ospf_-only naming guard on setMetrics/setBFDMetrics).
			setDebugMetrics(reg)
		}
		if eb := getEventBus(); eb != nil {
			e.setEventSink(newEventSink(eb))
		}
		return e
	}

	eng := wireV4Engine()
	var mInstances metrics.Gauge
	if reg := getMetricsRegistry(); reg != nil {
		mInstances = reg.Gauge("ze_ospf_instances", "Number of configured OSPFv2 instances (RFC 6549), one full engine each.")
	}
	// The base (Instance 0) engine additionally owns redistribution, default-origination,
	// and the show/clear surface; the manager stands up one more engine per non-zero
	// Instance ID configured on an interface (RFC 6549). newEngine ignores the Instance ID
	// argument -- setConfig adopts it -- so the builder just constructs a standard engine.
	instances := newInstanceManager(eng, func(uint8) *engine { return wireV4Engine() }, mInstances)

	// The OSPFv3 (IPv6-transport) address families run as additional engine instances over
	// the v6 codec and the ospfv3 transport (RFC 5340/5838). Each configured `ospf {
	// address-family { <af> { ... } } }` maps to one v6 engine keyed by its Instance-ID
	// range; v6set spawns, reconciles, and stops them per config. Engines stay idle (no
	// sockets, no goroutines) until openInterfaces, so an unconfigured AF costs nothing.
	// Each spawned engine wires the shared metric registry (setMetrics), so its RFC 7770
	// ze_ospf_ri_* series publish under the af=v3 label (spec-ospf-ext-3), deduped by name.
	v6set := newV6EngineSet()

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
		// spec-ospf-ext-5: validate the SR ranges (SRGB/SRLB Range Size > 0,
		// non-overlap, label bounds, prefix-SID index within the SRGB).
		if err := validateSRConfig(sdkConfigSections(sections)); err != nil {
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
		instances.setConfig(cfg)
		v6set.configure(cfg.v6Families(), cfg.multiAF())
		// spec-ospf-ext-5: resolve the SR config into srWire keyed by Router ID so
		// the RI/Extended TLV builders originate SR TLVs (both address families).
		applySRConfig(sdkConfigSections(sections), cfg)
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
		instances.reconcile(cfg)
		v6set.apply(cfg.v6Families(), cfg.multiAF())
		return nil
	})

	p.OnStarted(func(_ context.Context) error {
		// Wire redistribution before the idle check so a `redistribute { destination
		// ospf { import <source> } }` rule has a consumer even when OSPF is idle.
		// ReregisterConsumer (not RegisterConsumer) because OnStarted re-fires on SDK
		// reconnect with a fresh engine instance.
		consumer := ospfredistribute.NewConsumer(eng)
		// IPv6 redistributed routes originate OSPFv3 AS-External-LSAs through the default
		// IPv6-unicast engine; IPv4 routes divert to the IPv4-over-OSPFv3 engine ONLY when
		// that AF is actually running (RFC 5838 §2.7, AC-13). The injectors resolve the live
		// engine from v6set, and the IPv4-over-v3 injector reports itself inactive when its
		// AF engine is absent, so injectorFor falls back to the OSPFv2 engine and IPv4
		// redistribution still originates a Type 5 (regression guard, ext-15 review fix 1).
		consumer.SetV6Injector(v6InjectorAF{set: v6set, af: afIPv6Unicast})
		consumer.SetV4OverV3Injector(v6InjectorAF{set: v6set, af: afIPv4Unicast})
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
		// Stand up the base (Instance 0) engine and one engine per configured non-zero
		// Instance ID (RFC 6549), each subscribing to interface events and opening its own
		// transport. The base engine keeps the redistribution consumer and default-route
		// watcher wired above.
		if err := instances.start(cfg); err != nil {
			return err
		}
		log.Info("ospf: engine started", "router-id", cfg.RouterID.String(), "interfaces", eng.transport.OpenInterfaceCount())
		// Bring up every configured OSPFv3 address family (RFC 5838): one engine per AF.
		if err := v6set.start(cfg.v6Families(), cfg.multiAF()); err != nil {
			return fmt.Errorf("ospf: opening ipv6 interfaces: %w", err)
		}
		return nil
	})

	p.OnExecuteCommand(func(_, command string, cmdArgs []string, _ string) (string, any, error) {
		const statusDone = "done"
		const statusError = "error"
		switch command {
		case "show ospf":
			return statusDone, eng.processSummary(), nil
		case "show ospf ipv6":
			// RFC 5838 §2: identify each OSPFv3 address-family instance (AF + Instance ID).
			return statusDone, v6set.afSummary(), nil
		case "show ospf ipv6 interface":
			// RFC 4552 (spec-ospf-ext-16): the default IPv6-unicast AF engine's per-interface
			// IPsec status (protocol/SPI/installed), never the key. Empty when that AF is idle.
			if v6eng, ok := v6set.engineFor(afIPv6Unicast); ok {
				return statusDone, v6eng.ipsecInterfaceSnapshot(), nil
			}
			return statusDone, []any{}, nil
		case "show ospf instance":
			return statusDone, instances.instanceSnapshot(), nil
		case "show ospf neighbor":
			return statusDone, eng.neighborSnapshot(), nil
		case "show ospf interface":
			return statusDone, eng.interfaceSnapshot(), nil
		case "show ospf database":
			return statusDone, eng.databaseSnapshot(), nil
		case "show ospf database router", "show ospf database network", "show ospf database summary",
			"show ospf database asbr-summary", "show ospf database external", "show ospf database nssa-external",
			"show ospf database opaque-link":
			return statusDone, eng.databaseSnapshotByType(dbSubviewType[command]), nil
		case "show ospf database opaque-area":
			// RFC 3630 / RFC 5392: decode any TE LSA body inline (AC-16), not as raw hex.
			// RFC 7684 (spec-ospf-ext-4): also decode Extended Prefix/Link (Opaque Type 7/8).
			out := eng.databaseOpaqueWithTEDecode(dbSubviewType[command], OpaqueScopeArea)
			return statusDone, eng.appendExtOpaqueDecode(out, OpaqueScopeArea), nil
		case "show ospf database opaque-as":
			out := eng.databaseOpaqueWithTEDecode(dbSubviewType[command], OpaqueScopeAS)
			return statusDone, eng.appendExtOpaqueDecode(out, OpaqueScopeAS), nil
		case cmdShowDatabaseRI:
			// RFC 7770: decode the RI LSA bodies for both address families (OSPFv2 opaque
			// type 4 from eng, OSPFv3 function code 12 from the default IPv6-unicast AF
			// engine) into capability bits + TLVs. The v6 engine is nil when no v6 AF runs.
			v6ri, _ := v6set.engineFor(afIPv6Unicast)
			return statusDone, riDatabaseSnapshot(eng, v6ri), nil
		case "show ospf te-database":
			return statusDone, eng.teDatabaseSnapshot(), nil
		case "show ospf route":
			return statusDone, eng.routeSnapshot(), nil
		case "show ospf route fast-reroute":
			// spec-ospf-ext-6: RFC 5286 / TI-LFA per-prefix primary + backup + class.
			return statusDone, eng.fastRerouteSnapshot(), nil
		case "show ospf virtual-links":
			return statusDone, eng.virtualLinkSnapshot(), nil
		case "show ospf border-routers":
			return statusDone, eng.borderRouterSnapshot(), nil
		case "show ospf spf":
			return statusDone, eng.spfSnapshot(), nil
		case "show ospf ldp-sync":
			return statusDone, eng.ldpSyncSnapshot(), nil
		case cmdShowGracefulRestart:
			// RFC 3623: OSPFv2 Graceful Restart state (restarter + helper) from the base engine.
			return statusDone, eng.grSnapshot(), nil
		case cmdShowIPv6GracefulRestart:
			// RFC 5187: OSPFv3 Graceful Restart state from the default IPv6-unicast AF engine.
			if v6eng, ok := v6set.engineFor(afIPv6Unicast); ok {
				return statusDone, v6eng.grSnapshot(), nil
			}
			return statusDone, grShowSnapshot{Family: interfaceFamilyIPv6, Helpers: []grHelperView{}}, nil
		case cmdShowSegmentRouting:
			// RFC 8665: OSPFv2 Segment Routing state (SRGB/SRLB, Prefix-SIDs, Adj-SIDs).
			return statusDone, eng.srSnapshot(interfaceFamilyIPv4), nil
		case cmdShowIPv6SegmentRouting:
			// RFC 8666: OSPFv3 Segment Routing state. The RI capabilities are shared with
			// the IPv4 family (RFC 8666 §4); render them keyed to the same Router ID.
			if v6eng, ok := v6set.engineFor(afIPv6Unicast); ok {
				return statusDone, v6eng.srSnapshot(interfaceFamilyIPv6), nil
			}
			return statusDone, eng.srSnapshot(interfaceFamilyIPv6), nil
		case "clear ospf process":
			return statusDone, clearResult{Action: "clear ospf process", Cleared: eng.clearProcess()}, nil
		case "clear ospf neighbor":
			return statusDone, clearResult{Action: "clear ospf neighbor", Cleared: eng.clearNeighbors()}, nil
		case "clear ospf counters":
			eng.clearCounters()
			return statusDone, clearResult{Action: "clear ospf counters", Cleared: 0}, nil
		case cmdGRPrepare:
			// RFC 3623 sec 2.1: operator-triggered planned graceful restart on the base
			// (IPv4/OSPFv2) engine. Runs against live engine state, so it forwards here.
			return statusDone, eng.grPrepare(), nil

		// spec-ospf-ext-14 IPv4 deep-introspection views.
		case "show ospf database opaque-area detail":
			return statusDone, eng.opaqueDetailSnapshot(OpaqueScopeArea), nil
		case "show ospf database opaque-as detail":
			return statusDone, eng.opaqueDetailSnapshot(OpaqueScopeAS), nil
		case "show ospf database opaque-link detail":
			return statusDone, eng.opaqueDetailSnapshot(OpaqueScopeLink), nil
		case "show ospf spf detail":
			return statusDone, eng.spfExplainSnapshot(), nil
		case "show ospf neighbor detail":
			return statusDone, eng.neighborDetailSnapshot(), nil
		case "show ospf interface detail":
			return statusDone, eng.interfaceDetailSnapshot(), nil

		// spec-ospf-ext-14 IPv6 deep-introspection views (routed to the v6 engine instance).
		case "show ospf ipv6 database", "show ospf ipv6 database detail":
			return statusDone, v6DatabaseDetail(v6set, "", ""), nil
		case "show ospf ipv6 database router detail":
			return statusDone, v6DatabaseDetail(v6set, "router", ""), nil
		case "show ospf ipv6 database scope link":
			return statusDone, v6DatabaseDetail(v6set, "", "link"), nil
		case "show ospf ipv6 database scope area":
			return statusDone, v6DatabaseDetail(v6set, "", "area"), nil
		case "show ospf ipv6 database scope as":
			return statusDone, v6DatabaseDetail(v6set, "", "as"), nil
		case "show ospf ipv6 database router-information":
			v6ri, _ := v6set.engineFor(afIPv6Unicast)
			return statusDone, riDatabaseSnapshot(nil, v6ri), nil
		case "show ospf ipv6 database extended":
			return statusDone, v6DatabaseExtended(v6set), nil
		case "show ospf ipv6 database segment-routing":
			if v6eng, ok := v6set.engineFor(afIPv6Unicast); ok {
				return statusDone, v6eng.srSnapshot(interfaceFamilyIPv6), nil
			}
			return statusDone, eng.srSnapshot(interfaceFamilyIPv6), nil
		case "show ospf ipv6 instance":
			return statusDone, v6set.instanceListing(), nil
		case "show ospf ipv6 neighbor":
			if v6eng, ok := v6set.engineFor(afIPv6Unicast); ok {
				return statusDone, v6eng.neighborSnapshot(), nil
			}
			return statusDone, []any{}, nil
		case "show ospf ipv6 neighbor detail":
			if v6eng, ok := v6set.engineFor(afIPv6Unicast); ok {
				return statusDone, v6eng.v3NeighborDetailSnapshot(), nil
			}
			return statusDone, []any{}, nil
		case "show ospf ipv6 interface detail":
			if v6eng, ok := v6set.engineFor(afIPv6Unicast); ok {
				return statusDone, v6eng.v3InterfaceDetailSnapshot(), nil
			}
			return statusDone, []any{}, nil
		case "show ospf ipv6 spf":
			if v6eng, ok := v6set.engineFor(afIPv6Unicast); ok {
				return statusDone, v6eng.spfSnapshot(), nil
			}
			return statusDone, []any{}, nil
		case "show ospf ipv6 spf detail":
			if v6eng, ok := v6set.engineFor(afIPv6Unicast); ok {
				return statusDone, v6eng.spfExplainSnapshot(), nil
			}
			return statusDone, []any{}, nil

		// spec-ospf-ext-14 guarded LSA injection (both families) + the shared enablement.
		case "debug ospf inject enable":
			setDebugInjectEnabled(true)
			return statusDone, debugEnableResult{Action: "enable", Enabled: true}, nil
		case "debug ospf inject disable":
			setDebugInjectEnabled(false)
			return statusDone, debugEnableResult{Action: "disable", Enabled: false}, nil
		case "debug ip ospf inject opaque":
			res, err := eng.debugInjectOpaque(cmdArgs)
			if err != nil {
				return statusError, "", err
			}
			return statusDone, res, nil
		case "debug ipv6 ospf inject lsa":
			v6eng, ok := v6set.engineFor(afIPv6Unicast)
			if !ok {
				return statusError, "", errNoV6Engine
			}
			res, err := v6eng.debugInjectV3(cmdArgs)
			if err != nil {
				return statusError, "", err
			}
			return statusDone, res, nil

		default:
			return statusError, "", fmt.Errorf("unknown command: %s", command)
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
			{Name: "show ospf ipv6"},
			{Name: "show ospf ipv6 interface"},
			{Name: "show ospf instance"},
			{Name: "show ospf neighbor"},
			{Name: "show ospf interface"},
			{Name: "show ospf database"},
			{Name: "show ospf database router"},
			{Name: "show ospf database network"},
			{Name: "show ospf database summary"},
			{Name: "show ospf database asbr-summary"},
			{Name: "show ospf database external"},
			{Name: "show ospf database nssa-external"},
			{Name: "show ospf database opaque-link"},
			{Name: "show ospf database opaque-area"},
			{Name: "show ospf database opaque-as"},
			{Name: cmdShowDatabaseRI},
			{Name: "show ospf te-database"},
			{Name: "show ospf route"},
			{Name: "show ospf route fast-reroute"},
			{Name: "show ospf virtual-links"},
			{Name: "show ospf border-routers"},
			{Name: "show ospf spf"},
			{Name: "show ospf ldp-sync"},
			{Name: cmdShowGracefulRestart},
			{Name: cmdShowIPv6GracefulRestart},
			{Name: cmdShowSegmentRouting},
			{Name: cmdShowIPv6SegmentRouting},
			{Name: "clear ospf process"},
			{Name: "clear ospf neighbor"},
			{Name: "clear ospf counters"},
			{Name: cmdGRPrepare},
			// spec-ospf-ext-14 IPv4 deep-introspection views.
			{Name: "show ospf database opaque-area detail"},
			{Name: "show ospf database opaque-as detail"},
			{Name: "show ospf database opaque-link detail"},
			{Name: "show ospf spf detail"},
			{Name: "show ospf neighbor detail"},
			{Name: "show ospf interface detail"},
			// spec-ospf-ext-14 IPv6 deep-introspection views.
			{Name: "show ospf ipv6 database"},
			{Name: "show ospf ipv6 database detail"},
			{Name: "show ospf ipv6 database router detail"},
			{Name: "show ospf ipv6 database scope link"},
			{Name: "show ospf ipv6 database scope area"},
			{Name: "show ospf ipv6 database scope as"},
			{Name: "show ospf ipv6 database router-information"},
			{Name: "show ospf ipv6 database extended"},
			{Name: "show ospf ipv6 database segment-routing"},
			{Name: "show ospf ipv6 instance"},
			{Name: "show ospf ipv6 neighbor"},
			{Name: "show ospf ipv6 neighbor detail"},
			{Name: "show ospf ipv6 interface detail"},
			{Name: "show ospf ipv6 spf"},
			{Name: "show ospf ipv6 spf detail"},
			// spec-ospf-ext-14 guarded LSA injection (both families) + shared enablement.
			{Name: "debug ospf inject enable"},
			{Name: "debug ospf inject disable"},
			{Name: "debug ip ospf inject opaque"},
			{Name: "debug ipv6 ospf inject lsa"},
		},
	})
	if err != nil {
		log.Error("ospf engine failed", "error", err)
		instances.shutdownAll()
		v6set.shutdownAll()
		return 1
	}
	instances.shutdownAll()
	v6set.shutdownAll()
	return 0
}
