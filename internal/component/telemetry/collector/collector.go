// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

package collector

import (
	"log/slog"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

// Collector reads OS metrics from procfs/sysfs and updates Prometheus gauges.
// Init is called once at startup to create metric handles. Collect is called
// on every tick to read current values and update those handles.
type Collector interface {
	Name() string
	Init(reg metrics.Registry, prefix string)
	Collect() error
}

// CollectorOverride holds per-collector config from YANG.
// Zero Interval means inherit the global interval.
type CollectorOverride struct {
	Enabled  bool
	Interval time.Duration
}

type scheduledCollector struct {
	collector Collector
	interval  time.Duration
	lastRun   time.Time
}

// Manager runs all registered collectors on a configurable tick interval,
// feeding Netdata-compatible Prometheus metrics into the shared registry.
type Manager struct {
	reg        metrics.Registry
	prefix     string
	interval   time.Duration
	overrides  map[string]CollectorOverride
	scheduled  []scheduledCollector
	collectors []Collector
	stop       chan struct{}
	wg         sync.WaitGroup
	logger     *slog.Logger
}

// NewManager creates a Manager that will register metrics on reg with the
// given name prefix (default "netdata") and collect at the given interval.
func NewManager(reg metrics.Registry, prefix string, interval time.Duration, logger *slog.Logger) *Manager {
	if prefix == "" {
		prefix = "netdata"
	}
	if interval <= 0 {
		interval = time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		reg:       reg,
		prefix:    prefix,
		interval:  interval,
		overrides: make(map[string]CollectorOverride),
		stop:      make(chan struct{}),
		logger:    logger,
	}
}

// SetOverrides applies per-collector enable/disable and interval settings.
// Must be called before Start.
func (m *Manager) SetOverrides(overrides map[string]CollectorOverride) {
	m.overrides = overrides
}

// Register adds a collector to the manager. Must be called before Start.
func (m *Manager) Register(c Collector) {
	m.collectors = append(m.collectors, c)
}

// Start initializes enabled collectors and begins the collection loop.
func (m *Manager) Start() {
	for _, c := range m.collectors {
		ovr, hasOverride := m.overrides[c.Name()]
		if hasOverride && !ovr.Enabled {
			m.logger.Info("os collector disabled by config", "collector", c.Name())
			continue
		}

		c.Init(m.reg, m.prefix)

		interval := m.interval
		if hasOverride && ovr.Interval > 0 {
			interval = ovr.Interval
		}
		m.scheduled = append(m.scheduled, scheduledCollector{
			collector: c,
			interval:  interval,
		})
	}

	m.collectAll(true)

	m.wg.Add(1)
	go m.loop()
}

// Stop signals the collection loop to exit and waits for it to finish.
func (m *Manager) Stop() {
	close(m.stop)
	m.wg.Wait()
}

func (m *Manager) loop() {
	defer m.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.collectAll(false)
		}
	}
}

func (m *Manager) collectAll(force bool) {
	now := time.Now()
	for i := range m.scheduled {
		sc := &m.scheduled[i]
		if !force && now.Sub(sc.lastRun) < sc.interval {
			continue
		}
		sc.lastRun = now
		if err := sc.collector.Collect(); err != nil {
			m.logger.Warn("os collector failed",
				"collector", sc.collector.Name(), "error", err)
		}
	}
}

// StartOSCollectors creates a Manager with all platform-specific OS
// collectors and starts collection. Per-collector overrides control
// enable/disable and interval. Returns the Manager so callers can call
// Stop() on shutdown; returns nil if no collectors are available.
func StartOSCollectors(reg metrics.Registry, prefix string, interval time.Duration, overrides map[string]CollectorOverride, logger *slog.Logger) *Manager {
	m := NewManager(reg, prefix, interval, logger)
	m.SetOverrides(overrides)
	registerPlatformCollectors(m)
	if len(m.collectors) == 0 {
		return nil
	}
	m.Start()
	return m
}
