// Design: docs/architecture/isis/isis-7-flooding.md -- LSP flooding (receive-side algorithm + periodic SRM-driven TX).
// ISO/IEC 10589 clause 7.3.14-17: reliable flooding disseminates LSPs so every
// router in a level converges on the same LSDB. isis-6 owns the LSDB store, the
// freshness compare (LSDB.Receive) and the per-circuit SRM/SSN flag storage;
// THIS file owns the algorithm that consumes and produces those flags over the
// wire: it maps a received LSP's freshness to SRM (send) / SSN (acknowledge)
// transitions and drains SRM on a periodic timer by re-transmitting the stored
// raw LSP bytes verbatim (clause 7.3.14: unknown TLVs re-flood byte-for-byte).
//
// The Flooder holds NO LSDB storage: it carries a *LSDB reference plus two
// injected function fields (tx and circuits) the engine wires to the isis-3
// transport and the running circuit set. This keeps all flooding/SNP logic in
// the lsdb package (rule plugin-self-containment) with no import of the
// transport or circuit packages and no import cycle.

package lsdb

import (
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// FloodCircuit is the engine's view of one open circuit, as the flooding/SNP
// layer needs it. The engine builds it from the running InterfaceConfig so the
// Flooder never imports the circuit or transport packages. ID is the LSDB
// CircuitID the SRM/SSN flags are keyed by; Name is the transport circuit key
// for tx; P2P selects the initial-CSNP-at-Up behavior (SRM is cleared on
// acknowledgement on both P2P and LAN, so P2P no longer picks a distinct clear
// policy); Passive circuits are skipped on the flood timer (ISO/IEC 10589: a
// passive circuit forms no adjacency and floods nothing); FormsL1/FormsL2 gate
// per-level eligibility (an L1L2 circuit forms both).
type FloodCircuit struct {
	Name    string
	ID      CircuitID
	P2P     bool
	Passive bool
	FormsL1 bool
	FormsL2 bool
}

// formsLevel reports whether the circuit participates in the given level.
func (c FloodCircuit) formsLevel(level Level) bool {
	if level == Level2 {
		return c.FormsL2
	}
	return c.FormsL1
}

// TxFunc transmits a final PDU (already complete bytes) to the level multicast
// group on a named circuit. The engine wires it to transport.SendPDU; the
// transport adds only 802.3+LLC framing (umbrella "Final PDU bytes" contract).
type TxFunc func(circuitName string, level Level, pdu []byte) error

// CircuitsFunc returns the currently-open circuits the flooder may transmit on.
// The engine wires it to derive the set from the running InterfaceConfig under
// its own lock, so the Flooder always sees the live circuit set without holding
// engine state itself.
type CircuitsFunc func() []FloodCircuit

// Flooder runs the reliable-flooding algorithm over an LSDB. It is constructed
// by the engine with the LSDB, the transmit hook, and the circuit-set provider.
// The SNP (CSNP/PSNP) build/receive logic and the per-circuit pending-request
// set live in snp.go on the same type.
type Flooder struct {
	db       *LSDB
	tx       TxFunc
	circuits CircuitsFunc

	// own is the node's System ID, so a received copy of our own LSP is
	// recognized (passed to LSDB.Receive as own=true). Set via SetSystemID.
	ownMu    sync.RWMutex
	systemID types.SystemID

	// sign, when set, signs a freshly built CSNP/PSNP (inserts the TLV 10
	// authentication value as the first TLV) before it is transmitted (spec-
	// isis-10). CSNP/PSNP carry no Fletcher checksum, so signing only inserts
	// TLV 10. LSP signing happens at ORIGINATION (Originator.SetSigner), not here,
	// because the LSDB stores and re-floods raw LSP bytes verbatim. nil leaves the
	// SNP unsigned (unauthenticated operation, the default). Read under signMu.
	signMu sync.RWMutex
	sign   func(pdu []byte) []byte

	// pending is the per-circuit set of LSPs we know about (from a CSNP) but do
	// NOT yet hold, so a PSNP can request them. Owned here (snp.go drives it),
	// guarded by pendMu. Keyed by CircuitID then level then LSPID.
	pendMu  sync.Mutex
	pending map[CircuitID]map[Level]map[types.LSPID]pendingReq

	// ackOnly is the per-circuit set of LSPs this node must ACKNOWLEDGE in its
	// next PSNP but does NOT hold, so the SSN flag cannot carry the ack (SSN lives
	// on an LSDB entry). ISO/IEC 10589 clause 7.3.16.4 a) requires exactly that
	// pairing: "If no LSP from S is in memory, then the IS shall 1) send an
	// acknowledgement of the LSP on circuit C, but 2) shall not retain the LSP
	// after the acknowledgement has been sent." Each entry carries the ARRIVED
	// LSP's own sequence, lifetime and checksum, because an acknowledgement echoes
	// what was received. Drained (and cleared) by the next PSNP. Guarded by
	// pendMu, keyed like pending.
	ackOnly map[CircuitID]map[Level]map[types.LSPID]packet.LSPEntry

	// metrics holds the per-level Prometheus handles (umbrella canonical series,
	// owner isis-7) behind an atomic pointer. SetMetrics swaps in a fully-built set
	// with a single atomic store; the hot flooding/SNP paths load it with a single
	// atomic load (cheaper than a per-counter lock, and contention-free). Holding
	// all handles in one immutable struct behind one pointer means a reader never
	// observes a torn interface value while SetMetrics rebinds -- defense in depth:
	// SetMetrics runs once before any circuit starts, but the field stays race-free
	// regardless of future call sites. NewFlooder seeds it with no-op handles.
	mp atomic.Pointer[flooderMetrics]
}

// flooderMetrics groups the flooding/SNP Prometheus handles into one immutable
// value so they can be swapped atomically (see Flooder.mp). Every field is a
// CounterVec; a no-op registry leaves them inert until SetMetrics.
type flooderMetrics struct {
	lspsRecv  metrics.CounterVec // ze_isis_lsps_received_total{level}
	lspsTx    metrics.CounterVec // ze_isis_lsps_transmitted_total{level}
	csnpSent  metrics.CounterVec // ze_isis_csnp_sent_total{level}
	csnpRecv  metrics.CounterVec // ze_isis_csnp_received_total{level}
	psnpSent  metrics.CounterVec // ze_isis_psnp_sent_total{level}
	psnpRecv  metrics.CounterVec // ze_isis_psnp_received_total{level}
	srmResend metrics.CounterVec // ze_isis_srm_resends_total{level}
	dropped   metrics.CounterVec // ze_isis_lsps_dropped_total{level,reason}
}

// metricSet loads the current metric handles with a single atomic load. The
// pointer is never nil after NewFlooder (seeded with no-op handles).
func (f *Flooder) metricSet() *flooderMetrics { return f.mp.Load() }

// NewFlooder constructs a Flooder over db. tx and circuits are the engine hooks;
// either may be nil in a unit test that drives only the receive/flag path (the
// transmit then no-ops). Metrics start as no-ops until SetMetrics wires a real
// registry.
func NewFlooder(db *LSDB, tx TxFunc, circuits CircuitsFunc) *Flooder {
	nop := metrics.NopRegistry{}
	f := &Flooder{
		db:       db,
		tx:       tx,
		circuits: circuits,
		pending:  make(map[CircuitID]map[Level]map[types.LSPID]pendingReq),
		ackOnly:  make(map[CircuitID]map[Level]map[types.LSPID]packet.LSPEntry),
	}
	f.mp.Store(&flooderMetrics{
		lspsRecv:  nop.CounterVec("", "", nil),
		lspsTx:    nop.CounterVec("", "", nil),
		csnpSent:  nop.CounterVec("", "", nil),
		csnpRecv:  nop.CounterVec("", "", nil),
		psnpSent:  nop.CounterVec("", "", nil),
		psnpRecv:  nop.CounterVec("", "", nil),
		srmResend: nop.CounterVec("", "", nil),
		dropped:   nop.CounterVec("", "", nil),
	})
	return f
}

// SetSigner installs the per-level CSNP/PSNP signer (spec-isis-10). The signer
// takes a fully-encoded SNP and returns the signed bytes (TLV 10 inserted first;
// SNPs carry no Fletcher checksum). nil disables signing. Safe to call before any
// circuit opens; read under signMu on the build path.
func (f *Flooder) SetSigner(sign func(pdu []byte) []byte) {
	f.signMu.Lock()
	f.sign = sign
	f.signMu.Unlock()
}

// signSNP signs an encoded CSNP/PSNP when a signer is installed, else returns the
// bytes unchanged (unauthenticated operation, the default).
func (f *Flooder) signSNP(pdu []byte) []byte {
	f.signMu.RLock()
	sign := f.sign
	f.signMu.RUnlock()
	if sign == nil {
		return pdu
	}
	return sign(pdu)
}

// SetSystemID records the node's own System ID so a received copy of our own LSP
// is recognized as own (LSDB.Receive own flag). Safe to call before circuits open.
func (f *Flooder) SetSystemID(sys types.SystemID) {
	f.ownMu.Lock()
	f.systemID = sys
	f.ownMu.Unlock()
}

// isOwn reports whether an LSP ID was originated by this node (its System ID
// matches ours).
func (f *Flooder) isOwn(id types.LSPID) bool {
	f.ownMu.RLock()
	defer f.ownMu.RUnlock()
	return id.SystemID() == f.systemID
}

// SetMetrics registers the flooding/SNP Prometheus series on reg. This spec OWNS
// and registers exactly these rows from the umbrella canonical Metrics table
// (owner isis-7); isis-13 only scrapes them. Other ze_isis_* series belong to
// their owning specs.
func (f *Flooder) SetMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	// Build the full handle set, then publish it with a single atomic store so a
	// concurrent reader on the hot path observes either the old (no-op) set or the
	// new set in full, never a half-rebound struct (defense in depth).
	f.mp.Store(&flooderMetrics{
		lspsRecv:  reg.CounterVec("ze_isis_lsps_received_total", "Total IS-IS Link State PDUs received, by level.", []string{"level"}),
		lspsTx:    reg.CounterVec("ze_isis_lsps_transmitted_total", "Total IS-IS Link State PDUs transmitted (flooded) on a circuit, by level.", []string{"level"}),
		csnpSent:  reg.CounterVec("ze_isis_csnp_sent_total", "Total IS-IS Complete Sequence Numbers PDUs sent, by level.", []string{"level"}),
		csnpRecv:  reg.CounterVec("ze_isis_csnp_received_total", "Total IS-IS Complete Sequence Numbers PDUs received, by level.", []string{"level"}),
		psnpSent:  reg.CounterVec("ze_isis_psnp_sent_total", "Total IS-IS Partial Sequence Numbers PDUs sent, by level.", []string{"level"}),
		psnpRecv:  reg.CounterVec("ze_isis_psnp_received_total", "Total IS-IS Partial Sequence Numbers PDUs received, by level.", []string{"level"}),
		srmResend: reg.CounterVec("ze_isis_srm_resends_total", "Total IS-IS LSP re-floods caused by an unacknowledged SRM flag on the periodic timer, by level.", []string{"level"}),
		dropped:   reg.CounterVec("ze_isis_lsps_dropped_total", "Total IS-IS Link State PDUs dropped on receive, by level and reason.", []string{"level", "reason"}),
	})
}

// levelOf maps a received LSP/CSNP/PSNP PDU type to the database Level. The PDU
// type already encodes the level (ISO/IEC 10589 clause 9.5); a non-level type
// returns false so the caller drops it.
func levelOf(pt packet.PDUType) (Level, bool) {
	switch pt {
	case packet.PDUTypeL1LSP, packet.PDUTypeL1CSNP, packet.PDUTypeL1PSNP:
		return Level1, true
	case packet.PDUTypeL2LSP, packet.PDUTypeL2CSNP, packet.PDUTypeL2PSNP:
		return Level2, true
	default:
		return Level1, false
	}
}

// ReceiveLSP applies a received LSP PDU arriving on circuit cid and drives the
// SRM/SSN flags per ISO/IEC 10589 clause 7.3.14-16. raw is the verbatim PDU
// (the engine passes the transport's owned copy); lsp is the codec-parsed header
// (isis-2). The level is taken from the PDU type. The freshness decision is made
// by isis-6 (LSDB.Receive); this method maps the outcome to flags:
//
//   - Newer (stored): set SRM on every OTHER eligible circuit (flood onward) and
//     SSN on the incoming circuit (acknowledge); clear any pending-request entry
//     the arriving LSP satisfies (clause 7.3.16, AC-1, AC-15, AC-16 purge case).
//   - Equal (duplicate): no replace, no SRM; on a LAN set SSN on the incoming
//     circuit so a PSNP acknowledges it (AC-3). isis-6 refreshed the lifetime.
//   - Older: isis-6 kept our copy. Two sub-cases distinguished by sequence:
//   - incoming seq EQUALS our held seq (a differing-checksum/ambiguous copy):
//     set SSN on the incoming circuit so a PSNP requests the correct copy
//     (AC-2, clause 7.3.16.1 -- the held copy may be corrupt).
//   - incoming seq is strictly LOWER: set SRM on the incoming circuit to send
//     our newer copy back to the sender (AC-4).
//
// An LSP bearing one of OUR OWN LSP IDs short-circuits all three outcomes: the
// LSDB refuses the write and reports OwnConflict (clause 7.3.16.4 c-1), and the
// caller re-originates above the claimed sequence. See the OwnConflict branch.
//
// "do not re-flood on the incoming circuit" (ISO/IEC 10589 clause 7.3.14) is
// enforced by skipping cid when arming SRM-on-others.
//
// It returns the LSDB freshness/store outcome so the engine wiring can decide
// whether the receive actually changed the database (Stored => Newer) and only
// then emit an LSP-change event / re-run SPF; an Older or Equal LSP is not a
// topology change (ISO/IEC 10589 clause 7.3.15: only a newer LSP supersedes).
func (f *Flooder) ReceiveLSP(cid CircuitID, p2p bool, lsp *packet.LSP, raw []byte) ReceiveResult {
	level, ok := levelOf(lsp.PDUType)
	if !ok {
		f.metricSet().dropped.With(Level1.String(), "wrong-pdu-type").Inc()
		return ReceiveResult{Freshness: Older, Stored: false}
	}
	f.metricSet().lspsRecv.With(level.String()).Inc()

	id := lsp.LSPID
	res := f.db.Receive(level, lsp, raw, f.isOwn(id))

	if res.OwnConflict {
		// Another system claimed a sequence for one of OUR LSP IDs. The LSDB
		// refused the write (ISO/IEC 10589 clause 7.3.16.4 c-1) and only
		// re-origination at a higher sequence settles it (c-2..c-3), which the
		// engine drives from res.ConflictSequence. Two flags must NOT be armed
		// here: SRM on other circuits would flood a withdrawal of ourselves that
		// we just refused to hold, and SRM on the arrival circuit would answer with
		// the stale lower-sequence copy the sender has already superseded. c-4's
		// "set SRMflag on all circuits" is the engine's armFlood over the
		// REGENERATED LSP, not over this one.
		//
		// The arrival is acknowledged either way, because clause 7.3.16.4 a-1 says
		// "shall". SSN carries it when an entry exists to hold the flag. When one
		// does not -- the a) shape, no LSP from S in memory -- SSN is a no-op
		// (SetSSN needs an entry), so the acknowledgement goes in the ack-only set
		// and the next PSNP carries it at the ARRIVED sequence. It is not stored
		// (a-2), so this queue is the only thing that survives the arrival, and it
		// survives only until the PSNP goes out.
		if held := f.db.Lookup(level, id); held != nil {
			f.db.setSSN(level, id, cid)
		} else {
			f.recordAckOnly(cid, level, id, packet.LSPEntry{
				RemainingLifetime: lsp.RemainingLifetime,
				LSPID:             id,
				SequenceNumber:    lsp.SequenceNumber,
				Checksum:          lsp.Checksum,
			})
		}
		// A CSNP reporting our own LSP above our sequence records a pending request
		// (clause 7.3.15.2 b-3 carries no own-LSP exemption, so asking for it is
		// correct). The copy has now arrived and will never be stored, so the
		// request is as satisfied as it will ever be: drop it rather than keep
		// re-asking for a copy this node is about to supersede.
		f.clearPending(cid, level, id)
		f.metricSet().dropped.With(level.String(), "own-seq-conflict").Inc()
		return res
	}

	switch res.Freshness {
	case Newer:
		// Accepted and stored: flood onward on every other eligible circuit and
		// acknowledge on the incoming one. ISO/IEC 10589 clause 7.3.16: SRM on all
		// circuits except the one it arrived on; SSN on the arrival circuit.
		f.armSRMExcept(level, id, cid)
		f.db.setSSN(level, id, cid)
		// The request (if any) for this LSP on the incoming circuit is satisfied.
		f.clearPending(cid, level, id)
	case Equal:
		// Duplicate (clause 7.3.16): nothing to flood. On a broadcast circuit set
		// SSN so the periodic PSNP acknowledges the LSP to the DIS; a P2P duplicate
		// needs no explicit ack (the sender cleared SRM when it sent).
		if !p2p {
			f.db.setSSN(level, id, cid)
		}
	default: // Older
		f.handleOlderLSP(cid, level, id, lsp)
	}
	return res
}

// RecordBadChecksum bumps ze_isis_lsps_dropped_total{level,reason="bad-checksum"}
// for an LSP the engine dropped on receive because its Fletcher checksum failed
// (ISO/IEC 10589 clause 7.3.14.2). The verification (and the drop) happen in the
// engine receive wiring BEFORE the LSP reaches ReceiveLSP/LSDB.Receive, so the
// LSDB is never touched; this only surfaces the drop on the flooding-owned
// counter (the dropped series belongs to isis-7). pt selects the level label.
func (f *Flooder) RecordBadChecksum(pt packet.PDUType) {
	level, ok := levelOf(pt)
	if !ok {
		level = Level1
	}
	f.metricSet().dropped.With(level.String(), "bad-checksum").Inc()
}

// handleOlderLSP resolves the two clause-7.3.15 Older outcomes by comparing the
// incoming sequence to the held entry's sequence. Equal sequence => an ambiguous
// (e.g. differing-checksum) copy: request the correct one via SSN on the arrival
// circuit (AC-2). Strictly lower sequence => the sender is behind: send our newer
// copy back via SRM on the arrival circuit (AC-4). A missing entry (the level was
// full and the first sighting was rejected) is dropped.
func (f *Flooder) handleOlderLSP(cid CircuitID, level Level, id types.LSPID, lsp *packet.LSP) {
	held := f.db.Lookup(level, id)
	if held == nil {
		// Rejected first sighting (level full) or vanished: nothing to do.
		f.metricSet().dropped.With(level.String(), "lsdb-full").Inc()
		return
	}
	switch {
	case lsp.SequenceNumber == held.Sequence():
		// Same sequence, kept ours (differing checksum / purge-class mismatch):
		// ask for the authoritative copy. ISO/IEC 10589 clause 7.3.16.1.
		f.db.setSSN(level, id, cid)
		f.metricSet().dropped.With(level.String(), "same-seq-checksum").Inc()
	default:
		// Strictly older: send our newer copy back on the arrival circuit.
		f.db.SetSRM(level, id, cid)
		f.metricSet().dropped.With(level.String(), "older").Inc()
	}
}

// armSRMExcept sets the SRM flag for (level, id) on every eligible open circuit
// EXCEPT skip (the incoming circuit). ISO/IEC 10589 clause 7.3.14: an LSP is
// never re-flooded onto the circuit it arrived on. A circuit is eligible when it
// is non-passive and forms the level. Passive circuits flood nothing.
func (f *Flooder) armSRMExcept(level Level, id types.LSPID, skip CircuitID) {
	for _, c := range f.circuitSet() {
		if c.ID == skip || c.Passive || !c.formsLevel(level) {
			continue
		}
		f.db.SetSRM(level, id, c.ID)
	}
}

// circuitSet returns the live circuit set, or nil when no provider is wired (a
// unit test driving only the flag path).
func (f *Flooder) circuitSet() []FloodCircuit {
	if f.circuits == nil {
		return nil
	}
	return f.circuits()
}

// FloodTick drains the SRM flags once: for every open, non-passive circuit and
// every LSP at each level the circuit forms, if SRM is set on that circuit the
// stored raw LSP bytes are (re-)transmitted (ISO/IEC 10589 clause 7.3.14: send
// the LSP verbatim, preserving unknown TLVs). On BOTH point-to-point and broadcast
// circuits SRM is LEFT SET after a successful send and is cleared only when the
// flood is ACKNOWLEDGED: a PSNP at our sequence (ReceivePSNP), an equal CSNP entry
// (ReceiveCSNP), or -- on P2P -- the reciprocal PSNP the neighbor emits after it
// stores our LSP. Leaving SRM set is what makes flooding reliable: a first
// transmission lost or arriving before the neighbor can store it is retried on the
// next tick instead of being silently dropped (ISO/IEC 10589 clause 7.3.15.1: the
// SRM flag is cleared on acknowledgement, with periodic retransmission while it
// remains set). Each such unacknowledged resend bumps ze_isis_srm_resends_total so
// a storm is observable (AC-5, AC-6, R-2).
//
// The transmit re-uses the entry's owned raw slice without copying (buffer-first,
// the hot path makes no per-LSP allocation); raw is immutable after store.
func (f *Flooder) FloodTick() {
	for _, c := range f.circuitSet() {
		if c.Passive {
			continue
		}
		for _, level := range []Level{Level1, Level2} {
			if !c.formsLevel(level) {
				continue
			}
			f.floodCircuitLevel(c, level)
		}
	}
}

// floodCircuitLevel transmits every SRM-armed LSP at one level on one circuit.
// SRM is left set after the send (cleared only on acknowledgement: PSNP ack /
// equal CSNP entry) so a lost or not-yet-stored first transmission is retried on
// the next tick. This holds for P2P and LAN alike (ISO/IEC 10589 clause 7.3.15.1).
func (f *Flooder) floodCircuitLevel(c FloodCircuit, level Level) {
	for _, id := range f.db.LSPIDs(level) {
		if !f.db.SRM(level, id, c.ID) {
			continue
		}
		entry := f.db.Lookup(level, id)
		if entry == nil {
			f.db.ClearSRM(level, id, c.ID)
			continue
		}
		raw := entry.Raw()
		if len(raw) == 0 {
			f.db.ClearSRM(level, id, c.ID)
			continue
		}
		if err := f.transmit(c.Name, level, raw); err != nil {
			// Leave SRM set so the next tick retries; a transient send error must
			// not lose the obligation to flood.
			continue
		}
		f.metricSet().lspsTx.With(level.String()).Inc()
		// Leave SRM set until an ack clears it (PSNP at our sequence, or an equal
		// CSNP entry). Only an unacknowledged RE-send (a transmit while SRM stayed
		// set, with a prior transmit since SRM was armed) bumps
		// ze_isis_srm_resends_total so a stalled-flood storm is observable; the
		// FIRST send of a freshly armed LSP is the normal flood, not a resend
		// (ISO/IEC 10589 clause 7.3.15.1: periodic retransmission while SRM remains
		// set). This is the reliable-flooding obligation: clearing on send (without
		// an ack) loses an LSP whose first transmission the neighbor missed.
		if f.db.noteSRMTransmit(level, id, c.ID) {
			f.metricSet().srmResend.With(level.String()).Inc()
		}
	}
}

// transmit sends final PDU bytes on a circuit via the engine-wired tx hook. A
// nil hook (unit test) is a no-op success so the flag transitions are still
// exercised.
func (f *Flooder) transmit(circuitName string, level Level, pdu []byte) error {
	if f.tx == nil {
		return nil
	}
	return f.tx(circuitName, level, pdu)
}
