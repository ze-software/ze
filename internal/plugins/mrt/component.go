// Design: docs/architecture/mrt.md — daemon component lifecycle

package mrt

import (
	"log/slog"
	"net/netip"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/msgtype"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	mrtfmt "codeberg.org/thomas-mangin/ze/internal/mrt"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// Component writes MRT dump files by subscribing to BGP events.
// Three independent streams: updates-only, all-messages, and periodic RIB snapshots.
// Implements reactor.MessageObserver for raw wire byte delivery.
type Component struct {
	config    Config
	logger    *slog.Logger
	peerSet   map[netip.Addr]struct{} // precomputed from config.PeerFilter
	filterDir int8                    // -1=received only, 1=sent only, 0=both

	updates   *asyncWriter   // BGP4MP update stream (async, non-blocking)
	allMsgs   *asyncWriter   // BGP4MP all messages + state changes (async)
	routes    *mrtfmt.Writer // TABLE_DUMP_V2 periodic RIB snapshots (sync, own goroutine)
	ribDumper registry.RIBDumpCallback

	stopCh chan struct{}
	wg     sync.WaitGroup
	unsubs []func()
}

// New creates an MRT component with the given config.
func New(cfg Config, logger *slog.Logger) *Component {
	if logger == nil {
		logger = slog.Default()
	}
	c := &Component{
		config: cfg,
		logger: logger,
		stopCh: make(chan struct{}),
	}
	if len(cfg.PeerFilter) > 0 {
		c.peerSet = make(map[netip.Addr]struct{}, len(cfg.PeerFilter))
		for _, s := range cfg.PeerFilter {
			if a, err := netip.ParseAddr(s); err == nil {
				c.peerSet[a] = struct{}{}
			}
		}
	}
	switch cfg.Direction {
	case "received":
		c.filterDir = -1
	case "sent":
		c.filterDir = 1
	}
	return c
}

// Start opens writers and subscribes to BGP state change events on the bus.
// Raw BGP messages arrive via OnBGPMessage (reactor.MessageObserver), not the bus.
func (c *Component) Start(bus ze.EventBus) {
	if c.config.IsEmpty() {
		c.logger.Debug("mrt: no dump streams configured, idle")
		return
	}

	if c.config.HasUpdates() {
		w := mrtfmt.NewWriter(c.config.UpdatesPath,
			mrtfmt.WithInterval(c.config.UpdatesInterval))
		c.updates = newAsyncWriter(w, c.logger)
	}
	if c.config.HasAll() {
		w := mrtfmt.NewWriter(c.config.AllPath,
			mrtfmt.WithInterval(c.config.AllInterval))
		c.allMsgs = newAsyncWriter(w, c.logger)
	}
	if c.config.HasRoutes() {
		c.routes = mrtfmt.NewWriter(c.config.RoutesPath)
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
func (c *Component) OnBGPMessage(peer *plugin.PeerInfo, msgType msgtype.MessageType, sent bool, rawBytes []byte) {
	if c.updates == nil && c.allMsgs == nil {
		return
	}
	if !c.shouldRecord(peer, sent) {
		return
	}

	isUpdate := msgType == msgtype.TypeUPDATE

	pb := getBuf()
	defer bufPool.Put(pb)

	now := time.Now()
	typ, subtype := c.bgp4mpTypeSubtype()
	as4 := mrtfmt.IsAS4Subtype(subtype)
	if sent {
		subtype = localSubtype(subtype)
	}

	var ipBuf [32]byte
	var hdr mrtfmt.BGP4MPHeader
	peerInfoToHeader(peer, ipBuf[:], &hdr)
	off := c.headerSize()
	msgLen := mrtfmt.WriteBGP4MPMessage(pb.b, off, &hdr, as4, rawBytes)
	total := off + msgLen
	if total > len(pb.b) {
		// Defense in depth: pb.b is sized to the maximum possible record
		// (maxRecordLen), so this cannot happen today. Guard anyway so a future
		// header change can never turn OnBGPMessage into a panic on a peer's
		// oversized message (the reslice below would be out of range).
		c.logger.Warn("mrt: record exceeds buffer, dropping",
			"size", total, "cap", len(pb.b))
		return
	}
	record := pb.b[:total]
	writeHeader(record, c.config.ExtendedTimestamp, now, typ, subtype, msgLen)

	if c.updates != nil && isUpdate {
		c.updates.Write(record)
	}
	if c.allMsgs != nil {
		c.allMsgs.Write(record)
	}
}

// onBGPMessageAny is the any-typed bridge from the coordinator callback.
func (c *Component) onBGPMessageAny(peer any, msgType uint8, sent bool, rawBytes []byte) {
	pi, ok := peer.(*plugin.PeerInfo)
	if !ok || pi == nil {
		return
	}
	c.OnBGPMessage(pi, msgtype.MessageType(msgType), sent, rawBytes)
}

// onPeerEstablished records a BGP4MP_STATE_CHANGE_AS4 (Idle -> Established).
func (c *Component) onPeerEstablished(peer any) {
	pi, ok := peer.(*plugin.PeerInfo)
	if !ok || pi == nil {
		return
	}
	c.writeStateChange(pi, mrtfmt.FSMIdle, mrtfmt.FSMEstablished)
}

// onPeerClosed records a BGP4MP_STATE_CHANGE_AS4 (Established -> Idle).
func (c *Component) onPeerClosed(peer any) {
	pi, ok := peer.(*plugin.PeerInfo)
	if !ok || pi == nil {
		return
	}
	c.writeStateChange(pi, mrtfmt.FSMEstablished, mrtfmt.FSMIdle)
}

func (c *Component) shouldRecord(peer *plugin.PeerInfo, sent bool) bool {
	if c.filterDir != 0 {
		if (c.filterDir == -1 && sent) || (c.filterDir == 1 && !sent) {
			return false
		}
	}
	if c.peerSet != nil {
		if _, ok := c.peerSet[peer.Address]; !ok {
			return false
		}
	}
	return true
}

func (c *Component) writeStateChange(peer *plugin.PeerInfo, oldState, newState uint16) {
	if c.allMsgs == nil {
		return
	}

	pb := getBuf()
	defer bufPool.Put(pb)

	typ := mrtfmt.TypeBGP4MP
	if c.config.ExtendedTimestamp {
		typ = mrtfmt.TypeBGP4MPET
	}

	var ipBuf [32]byte
	var hdr mrtfmt.BGP4MPHeader
	peerInfoToHeader(peer, ipBuf[:], &hdr)
	off := c.headerSize()
	now := time.Now()
	msgLen := mrtfmt.WriteBGP4MPStateChange(pb.b, off, &hdr, true, oldState, newState)
	writeHeader(pb.b, c.config.ExtendedTimestamp, now, typ, mrtfmt.BGP4MPStateChangeAS4, msgLen)

	c.allMsgs.Write(pb.b[:off+msgLen])
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
	if c.updates != nil {
		if err := c.updates.Close(); err != nil {
			c.logger.Warn("mrt: close updates writer", "error", err)
		}
	}
	if c.allMsgs != nil {
		if err := c.allMsgs.Close(); err != nil {
			c.logger.Warn("mrt: close all writer", "error", err)
		}
	}
	if c.routes != nil {
		if err := c.routes.Close(); err != nil {
			c.logger.Warn("mrt: close routes writer", "error", err)
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
