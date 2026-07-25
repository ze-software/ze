package detect

import (
	"fmt"
	"net"
	"os"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/trafficstat"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	detectyang "github.com/ze-software/ze/internal/plugins/ddos/detect/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

var eventBusPtr atomic.Pointer[ze.EventBus]

func loadBus() (ze.EventBus, error) {
	p := eventBusPtr.Load()
	if p == nil {
		return nil, fmt.Errorf("ddos-detect: event bus not configured")
	}
	return *p, nil
}

func init() {
	reg := registry.Registration{
		Name:         Name,
		Description:  "Automatic DDoS attack detector with two-stage detection",
		Features:     "yang",
		YANG:         detectyang.ZeDdosDetectConfYANG,
		ConfigRoots:  []string{configRoot},
		Dependencies: []string{"interface"},
		RunEngine:    runEngine,
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
			Codes:        []string{"doctor-ddos-detect-no-flow-source"},
			Check:        checkFlowSource,
		}},
	}
	reg.CLIHandler = func(_ []string) int { return 1 }
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "ddos-detect: registration failed: %v\n", err)
		os.Exit(1)
	}
}

// warnIfExternal logs a warning when ddos-detect is not running in-process.
// Both subscribe paths below (trafficstat.EnsureGlobal/SubscribeRates, and
// the iface.SubscribeCollectNotify fallback when trafficstat is unavailable)
// register a callback into a package-level subscriber list as a plain Go
// function call, not through DirectBridge/DispatchCommand -- that only
// reaches the real background tracker when this plugin shares process
// memory with it. An external ddos-detect would silently never receive a
// rate signal on either path, with no error anywhere.
//
// Unlike traffic-usage/flow-export (which refuse to start entirely when
// external, since their whole purpose depends on the same-process call
// succeeding), ddos-detect already frames the trafficstat-unavailable case
// as graceful degradation rather than a hard failure, so this warns rather
// than refuses -- consistent with that existing severity framing, even
// though the fallback path has the identical process-boundary problem.
func warnIfExternal(isInternal bool) {
	if isInternal {
		return
	}
	logger().Warn("ddos-detect: running as an external plugin process -- neither the trafficstat subscription nor the iface rate-tracker fallback can reach the engine's real background tracker (both are same-process calls), so the detector will never receive a rate signal; configure ddos-detect to run internal")
}

func runEngine(conn net.Conn) int {
	log := logger()
	log.Debug("ddos-detect plugin starting")

	p := sdk.NewWithConn(Name, conn)
	warnIfExternal(p.IsInternal())
	defer func() { _ = p.Close() }()

	var det *detector
	var collectSubID int
	var rateSubID int

	// subscribe wires the detector to the trafficstat service (preferred)
	// or falls back to the raw iface tick.
	//
	// EnsureGlobal (not Global): the trafficstat service is created lazily and
	// nothing else guarantees it exists at detector-config time, so Global()
	// would almost always be nil here and we would silently fall back to the raw
	// tick -- defeating the layering. EnsureGlobal makes the detector a real
	// consumer of the shared usage service.
	subscribe := func(d *detector) {
		if svc := trafficstat.EnsureGlobal(); svc != nil {
			rateSubID = svc.SubscribeRates(d.onRates)
			log.Info("ddos-detect: enabled, subscribing to trafficstat")
		} else {
			collectSubID = iface.SubscribeCollectNotify(d.onRate)
			log.Info("ddos-detect: enabled, subscribing to iface rate (trafficstat unavailable)")
		}
	}

	unsubscribe := func() {
		if rateSubID != 0 {
			if svc := trafficstat.Global(); svc != nil {
				svc.UnsubscribeRates(rateSubID)
			}
			rateSubID = 0
		}
		if collectSubID != 0 {
			iface.UnsubscribeCollectNotify(collectSubID)
			collectSubID = 0
		}
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

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseSections(sections)
		if err != nil {
			return err
		}
		bus, err := loadBus()
		if err != nil {
			return err
		}
		unsubscribe()
		if det != nil {
			det.Stop()
		}
		det = newDetector(cfg, bus, p.DispatchCommand)
		det.restore()
		if cfg.Enabled {
			subscribe(det)
		}
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
		unsubscribe()
		if det != nil {
			det.Stop()
		}
		det = newDetector(cfg, bus, p.DispatchCommand)
		det.restore()
		if cfg.Enabled {
			subscribe(det)
		}
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRoot},
		VerifyBudget: 2,
		ApplyBudget:  10,
	}); err != nil {
		log.Error("ddos-detect plugin failed", "error", err)
		unsubscribe()
		if det != nil {
			det.Stop()
		}
		return 1
	}

	unsubscribe()
	if det != nil {
		det.Stop()
	}
	log.Info("ddos-detect plugin stopped")
	return 0
}
