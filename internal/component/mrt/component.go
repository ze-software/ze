// Design: docs/architecture/mrt.md — daemon component lifecycle

package mrt

import (
	"log/slog"
	"sync"
	"time"

	mrtfmt "codeberg.org/thomas-mangin/ze/internal/mrt"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// Component writes MRT dump files by subscribing to BGP events.
// Three independent streams: updates-only, all-messages, and periodic RIB snapshots.
type Component struct {
	config Config
	logger *slog.Logger

	updates *mrtfmt.Writer // BGP4MP update stream
	allMsgs *mrtfmt.Writer // BGP4MP all messages + state changes
	routes  *mrtfmt.Writer // TABLE_DUMP_V2 periodic RIB snapshots

	stopCh chan struct{}
	wg     sync.WaitGroup
	unsubs []func()
}

// New creates an MRT component with the given config.
func New(cfg Config, logger *slog.Logger) *Component {
	if logger == nil {
		logger = slog.Default()
	}
	return &Component{
		config: cfg,
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

// Start opens writers and subscribes to BGP events on the event bus.
func (c *Component) Start(bus ze.EventBus) {
	if c.config.IsEmpty() {
		c.logger.Debug("mrt: no dump streams configured, idle")
		return
	}

	if c.config.HasUpdates() {
		c.updates = mrtfmt.NewWriter(c.config.UpdatesPath,
			mrtfmt.WithInterval(c.config.UpdatesInterval))
	}
	if c.config.HasAll() {
		c.allMsgs = mrtfmt.NewWriter(c.config.AllPath,
			mrtfmt.WithInterval(c.config.AllInterval))
	}
	if c.config.HasRoutes() {
		c.routes = mrtfmt.NewWriter(c.config.RoutesPath)
	}

	if bus != nil {
		c.subscribeBGPEvents(bus)
	}

	if c.config.HasRoutes() {
		c.wg.Add(1)
		go c.ribDumpLoop()
	}

	c.logger.Info("mrt: started",
		"updates", c.config.HasUpdates(),
		"all", c.config.HasAll(),
		"routes", c.config.HasRoutes())
}

// Stop unsubscribes from events, stops the RIB dump loop, and closes all writers.
func (c *Component) Stop() {
	for _, unsub := range c.unsubs {
		unsub()
	}
	c.unsubs = nil

	close(c.stopCh)
	c.wg.Wait()

	c.closeWriters()
	c.logger.Info("mrt: stopped")
}

func (c *Component) closeWriters() {
	for _, w := range []*mrtfmt.Writer{c.updates, c.allMsgs, c.routes} {
		if w != nil {
			if err := w.Close(); err != nil {
				c.logger.Warn("mrt: close writer", "error", err)
			}
		}
	}
}

func (c *Component) subscribeBGPEvents(bus ze.EventBus) {
	sub := func(eventType string, handler func(payload any)) {
		unsub := bus.Subscribe("bgp", eventType, handler)
		c.unsubs = append(c.unsubs, unsub)
	}

	sub("update", c.handleUpdate)
	sub("state", c.handleStateChange)
	sub("open", c.handleMessage)
	sub("notification", c.handleMessage)
	sub("keepalive", c.handleMessage)
	sub("refresh", c.handleMessage)
}

func (c *Component) ribDumpLoop() {
	defer c.wg.Done()
	ticker := time.NewTicker(c.config.RoutesInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.dumpRIB()
		}
	}
}
