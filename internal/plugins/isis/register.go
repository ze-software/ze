// Design: docs/architecture/isis/isis-4-component-config.md -- IS-IS component registration
// Related: config.go -- typed config resolution
// Related: server.go -- engine orchestration + PDU dispatcher
// Related: events.go -- event namespace + typed handles
//
// Package isis implements native IS-IS (ISO/IEC 10589, RFC 1195 / 5305 / 5301),
// a link-state interior gateway protocol that runs directly over Layer 2. This
// file is the component wiring backbone (spec-isis-4): it registers the
// component in the plugin registry, embeds ze-isis-conf.yang, and runs the SDK
// lifecycle (verify/configure/apply/start/command). Runtime behavior
// (adjacency, LSDB, flooding, SPF, route install) is delivered by sibling specs;
// here the per-circuit goroutines are stubs and the show commands return stubs.
package isis

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
	isisredistribute "github.com/ze-software/ze/internal/plugins/isis/redistribute"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	isisyang "github.com/ze-software/ze/internal/plugins/isis/yang"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

var (
	loggerPtr       atomic.Pointer[slog.Logger]
	eventBusPtr     atomic.Pointer[ze.EventBus]
	metricsPtr      atomic.Pointer[metrics.Registry]
	routeInstallPtr atomic.Pointer[sdk.Plugin]
)

func init() {
	loggerPtr.Store(slogutil.DiscardLogger())
}

// logger returns the component logger (a discard logger until configured).
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

// setRouteInstallClient records the SDK plugin handle SPF uses to ship computed
// routes to the engine when IS-IS runs FORKED (locrib.Default() is nil in a
// subprocess). Set once at engine start, before initSPF builds the installers.
// In-process this stays nil and installers write the local Loc-RIB.
func setRouteInstallClient(p *sdk.Plugin) {
	if p != nil {
		routeInstallPtr.Store(p)
	}
}

// routeInstallClient returns the forked route-install SDK handle, or nil in-process.
func routeInstallClient() *sdk.Plugin { return routeInstallPtr.Load() }

func getMetricsRegistry() metrics.Registry {
	p := metricsPtr.Load()
	if p == nil {
		return nil
	}
	return *p
}

// registerISIS builds and registers the IS-IS component. It registers the event
// namespace, the NET/system-id completion hooks (the ValidateFns live centrally
// per the mac-address precedent), and the registry.Registration with the
// embedded YANG. Called from init().
func registerISIS() {
	_ = events.RegisterNamespace(Namespace, EventSessionUp, EventSessionDown, EventLSPChange)

	// Register the SINGLE IS-IS redistribute config source "isis" at init so the
	// `redistribute-source` YANG validator and editor completion accept `import
	// isis` during `ze config validate` -- which links in all components but does
	// NOT start the IS-IS engine, so an OnStarted-only registration would be too
	// late (AC-2 / AC-9). Idempotent (sync.Once). The redistevents PRODUCER is
	// registered separately by importing the redistribute/events package (done via
	// the redistribute package import below); see isis-11 producer wiring.
	isisredistribute.RegisterISISSources()

	// Self-contained CLI completion for the NET and system-id leaves: the central
	// config package owns the ValidateFn (config can't import isis without a
	// cycle), but the completion guidance is registered here so the IS-IS package
	// owns its own CompleteFn (mac-address precedent, ai/rules/plugins.md).
	configyang.RegisterCompleteFn("isis-net", netCompletions)
	configyang.RegisterCompleteFn("isis-system-id", systemIDCompletions)

	// Register the IS-IS config-sanity diagnostic codes (codes.go) this component
	// OWNS, so `ze explain` can describe them and deleting the component removes
	// them (they are not in the central diagnostic.builtinCodes slice;
	// ai/rules/plugins.md). The raw-socket code is owned/listed by
	// the transport, never here (one code, one owner).
	registerISISDiagnosticCodes()

	// Register the IS-IS config-sanity doctor checks (doctor.go). The raw-socket
	// check is registered separately by the transport (isis-3); this adds only the
	// two config-sanity codes, never the raw-socket code (one code, one owner).
	registerISISDoctor()

	reg := registry.Registration{
		Name:         "isis",
		Description:  "Intermediate System to Intermediate System (ISO/IEC 10589, RFC 1195): native link-state IGP",
		Features:     "yang",
		YANG:         isisyang.ZeIsisConfYANG,
		ConfigRoots:  []string{"isis"},
		Dependencies: []string{"fib-kernel", "sysctl"},
		RFCs:         []string{"1195", "5301", "5303", "5305", "5308", "5310"},
		RunEngine:    runISISEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
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
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "isis: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func init() { registerISIS() }

// registerISISDiagnosticCodes registers the IS-IS config-sanity diagnostic codes
// (metadata in codes.go) the component OWNS, so `ze explain <code>` can describe
// them. They are deliberately NOT in the central diagnostic.builtinCodes slice:
// owning them here means deleting the IS-IS component removes the codes with it
// (ai/rules/plugins.md). A duplicate registration is benign (the
// code is already explainable), so the error is ignored like RegisterBuiltinCodes
// does; this function is the only registrant of these two codes.
func registerISISDiagnosticCodes() {
	for _, m := range isisDiagnosticCodes {
		_ = diagnostic.Register(m)
	}
}

// registerISISDoctor registers the IS-IS config-sanity doctor check
// (checkISISConfigSanity in doctor.go). It lives in register.go (not doctor.go)
// so the registration side effect, and the os.Exit on a registration failure,
// are confined to the component's registration file (ai/patterns/registration.md).
func registerISISDoctor() {
	check := diagnostic.DoctorCheck{
		Name:         "isis-config-sanity",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        731, // just after the isis-3 raw-socket check (730)
		Component:    "isis",
		Dependencies: []string{"config-tree"}, // reads the parsed isis config block
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{codeNETMissing, codeSystemIDMismatch},
		Check:        checkISISConfigSanity,
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		fmt.Fprintf(os.Stderr, "isis: doctor config-sanity registration: %v\n", err)
		os.Exit(2)
	}
}

// netCompletions returns example NETs for `net` tab-completion. There is no
// enumerable set, so it offers a canonical template the operator can edit.
func netCompletions() []string {
	return []string{"49.0001.0000.0000.0001.00"}
}

// systemIDCompletions returns an example System ID for `system-id` completion.
func systemIDCompletions() []string {
	return []string{"0000.0000.0001"}
}

// toConfigSections converts the SDK-delivered sections into the config package's
// SDK-free section shape.
func toConfigSections(sections []sdk.ConfigSection) []configSection {
	out := make([]configSection, len(sections))
	for i, s := range sections {
		out[i] = configSection{Root: s.Root, Data: s.Data}
	}
	return out
}

// runISISEngine is the SDK lifecycle entry point (registry.RunEngine). It wires
// the verify/configure/apply/start/command callbacks and runs the plugin until
// shutdown, then tears the engine down cleanly.
func runISISEngine(conn net.Conn) int {
	log := logger()
	log.Debug("isis engine starting")

	p := sdk.NewWithConn("isis", conn)
	defer func() { _ = p.Close() }()

	// Ship SPF routes to the engine over RPC when IS-IS runs FORKED (locrib.Default()
	// is nil in a subprocess). No-op in-process. Set before newEngine builds the SPF
	// installers via initSPF. (spec-forked-route-install)
	setRouteInstallClient(p)

	eng := newEngine(transport.New(transport.NewBackend()))
	// Forward the metrics registry to the transport, which OWNS and registers the
	// ze_isis_frames_* / ze_isis_sockets_open series (umbrella Metrics table,
	// owner isis-3). isis-4 only wires the registry through; the other ze_isis_*
	// series are registered per-owner by the runtime siblings. The adjacency
	// series ze_isis_adjacencies_up / ze_isis_adjacencies_total are owned and
	// registered HERE (isis-5) via eng.setMetrics.
	if reg := getMetricsRegistry(); reg != nil {
		eng.transport.SetMetrics(reg)
		eng.setMetrics(reg)
	}
	// Wire the session up/down event sink so circuits emit on the IS-IS events
	// namespace (isis-5). nil bus leaves a no-op sink.
	if eb := getEventBus(); eb != nil {
		eng.setEventSink(newEventSink(eb))
	}

	var (
		cfgMu       sync.Mutex
		activeCfg   Config
		pendingCfg  Config
		havePending bool
	)

	// OnConfigVerify: parse-check only, rejecting malformed config (no NET, bad
	// NET, system-id mismatch) before any state mutation (AC-3/AC-4). The
	// verified config is stashed so OnConfigApply (the reload-commit step) can
	// reconcile to it.
	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		cfg, err := parseISISConfig(toConfigSections(sections))
		if err != nil {
			return err
		}
		if cfg.Present() {
			if verr := validateConfig(cfg); verr != nil {
				return verr
			}
		}
		cfgMu.Lock()
		pendingCfg = cfg
		havePending = true
		cfgMu.Unlock()
		return nil
	})

	// OnConfigure: startup-only delivery; resolve the typed config and stage it
	// (AC-2). It does not open circuits (that is OnStarted).
	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseISISConfig(toConfigSections(sections))
		if err != nil {
			return err
		}
		cfgMu.Lock()
		activeCfg = cfg
		cfgMu.Unlock()
		eng.setConfig(cfg)
		return nil
	})

	// OnConfigApply: the reload-commit step (OnConfigure does not fire on reload).
	// Adopt the verified pending config and reconcile circuits incrementally so a
	// changed interface metric does not flap unrelated circuits (AC-8).
	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		cfgMu.Lock()
		if havePending {
			activeCfg = pendingCfg
			havePending = false
		}
		cfg := activeCfg
		cfgMu.Unlock()
		eng.reconcile(cfg)
		return nil
	})

	// OnStarted: open a circuit per enabled interface via the spec-isis-3
	// transport and launch the per-circuit goroutine stubs (AC-1). A config with
	// no NET leaves the engine idle (like LDP with no lsr-id).
	p.OnStarted(func(_ context.Context) error {
		// Redistribution consumer + producer wiring (isis-11), done BEFORE the idle
		// check so the `redistribute { destination isis { import ... } }` config has
		// a consumer to dispatch to even when the engine is idle. The SINGLE config
		// source "isis" is registered at init (registerISIS) so it validates without
		// the engine running; the redistevents PRODUCER is registered by importing
		// the events package (source.go does). The consumer writes imported routes
		// into the engine's LSPs (eng implements LSPInjector). The producer Source
		// emits SPF route changes via the SPF Computer OnChange hook (IS-IS -> BGP).
		consumer := isisredistribute.NewConsumer(eng)
		if reg := getMetricsRegistry(); reg != nil {
			consumer.SetMetrics(reg)
		}
		// Idempotent rewire: OnStarted can re-fire on an SDK reconnect, creating a
		// fresh engine instance. A plain RegisterConsumer would then fail with
		// ErrConsumerConflict and redistribution into IS-IS would silently stop for
		// the new instance. ReregisterConsumer replaces the stale consumer so the
		// new engine receives redistributed routes (isis-11).
		if replaced := configredist.ReregisterConsumer(consumer); replaced {
			log.Info("isis: rewired redistribution consumer for new engine instance")
		}
		eng.wireRedistProducer(isisredistribute.NewSource(getEventBus()))

		cfgMu.Lock()
		cfg := activeCfg
		cfgMu.Unlock()
		if !cfg.Present() {
			log.Warn("isis: no net configured, engine idle")
			return nil
		}
		eng.setConfig(cfg)
		// Wire link up/down so a configured interface that comes up after start
		// opens its circuit (and a down link closes it).
		eng.subscribeIfaceEvents(getEventBus())
		if err := eng.openCircuits(); err != nil {
			return fmt.Errorf("isis: opening circuits: %w", err)
		}
		log.Info("isis: engine started",
			"system-id", cfg.SystemID.String(),
			"level", cfg.Level.String(),
			"circuits", eng.transport.OpenCircuitCount())
		return nil
	})

	// OnExecuteCommand: dispatch the full `show isis <noun>` / `clear isis
	// <action>` surface (spec-isis-13). Each show command returns a read-only
	// snapshot the siblings produce; the clear actions mutate runtime state and
	// return a status payload. The CLI proxy (cmd_show.go) forwards a fixed
	// command string here, so the switch is exhaustive over the registered set.
	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, any, error) {
		const statusDone = "done"
		switch command {
		case "show isis neighbor":
			return statusDone, eng.neighborSnapshot(), nil
		case "show isis database":
			return statusDone, eng.databaseSnapshot(), nil
		case "show isis database detail":
			return statusDone, eng.databaseDetailSnapshot(), nil
		case "show isis route":
			return statusDone, eng.routeSnapshot(), nil
		case "show isis route ipv6":
			// IPv6 route table (isis-12); isis-13 refines the grammar/rendering.
			return statusDone, eng.routeSnapshotV6(), nil
		case "show isis interface":
			return statusDone, eng.interfaceSnapshot(), nil
		case "show isis hostname":
			// System ID -> dynamic hostname mapping (TLV 137, RFC 5301).
			return statusDone, eng.hostnameSnapshot(), nil
		case "show isis spf-log":
			return statusDone, eng.spfLogView(), nil
		case "clear isis adjacency":
			n := eng.clearAdjacencies()
			return statusDone, clearResult{Action: "clear isis adjacency", Cleared: n}, nil
		case "clear isis counters":
			eng.clearCounters()
			return statusDone, clearResult{Action: "clear isis counters", Cleared: 0}, nil
		default:
			return "error", "", fmt.Errorf("unknown command: %s", command)
		}
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{"isis"},
		VerifyBudget: 1,
		ApplyBudget:  1,
		Commands: []sdk.CommandDecl{
			{Name: "show isis neighbor"},
			{Name: "show isis database"},
			{Name: "show isis database detail"},
			{Name: "show isis route"},
			{Name: "show isis route ipv6"},
			{Name: "show isis interface"},
			{Name: "show isis hostname"},
			{Name: "show isis spf-log"},
			{Name: "clear isis adjacency"},
			{Name: "clear isis counters"},
		},
	})
	if err != nil {
		log.Error("isis engine failed", "error", err)
		eng.shutdown()
		return 1
	}

	eng.shutdown()
	return 0
}

// clearResult is the status payload a `clear isis <action>` returns: the action
// performed and how many records it affected (adjacencies dropped; 0 for a
// counter reset). Flat value, JSON-tagged so the pipe machinery and the web
// disconnect action read it uniformly (spec-isis-13 AC-8).
type clearResult struct {
	Action  string `json:"action"`
	Cleared int    `json:"cleared"`
}
