// Design: docs/architecture/mrt.md — MRT record building from BGP events

package mrt

import (
	"sync"
	"time"

	mrtfmt "codeberg.org/thomas-mangin/ze/internal/mrt"
)

type poolBuf struct {
	b []byte
}

var bufPool = sync.Pool{
	New: func() any { return &poolBuf{b: make([]byte, 64*1024)} },
}

func getBuf() *poolBuf {
	pb, ok := bufPool.Get().(*poolBuf)
	if !ok {
		return &poolBuf{b: make([]byte, 64*1024)}
	}
	return pb
}

func (c *Component) writeRecord(buf []byte, off, msgLen int, typ, subtype uint16, now time.Time, w *mrtfmt.Writer, label string) {
	ts := uint32(now.Unix())
	if c.config.ExtendedTimestamp {
		usec := uint32(now.Nanosecond() / 1000)
		mrtfmt.WriteExtendedHeader(buf, 0, ts, usec, typ, subtype, uint32(msgLen+mrtfmt.ExtTimestampLen))
	} else {
		mrtfmt.WriteCommonHeader(buf, 0, ts, typ, subtype, uint32(msgLen))
	}
	if err := w.Write(buf[:off+msgLen]); err != nil {
		c.logger.Warn("mrt: write "+label, "error", err)
	}
}

func (c *Component) handleUpdate(payload any) {
	if c.updates == nil && c.allMsgs == nil {
		return
	}

	bp := extractBGPPayload(payload)
	if bp == nil {
		return
	}

	typ, subtype := c.bgp4mpTypeSubtype()
	as4 := mrtfmt.IsAS4Subtype(subtype)
	hdr := bp.header()

	pb := getBuf()
	defer bufPool.Put(pb)

	off := c.headerSize()
	now := time.Now()
	msgLen := mrtfmt.WriteBGP4MPMessage(pb.b, off, hdr, as4, bp.BGPMsg)

	if c.updates != nil {
		c.writeRecord(pb.b, off, msgLen, typ, subtype, now, c.updates, "update")
	}
	if c.allMsgs != nil {
		c.writeRecord(pb.b, off, msgLen, typ, subtype, now, c.allMsgs, "all")
	}
}

func (c *Component) handleStateChange(payload any) {
	if c.allMsgs == nil {
		return
	}

	sp := extractStatePayload(payload)
	if sp == nil {
		return
	}

	typ := mrtfmt.TypeBGP4MP
	if c.config.ExtendedTimestamp {
		typ = mrtfmt.TypeBGP4MPET
	}
	subtype := mrtfmt.BGP4MPStateChangeAS4
	hdr := sp.header()

	pb := getBuf()
	defer bufPool.Put(pb)

	off := c.headerSize()
	now := time.Now()
	msgLen := mrtfmt.WriteBGP4MPStateChange(pb.b, off, hdr, true, sp.OldState, sp.NewState)

	c.writeRecord(pb.b, off, msgLen, typ, subtype, now, c.allMsgs, "state-change")
}

func (c *Component) handleMessage(payload any) {
	if c.allMsgs == nil {
		return
	}

	bp := extractBGPPayload(payload)
	if bp == nil {
		return
	}

	typ, subtype := c.bgp4mpTypeSubtype()
	as4 := mrtfmt.IsAS4Subtype(subtype)
	hdr := bp.header()

	pb := getBuf()
	defer bufPool.Put(pb)

	off := c.headerSize()
	now := time.Now()
	msgLen := mrtfmt.WriteBGP4MPMessage(pb.b, off, hdr, as4, bp.BGPMsg)

	c.writeRecord(pb.b, off, msgLen, typ, subtype, now, c.allMsgs, "message")
}

func (c *Component) dumpRIB() {
	if c.routes == nil {
		return
	}
	if err := c.routes.Rotate(); err != nil {
		c.logger.Warn("mrt: rotate rib file", "error", err)
	}
	c.logger.Debug("mrt: periodic RIB dump (placeholder)")
}

func (c *Component) headerSize() int {
	if c.config.ExtendedTimestamp {
		return mrtfmt.CommonHeaderLen + mrtfmt.ExtTimestampLen
	}
	return mrtfmt.CommonHeaderLen
}

func (c *Component) bgp4mpTypeSubtype() (uint16, uint16) {
	typ := mrtfmt.TypeBGP4MP
	if c.config.ExtendedTimestamp {
		typ = mrtfmt.TypeBGP4MPET
	}
	subtype := mrtfmt.BGP4MPMessageAS4
	if c.config.AddPath {
		subtype = mrtfmt.BGP4MPMessageAS4AP
	}
	return typ, subtype
}

type bgpPayload struct {
	BGPMsg  []byte
	PeerAS  uint32
	LocalAS uint32
	PeerIP  []byte
	LocalIP []byte
	AFI     uint16
}

func (b *bgpPayload) header() *mrtfmt.BGP4MPHeader {
	return &mrtfmt.BGP4MPHeader{
		PeerAS: b.PeerAS, LocalAS: b.LocalAS,
		AFI: b.AFI, PeerIP: b.PeerIP, LocalIP: b.LocalIP,
	}
}

type statePayload struct {
	PeerAS   uint32
	LocalAS  uint32
	PeerIP   []byte
	LocalIP  []byte
	AFI      uint16
	OldState uint16
	NewState uint16
}

func (s *statePayload) header() *mrtfmt.BGP4MPHeader {
	return &mrtfmt.BGP4MPHeader{
		PeerAS: s.PeerAS, LocalAS: s.LocalAS,
		AFI: s.AFI, PeerIP: s.PeerIP, LocalIP: s.LocalIP,
	}
}

// extractBGPPayload extracts BGP wire bytes and peer info from a bus payload.
// The BGP EventBus delivers payloads as JSON strings with peer info but not
// raw wire bytes. Full wire access requires the EventDispatcher (plugin path).
// Phase 2 will tap into the reactor's EventDispatcher for raw BGP messages.
func extractBGPPayload(payload any) *bgpPayload {
	_ = payload
	return nil
}

// extractStatePayload extracts FSM state change info from a bus payload.
func extractStatePayload(payload any) *statePayload {
	_ = payload
	return nil
}
