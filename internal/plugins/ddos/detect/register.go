package detect

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/trafficstat"
	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/statestore"
	detectyang "github.com/ze-software/ze/internal/plugins/ddos/detect/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
	"github.com/ze-software/ze/pkg/zefs"
)

var eventBusPtr atomic.Pointer[ze.EventBus]

// The registration names of the two flow sources detection characterizes an
// attack from (characterize.go). Either one meets the need.
const (
	flowExportPlugin   = "flow-export"
	trafficUsagePlugin = "traffic-usage"
)

func loadBus() (ze.EventBus, error) {
	p := eventBusPtr.Load()
	if p == nil {
		return nil, fmt.Errorf("ddos-detect: event bus not configured")
	}
	return *p, nil
}

func ddosDetectSends() []rpc.MessageSendDecl {
	return []rpc.MessageSendDecl{
		{Message: events.NamespaceCLI + ":show traffic usage"},
		{Message: events.NamespaceCLI + ":show flow recent"},
		{Message: ddosevent.Namespace + ":attack-detected"},
		{Message: ddosevent.Namespace + ":attack-characterized"},
		{Message: ddosevent.Namespace + ":attack-ongoing"},
		{Message: ddosevent.Namespace + ":attack-cleared"},
	}
}

func init() {
	statestore.RegisterPluginKeys(Name, zefs.KeyDDoSDetectBaseline)

	reg := registry.Registration{
		Name:         Name,
		Description:  "Automatic DDoS attack detector with two-stage detection",
		Features:     "yang",
		YANG:         detectyang.ZeDdosDetectConfYANG,
		ConfigRoots:  []string{configRoot},
		Dependencies: []string{"interface"},
		// Detection with no flow source compiled in has nothing to
		// characterize an attack from, so a build that carries neither refuses
		// `ddos detect enabled true`. The need is about the build only:
		// whether a source is configured stays the ddos-detect-flow-source
		// doctor warning below. The need names two sources, so they are
		// alternatives and the need pulls neither: the source the operator
		// configured loads from its own config root (registry
		// optionalProviderPulled).
		OptionalDependencies: []string{flowExportPlugin, trafficUsagePlugin},
		ConfigNeeds: []registry.ConfigNeed{{
			Path:      []string{"ddos", "detect", "enabled"},
			Value:     "true",
			Providers: []string{flowExportPlugin, trafficUsagePlugin},
		}},
		Sends:     ddosDetectSends(),
		RunEngine: runEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			eventBusPtr.Store(&eb)
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			setMetricsRegistry(reg)
		},
		DoctorChecks: []registry.DoctorCheckDef{{
			Name:         "ddos-detect-flow-source",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        760,
			Dependencies: []string{"config-loaded"},
			Platforms:    []string{"any"},
			Codes:        []string{codeDoctorDDoSDetectNoFlowSource},
			Check:        checkFlowSource,
		}},
	}
	reg.CLIHandler = func(_ []string) int { return 1 }
	if err := registry.Register(reg); err != nil {
		panic("BUG: ddos-detect: registration failed: " + err.Error())
	}
}

// warnIfExternal logs a warning when ddos-detect is not running in-process.
// The subscribe path below (trafficstat.EnsureGlobal/SubscribeRates)
// registers a callback into a package-level subscriber list as a plain Go
// function call, not through DirectBridge/DispatchCommand -- that only
// reaches the real background tracker when this plugin shares process
// memory with it. An external ddos-detect never receives a rate signal: its
// own process's trafficstat service ticks over an empty interface list.
//
// Unlike traffic-usage/flow-export (which refuse to start entirely when
// external, since their whole purpose depends on the same-process call
// succeeding), ddos-detect warns rather than refuses, and the
// ddos-detect-rate-feed doctor check (blind.go) reports the cause for as
// long as the detector stays armed. Today an external ddos-detect never
// arms: loadBus refuses OnConfigure with "ddos-detect: event bus not
// configured", because only the in-process spawn calls ConfigureEventBus
// (plan/release/fixes/spec-plugin-event-bus-across-the-process-boundary.md).
func warnIfExternal(isInternal bool) {
	if isInternal {
		return
	}
	logger().Warn("ddos-detect: running as an external plugin process -- the trafficstat subscription is a same-process call that cannot reach the engine's real background tracker, so the detector will never receive a rate signal; remove the `bus { endpoint ddos-detect { external { ... } } }` block so ze runs ddos-detect internal",
		"code", codeDoctorDDoSDetectBlind)
}

// byeSaveBudget bounds the baseline save a bye performs. It sits below
// pluginRemovalGrace, the 500 ms the engine waits for the bye answer before it
// refuses the reload (internal/component/plugin/server/startup_removal.go), so
// a slow store costs the newest baseline samples, which the last periodic save
// already holds up to its interval, and never the removal.
const byeSaveBudget = 400 * time.Millisecond

func runEngine(conn net.Conn) int {
	log := logger()
	log.Debug("ddos-detect plugin starting")

	p := sdk.NewWithConn(Name, conn)
	warnIfExternal(p.IsInternal())
	defer func() { _ = p.Close() }()

	// Report-bus queue over the SDK connection; Stop runs after stopCurrent
	// finishes (including final baseline saves) and before p.Close().
	issues := issuequeue.Start(p)
	defer issues.Stop()

	var (
		detMu     sync.Mutex
		det       *detector
		rateSubID int
		started   atomic.Bool
	)
	// armed is the detector the doctor callback judges: the subscribed one, or
	// nil while ddos detect is disabled. The callback runs on the SDK's own
	// goroutine, so it reads this pointer and never det.
	var armed atomic.Pointer[detector]

	// subscribeLocked wires the detector to the trafficstat service, then arms the
	// blind-feed watch (blind.go) and publishes the detector to the doctor
	// callback. Caller MUST hold detMu.
	//
	// EnsureGlobal (not Global): the trafficstat service is created lazily and
	// nothing else guarantees it exists at detector-config time. EnsureGlobal
	// creates it on first use, so it never answers nil.
	internal := p.IsInternal()
	subscribeLocked := func(d *detector) {
		svc := trafficstat.EnsureGlobal()
		if svc == nil {
			panic("BUG: trafficstat.EnsureGlobal returned no service")
		}
		rateSubID = svc.SubscribeRates(d.onRates)
		d.arm(feedTrafficstat, internal)
		log.Info("ddos-detect: enabled, subscribing to trafficstat")
		armed.Store(d)
	}

	// unsubscribeLocked removes the trafficstat subscription. Caller MUST hold detMu.
	unsubscribeLocked := func() {
		armed.Store(nil)
		if rateSubID != 0 {
			if svc := trafficstat.Global(); svc != nil {
				svc.UnsubscribeRates(rateSubID)
			}
			rateSubID = 0
		}
	}

	stopCurrent := func(ctx context.Context, clearState bool) {
		detMu.Lock()
		unsubscribeLocked()
		current := det
		det = nil
		detMu.Unlock()
		if current == nil {
			return
		}
		if clearState {
			current.state = nil
		}
		current.stopContext(ctx)
	}

	parseSections := func(sections []sdk.ConfigSection) (*Config, error) {
		for _, s := range sections {
			if s.Root != configRoot {
				continue
			}
			cfg, err := ParseConfig(s.Data)
			if err != nil {
				return nil, fmt.Errorf("ddos-detect config: %w", err)
			}
			if err := cfg.Validate(); err != nil {
				return nil, fmt.Errorf("ddos-detect config: %w", err)
			}
			return cfg, nil
		}
		return DefaultConfig(), nil
	}

	var pendingCfg *Config

	// During Stage-2 configure, build the detector without calling StateGet or
	// subscribing yet: SDK state RPCs MUST begin no earlier than OnStarted.
	// A redelivered configure after OnStarted restores and subscribes inline.
	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseSections(sections)
		if err != nil {
			return err
		}
		bus, err := loadBus()
		if err != nil {
			return err
		}
		isStarted := started.Load()
		stopCurrent(context.Background(), !isStarted)
		next := newDetectorWithState(cfg, bus, p.DispatchCommand, p)
		next.setIssueQueue(issues)
		if isStarted {
			next.restore()
		}
		detMu.Lock()
		det = next
		if isStarted {
			if cfg.Enabled {
				subscribeLocked(next)
			}
		}
		detMu.Unlock()
		return nil
	})

	p.OnStarted(func(startCtx context.Context) error {
		started.Store(true)
		detMu.Lock()
		current := det
		detMu.Unlock()
		if current == nil {
			return nil
		}
		current.restoreContext(startCtx)
		detMu.Lock()
		if det == current {
			if current.cfg.Enabled {
				subscribeLocked(current)
			}
		}
		detMu.Unlock()
		return nil
	})

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		cfg, err := parseSections(sections)
		if err != nil {
			return err
		}
		pendingCfg = cfg
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		cfg := pendingCfg
		pendingCfg = nil
		if cfg == nil {
			return nil
		}
		bus, err := loadBus()
		if err != nil {
			return err
		}
		stopCurrent(context.Background(), false)
		next := newDetectorWithState(cfg, bus, p.DispatchCommand, p)
		next.setIssueQueue(issues)
		next.restore()
		detMu.Lock()
		det = next
		if cfg.Enabled {
			subscribeLocked(next)
		}
		detMu.Unlock()
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		return nil
	})

	// Persist the final baseline snapshot while the daemon RPC transport is
	// still live (works identically for internal and external plugins).
	p.OnDrain(func(drainCtx context.Context, _ rpc.DrainInput) error {
		stopCurrent(drainCtx, false)
		return nil
	})

	// The engine waits pluginRemovalGrace (500 ms) for this answer and refuses
	// the reload when it is late, so the final baseline save runs under
	// byeSaveBudget. The stop before it is prompt: it cancels the detector's
	// context, which ends its source queries and a periodic save still waiting
	// on the store (savePeriodicBaseline). The put is issued from the SDK's
	// callback loop and does not wait on it: its answer arrives on the
	// connection's own read loop, or is the return of a direct call for a
	// plugin in the daemon.
	p.OnBye(func(_ string) error {
		byeCtx, cancelBye := context.WithTimeout(context.Background(), byeSaveBudget)
		defer cancelBye()
		stopCurrent(byeCtx, !started.Load())
		return nil
	})

	p.OnDoctorCheck(func(name string) ([]rpc.DoctorCheckDiagnostic, error) {
		return checkRateFeed(name, armed.Load())
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, &sdk.Registration{
		WantsConfig:  []string{configRoot},
		Sends:        ddosDetectSends(),
		VerifyBudget: 2,
		ApplyBudget:  10,
		DrainBudget:  5,
		DoctorChecks: []rpc.DoctorCheckDecl{rateFeedCheckDecl},
	}); err != nil {
		log.Error("ddos-detect plugin failed", "error", err)
		stopCurrent(context.Background(), !started.Load())
		return 1
	}

	stopCurrent(context.Background(), !started.Load())
	log.Info("ddos-detect plugin stopped")
	return 0
}
