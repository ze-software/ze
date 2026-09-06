package observe

import (
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/internal/core/slogutil"
	observeyang "github.com/ze-software/ze/internal/plugins/ddos/observe/yang"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

// sweepInterval is how often the stale sweep runs. One second matches the
// detector's own tick, so an incident is finalized within one tick of
// stale-incident-timeout; the scan it costs is bounded by the ring capacity.
const sweepInterval = time.Second

var eventBusPtr atomic.Pointer[ze.EventBus]

// activeStore publishes the live incident ring to the in-process show handlers
// (show.go). The plugin runs as a goroutine, so the handler reads it directly.
// Nil when the plugin is not configured/running.
var activeStore atomic.Pointer[store]

func loadBus() (ze.EventBus, error) {
	p := eventBusPtr.Load()
	if p == nil {
		return nil, fmt.Errorf("ddos-observe: event bus not configured")
	}
	return *p, nil
}

func init() {
	reg := registry.Registration{
		Name:        Name,
		Description: "DDoS observability: incident store and show ddos status/incidents CLI",
		Features:    "yang",
		YANG:        observeyang.ZeDdosObserveConfYANG,
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
		fmt.Fprintf(os.Stderr, "ddos-observe: registration failed: %v\n", err)
		os.Exit(1)
	}
}

// subscribeStore attaches one incident store to the detector's event stream and
// returns the detach. Returning the unsubscribe keeps it bound to the store it
// was created for, so a config apply that replaces the store cannot leave a
// handler writing into the old one.
func subscribeStore(bus ze.EventBus, s *store) (unsubscribe func()) {
	unsubDetected := ddosevent.Detected.Subscribe(bus, func(e *ddosevent.AttackDetected) {
		s.open(e)
	})
	// Characterized carries the confidence score and refined signals; record the
	// confidence onto the incident the matching Detected already opened.
	unsubCharacterized := ddosevent.Characterized.Subscribe(bus, func(e *ddosevent.AttackCharacterized) {
		s.characterize(e)
	})
	unsubOngoing := ddosevent.Ongoing.Subscribe(bus, func(_ *ddosevent.AttackOngoing) {})
	unsubCleared := ddosevent.Cleared.Subscribe(bus, func(e *ddosevent.AttackCleared) {
		s.finalize(e.Target)
	})

	return func() {
		unsubDetected()
		unsubCharacterized()
		unsubOngoing()
		unsubCleared()
	}
}

// startStaleSweep starts the one worker goroutine that finalizes incidents which
// never received an AttackCleared. The detector emits Cleared with an empty
// target, so an incident opened against a resolved victim is never matched by
// it, and a detector that is reconfigured or stopped mid-attack emits no Cleared
// at all. This sweep is the only path that closes those incidents, and
// stale-incident-timeout is the age at which it does.
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
	log.Debug("ddos-observe plugin starting")

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	defer activeStore.Store(nil)

	parseSections := func(sections []sdk.ConfigSection) (*Config, error) {
		for _, s := range sections {
			if s.Root != configRoot {
				continue
			}
			cfg, err := ParseConfig(s.Data)
			if err != nil {
				return nil, fmt.Errorf("ddos-observe config: %w", err)
			}
			if err := cfg.Validate(); err != nil {
				return nil, fmt.Errorf("ddos-observe config: %w", err)
			}
			return cfg, nil
		}
		return DefaultConfig(), nil
	}

	var (
		pendingCfg  *Config
		unsubscribe func()
		stopSweep   func()
	)

	// teardown detaches the bus and stops the worker, in that order: a handler
	// that fires during teardown must not race the goroutine that is exiting.
	teardown := func() {
		if unsubscribe != nil {
			unsubscribe()
			unsubscribe = nil
		}
		if stopSweep != nil {
			stopSweep()
			stopSweep = nil
		}
	}
	defer teardown()

	// apply installs one config: a fresh store, the subscriptions that fill it,
	// and the sweep worker that closes what never clears. Both the first
	// configure and every later apply go through it, so the two paths cannot
	// disagree about which of the three a config change replaces.
	apply := func(cfg *Config) error {
		bus, err := loadBus()
		if err != nil {
			return err
		}
		teardown()
		staleTimeout := time.Duration(cfg.StaleIncidentTimeout) * time.Second
		incidents := newStore(cfg.IncidentRingSize, staleTimeout)
		activeStore.Store(incidents)
		unsubscribe = subscribeStore(bus, incidents)
		stopSweep = startStaleSweep(incidents, sweepInterval)
		log.Info("ddos-observe: configured",
			"ring-size", cfg.IncidentRingSize,
			"stale-incident-timeout", cfg.StaleIncidentTimeout)
		return nil
	}

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseSections(sections)
		if err != nil {
			return err
		}
		return apply(cfg)
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
		log.Error("ddos-observe plugin failed", "error", err)
		return 1
	}

	log.Info("ddos-observe plugin stopped")
	return 0
}
