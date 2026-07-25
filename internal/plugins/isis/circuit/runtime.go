// Design: plan/learned/931-isis-5-adjacency.md -- circuit RX dispatch, Hello send, sweep.
// ISO/IEC 10589 section 8.2 (adjacency), section 8.2.3 (hold-timer timeout),
// clause 9.5/9.6 (IIH decode).
//
// RFC: rfc/short/rfc5303.md -- P2P three-way state derived for our TLV 240
// RFC: rfc/short/rfc1195.md -- TLV 132/232 stored as the SPF next-hop source
//
// This file is the circuit's runtime glue: it decodes a received IIH with the
// spec-isis-2 codec, builds the FSM input, drives the pure FSM, emits session
// events on a transition, builds and sends the periodic padded Hello, and runs
// the hold-timer sweep that times adjacencies out. All table mutation runs from
// the circuit goroutine (the single writer).

package circuit

import (
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// Receive handles one received PDU on this circuit. It is the IIH path: the
// engine dispatcher routes IIH PDU types (0x0f/0x10/0x11) here with the source
// SNPA. Non-IIH PDUs and PDUs not for this circuit's ifindex are ignored (the
// engine routes by ifindex, but Receive re-checks defensively). A malformed PDU
// is dropped (the codec never panics). Receive returns the resulting Transition
// for the matched level, or a zero Transition when nothing changed.
func (c *Circuit) Receive(srcSNPA adjacency.SNPA, pdu []byte) adjacency.Transition {
	p, err := packet.DecodePDU(pdu)
	if err != nil {
		return adjacency.Transition{Rejected: true, RejectReason: "decode"}
	}
	switch {
	case p.LANHello != nil:
		defer packet.ReleaseTLVs(p.LANHello.TLVs)
		return c.handleLANHello(srcSNPA, p.LANHello)
	case p.P2PHello != nil:
		defer packet.ReleaseTLVs(p.P2PHello.TLVs)
		return c.handleP2PHello(p.P2PHello)
	default:
		return adjacency.Transition{}
	}
}

// handleLANHello drives the FSM from a LAN IIH at the PDU's level.
func (c *Circuit) handleLANHello(srcSNPA adjacency.SNPA, h *packet.LANHello) adjacency.Transition {
	lvl, ok := h.PDUType.Level()
	if !ok {
		return adjacency.Transition{}
	}
	level := adjacency.Level(lvl)
	if !c.formsLevel(level) {
		return adjacency.Transition{Rejected: true, RejectReason: "level-not-enabled"}
	}
	in := c.helloInput(h.SystemID, srcSNPA, level, h.HoldingTime.Seconds(), h.TLVs, false)
	// ISO/IEC 10589 clause 8.4.5: a LAN IIH carries the sender's DIS priority in
	// its fixed header; record it so the circuit's per-level election (isis-8) can
	// compare candidates. A P2P IIH has no priority field (handleP2PHello leaves
	// it 0).
	in.Priority = h.Priority
	// ISO/IEC 10589 clause 8.4.1: carry the sender's Maximum Area Addresses so the
	// FSM rejects an IIH whose TLV-1 advertises more areas than the sender claims
	// to support (0 means the default 3).
	in.MaxAreaAddresses = h.MaxAreaAddresses
	return c.applyHello(in)
}

// handleP2PHello drives the FSM from a P2P IIH. The level is taken from the
// IIH's circuit-type field intersected with the circuit's configured levels;
// when both support L1 we use L1, else L2 (a P2P adjacency is a single record).
func (c *Circuit) handleP2PHello(h *packet.P2PHello) adjacency.Transition {
	level := c.p2pLevel(h.CircuitType)
	if level == 0 {
		return adjacency.Transition{Rejected: true, RejectReason: "level-mismatch"}
	}
	in := c.helloInput(h.SystemID, adjacency.SNPA{}, level, h.HoldingTime.Seconds(), h.TLVs, true)
	// ISO/IEC 10589 clause 8.4.1: a P2P IIH also carries Maximum Area Addresses in
	// its common header; carry it so the FSM applies the same TLV-1 area-count cap.
	in.MaxAreaAddresses = h.MaxAreaAddresses
	return c.applyHello(in)
}

// p2pLevel chooses the adjacency level for a P2P Hello from the neighbor's
// circuit-type field and our configured levels. Returns 0 when there is no
// common level.
func (c *Circuit) p2pLevel(ct packet.CircuitType) adjacency.Level {
	neighL1 := ct == packet.CircuitL1 || ct == packet.CircuitL1L2
	neighL2 := ct == packet.CircuitL2 || ct == packet.CircuitL1L2
	if neighL1 && c.formsLevel(adjacency.Level1) {
		return adjacency.Level1
	}
	if neighL2 && c.formsLevel(adjacency.Level2) {
		return adjacency.Level2
	}
	return 0
}

// formsLevel reports whether this circuit forms adjacencies at level.
func (c *Circuit) formsLevel(level adjacency.Level) bool {
	return slices.Contains(c.levels, level)
}

// helloInput parses the FSM-relevant TLVs out of a decoded IIH into a
// HelloInput. It extracts TLV 1 (areas), TLV 6 (neighbor SNPAs, LAN), TLV 132 /
// 232 (IPv4/IPv6 next-hop source), and TLV 240 (P2P three-way). isP2P selects
// whether TLV 240 / TLV 6 are relevant.
func (c *Circuit) helloInput(sys types.SystemID, srcSNPA adjacency.SNPA, level adjacency.Level, holdTime uint16, tlvs []packet.TLV, isP2P bool) adjacency.HelloInput {
	in := adjacency.HelloInput{
		SystemID: sys,
		SNPA:     srcSNPA,
		Level:    level,
		HoldTime: holdTime,
	}
	for _, t := range tlvs {
		switch t.Type {
		case packet.TLVAreaAddresses:
			if a, err := packet.DecodeAreaAddressesTLV(t.Value); err == nil {
				in.Areas = a.Areas
			}
		case packet.TLVISNeighbors:
			if !isP2P {
				if n, err := packet.DecodeISNeighborsTLV(t.Value); err == nil {
					in.NeighborSNPAs = make([]adjacency.SNPA, len(n.SNPAs))
					for i, s := range n.SNPAs {
						in.NeighborSNPAs[i] = adjacency.SNPA(s)
					}
				}
			}
		case packet.TLVIPInterfaceAddress:
			if a, err := packet.DecodeIPv4InterfaceAddrTLV(t.Value); err == nil && len(a.Addresses) > 0 {
				in.IPv4 = a.Addresses[0]
			}
		case packet.TLVIPv6InterfaceAddress:
			if addr, ok := firstIPv6(t.Value); ok {
				in.IPv6 = addr
			}
		case packet.TLVP2PThreeWay:
			if isP2P {
				if tw, err := packet.DecodeP2PThreeWayTLV(t.Value); err == nil {
					in.HasThreeWay = true
					in.ThreeWay = tw
				}
			}
		}
	}
	return in
}

// firstIPv6 reads the first 16-octet IPv6 address from a TLV 232 value (RFC 5308
// sec 3: a flat list of 16-octet addresses). Returns ok=false when the value is
// too short. The full TLV 232 codec lives in isis-12; this reads only the first
// entry the adjacency next-hop needs.
func firstIPv6(value []byte) (netip.Addr, bool) {
	const ipv6Len = 16
	if len(value) < ipv6Len {
		return netip.Addr{}, false
	}
	var a16 [ipv6Len]byte
	copy(a16[:], value[:ipv6Len])
	return netip.AddrFrom16(a16), true
}

// applyHello looks up (or creates) the adjacency for the input, drives the FSM
// under the table write lock (so it never races the timer sweep), and emits the
// session event AFTER releasing the lock. It enforces MaxNeighbors: a Hello from
// a new neighbor on a full table is dropped.
func (c *Circuit) applyHello(in adjacency.HelloInput) adjacency.Transition {
	local := c.local()
	now := c.now()
	var (
		tr   adjacency.Transition
		snap adjacency.NeighborSnapshot
	)
	ok := c.table.Update(in.SystemID, in.Level, func(adj *adjacency.Adjacency, _ bool) {
		tr = adjacency.ReceiveHello(adj, local, in, now)
		snap = adj.Snapshot()
	})
	if !ok {
		return adjacency.Transition{Rejected: true, RejectReason: "table-full"}
	}
	c.fireEvents(tr, snap)
	return tr
}

// local builds the immutable Local identity the FSM matches against.
func (c *Circuit) local() adjacency.Local {
	return adjacency.Local{
		SystemID: c.systemID,
		SNPA:     c.snpa,
		Areas:    c.areas,
		Kind:     c.kind,
	}
}

// fireEvents emits the session up/down event and runs the metric transition
// hooks for a Transition, using the pre-rendered snapshot row. It MUST be called
// AFTER releasing the table lock: the metric hook (publishAdjMetrics) re-reads
// the table via Snapshot, which would deadlock if fireEvents ran while the lock
// was held. The snapshot is captured under the lock by the caller so the event
// payload reflects the record at the moment of transition.
func (c *Circuit) fireEvents(tr adjacency.Transition, snap adjacency.NeighborSnapshot) {
	c.mu.Lock()
	sink := c.sink
	onUp := c.onUp
	onDown := c.onDown
	c.mu.Unlock()

	level := adjacency.Level1
	if snap.Level == adjacency.Level2.String() {
		level = adjacency.Level2
	}

	switch {
	case tr.SessionUp:
		if onUp != nil {
			onUp(level)
		}
		sink.SessionUp(snap)
	case tr.SessionDown:
		if onDown != nil {
			onDown(level)
		}
		sink.SessionDown(snap)
	}
}

// SendHello builds and sends the periodic Hello(s) for this circuit. On a LAN it
// sends one IIH per configured level (different PDU types) to that level's
// multicast group, carrying the heard-SNPA list in TLV 6. On a P2P link it sends
// one IIH to both multicast groups carrying TLV 240 with our three-way state.
// The full IIH is built (origination TLVs + TLV 8 padding to the MTU) BEFORE any
// authentication; the transport frames the final bytes without padding.
func (c *Circuit) SendHello() error {
	ifMTU, ok := c.sender.InterfaceMTU(c.name)
	if !ok {
		ifMTU = 0 // no MTU known: send unpadded (still valid)
	}
	// The interface MTU bounds the 802.3 frame PAYLOAD (LLC header + IS-IS PDU),
	// not the IS-IS PDU alone (ISO/IEC 10589 clause 8.2.3 / mtu.go: a frame filling
	// a link is FrameHeaderLen + (MTU - LLCHeaderLen)). The Padding TLV is sized on
	// the PDU, so the pad target is MTU - LLCHeaderLen; padding to the full MTU
	// makes the framed Hello (LLC + PDU) exceed the link MTU and the kernel rejects
	// the send (EMSGSIZE), which silently kills every Hello on a real socket while
	// the smaller LSP/CSNP frames still flow -- the interop adjacency never forms.
	mtu := padMTU(ifMTU)
	if c.kind == adjacency.KindP2P {
		return c.sendP2PHello(mtu)
	}
	return c.sendLANHellos(mtu)
}

// padMTU converts the interface (L2 frame-payload) MTU into the maximum IS-IS PDU
// length that fits in one frame: MTU minus the LLC header the transport prepends.
// A zero or sub-LLC MTU yields 0 so padHello leaves the PDU unpadded (still a
// valid, shorter Hello). Keeping the subtraction here -- where the transport
// framing overhead is known -- lets padHello stay a pure "pad to N bytes" helper.
func padMTU(ifMTU int) int {
	if ifMTU <= transport.LLCHeaderLen {
		return 0
	}
	return ifMTU - transport.LLCHeaderLen
}

// sendLANHellos sends one padded LAN IIH per configured level.
func (c *Circuit) sendLANHellos(mtu int) error {
	snpas := c.heardSNPAs()
	for _, level := range c.levels {
		pdu := c.buildLANHello(level, snpas, mtu)
		pdu = padHello(pdu, mtu)
		// Sign AFTER padding, BEFORE framing (RFC 5304 sec 2 signs padded Hellos;
		// spec-isis-10). Unsigned when no IIH chain is configured.
		pdu = c.signHello(level, pdu)
		tl := transport.Level1
		if level == adjacency.Level2 {
			tl = transport.Level2
		}
		if err := c.sender.SendPDU(c.name, tl, pdu); err != nil {
			return err
		}
	}
	return nil
}

// sendP2PHello sends one padded P2P IIH to both level groups. Our three-way
// state, the neighbor echo, and the signing level are derived from the single
// P2P adjacency (if any).
func (c *Circuit) sendP2PHello(mtu int) error {
	state, neighborID, haveNeighbor, adjLevel := c.p2pThreeWayState()
	pdu := c.buildP2PHello(state, neighborID, haveNeighbor, mtu)
	pdu = padHello(pdu, mtu)
	// Sign AFTER padding, BEFORE framing (spec-isis-10). RFC 5303 sec 3: a P2P IIH
	// is level-agnostic on the wire (one PDU type, no level bit), so the IIH chain
	// is selected by the NEGOTIATED adjacency level, not the circuit's first
	// configured level. On an L1L2 circuit c.levels[0] is always Level1, which
	// would sign an L2-negotiated P2P session with the L1 key when the two IIH
	// chains differ; signing with adjLevel sends the key the L2 chain expects.
	// Before any neighbor is heard there is no negotiated level, so fall back to
	// the circuit's preferred P2P level (p2pPreferredLevel). Unsigned when none
	// configured.
	pdu = c.signHello(adjLevel, pdu)
	return c.sender.SendPDUBothLevels(c.name, pdu)
}

// p2pPreferredLevel is the level a P2P circuit signs its IIH with before any
// neighbor is heard (no negotiated adjacency level yet). It mirrors the receive
// side's p2pLevel preference: L1 when the circuit forms L1 (the common L1L2 and
// L1-only case), else L2 for an L2-only P2P circuit. This keeps the unsigned-
// neighbor case consistent with the level the eventual adjacency will negotiate.
func (c *Circuit) p2pPreferredLevel() adjacency.Level {
	if c.formsLevel(adjacency.Level1) {
		return adjacency.Level1
	}
	return adjacency.Level2
}

// p2pThreeWayState reports our RFC 5303 three-way state toward the P2P neighbor
// for our outgoing TLV 240: Up when our adjacency is Up, Initializing when we
// have heard the neighbor but not yet completed the handshake, Down when we have
// no adjacency. haveNeighbor is set (echoing the neighbor's System ID) once we
// have heard a Hello, which is the proof the neighbor needs to reach Up. The
// returned level is the active adjacency's NEGOTIATED level (the IIH signing
// chain selector); it is the circuit's preferred P2P level when there is no
// active adjacency yet.
func (c *Circuit) p2pThreeWayState() (packet.AdjThreeWayState, types.SystemID, bool, adjacency.Level) {
	var (
		state        = packet.AdjThreeWayDown
		neighborID   types.SystemID
		haveNeighbor bool
		level        = c.p2pPreferredLevel()
	)
	c.table.Each(func(a *adjacency.Adjacency) {
		if a.State == adjacency.StateDown {
			return
		}
		haveNeighbor = true
		neighborID = a.SystemID
		// The negotiated level the FSM stored on the active P2P adjacency
		// (adj.Level = in.Level, from p2pLevel) selects the IIH auth chain.
		level = a.Level
		if a.State == adjacency.StateUp {
			state = packet.AdjThreeWayUp
		} else {
			state = packet.AdjThreeWayInitializing
		}
	})
	return state, neighborID, haveNeighbor, level
}

// downEvent pairs a session-down transition with the snapshot captured under the
// table lock, so the event can be fired after the lock is released.
type downEvent struct {
	tr   adjacency.Transition
	snap adjacency.NeighborSnapshot
}

// Sweep runs the hold-timer timeout over every adjacency and reaps records whose
// grace period has elapsed. It is called periodically by the engine. Any Up
// adjacency that has not heard a Hello within its advertised hold time
// transitions to Down (ISO/IEC 10589 section 8.2.3) and a session-down event is
// emitted. Session events are fired AFTER the table lock is released (the metric
// hook re-reads the table). Sweep returns the number of adjacencies that went
// Down on this pass.
func (c *Circuit) Sweep() int {
	now := c.now()
	var events []downEvent
	c.table.Each(func(a *adjacency.Adjacency) {
		tr := adjacency.Expire(a, now, c.grace)
		if tr.SessionDown {
			events = append(events, downEvent{tr: tr, snap: a.Snapshot()})
		}
	})
	c.table.Reap(now)
	for _, e := range events {
		c.fireEvents(e.tr, e.snap)
	}
	return len(events)
}

// Teardown forces every adjacency Down (a circuit-down event) and emits the
// session-down events, then arms the grace period so the records linger briefly.
// Called by the engine when the iface EventBus reports the link down or IS-IS is
// disabled on the interface. Session events fire after the lock is released.
func (c *Circuit) Teardown() {
	now := c.now()
	var events []downEvent
	c.table.Each(func(a *adjacency.Adjacency) {
		tr := adjacency.Down(a, now, c.grace)
		if tr.SessionDown {
			events = append(events, downEvent{tr: tr, snap: a.Snapshot()})
		}
	})
	for _, e := range events {
		c.fireEvents(e.tr, e.snap)
	}
}
