package trafficusage

import (
	"fmt"
	"net"
	"os"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	trafficusageyang "github.com/ze-software/ze/internal/plugins/trafficusage/yang"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	reg := registry.Registration{
		Name:        Name,
		Description: "eBPF TCX per-port and per-IP byte accounting",
		Features:    "yang",
		YANG:        trafficusageyang.ZeTrafficUsageConfYANG,
		ConfigRoots: []string{configRoot},
		// The poller and the snapshot-driven interface lifecycle run off the
		// iface rate tracker's per-second callback (RegisterCollectNotify),
		// which only runs when an interface{} section is present. Declaring the
		// dependency makes configuring traffic-usage alone enough to start it.
		Dependencies: []string{dependencyInterface},
		RunEngine:    runEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			BindMetrics(reg)
		},
		DoctorChecks: doctorChecks(),
	}
	reg.CLIHandler = func(_ []string) int {
		return 1
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "traffic-usage: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func runEngine(conn net.Conn) int {
	log := loggerPtr.Load()
	log.Debug("traffic-usage plugin starting")

	att := newAttacher()
	if err := att.Available(); err != nil {
		log.Warn("traffic-usage: eBPF accounting unavailable; plugin will run inert", "error", err)
	}

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	// iface.SubscribeCollectNotify below registers a callback into iface's
	// package-level subscriber list as a plain Go function call, not through
	// DirectBridge/DispatchCommand -- that only reaches the engine's real
	// rate tracker (internal/component/iface's own background collect loop)
	// when this plugin shares process memory with it. It is the monitor's
	// only attach/detach mechanism, so an external traffic-usage would
	// silently never attach to any interface, with ze_traffic_usage_*
	// permanently empty and no error anywhere. Refuse to start rather than
	// degrade silently.
	if !p.IsInternal() {
		log.Error("traffic-usage: refusing to start as an external plugin process -- the interface rate-tracker subscription (iface.SubscribeCollectNotify) is a same-process call and would silently no-op across a process boundary; configure traffic-usage to run internal")
		return 1
	}

	mon := newMonitor(att, resolveBinding)
	activeMonitor.Store(mon)

	// The iface rate tracker delivers a ~1 Hz interface snapshot; the monitor
	// uses it to (re)attach on interface up and detach on down/removal. The
	// tracker runs because of the "interface" dependency declared above.
	collectSubID := iface.SubscribeCollectNotify(mon.onSnapshot)

	// Start the metrics poller; it reads the maps and publishes ze_traffic_usage_*
	// every cfg.Interval (idle until interfaces are configured and attached).
	mon.Start()

	// configure (re)builds the attached set and poller from a parsed config.
	// Shared by OnConfigure (boot) and OnConfigApply (reload). A nil/empty cfg
	// reconciles to an empty desired set, detaching everything.
	configure := func(cfg *Config) error {
		return mon.Reconcile(cfg)
	}

	parseSections := func(sections []sdk.ConfigSection) (*Config, error) {
		for _, s := range sections {
			if s.Root != configRoot {
				continue
			}
			cfg, err := ParseConfig(s.Data)
			if err != nil {
				return nil, fmt.Errorf("traffic-usage config: %w", err)
			}
			if err := cfg.Validate(); err != nil {
				return nil, fmt.Errorf("traffic-usage config: %w", err)
			}
			return cfg, nil
		}
		return &Config{}, nil
	}

	// pendingCfg carries the verified reload config from OnConfigVerify into
	// OnConfigApply, which receives only a diff. Config transactions are
	// serialized by the engine, so a plain captured variable is safe.
	var pendingCfg *Config

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseSections(sections)
		if err != nil {
			return err
		}
		return configure(cfg)
	})

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		cfg, err := parseSections(sections)
		if err != nil {
			return fmt.Errorf("traffic-usage config verify: %w", err)
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
		return configure(cfg)
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
		log.Error("traffic-usage plugin failed", "error", err)
		iface.UnsubscribeCollectNotify(collectSubID)
		activeMonitor.Store(nil)
		mon.Stop()
		return 1
	}

	iface.UnsubscribeCollectNotify(collectSubID)
	activeMonitor.Store(nil)
	mon.Stop()
	log.Info("traffic-usage plugin stopped")
	return 0
}
