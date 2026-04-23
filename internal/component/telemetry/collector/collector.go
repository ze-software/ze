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

// Manager runs all registered collectors on a configurable tick interval,
// feeding Netdata-compatible Prometheus metrics into the shared registry.
type Manager struct {
	reg        metrics.Registry
	prefix     string
	interval   time.Duration
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
		reg:      reg,
		prefix:   prefix,
		interval: interval,
		stop:     make(chan struct{}),
		logger:   logger,
	}
}

// Register adds a collector to the manager. Must be called before Start.
func (m *Manager) Register(c Collector) {
	m.collectors = append(m.collectors, c)
}

// Start initializes all collectors and begins the collection loop in a
// background goroutine.
func (m *Manager) Start() {
	for _, c := range m.collectors {
		c.Init(m.reg, m.prefix)
	}
	m.collectAll()

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
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.collectAll()
		}
	}
}

func (m *Manager) collectAll() {
	for _, c := range m.collectors {
		if err := c.Collect(); err != nil {
			m.logger.Warn("os collector failed",
				"collector", c.Name(), "error", err)
		}
	}
}

// StartOSCollectors creates a Manager with all platform-specific OS
// collectors and starts collection at the given interval. Returns the
// Manager so callers can call Stop() on shutdown; returns nil if no
// collectors are available for this platform.
func StartOSCollectors(reg metrics.Registry, prefix string, interval time.Duration, logger *slog.Logger) *Manager {
	m := NewManager(reg, prefix, interval, logger)
	registerPlatformCollectors(m)
	if len(m.collectors) == 0 {
		return nil
	}
	m.Start()
	return m
}
