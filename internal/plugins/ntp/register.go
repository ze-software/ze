package ntp

import (
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/slogutil"
	ntpevents "github.com/ze-software/ze/internal/plugins/ntp/events"
	ntpyang "github.com/ze-software/ze/internal/plugins/ntp/yang"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

// eventBusMu guards eventBusRef.
var (
	eventBusMu  sync.Mutex
	eventBusRef ze.EventBus
)

const configRootEnvironment = "environment"

func setEventBus(eb ze.EventBus) {
	eventBusMu.Lock()
	defer eventBusMu.Unlock()
	eventBusRef = eb
}

func getEventBus() ze.EventBus {
	eventBusMu.Lock()
	defer eventBusMu.Unlock()
	return eventBusRef
}

func init() {
	_ = events.RegisterNamespace(ntpevents.Namespace, ntpevents.EventClockSynced)

	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)

	reg := registry.Registration{
		Name:                    "ntp",
		Description:             "NTP client: system clock synchronization",
		Features:                "yang",
		YANG:                    ntpyang.ZeNTPConfYANG,
		ConfigRoots:             []string{configRootEnvironment},
		InProcessConfigVerifier: verifyNTPConfig,
		RunEngine:               runNTPPlugin,
	}
	registry.SetNTPSyncProvider(ntpSyncInfo)

	reg.CLIHandler = func(_ []string) int { return 1 }

	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:system-ntp",
			Handler:    handleShowSystemNTP,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:system-ntp-peers",
			Handler:    handleShowSystemNTPPeers,
		},
	)

	reg.ConfigureEngineLogger = func(loggerName string) {
		l := slogutil.Logger(loggerName)
		if l != nil {
			loggerPtr.Store(l)
		}
	}
	reg.ConfigureEventBus = func(eb ze.EventBus) {
		setEventBus(eb)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "ntp: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func verifyNTPConfig(sections []sdk.ConfigSection) error {
	for _, s := range sections {
		if s.Root != configRootEnvironment {
			continue
		}
		if _, err := parseNTPConfig(s.Data); err != nil {
			return fmt.Errorf("ntp: %w", err)
		}
	}
	return nil
}

// runNTPPlugin is the engine-mode entry point for the NTP plugin.
func runNTPPlugin(conn net.Conn) int {
	log := loggerPtr.Load()
	log.Debug("ntp plugin starting")

	p := sdk.NewWithConn("ntp", conn)
	defer func() { _ = p.Close() }()

	var worker *syncWorker
	var unsubscribe func()

	// startWorker stops any existing worker, then starts a new one
	// with the given config. Safe to call multiple times (reload).
	startWorker := func(cfg ntpConfig) {
		if worker != nil {
			worker.stopAndWait()
			worker = nil
		}
		if unsubscribe != nil {
			unsubscribe()
			unsubscribe = nil
		}
		if !cfg.Enabled {
			log.Debug("ntp: disabled in config")
			storeState(&syncState{Enabled: false})
			return
		}
		worker = newSyncWorker(cfg, getEventBus())
		worker.start()
		log.Info("ntp: sync worker started",
			"servers", cfg.Servers, "interval", cfg.IntervalSec)

		eb := getEventBus()
		if eb != nil {
			unsubscribe = subscribeDHCP(eb, worker)
		}
	}

	// pendingCfg holds config between verify and apply phases.
	var pendingCfg *ntpConfig

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, s := range sections {
			if s.Root != configRootEnvironment {
				continue
			}
			cfg, err := parseNTPConfig(s.Data)
			if err != nil {
				return fmt.Errorf("ntp: %w", err)
			}
			startWorker(cfg)
			return nil
		}
		return nil
	})

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		for _, s := range sections {
			if s.Root != configRootEnvironment {
				continue
			}
			cfg, err := parseNTPConfig(s.Data)
			if err != nil {
				return fmt.Errorf("ntp: %w", err)
			}
			pendingCfg = &cfg
			return nil
		}
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		cfg := pendingCfg
		pendingCfg = nil
		if cfg == nil {
			return nil
		}
		startWorker(*cfg)
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRootEnvironment},
		VerifyBudget: 2,
		// ApplyBudget in seconds (orchestrator sums per-tier maxima). The
		// worst-case OnConfigApply cost is the synchronous worker handoff:
		// stopAndWait now returns within one in-flight ntp.Query (~5s, library
		// default) plus up to 250ms jitter and goroutine handoff. 10s leaves
		// comfortable headroom over that residual one-query wait while dead
		// servers keep the phase-2 retry loop busy (startup-resilience FIX 1).
		ApplyBudget: 10,
	}); err != nil {
		log.Error("ntp plugin failed", "error", err)
		return 1
	}

	// Shutdown: save time and stop worker.
	if worker != nil {
		// persist-path is vestigial: a non-empty value enables the final save; the
		// store location is the shared zefs store, not the path.
		if worker.cfg.PersistPath != "" {
			if err := saveTime(currentTime()); err != nil {
				log.Debug("ntp: final time save failed", "err", err)
			}
		}
		worker.stopAndWait()
	}
	if unsubscribe != nil {
		unsubscribe()
	}

	log.Info("ntp plugin stopped")
	return 0
}

// currentTime returns the current system time.
func currentTime() time.Time {
	return time.Now()
}

// ntpSyncInfo returns NTP sync metadata for show system date enrichment.
func ntpSyncInfo() map[string]any {
	st := loadState()
	if st == nil || !st.Enabled {
		return nil
	}
	return map[string]any{
		"ntp-synced": st.Synced,
		"ntp-source": st.Source,
		"ntp-offset": st.Offset.String(),
	}
}

// fieldEnabled is the YANG leaf that switches NTP on, and the JSON key the
// status view answers with.
const fieldEnabled = "enabled"

// handleShowSystemNTP returns the NTP sync status summary.
func handleShowSystemNTP(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	st := loadState()
	if st == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{fieldEnabled: false},
		}, nil
	}
	if !st.Enabled {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{fieldEnabled: false},
		}, nil
	}
	data := map[string]any{
		fieldEnabled:    true,
		"synced":        st.Synced,
		"source":        st.Source,
		"offset":        st.Offset.String(),
		"stratum":       st.Stratum,
		"poll-interval": st.PollInterval,
	}
	if !st.LastSync.IsZero() {
		data["last-sync"] = st.LastSync.Format(time.RFC3339)
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(data)}, nil
}

// handleShowSystemNTPPeers returns per-server NTP state.
func handleShowSystemNTPPeers(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	st := loadState()
	if st == nil || len(st.Servers) == 0 {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"peers": []map[string]any{}, "count": 0},
		}, nil
	}
	peers := make([]map[string]any, 0, len(st.Servers))
	for i := range st.Servers {
		s := &st.Servers[i]
		row := map[string]any{
			"address": s.Address,
			"offset":  s.Offset.String(),
			"rtt":     s.RTT.String(),
			"stratum": s.Stratum,
			"reach":   s.Reach,
		}
		if !s.LastQuery.IsZero() {
			row["last-query"] = s.LastQuery.Format(time.RFC3339)
		}
		if s.LastError != "" {
			row["last-error"] = s.LastError
		}
		peers = append(peers, row)
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"peers": peers, "count": len(peers)},
	}, nil
}
