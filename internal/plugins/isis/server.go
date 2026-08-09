// Design: docs/architecture/isis/isis-4-component-config.md -- IS-IS engine orchestration
// Related: config.go -- typed Config the engine reconciles to
// Related: events.go -- lifecycle events the engine emits
//
// server.go owns the top-level IS-IS engine: it opens a circuit per enabled
// interface over the spec-isis-3 L2 transport, launches per-circuit goroutine
// stubs (later specs fill RX/TX/timers/adjacency), and runs the PDU-type receive
// dispatcher. The transport delivers (ifindex, pdu) after stripping 802.3+LLC
// and holds NO protocol switch; the dispatcher here routes by the 5-bit PDU type
// (Shared Contracts "PDU receive dispatcher", owner isis-4):
//   - IIH (0x0f L1 LAN, 0x10 L2 LAN, 0x11 P2P) -> adjacency (isis-5)
//   - LSP (0x12 L1, 0x14 L2) + CSNP/PSNP (0x18/0x19/0x1a/0x1b) -> lsdb/flooding
//     (isis-6/isis-7)
// Handlers register at startup; this spec installs the dispatcher with stub
// handlers. Reload reconciles circuits incrementally via a journal (AC-8), not
// restart-everything.

package isis

import (
	"context"
	"log/slog"
	"maps"
	"net/netip"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/circuit"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/spf"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
	"github.com/ze-software/ze/pkg/ze"
)

// sweepInterval is how often the engine runs the per-circuit hold-timer sweep
// (ISO/IEC 10589 section 8.2.3). It is short relative to the minimum hold time
// (1s) so an expired adjacency is detected promptly without busy-waiting.
const sweepInterval = 1 * time.Second

// offPDUType is the byte offset of the PDU type octet in the IS-IS common header
// (ISO/IEC 10589 clause 9.5). The low 5 bits are the PDU type code.
const offPDUType = 4

// pduTypeMask isolates the 5-bit PDU type from the type octet (the upper 3 bits
// are reserved). Matches packet.pduTypeMask, kept local so the dispatcher can
// extract the type from a raw, possibly-malformed PDU without round-tripping
// through the full header decoder (which rejects unknown types).
const pduTypeMask = 0x1f

// pduHandler handles one received PDU on a circuit. It is given the full
// received frame (source ifindex, source/destination MAC, and the LLC-stripped
// PDU bytes). The adjacency IIH handler needs the source MAC (SNPA) for the LAN
// three-way check, so the dispatcher passes the whole RawFrame rather than only
// (ifindex, pdu).
type pduHandler func(rf transport.RawFrame)

// dispatcher routes received PDUs to handlers keyed by the 5-bit PDU type. The
// transport feeds it a RawFrame; the dispatcher never inspects anything but the
// type octet. Handlers register at startup. Unknown or short PDUs are dropped
// (counted), never panicked.
type dispatcher struct {
	mu         sync.RWMutex
	handlers   map[packet.PDUType]pduHandler
	droppedCnt uint64

	// verify, when set, authenticates a received frame before it is routed to a
	// handler (spec-isis-10). It returns true to accept (proceed to the handler)
	// and false to reject (drop the frame; the verify hook itself increments
	// ze_isis_auth_failures_total). nil means no authentication is configured, so
	// every frame proceeds (unauthenticated operation, the default).
	verify func(rf transport.RawFrame) bool
}

// newDispatcher constructs an empty dispatcher.
func newDispatcher() *dispatcher {
	return &dispatcher{handlers: make(map[packet.PDUType]pduHandler)}
}

// register binds a handler to a PDU type. Called at startup by the engine
// (isis-4 stubs, isis-5 adjacency) and, in later specs, by the LSDB subsystems.
func (d *dispatcher) register(pt packet.PDUType, h pduHandler) {
	d.mu.Lock()
	d.handlers[pt] = h
	d.mu.Unlock()
}

// dispatch routes one received frame by its PDU's 5-bit type. A PDU too short to
// carry a type octet, or one whose type has no registered handler, is dropped
// and counted (security review: the receive path bound-checks before indexing
// and never panics on attacker-controlled bytes).
func (d *dispatcher) dispatch(rf transport.RawFrame) {
	if len(rf.PDU) <= offPDUType {
		d.drop()
		return
	}
	pt := packet.PDUType(rf.PDU[offPDUType] & pduTypeMask)
	d.mu.RLock()
	h := d.handlers[pt]
	verify := d.verify
	d.mu.RUnlock()
	if h == nil {
		d.drop()
		return
	}
	// Authenticate before routing (spec-isis-10): a PDU that fails verification is
	// dropped before any adjacency/LSDB/SNP processing (it must not form or sustain
	// an adjacency, be stored, or satisfy synchronization). The verify hook itself
	// increments ze_isis_auth_failures_total on rejection.
	if verify != nil && !verify(rf) {
		return
	}
	h(rf)
}

// setVerify installs the authentication verify hook (spec-isis-10). nil disables
// verification (unauthenticated operation).
func (d *dispatcher) setVerify(verify func(rf transport.RawFrame) bool) {
	d.mu.Lock()
	d.verify = verify
	d.mu.Unlock()
}

func (d *dispatcher) drop() {
	d.mu.Lock()
	d.droppedCnt++
	d.mu.Unlock()
}

// dropped returns the count of PDUs dropped (unknown type or too short).
func (d *dispatcher) dropped() uint64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.droppedCnt
}

// engine is the running IS-IS instance: config, transport, dispatcher, the
// adjacency circuits, and the per-circuit goroutine lifecycle.
type engine struct {
	transport *transport.Transport
	dispatch  *dispatcher
	log       *slog.Logger

	mu      sync.Mutex
	cfg     Config
	running map[string]InterfaceConfig // circuits the engine has opened, by name

	// circuitsMu guards the adjacency-circuit maps. Circuits are added on open
	// and removed on close; the receive fan and the CLI snapshot read them under
	// this lock. Keyed by ifindex (RX dispatch) and by name (close/snapshot).
	circuitsMu    sync.RWMutex
	circuits      map[int]*circuit.Circuit
	circuitByName map[string]*circuit.Circuit
	// circuitStop holds the per-circuit goroutine stop channel keyed by interface
	// name. launchCircuitGoroutine creates one when it starts the hello+sweep loop;
	// onCircuitDown/closeCircuit close it so the goroutine exits on circuit removal
	// (link-down/disable/reconcile-remove) instead of leaking until engine shutdown.
	// Keyed by name so a reopen of the same interface (a fresh ifindex) reuses the
	// slot and can never run two goroutines for one circuit.
	circuitStop map[string]chan struct{}

	// Adjacency metrics (isis-5 owns these umbrella-canonical series). A gauge
	// vector by (level, interface) for up adjacencies, and a per-level total.
	adjUp    metrics.GaugeVec // ze_isis_adjacencies_up{level,interface}
	adjTotal metrics.GaugeVec // ze_isis_adjacencies_total{level}

	// LSDB subsystem (isis-6): the two-level link-state database, the own-LSP
	// originator, the per-circuit flag-index assignment, and the
	// connected/redistributed prefixes origination advertises (fed by isis-11).
	lsdb          *lsdb.LSDB
	originator    *lsdb.Originator
	circuitIDs    map[string]lsdb.CircuitID
	nextCircuitID lsdb.CircuitID
	// prefixes holds the node's own connected-interface prefixes per level (TLV
	// 135 internal reachability, enumerated at circuit-up by isis-11's
	// connected-prefix advertisement). setPrefixes replaces a level's set.
	prefixes map[lsdb.Level][]lsdb.PrefixInfo
	// redistPrefixes holds routes IMPORTED into IS-IS by the redistribution
	// consumer (connected/static/BGP -> TLV 135), keyed per level by prefix so a
	// withdraw removes exactly one. Merged with prefixes in levelState. Distinct
	// from prefixes so connected advertisement and redistribution import do not
	// clobber each other (isis-11).
	redistPrefixes map[lsdb.Level]map[netip.Prefix]lsdb.PrefixInfo
	// prefixesV6 / redistPrefixesV6 are the IPv6 (TLV 236) equivalents of
	// prefixes / redistPrefixes (isis-12): connected IPv6 prefixes and redistributed
	// IPv6 routes, merged into the IPv6 LevelState on origination. Kept separate
	// from the IPv4 maps so IPv6 origination is gated independently and never
	// clobbers the IPv4 set.
	prefixesV6       map[lsdb.Level][]lsdb.PrefixInfoV6
	redistPrefixesV6 map[lsdb.Level]map[netip.Prefix]lsdb.PrefixInfoV6
	// leakedPrefixes / leakedPrefixesV6 hold the RFC 2966 inter-level leak set the
	// SPF Computer hands back after each run (applyLeak): on an L1L2 router the
	// OTHER level's reachable IS-IS prefixes, re-originated into this level's own
	// LSP with the up/down bit set for an L2->L1 down leak. Kept separate from
	// prefixes/redistPrefixes (which carry the up/down bit clear) so a leaked entry
	// never clobbers a connected/redistributed one and levelState can merge all
	// three. Guarded by e.mu. AC-4/AC-5; loop prevention lives in spf.LeakPrefixes.
	leakedPrefixes   map[lsdb.Level][]lsdb.PrefixInfo
	leakedPrefixesV6 map[lsdb.Level][]lsdb.PrefixInfoV6
	// lspReorigs is ze_isis_lsp_reoriginations_total{level}, OWNED and registered
	// by isis-11: incremented per level when a redistribution inject/withdraw
	// drives a re-origination of the own LSP set. no-op until setMetrics wires it.
	lspReorigs metrics.CounterVec

	// flooder (isis-7): the reliable-flooding + CSNP/PSNP synchronization engine.
	// It drives the LSDB SRM/SSN flags over the wire and owns the per-circuit
	// pending-request set. Constructed in initFlooding (after initLSDB) so it
	// shares the engine's LSDB; the LSP/CSNP/PSNP dispatcher handlers route here.
	flooder *lsdb.Flooder

	// spf (isis-9): the SPF computer that builds the per-level graph from the
	// LSDB, runs Dijkstra, and INSERTS the resulting routes into the shared
	// Loc-RIB (sysrib + fibkernel program the kernel). It is debounce-triggered on
	// every LSDB change (triggerSPF) and reads the LSDB via the spf.Source adapter.
	spf *spf.Computer

	// DIS subsystem (isis-8): the per-(circuit,level) elected pseudo-node identity
	// the own-LSP star encoding points at, plus the DIS-election Prometheus series
	// this spec OWNS. disMu guards disPseudonode. A broadcast circuit on which the
	// local node is DIS originates a pseudo-node LSP and sources the LAN CSNP; every
	// router (DIS or not) records the elected pseudo-node so its own LSP advertises
	// the LAN as a single TLV 22 entry (the star, AC-7) rather than per-peer.
	disMu         sync.Mutex
	disPseudonode map[disKey]types.SourceID // (circuit name, level) -> elected pseudo-node Source ID
	disElections  metrics.CounterVec        // ze_isis_dis_elections_total{level}
	pseudonodeG   metrics.GaugeVec          // ze_isis_pseudonode_lsps{level}
	// pnLastInput records, per (circuit,level) where the local node is DIS, the
	// INPUT of the last pseudo-node LSP origination (the member set plus the LSP
	// attributes). originatePseudonode is a pure function of this input, so an
	// identical input means an identical pseudo-node LSP: the DIS skips the
	// regenerate/store/flood when the input is unchanged AND no refresh is yet due.
	// This collapses the per-second re-election tick (reelectTick -> runElection ->
	// originatePseudonode on every IsLocalDIS pass) from one re-flood / sequence bump
	// per second to one per real change (the pseudo-node twin of lastOrigInput; the
	// flooding-amplification fix for the DIS, Bundle E deferred item 2). Guarded by
	// disMu.
	pnLastInput map[disKey]pnInput
	// pnLastOrigAt records, per (circuit,level) where the local node is DIS, the
	// wall-clock time of the last pseudo-node LSP origination. The aging loop forces
	// a refresh (re-originates even on an unchanged input) once lsp-refresh-interval
	// has elapsed, so a DIS's pseudo-node LSP is re-stamped well before MaxAge and
	// never ages out of peers' LSDBs (ISO/IEC 10589 clause 7.3.16.1, the pseudo-node
	// twin of lastOrigAt). Guarded by disMu.
	pnLastOrigAt map[disKey]time.Time

	sink *eventSink

	// keystore (isis-10): the resolved authentication key chains, rebuilt on every
	// config apply. Guarded by ksMu; read on the hot path (sign on TX, verify on
	// RX). nil/unconfigured means unauthenticated operation (the default).
	ksMu     sync.RWMutex
	keystore *keyStore
	// authFailures (isis-10) OWNS and registers ze_isis_auth_failures_total
	// {level,interface}; incremented at the verify-reject site. isis-13 only
	// scrapes it.
	authFailures metrics.CounterVec

	// origMu serializes the engine's own-LSP origination reaction (lsdb_wiring.go
	// originate()). originate() is called from many goroutines (every adjacency
	// transition hook, the DIS re-elect tick, circuit close, redistribution, the
	// SPF leak callback). Serializing it makes the "did anything change?"
	// compare-and-originate atomic so a burst of identical re-origination requests
	// under an adjacency flap collapses to ONE re-flood / ONE sequence bump instead
	// of N (the flooding-amplification fix). origMu is acquired FIRST in
	// originate(); the body then takes e.mu / e.circuitsMu / the Originator/LSDB
	// locks, so there is no lock inversion (nothing acquires those THEN origMu).
	origMu sync.Mutex
	// lastOrigInput records, per level, the origination INPUT (node identity +
	// live level state) of the last own-LSP origination. origination is a pure
	// function of this input, so an identical input means an identical own LSP:
	// originate() skips the regenerate/store/flood when the input is unchanged AND
	// no refresh is yet due (so an own LSP never ages out, ISO/IEC 10589 clause
	// 7.3.16.1). Guarded by e.mu (written under origMu in originate()).
	lastOrigInput map[lsdb.Level]origInput
	// lastOrigAt records, per level, the wall-clock time of the last own-LSP
	// origination. originate() forces a refresh (re-originates even on an unchanged
	// input) once lsp-refresh-interval has elapsed since lastOrigAt, so a stable
	// node's own LSP is re-stamped well before MaxAge. Measuring elapsed time here
	// (rather than the stored entry's Remaining Lifetime) keeps the freshness probe
	// race-free: the LSDB aging goroutine mutates the entry's lifetime under the
	// LSDB lock, and originate() holds no LSDB lock at the decision point. Guarded
	// by e.mu.
	lastOrigAt map[lsdb.Level]time.Time

	// electMu serializes the DIS-election REACTION for the engine (dis_wiring.go
	// runElection): the election commit (atomic under the circuit mutex) PLUS the
	// follow-on pseudo-node allocate/originate/record + own-LSP re-origination
	// decision, which run OUTSIDE the circuit mutex. runElection is invoked
	// concurrently for the same circuit from the receive, hold-timer-sweep, and
	// DIS-loop goroutines; without this lock two reactions could interleave to a
	// stale pseudo-node/own-LSP outcome. Holding electMu across the whole reaction
	// makes each election+reaction atomic and ordered (the last to commit wins, and
	// its reaction is the one that lands). electMu is acquired FIRST in runElection;
	// the body then takes the circuit mutex / e.mu / e.disMu, so no inversion.
	electMu sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// newEngine constructs an engine over the given transport. The transport's
// backend is the only platform dependency (a fake in tests, AF_PACKET in
// production).
func newEngine(t *transport.Transport) *engine {
	ctx, cancel := context.WithCancel(context.Background())
	e := &engine{
		transport:     t,
		dispatch:      newDispatcher(),
		log:           logger(),
		running:       make(map[string]InterfaceConfig),
		circuits:      make(map[int]*circuit.Circuit),
		circuitByName: make(map[string]*circuit.Circuit),
		circuitStop:   make(map[string]chan struct{}),
		adjUp:         metrics.NopRegistry{}.GaugeVec("", "", nil),
		adjTotal:      metrics.NopRegistry{}.GaugeVec("", "", nil),
		disPseudonode: make(map[disKey]types.SourceID),
		disElections:  metrics.NopRegistry{}.CounterVec("", "", nil),
		pseudonodeG:   metrics.NopRegistry{}.GaugeVec("", "", nil),
		authFailures:  metrics.NopRegistry{}.CounterVec("", "", nil),
		lspReorigs:    metrics.NopRegistry{}.CounterVec("", "", nil),
		ctx:           ctx,
		cancel:        cancel,
	}
	e.initLSDB()
	e.initFlooding()
	e.initSPF()
	e.installStubHandlers()
	e.installFloodHandlers()
	t.OnCircuitDown(e.onCircuitDown)
	return e
}

// setMetrics registers the adjacency-owned Prometheus series on reg. This spec
// OWNS and registers ze_isis_adjacencies_up{level,interface} and
// ze_isis_adjacencies_total{level} (umbrella Metrics table, owner isis-5). Other
// ze_isis_* series are registered by their owning specs.
func (e *engine) setMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	e.mu.Lock()
	e.adjUp = reg.GaugeVec(
		"ze_isis_adjacencies_up",
		"Current number of IS-IS adjacencies in the Up state, by level and interface.",
		[]string{"level", "interface"},
	)
	e.adjTotal = reg.GaugeVec(
		"ze_isis_adjacencies_total",
		"Current number of IS-IS adjacencies (any state) by level.",
		[]string{"level"},
	)
	e.mu.Unlock()
	// The DIS subsystem (isis-8) OWNS and registers exactly these umbrella-canonical
	// series: the DIS-election counter and the pseudo-node-LSP gauge, both by level.
	e.disMu.Lock()
	e.disElections = reg.CounterVec(
		"ze_isis_dis_elections_total",
		"Total IS-IS Designated IS elections that changed the elected DIS, by level.",
		[]string{"level"},
	)
	e.pseudonodeG = reg.GaugeVec(
		"ze_isis_pseudonode_lsps",
		"Current number of pseudo-node LSPs this node originates as the elected DIS, by level.",
		[]string{"level"},
	)
	e.disMu.Unlock()
	// The authentication subsystem (isis-10) OWNS and registers exactly this
	// umbrella-canonical series; isis-13 only scrapes it.
	e.ksMu.Lock()
	e.authFailures = reg.CounterVec(
		"ze_isis_auth_failures_total",
		"Total IS-IS PDUs rejected because authentication verification failed, by level and interface.",
		[]string{"level", "interface"},
	)
	e.ksMu.Unlock()
	e.publishPseudonodeMetric()
	// The LSDB subsystem (isis-6) OWNS and registers its own umbrella-canonical
	// series (ze_isis_lsps / lsp_fragments / lsp_originations_total /
	// sequence_wraps_total / purges_total).
	if e.lsdb != nil {
		e.lsdb.SetMetrics(reg)
	}
	// The flooding subsystem (isis-7) OWNS and registers the flooding/SNP series
	// (ze_isis_lsps_received_total / lsps_transmitted_total / csnp_sent_total /
	// csnp_received_total / psnp_sent_total / psnp_received_total /
	// srm_resends_total / lsps_dropped_total).
	if e.flooder != nil {
		e.flooder.SetMetrics(reg)
	}
	// The SPF subsystem (isis-9) OWNS and registers the SPF series
	// (ze_isis_spf_runs_total / spf_duration_seconds / spf_nodes) and, via its
	// Installer, ze_isis_routes_installed.
	if e.spf != nil {
		e.spf.SetMetrics(reg)
	}
	// Redistribution re-origination counter (isis-11 OWNS this row):
	// ze_isis_lsp_reoriginations_total{level}, incremented when a redistribution
	// inject/withdraw drives a re-origination.
	e.lspReorigs = reg.CounterVec(
		"ze_isis_lsp_reoriginations_total",
		"Total IS-IS own-LSP re-originations driven by redistribution inject/withdraw, by level.",
		[]string{"level"},
	)
}

// setEventSink wires the session up/down event bus so circuits emit on the
// IS-IS events namespace (isis-4 events.go). nil leaves a discard sink.
func (e *engine) setEventSink(s *eventSink) { e.sink = s }

// installStubHandlers registers handlers for every PDU type so the dispatcher is
// wired end-to-end. The IIH types (0x0f/0x10/0x11) route to the adjacency
// handler (isis-5); the LSP/CSNP/PSNP types are stubbed until isis-6/isis-7.
func (e *engine) installStubHandlers() {
	for _, pt := range []packet.PDUType{
		packet.PDUTypeL1LANHello, packet.PDUTypeL2LANHello, packet.PDUTypeP2PHello,
	} {
		e.dispatch.register(pt, e.handleIIH)
	}
	stub := func(transport.RawFrame) {} // no-op: lsdb/flooding land in isis-6/7
	for _, pt := range []packet.PDUType{
		packet.PDUTypeL1LSP, packet.PDUTypeL2LSP,
		packet.PDUTypeL1CSNP, packet.PDUTypeL2CSNP,
		packet.PDUTypeL1PSNP, packet.PDUTypeL2PSNP,
	} {
		e.dispatch.register(pt, stub)
	}
}

// handleIIH routes a received IIH to the circuit that owns its source ifindex.
// The transport delivers frames keyed by ifindex; the engine maps ifindex ->
// circuit and feeds the circuit the source SNPA (for the LAN three-way check)
// and the PDU. A frame whose ifindex has no live circuit is dropped.
func (e *engine) handleIIH(rf transport.RawFrame) {
	e.circuitsMu.RLock()
	c := e.circuits[rf.IfIndex]
	e.circuitsMu.RUnlock()
	if c == nil {
		return
	}
	c.Receive(adjacency.SNPA(rf.SrcMAC), rf.PDU)
}

// setConfig stores the active config (used by openCircuits and reconcile). It
// also records the node's own System ID on the flooder so a received copy of our
// own LSP is recognized (isis-7 ReceiveLSP own flag).
func (e *engine) setConfig(cfg Config) {
	e.mu.Lock()
	e.cfg = cfg
	e.mu.Unlock()
	if e.flooder != nil {
		e.flooder.SetSystemID(cfg.SystemID)
	}
	// Rebuild the authentication key store and (re)install the sign/verify hooks
	// from the resolved key chains (isis-10). A key-chain change on reload takes
	// effect here without a restart (hitless rotation, AC-4).
	e.setKeyStore(cfg)
	// Root SPF at the node's own System ID so Dijkstra runs from this node, and
	// scope it to the configured levels (isis-9).
	if e.spf != nil {
		e.spf.SetRoot(cfg.SystemID)
		e.spf.SetLevels(spfLevelsFor(cfg.Level))
	}
}

// spfLevelsFor maps the node's configured level to the SPF levels it computes.
func spfLevelsFor(l Level) []spf.Level {
	switch l {
	case LevelL1:
		return []spf.Level{spf.Level1}
	case LevelL2:
		return []spf.Level{spf.Level2}
	default:
		return []spf.Level{spf.Level1, spf.Level2}
	}
}

// openCircuits opens a circuit per enabled, non-passive interface via the
// transport, marking each interface enabled so a later link-up event reopens it.
// It launches the receive-fan goroutine that feeds the dispatcher.
func (e *engine) openCircuits() error {
	e.mu.Lock()
	circuits := e.cfg.EnabledCircuits()
	e.mu.Unlock()

	// Start the single delivery-fan goroutine once: it reads the transport's
	// merged receive channel and dispatches each PDU by type.
	e.startReceiveLoop()
	// Start the per-second LSP aging loop (isis-6): decrement Remaining Lifetime,
	// purge at 0, garbage-collect after the grace period.
	e.startAgingLoop()
	// Start the flooding loops (isis-7): the periodic SRM-draining flood timer,
	// the PSNP ack/request timer, and the P2P periodic CSNP timer.
	e.startFloodLoops()
	// Start the DIS loop (isis-8): the periodic LAN CSNP cadence the DIS sources
	// and the periodic re-election that catches a DIS lost via the hold-timer sweep.
	e.startDISLoop()

	for _, ic := range circuits {
		if err := e.openCircuit(ic); err != nil {
			return err
		}
	}
	// Enumerate the node's own enabled/passive interface prefixes as internal TLV
	// 135 reachability (isis-11 connected-prefix advertisement, AC-8) before the
	// first origination, so a passive interface's prefix appears in the own LSP
	// without an adjacency.
	e.refreshConnectedPrefixes()
	// Originate the node's own LSP set now that the circuits exist (an idle
	// adjacency set still produces fragment 0 advertising the node, its areas,
	// protocols, interface addresses, hostname, and overload bit). Adjacency
	// transitions re-originate as neighbors come up (the Wiring Test).
	e.originate()
	return nil
}

// openCircuit enables the interface at its level and opens the raw circuit. The
// transport's EnableInterface + HandleLinkUp split lets a link that is down at
// startup open later on `interface/up`.
func (e *engine) openCircuit(ic InterfaceConfig) error {
	e.transport.EnableInterface(ic.Name, ic.Level.TransportLevel())
	if err := e.transport.HandleLinkUp(ic.Name); err != nil {
		return err
	}
	e.mu.Lock()
	e.running[ic.Name] = ic
	e.mu.Unlock()
	e.launchCircuitGoroutine(ic)
	return nil
}

// startReceiveLoop runs one goroutine that drains the transport's merged receive
// channel and routes each PDU through the dispatcher. The transport never
// switches on PDU type; this loop is the single delivery point into the
// dispatcher (Shared Contracts "Transport <-> PDU dispatcher").
func (e *engine) startReceiveLoop() {
	recv := e.transport.Receive()
	e.wg.Go(func() {
		for {
			select {
			case <-e.ctx.Done():
				return
			case rf, ok := <-recv:
				if !ok {
					return
				}
				e.dispatch.dispatch(rf)
			}
		}
	})
}

// subscribeIfaceEvents wires the iface EventBus through the transport so a
// configured interface that comes up after start opens its circuit and a down
// link closes it. The transport owns the EventBus handler (non-blocking enqueue
// + worker); the engine only passes the bus through. The unsubscribe runs on
// transport.Close during shutdown.
func (e *engine) subscribeIfaceEvents(eb ze.EventBus) {
	if eb == nil {
		return
	}
	_ = e.transport.SubscribeIfaceEvents(eb)
}

// reconcileResult is the journal of a reload reconcile: which circuits were
// opened, closed, or changed-in-place. A metric-only change marks the circuit
// changed but opens/closes nothing (AC-8).
type reconcileResult struct {
	opened  []string
	closed  []string
	changed map[string]bool
}

// reconcile diffs newCfg against the running circuits and applies the minimal
// set of changes: open added circuits, close removed ones, and mark in-place
// parameter changes (metric/hello/etc.) without tearing the circuit down. This
// is the journal-based incremental reload (AC-8, R-2), not restart-everything.
func (e *engine) reconcile(newCfg Config) reconcileResult {
	res := reconcileResult{changed: make(map[string]bool)}

	e.mu.Lock()
	e.cfg = newCfg
	e.mu.Unlock()

	desired := make(map[string]InterfaceConfig)
	for _, ic := range newCfg.EnabledCircuits() {
		desired[ic.Name] = ic
	}

	e.mu.Lock()
	current := make(map[string]InterfaceConfig, len(e.running))
	maps.Copy(current, e.running)
	e.mu.Unlock()

	// Remove circuits no longer desired.
	for name := range current {
		if _, keep := desired[name]; !keep {
			e.closeCircuit(name)
			res.closed = append(res.closed, name)
		}
	}

	// Add new circuits and reconcile changed parameters in place.
	for name, want := range desired {
		have, exists := current[name]
		switch {
		case !exists:
			if err := e.openCircuit(want); err != nil {
				e.log.Warn("isis: reconcile open failed", "interface", name, "err", err)
				continue
			}
			res.opened = append(res.opened, name)
		case !circuitParamsEqual(have, want):
			// In-place parameter change: update the stored config; the live
			// socket stays open (no flap). Runtime application of the new
			// parameters (hello timer, metric in LSP) lands in isis-5/6.
			e.mu.Lock()
			e.running[name] = want
			e.mu.Unlock()
			res.changed[name] = true
		}
	}
	return res
}

// closeCircuit disables the interface and closes its circuit. DisableInterface
// routes through HandleLinkDown -> onCircuitDown, which closes the per-circuit
// stop channel so the hello+sweep goroutine exits and clears the maps. The
// fallback close below covers the case where the transport circuit was already
// down (no onDown fires) yet a stop channel lingers: it must never leak.
func (e *engine) closeCircuit(name string) {
	e.transport.DisableInterface(name)
	e.circuitsMu.Lock()
	if stop, ok := e.circuitStop[name]; ok {
		close(stop)
		delete(e.circuitStop, name)
	}
	e.circuitsMu.Unlock()
	e.mu.Lock()
	delete(e.running, name)
	e.mu.Unlock()
}

// circuitParamsEqual reports whether two interface configs are identical for the
// fields that affect a running circuit. Name equality is implied by the caller.
func circuitParamsEqual(a, b InterfaceConfig) bool {
	return a.Enabled == b.Enabled &&
		a.Passive == b.Passive &&
		a.CircuitType == b.CircuitType &&
		a.Level == b.Level &&
		a.Metric == b.Metric &&
		a.HelloInterval == b.HelloInterval &&
		a.HoldMult == b.HoldMult &&
		a.Priority == b.Priority
}

// shutdown stops all circuit goroutines, the receive loop, and the transport,
// then waits for the goroutines to exit (no leak on reload or stop).
func (e *engine) shutdown() {
	e.cancel()
	// Forward-remove every IS-IS route from the Loc-RIB so a stopped engine
	// leaves no stale FIB entries (isis-9; mirrors withdrawing on neighbor loss).
	if e.spf != nil {
		e.spf.Stop()
	}
	e.transport.Close()
	e.wg.Wait()
}
