// Design: docs/architecture/anomaly/anomaly-3-observe.md -- plugin lifecycle, event wiring, sweep worker
//
// Related: store.go holds the lifecycle ring, show.go reads it, config.go parses
// the two bounds.
//
// The plugin owns three things: the store, the subscription that feeds it, and the
// worker that finalizes an incident which never cleared. All three are rebuilt
// together on a reconfigure, in that order, so no handler is ever attached to a
// store the plugin has dropped.

package observe

import (
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/anomalyevent"
	"github.com/ze-software/ze/internal/core/slogutil"
	observeyang "github.com/ze-software/ze/internal/plugins/anomaly/observe/yang"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

// sweepInterval is how often the stale sweep runs. One second matches the
// detector's own tick, so an incident is finalized within one tick of its stale
// timeout; the scan it costs is bounded by the ring capacity.
const sweepInterval = time.Second

var eventBusPtr atomic.Pointer[ze.EventBus]

// activeStore publishes the live incident ring to the in-process show handler
// (show.go). A plugin runs as a goroutine, so the handler reads this pointer
// instead of calling back into the plugin. Nil when the plugin is not running.
var activeStore atomic.Pointer[store]

func loadBus() (ze.EventBus, error) {
	p := eventBusPtr.Load()
	if p == nil {
		return nil, fmt.Errorf("anomaly-observe: event bus not configured")
	}
	return *p, nil
}

func init() {
	reg := registry.Registration{
		Name:        Name,
		Description: "Behavioral anomaly observability: incident lifecycle store and show anomaly observe CLI",
		Features:    "yang",
		YANG:        observeyang.ZeAnomalyObserveConfYANG,
		ConfigRoots: []string{configRoot},
		RunEngine:   runEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			eventBusPtr.Store(&eb)
		},
	}
	reg.CLIHandler = func(_ []string) int { return 1 }
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "anomaly-observe: registration failed: %v\n", err)
		os.Exit(1)
	}
}

// subscribeStore attaches s to the anomaly lifecycle events on bus: Detected opens
// an incident, Cleared finalizes it. Ongoing is subscribed and ignored, because the
// store records episodes rather than samples, and subscribing it documents that the
// whole contract was considered.
//
// The caller MUST call the returned unsubscribe before it drops s, or the bus keeps
// feeding a store nobody reads.
func subscribeStore(bus ze.EventBus, s *store) (unsubscribe func()) {
	unsubDetected := anomalyevent.Detected.Subscribe(bus, func(e *anomalyevent.AnomalyDetected) {
		s.open(e)
	})
	unsubOngoing := anomalyevent.Ongoing.Subscribe(bus, func(_ *anomalyevent.AnomalyOngoing) {})
	unsubCleared := anomalyevent.Cleared.Subscribe(bus, func(e *anomalyevent.AnomalyCleared) {
		s.finalize(e.Entity)
	})

	return func() {
		unsubDetected()
		unsubOngoing()
		unsubCleared()
	}
}

// startStaleSweep starts the one worker goroutine that finalizes incidents which
// never received a clear. The detector evicts an idle entity without emitting
// Cleared, so this sweep is the only path that closes those incidents.
//
// The caller MUST call the returned stop exactly once. stop closes the worker's
// channel and returns only after the worker has exited, so the store can then be
// replaced safely.
func startStaleSweep(s *store, every time.Duration) (stop func()) {
	done := make(chan struct{})
	var wg sync.WaitGroup

	wg.Go(func() {
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				s.sweepStale()
			}
		}
	})

	return func() {
		close(done)
		wg.Wait()
	}
}

func runEngine(conn net.Conn) int {
	log := logger()
	log.Debug("anomaly-observe plugin starting")

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	var (
		unsubscribe func()
		stopSweep   func()
		pendingCfg  *Config
	)

	// teardown detaches the bus and stops the worker, in that order: a handler that
	// fires during teardown must not race the goroutine that is exiting.
	teardown := func() {
		if unsubscribe != nil {
			unsubscribe()
			unsubscribe = nil
		}
		if stopSweep != nil {
			stopSweep()
			stopSweep = nil
		}
		activeStore.Store(nil)
	}
	defer teardown()

	apply := func(cfg *Config) error {
		bus, err := loadBus()
		if err != nil {
			return err
		}
		teardown()
		incidents := newStore(cfg.IncidentRingSize, time.Duration(cfg.StaleIncidentTimeout)*time.Second)
		activeStore.Store(incidents)
		unsubscribe = subscribeStore(bus, incidents)
		stopSweep = startStaleSweep(incidents, sweepInterval)
		log.Info("anomaly-observe: configured",
			"ring-size", cfg.IncidentRingSize,
			"stale-incident-timeout", cfg.StaleIncidentTimeout)
		return nil
	}

	parse := func(sections []sdk.ConfigSection) (*Config, error) {
		for _, s := range sections {
			if s.Root != configRoot {
				continue
			}
			cfg, err := ParseConfig(s.Data)
			if err != nil {
				return nil, fmt.Errorf("anomaly-observe config: %w", err)
			}
			if err := cfg.Validate(); err != nil {
				return nil, fmt.Errorf("anomaly-observe config: %w", err)
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
		log.Error("anomaly-observe plugin failed", "error", err)
		return 1
	}

	log.Info("anomaly-observe plugin stopped")
	return 0
}
