package shape

import (
	"fmt"
	"net"
	"os"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/anomalyevent"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	shapeyang "github.com/ze-software/ze/internal/plugins/anomaly/shape/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

var eventBusPtr atomic.Pointer[ze.EventBus]

func loadBus() (ze.EventBus, error) {
	p := eventBusPtr.Load()
	if p == nil {
		return nil, fmt.Errorf("anomaly-shape: event bus not configured")
	}
	return *p, nil
}

// globalResponder lets the in-process show handler read responder status.
var globalResponder atomic.Pointer[responder]

func setGlobalResponder(r *responder) { globalResponder.Store(r) }
func loadGlobalResponder() *responder { return globalResponder.Load() }

func init() {
	reg := registry.Registration{
		Name:         Name,
		Description:  "Shadow-first autonomous anomaly responder: per-source rate-limit with arm/auto-revert/kill-switch",
		Features:     "yang",
		YANG:         shapeyang.ZeAnomalyShapeConfYANG,
		ConfigRoots:  []string{configRoot},
		Dependencies: []string{"firewall"},
		RunEngine:    runEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			eventBusPtr.Store(&eb)
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			bindMetrics(reg)
		},
		DoctorChecks: []registry.DoctorCheckDef{{
			Name:         "anomaly-shape-firewall",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        780,
			Dependencies: []string{"config-loaded"},
			Platforms:    []string{"any"},
			Codes:        []string{"doctor-anomaly-shape-armed-no-firewall"},
			Check:        checkFirewall,
		}},
	}
	reg.CLIHandler = func(_ []string) int { return 1 }
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "anomaly-shape: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func runEngine(conn net.Conn) int {
	log := logger()
	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	var (
		resp                         *responder
		unsubDet, unsubOng, unsubClr func()
		pendingCfg                   *Config
	)

	unsubscribe := func() {
		for _, u := range []func(){unsubDet, unsubOng, unsubClr} {
			if u != nil {
				u()
			}
		}
		unsubDet, unsubOng, unsubClr = nil, nil, nil
	}

	activate := func(cfg *Config) error {
		bus, err := loadBus()
		if err != nil {
			return err
		}
		unsubscribe()
		if resp != nil {
			resp.Stop() // reconfigure reverts all armed actions (AC-9)
		}
		resp = newResponder(cfg)
		setGlobalResponder(resp)

		// One empty reconcile while the one-time removal of the tables an older
		// ze build wrote is still pending. This responder's own tables are two
		// of them, and a box that never arms one gets no other reconcile that
		// could reach them (internal/component/firewall/legacy_tables.go).
		if firewall.LegacySweepPending() {
			if err := applyAll(); err != nil {
				log.Warn("anomaly-shape: the one-time removal of an older ze build's tables did not run", "error", err)
			}
		}
		unsubDet = anomalyevent.Detected.Subscribe(bus, resp.onDetected)
		unsubOng = anomalyevent.Ongoing.Subscribe(bus, resp.onOngoing)
		unsubClr = anomalyevent.Cleared.Subscribe(bus, resp.onCleared)
		if cfg.KillSwitch {
			resp.killSwitch() // reverts anything the prior instance left + forces shadow
		}
		log.Info("anomaly-shape: configured", "mode", cfg.Mode, "action", cfg.Action)
		return nil
	}

	parse := func(sections []sdk.ConfigSection) (*Config, error) {
		for _, s := range sections {
			if s.Root != configRoot {
				continue
			}
			cfg, err := ParseConfig(s.Data)
			if err != nil {
				return nil, fmt.Errorf("anomaly-shape config: %w", err)
			}
			if err := cfg.Validate(); err != nil {
				return nil, fmt.Errorf("anomaly-shape config: %w", err)
			}
			return cfg, nil
		}
		return DefaultConfig(), nil
	}

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parse(sections)
		if err != nil {
			return err
		}
		return activate(cfg)
	})
	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		cfg, err := parse(sections)
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
		return activate(cfg)
	})
	p.OnConfigRollback(func(_ string) error { return nil })

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRoot},
		VerifyBudget: 2,
		ApplyBudget:  10,
	}); err != nil {
		log.Error("anomaly-shape plugin failed", "error", err)
		unsubscribe()
		if resp != nil {
			resp.Stop()
		}
		return 1
	}

	unsubscribe()
	if resp != nil {
		resp.Stop()
	}
	log.Info("anomaly-shape plugin stopped")
	return 0
}
