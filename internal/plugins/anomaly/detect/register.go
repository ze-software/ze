package detect

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/component/trafficfeature"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	detectyang "codeberg.org/thomas-mangin/ze/internal/plugins/anomaly/detect/yang"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

var (
	eventBusPtr atomic.Pointer[ze.EventBus]
	loggerPtr   atomic.Pointer[slog.Logger]
)

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

func logger() *slog.Logger {
	if l := loggerPtr.Load(); l != nil {
		return l
	}
	return slog.Default()
}

func loadBus() (ze.EventBus, error) {
	p := eventBusPtr.Load()
	if p == nil {
		return nil, fmt.Errorf("anomaly-detect: event bus not configured")
	}
	return *p, nil
}

func init() {
	reg := registry.Registration{
		Name:        Name,
		Description: "Behavioral anomaly detector (report-only): per-entity pattern-of-life over trafficfeature",
		Features:    "yang",
		YANG:        detectyang.ZeAnomalyDetectConfYANG,
		ConfigRoots: []string{configRoot},
		RunEngine:   runEngine,
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
			Name:         "anomaly-detect-feature-source",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        770,
			Dependencies: []string{"config-loaded"},
			Platforms:    []string{"any"},
			Codes:        []string{"doctor-anomaly-detect-no-feature-source"},
			Check:        checkFeatureSource,
		}},
	}
	reg.CLIHandler = func(_ []string) int { return 1 }
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "anomaly-detect: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func runEngine(conn net.Conn) int {
	log := logger()
	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	var (
		stopCh chan struct{}
		tfID   int
		wg     sync.WaitGroup
	)

	startTicker := func(d *detector) {
		svc := trafficfeature.EnsureGlobal()
		if svc == nil {
			log.Warn("anomaly-detect: trafficfeature service unavailable; detector idle")
			return
		}
		tfID = svc.Attach()
		stopCh = make(chan struct{})
		ch := stopCh
		wg.Go(func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ch:
					return
				case <-ticker.C:
					d.onTick(svc.Snapshot())
				}
			}
		})
	}

	stopTicker := func() {
		if stopCh != nil {
			close(stopCh)
			wg.Wait()
			stopCh = nil
		}
		if tfID != 0 {
			if svc := trafficfeature.Global(); svc != nil {
				svc.Detach(tfID)
			}
			tfID = 0
		}
	}

	apply := func(cfg *Config) error {
		bus, err := loadBus()
		if err != nil {
			return err
		}
		stopTicker()
		d := newDetector(cfg, bus)
		setGlobalDetector(d)
		if cfg.Enabled {
			startTicker(d)
			log.Info("anomaly-detect: enabled, consuming trafficfeature")
		}
		return nil
	}

	parse := func(sections []sdk.ConfigSection) (*Config, error) {
		for _, s := range sections {
			if s.Root != configRoot {
				continue
			}
			cfg, err := ParseConfig(s.Data)
			if err != nil {
				return nil, fmt.Errorf("anomaly-detect config: %w", err)
			}
			if err := cfg.Validate(); err != nil {
				return nil, fmt.Errorf("anomaly-detect config: %w", err)
			}
			return cfg, nil
		}
		return DefaultConfig(), nil
	}

	var pendingCfg *Config

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parse(sections)
		if err != nil {
			return err
		}
		return apply(cfg)
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
		return apply(cfg)
	})
	p.OnConfigRollback(func(_ string) error { return nil })

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRoot},
		VerifyBudget: 2,
		ApplyBudget:  10,
	}); err != nil {
		log.Error("anomaly-detect plugin failed", "error", err)
		stopTicker()
		return 1
	}

	stopTicker()
	log.Info("anomaly-detect plugin stopped")
	return 0
}
