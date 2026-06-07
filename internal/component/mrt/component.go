// Design: docs/architecture/mrt.md — daemon component lifecycle

package mrt

import (
	"log/slog"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	mrtfmt "codeberg.org/thomas-mangin/ze/internal/mrt"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// Component writes MRT dump files by subscribing to BGP events.
// Three independent streams: updates-only, all-messages, and periodic RIB snapshots.
// Implements reactor.MessageObserver for raw wire byte delivery.
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

// Start opens writers and subscribes to BGP state change events on the bus.
// Raw BGP messages arrive via OnBGPMessage (reactor.MessageObserver), not the bus.
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
		unsub := bus.Subscribe("bgp", "state", c.handleStateChange)
		c.unsubs = append(c.unsubs, unsub)
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

// OnBGPMessage implements reactor.MessageObserver.
// Called synchronously on the session goroutine with raw wire bytes.
func (c *Component) OnBGPMessage(peer *plugin.PeerInfo, msgType message.MessageType, sent bool, rawBytes []byte) {
	if c.updates == nil && c.allMsgs == nil {
		return
	}

	isUpdate := msgType == message.TypeUPDATE

	pb := getBuf()
	defer bufPool.Put(pb)

	now := time.Now()
	typ, subtype := c.bgp4mpTypeSubtype()
	as4 := mrtfmt.IsAS4Subtype(subtype)
	if sent {
		subtype = localSubtype(subtype)
	}

	var ipBuf [32]byte
	hdr := peerInfoToHeader(peer, ipBuf[:])
	off := c.headerSize()
	msgLen := mrtfmt.WriteBGP4MPMessage(pb.b, off, hdr, as4, rawBytes)
	record := pb.b[:off+msgLen]
	writeHeader(record, c.config.ExtendedTimestamp, now, typ, subtype, msgLen)

	if c.updates != nil && isUpdate {
		if err := c.updates.Write(record); err != nil {
			c.logger.Warn("mrt: write update", "error", err)
		}
	}
	if c.allMsgs != nil {
		if err := c.allMsgs.Write(record); err != nil {
			c.logger.Warn("mrt: write all", "error", err)
		}
	}
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
