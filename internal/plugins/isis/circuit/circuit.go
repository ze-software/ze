// Design: docs/architecture/isis/isis-5-adjacency.md -- per-interface IS-IS circuit runtime.
// ISO/IEC 10589 section 8.2 (adjacency), clause 9.5/9.6 (IIH), section 8.2.3
// (hold timer). A circuit is the per-interface runtime object created for each
// interface IS-IS is enabled on. It owns the adjacency table, the periodic Hello
// sender, the received-IIH dispatch into the FSM, and the hold-timer sweep.
//
// RFC: rfc/short/rfc5303.md -- P2P three-way state we report in our TLV 240
// RFC: rfc/short/rfc1195.md -- origination TLVs (1, 129, 132)
//
// The circuit performs I/O ONLY through the injected Sender (the spec-isis-3
// transport in production, a fake in tests) and the injected event/metric/clock
// hooks; the protocol decision lives in the pure adjacency FSM. The circuit
// goroutine is the single writer of the adjacency table.

package circuit

import (
	"net/netip"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// Sender is the transport surface a circuit needs to send IIHs and size the
// Padding TLV. *transport.Transport satisfies it; tests inject a fake. The
// transport adds ONLY 802.3+LLC framing to the final bytes (umbrella "Final PDU
// bytes" contract); the circuit owns padding and (later) signing.
type Sender interface {
	// SendPDU sends a final PDU to the level multicast group on the named circuit.
	SendPDU(name string, level transport.Level, pdu []byte) error
	// SendPDUBothLevels sends to both AllL1ISs and AllL2ISs (an L1L2 circuit).
	SendPDUBothLevels(name string, pdu []byte) error
	// InterfaceMTU returns the circuit MTU so the engine can size the Padding TLV.
	InterfaceMTU(name string) (int, bool)
}

// EventSink receives adjacency session up/down notifications so the circuit does
// not depend on the event-bus concrete type. The engine wires it to emit on the
// IS-IS events namespace (isis-4 events.go).
type EventSink interface {
	SessionUp(s adjacency.NeighborSnapshot)
	SessionDown(s adjacency.NeighborSnapshot)
}

// nopEventSink drops events; the default until the engine wires a real sink.
type nopEventSink struct{}

func (nopEventSink) SessionUp(adjacency.NeighborSnapshot)   {}
func (nopEventSink) SessionDown(adjacency.NeighborSnapshot) {}

// Config is the immutable per-circuit identity and parameters resolved from the
// component config (isis-4) and the transport (own MAC / ifindex / MTU). It is
// passed once to New; a config change rebuilds the circuit.
type Config struct {
	// Name is the interface name (the transport circuit key).
	Name string
	// IfIndex is the kernel interface index (the RX dispatch key).
	IfIndex int
	// SystemID is our own System ID.
	SystemID types.SystemID
	// SNPA is our own source MAC (the LAN three-way echo target). Zero on P2P.
	SNPA adjacency.SNPA
	// Areas are our configured area addresses (TLV 1 origination + L1 match).
	Areas []types.AreaID
	// IPv4 is our IPv4 interface address (TLV 132 origination + SPF next-hop).
	IPv4 netip.Addr
	// AdvertiseIPv6 adds NLPID 0x8E to TLV 129 (dual-stack circuits).
	AdvertiseIPv6 bool
	// IPv6LinkLocal is our IPv6 link-local interface address (fe80::/10). It is
	// originated in the IIH TLV 232 (RFC 5308 sec 3: a Hello carries ONLY
	// link-local addresses) so a dual-stack neighbor learns the IPv6 next-hop.
	// Invalid (zero) when the circuit has no IPv6 link-local address; the Hello
	// then omits TLV 232.
	IPv6LinkLocal netip.Addr
	// Kind is the circuit medium (broadcast or point-to-point).
	Kind adjacency.CircuitKind
	// Levels enumerates the routing levels this circuit forms adjacencies at.
	Levels []adjacency.Level
	// HelloInterval is the Hello send period in seconds.
	HelloInterval uint16
	// HoldMult is the advertised-hold-time multiplier (hold = interval * mult).
	HoldMult uint8
	// Priority is the DIS election priority advertised in a LAN IIH (0..127).
	Priority uint8
	// LocalCircuitID is the 1-octet local circuit ID for the P2P IIH / TLV 240.
	LocalCircuitID uint8
	// LANID is the LAN ID (DIS SourceID) advertised in a LAN IIH; zero until a
	// DIS is elected (isis-8 owns election; this spec sends zero).
	LANID types.SourceID
}

// Circuit is the per-interface IS-IS runtime.
type Circuit struct {
	// Immutable identity (set at construction).
	name           string
	ifIndex        int
	systemID       types.SystemID
	snpa           adjacency.SNPA
	areas          []types.AreaID
	ipv4           netip.Addr
	advertiseIPv6  bool
	ipv6LinkLocal  netip.Addr
	kind           adjacency.CircuitKind
	levels         []adjacency.Level
	helloInterval  uint16
	holdMult       uint8
	holdTime       uint16 // precomputed advertised hold time
	priority       uint8
	localCircuitID uint8
	lanID          types.SourceID

	sender Sender
	now    func() time.Time
	grace  time.Duration

	mu     sync.Mutex
	table  *adjacency.Table
	sink   EventSink
	onUp   func(level adjacency.Level)
	onDown func(level adjacency.Level)

	// sign, when set, signs a fully-built, padded IIH (inserts the TLV 10
	// authentication value as the first TLV) before it is handed to the transport
	// (spec-isis-10). Signing happens AFTER padding (RFC 5304 sec 2 signs padded
	// Hellos) and BEFORE framing. The level lets the engine pick the per-interface
	// (IIH) chain for that level. nil leaves the Hello unsigned (the default).
	sign func(level adjacency.Level, pdu []byte) []byte

	// dis holds the per-level DIS election state on a broadcast circuit (isis-8).
	// L1 and L2 elect independently (ISO/IEC 10589 clause 8.4.5), so there is one
	// DISState per level keyed by the adjacency.Level. A point-to-point circuit
	// never populates this map (P2P has no DIS and no pseudo-node). Guarded by mu.
	dis map[adjacency.Level]*DISState
}

// New constructs a circuit from cfg, sending via s. now supplies the current
// time (time.Now in production, a fake clock in tests). The circuit starts with
// no adjacencies; Receive feeds it Hellos and Sweep / SendHello drive the timers.
func New(cfg Config, s Sender, now func() time.Time) *Circuit {
	if now == nil {
		now = time.Now
	}
	c := &Circuit{
		name:           cfg.Name,
		ifIndex:        cfg.IfIndex,
		systemID:       cfg.SystemID,
		snpa:           cfg.SNPA,
		areas:          cfg.Areas,
		ipv4:           cfg.IPv4,
		advertiseIPv6:  cfg.AdvertiseIPv6,
		ipv6LinkLocal:  cfg.IPv6LinkLocal,
		kind:           cfg.Kind,
		levels:         cfg.Levels,
		helloInterval:  cfg.HelloInterval,
		holdMult:       cfg.HoldMult,
		holdTime:       HoldTime(cfg.HelloInterval, cfg.HoldMult),
		priority:       cfg.Priority,
		localCircuitID: cfg.LocalCircuitID,
		lanID:          cfg.LANID,
		sender:         s,
		now:            now,
		grace:          adjacency.DefaultGracePeriod,
		table:          adjacency.NewTable(),
		sink:           nopEventSink{},
	}
	if len(c.levels) == 0 {
		c.levels = []adjacency.Level{adjacency.Level1}
	}
	// A broadcast circuit holds a per-level DIS election state (isis-8). P2P has no
	// DIS, so the map stays nil there and the election methods are no-ops.
	if c.kind == adjacency.KindBroadcast {
		c.dis = make(map[adjacency.Level]*DISState, len(c.levels))
		for _, l := range c.levels {
			c.dis[l] = &DISState{}
		}
	}
	return c
}

// Name returns the interface name.
func (c *Circuit) Name() string { return c.name }

// IfIndex returns the kernel interface index (the RX dispatch key).
func (c *Circuit) IfIndex() int { return c.ifIndex }

// advertisedHoldTime returns the precomputed advertised holding time
// (hello-interval * hold-multiplier, clamped). Exposed for tests and metrics.
func (c *Circuit) advertisedHoldTime() uint16 { return c.holdTime }

// Table returns the per-circuit neighbor table (for the snapshot API).
func (c *Circuit) Table() *adjacency.Table { return c.table }

// SetEventSink wires the session up/down sink. Called by the engine at start.
func (c *Circuit) SetEventSink(s EventSink) {
	if s == nil {
		return
	}
	c.mu.Lock()
	c.sink = s
	c.mu.Unlock()
}

// SetTransitionHooks wires per-level up/down callbacks the engine uses to drive
// the ze_isis_adjacencies_up gauge. Either may be nil.
func (c *Circuit) SetTransitionHooks(onUp, onDown func(level adjacency.Level)) {
	c.mu.Lock()
	c.onUp = onUp
	c.onDown = onDown
	c.mu.Unlock()
}

// SetSigner installs the per-interface (IIH) signer (spec-isis-10). The signer
// takes the adjacency level and a fully-built, padded IIH and returns the signed
// bytes (TLV 10 inserted first). nil disables signing. Safe to call before the
// circuit goroutine starts; the send path reads it under c.mu.
func (c *Circuit) SetSigner(sign func(level adjacency.Level, pdu []byte) []byte) {
	c.mu.Lock()
	c.sign = sign
	c.mu.Unlock()
}

// signHello signs a padded IIH for level when a signer is installed, else returns
// the bytes unchanged (unauthenticated operation, the default).
func (c *Circuit) signHello(level adjacency.Level, pdu []byte) []byte {
	c.mu.Lock()
	sign := c.sign
	c.mu.Unlock()
	if sign == nil {
		return pdu
	}
	return sign(level, pdu)
}

// circuitTypeField maps the circuit's levels to the 1-octet IIH circuit-type
// field (ISO/IEC 10589 clause 9.5: low two bits select the levels). A circuit
// configured for both levels advertises CircuitL1L2.
func (c *Circuit) circuitTypeField() packet.CircuitType {
	hasL1, hasL2 := false, false
	for _, l := range c.levels {
		switch l {
		case adjacency.Level1:
			hasL1 = true
		case adjacency.Level2:
			hasL2 = true
		}
	}
	switch {
	case hasL1 && hasL2:
		return packet.CircuitL1L2
	case hasL2:
		return packet.CircuitL2
	default:
		return packet.CircuitL1
	}
}

// heardSNPAs returns the SNPAs of every neighbor this circuit currently has a
// non-Down adjacency to, for the TLV 6 (IS Neighbors) origination on a LAN. A
// neighbor reaches Up once it sees its own SNPA echoed in our TLV 6.
func (c *Circuit) heardSNPAs() []adjacency.SNPA {
	var out []adjacency.SNPA
	c.table.Each(func(a *adjacency.Adjacency) {
		if a.State != adjacency.StateDown && a.SNPA != (adjacency.SNPA{}) {
			out = append(out, a.SNPA)
		}
	})
	return out
}
