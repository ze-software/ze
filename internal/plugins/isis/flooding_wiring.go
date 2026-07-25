// Design: plan/learned/933-isis-7-flooding.md -- engine <-> flooding wiring (handlers, timers, P2P initial CSNP).
// Related: server.go -- the engine struct, dispatcher, and lifecycle this extends
// Related: lsdb_wiring.go -- the LSDB instance + SRM arming this flooding drains
// Related: circuits.go -- the adjacency circuits whose Up event triggers the P2P initial CSNP
//
// This file is the root-package glue between the per-circuit transport/dispatcher
// (isis-3/isis-4) and the flooding subsystem (isis-7, internal/plugins/isis/lsdb
// flooding.go + snp.go). It constructs the Flooder over the engine's LSDB, wires
// the LSP/CSNP/PSNP dispatcher handlers to it, runs the periodic flood timer
// (drains SRM) and the CSNP/PSNP cadence timer, and triggers the initial CSNP on a
// P2P adjacency reaching Up. The flooding ALGORITHM lives in the lsdb package; this
// file only injects the transmit + circuit-set hooks and schedules the timers.

package isis

import (
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// Flooding cadence constants (ISO/IEC 10589 clause 7.3; the guide's recommended
// 5 s flood, level-appropriate CSNP). v1 keeps them as runtime constants (spec
// Known Limitations): not yet YANG-tunable.
const (
	// floodInterval is how often the periodic flood timer drains SRM and
	// (re)transmits SRM-armed LSPs (ISO/IEC 10589 clause 7.3.14 recommends
	// 5..30 s; 5 s is the responsive default). Clamped into [minFloodInterval,
	// maxFloodInterval] when ever made configurable.
	floodInterval = 5 * time.Second

	// psnpInterval is how often a circuit drains its SSN acknowledges and
	// pending-request list into PSNP(s). It matches the flood cadence so an ack
	// follows a received LSP within one flood period.
	psnpInterval = 5 * time.Second

	// csnpInterval is the P2P periodic CSNP cadence (slower than the flood timer:
	// the initial CSNP at adjacency Up does the heavy sync, then a slow periodic
	// CSNP catches any drift). On a LAN the DIS sources periodic CSNPs at its own
	// cadence (isis-8 policy); this timer only fires on P2P circuits here. ISO/IEC
	// 10589 clause 7.3.14 bounds a configurable flood interval to 5..30 s; v1 fixes
	// floodInterval at 5 s within that range (spec boundary table).
	csnpInterval = 10 * time.Second
)

// initFlooding constructs the engine's Flooder over the shared LSDB, wiring the
// transmit hook (transport.SendPDU) and the circuit-set provider (derived from
// the running InterfaceConfig). Called from newEngine after initLSDB so the
// Flooder shares the engine's database. The System ID is set later (setConfig)
// once the config is known; initFlooding only needs the LSDB to exist.
func (e *engine) initFlooding() {
	e.flooder = lsdb.NewFlooder(e.lsdb, e.floodTx, e.floodCircuits)
}

// floodTx is the Flooder's transmit hook: send final PDU bytes to the level
// multicast group on a named circuit via the isis-3 transport (which adds only
// 802.3+LLC framing). It maps the lsdb.Level to the transport.Level.
func (e *engine) floodTx(circuitName string, level lsdb.Level, pdu []byte) error {
	tl := transport.Level1
	if level == lsdb.Level2 {
		tl = transport.Level2
	}
	return e.transport.SendPDU(circuitName, tl, pdu)
}

// floodCircuits is the Flooder's circuit-set provider: it derives the live
// FloodCircuit set from the engine's running circuits and their InterfaceConfig
// (the levels a circuit forms, whether it is P2P, whether it is passive). The
// Flooder uses this to arm SRM on the right circuits and to skip passive ones,
// without importing the circuit or transport packages.
func (e *engine) floodCircuits() []lsdb.FloodCircuit {
	e.circuitsMu.RLock()
	names := make([]string, 0, len(e.circuitByName))
	for name := range e.circuitByName {
		names = append(names, name)
	}
	e.circuitsMu.RUnlock()

	out := make([]lsdb.FloodCircuit, 0, len(names))
	for _, name := range names {
		ic, ok := e.interfaceConfig(name)
		if !ok {
			continue
		}
		out = append(out, lsdb.FloodCircuit{
			Name:    name,
			ID:      e.circuitIDFor(name),
			P2P:     ic.CircuitType == CircuitPointToPoint,
			Passive: ic.Passive,
			FormsL1: ic.Level.HasL1(),
			FormsL2: ic.Level.HasL2(),
		})
	}
	return out
}

// floodCircuitFor returns the FloodCircuit for one interface name (used by the
// P2P-initial-CSNP hook), or false when the circuit is not running.
func (e *engine) floodCircuitFor(name string) (lsdb.FloodCircuit, bool) {
	ic, ok := e.interfaceConfig(name)
	if !ok {
		return lsdb.FloodCircuit{}, false
	}
	return lsdb.FloodCircuit{
		Name:    name,
		ID:      e.circuitIDFor(name),
		P2P:     ic.CircuitType == CircuitPointToPoint,
		Passive: ic.Passive,
		FormsL1: ic.Level.HasL1(),
		FormsL2: ic.Level.HasL2(),
	}, true
}

// installFloodHandlers replaces the LSP/CSNP/PSNP no-op stubs that
// installStubHandlers registered with the real flooding handlers. Called from
// newEngine AFTER installStubHandlers so these overwrite the stubs (the
// dispatcher keeps the last handler registered per PDU type). The IIH handlers
// (isis-5) are untouched. Each handler maps the received frame's ifindex to the
// circuit's LSDB CircuitID and P2P-ness, parses the PDU via the isis-2 codec, and
// drives the Flooder.
func (e *engine) installFloodHandlers() {
	e.dispatch.register(packet.PDUTypeL1LSP, e.handleLSP)
	e.dispatch.register(packet.PDUTypeL2LSP, e.handleLSP)
	e.dispatch.register(packet.PDUTypeL1CSNP, e.handleCSNP)
	e.dispatch.register(packet.PDUTypeL2CSNP, e.handleCSNP)
	e.dispatch.register(packet.PDUTypeL1PSNP, e.handlePSNP)
	e.dispatch.register(packet.PDUTypeL2PSNP, e.handlePSNP)
}

// circuitContext resolves a received frame's source ifindex to the LSDB
// CircuitID and whether the circuit is point-to-point, so a flooding handler can
// drive the SRM/SSN flags (P2P selects whether a duplicate sets SSN; SRM is
// cleared on acknowledgement on both media). A frame whose ifindex has no live
// circuit is reported absent (dropped by the caller).
func (e *engine) circuitContext(ifindex int) (cid lsdb.CircuitID, p2p, ok bool) {
	e.circuitsMu.RLock()
	c := e.circuits[ifindex]
	e.circuitsMu.RUnlock()
	if c == nil {
		return 0, false, false
	}
	name := c.Name()
	ic, found := e.interfaceConfig(name)
	if !found {
		return 0, false, false
	}
	return e.circuitIDFor(name), ic.CircuitType == CircuitPointToPoint, true
}

// handleLSP is the dispatcher handler for L1/L2 LSP PDUs: decode the LSP via the
// isis-2 codec, resolve the receiving circuit, and drive the Flooder receive
// algorithm (freshness compare via isis-6, SRM/SSN per isis-7). A malformed PDU
// or a frame on an unknown circuit is dropped (the codec bound-checks before
// slicing; security review).
func (e *engine) handleLSP(rf transport.RawFrame) {
	pdu, err := packet.DecodePDU(rf.PDU)
	if err != nil || pdu.LSP == nil {
		return
	}
	defer packet.ReleaseTLVs(pdu.LSP.TLVs)
	// ISO/IEC 10589 sec 7.3.14.2: a received LSP whose Fletcher checksum does not
	// verify over its raw bytes is corrupt; it MUST be discarded -- not stored and
	// not re-flooded -- so a bit-flipped LSP cannot poison the LSDB or be relayed
	// onward. The check runs BEFORE LSDB.Receive (and before circuit resolution, so
	// a corrupt LSP is always counted) and the database is never touched on failure.
	if !pdu.LSP.VerifyChecksum() {
		e.flooder.RecordBadChecksum(pdu.LSP.PDUType)
		return
	}
	cid, p2p, ok := e.circuitContext(rf.IfIndex)
	if !ok {
		return
	}
	res := e.flooder.ReceiveLSP(cid, p2p, pdu.LSP, rf.PDU)
	// Only a NEWER LSP (Stored) changed the topology: emit an LSP-change event and
	// re-run SPF (isis-9). An Equal duplicate only refreshed the held lifetime (no
	// topology change) -- emit a non-"add" "refresh" event but do not pretend a new
	// LSP appeared. An Older LSP changed nothing: emit nothing and do not re-run
	// SPF, so a steady stream of stale/duplicate LSPs cannot thrash SPF.
	switch res.Freshness {
	case lsdb.Newer:
		// Topology changed: emit "add" AND re-run SPF (emitLSPChange triggers SPF).
		e.emitLSPChange(levelToken(pdu.LSP.PDUType), pdu.LSP.LSPID.String(), uint32(pdu.LSP.SequenceNumber), "add")
	case lsdb.Equal:
		// Duplicate refreshed only the held lifetime (no topology change): notify
		// consumers with "refresh" but do NOT re-run SPF (publishLSPChange skips it).
		e.publishLSPChange(levelToken(pdu.LSP.PDUType), pdu.LSP.LSPID.String(), uint32(pdu.LSP.SequenceNumber), "refresh")
	default: // lsdb.Older: no change, no event, no SPF.
	}
}

// handleCSNP is the dispatcher handler for L1/L2 CSNP PDUs: decode and reconcile
// against the LSDB (set SRM for LSPs we hold newer, SSN/pending-request for LSPs
// the neighbor holds newer, clear SRM on equal acks).
func (e *engine) handleCSNP(rf transport.RawFrame) {
	pdu, err := packet.DecodePDU(rf.PDU)
	if err != nil || pdu.CSNP == nil {
		return
	}
	defer packet.ReleaseTLVs(pdu.CSNP.TLVs)
	cid, _, ok := e.circuitContext(rf.IfIndex)
	if !ok {
		return
	}
	e.flooder.ReceiveCSNP(cid, pdu.CSNP)
}

// handlePSNP is the dispatcher handler for L1/L2 PSNP PDUs: decode and apply the
// ack/request semantics (clear SRM on an ack at our sequence, set SRM to supply a
// requested LSP).
func (e *engine) handlePSNP(rf transport.RawFrame) {
	pdu, err := packet.DecodePDU(rf.PDU)
	if err != nil || pdu.PSNP == nil {
		return
	}
	defer packet.ReleaseTLVs(pdu.PSNP.TLVs)
	cid, _, ok := e.circuitContext(rf.IfIndex)
	if !ok {
		return
	}
	e.flooder.ReceivePSNP(cid, pdu.PSNP)
}

// startFloodLoops launches the periodic flood, PSNP, and (P2P) CSNP timers. The
// flood timer drains SRM and (re)transmits SRM-armed LSPs every floodInterval;
// the PSNP timer drains SSN acks + pending requests every psnpInterval; the CSNP
// timer sources a periodic CSNP on each P2P circuit every csnpInterval (LAN
// periodic CSNP is DIS-sourced, isis-8). All stop on ctx cancellation (shutdown).
func (e *engine) startFloodLoops() {
	e.wg.Go(func() {
		flood := time.NewTicker(floodInterval)
		psnp := time.NewTicker(psnpInterval)
		csnp := time.NewTicker(csnpInterval)
		defer flood.Stop()
		defer psnp.Stop()
		defer csnp.Stop()
		for {
			select {
			case <-e.ctx.Done():
				return
			case <-flood.C:
				e.flooder.FloodTick()
			case <-psnp.C:
				e.psnpTick()
			case <-csnp.C:
				e.periodicCSNPTick()
			}
		}
	})
}

// psnpTick drains SSN acks and pending requests into PSNP(s) on every open,
// non-passive circuit at each level it forms. Sourced from the node's own Source
// ID (System ID, pseudonode 0).
func (e *engine) psnpTick() {
	src := e.ownSourceID()
	for _, c := range e.floodCircuits() {
		if c.Passive {
			continue
		}
		for _, level := range floodLevels(c) {
			e.flooder.SendPSNP(c, level, src)
		}
	}
}

// periodicCSNPTick sources a periodic CSNP on every open P2P circuit at each
// level it forms (the slow P2P catch-up after the initial CSNP). LAN CSNP cadence
// is DIS policy (isis-8) and is not driven here.
func (e *engine) periodicCSNPTick() {
	src := e.ownSourceID()
	for _, c := range e.floodCircuits() {
		if !c.P2P || c.Passive {
			continue
		}
		for _, level := range floodLevels(c) {
			e.flooder.SendCSNP(c, level, src)
		}
	}
}

// onAdjacencyUpFlood is invoked from the circuit's session-up transition hook
// (circuits.go) on a P2P circuit: it sends the initial CSNP to synchronize the
// two LSDBs immediately (ISO/IEC 10589 clause 7.3.15.2, spec AC-11, R-5). The
// level is the adjacency level that came Up. A non-P2P circuit is ignored (the
// Flooder.InitialCSNP guards on P2P too).
func (e *engine) onAdjacencyUpFlood(name string, level adjacency.Level) {
	c, ok := e.floodCircuitFor(name)
	if !ok || !c.P2P {
		return
	}
	e.flooder.InitialCSNP(c, adjToLSDBLevel(level), e.ownSourceID())
}

// ownSourceID returns the node's own Source ID (System ID + pseudonode 0), the
// source field for CSNP/PSNP this node originates.
func (e *engine) ownSourceID() types.SourceID {
	e.mu.Lock()
	sys := e.cfg.SystemID
	e.mu.Unlock()
	return types.NewSourceID(sys, 0)
}

// ---- small helpers ----

// floodLevels returns the LSDB levels a FloodCircuit forms (L1, L2, or both).
func floodLevels(c lsdb.FloodCircuit) []lsdb.Level {
	switch {
	case c.FormsL1 && c.FormsL2:
		return []lsdb.Level{lsdb.Level1, lsdb.Level2}
	case c.FormsL2:
		return []lsdb.Level{lsdb.Level2}
	default:
		return []lsdb.Level{lsdb.Level1}
	}
}

// levelToken maps an LSP PDU type to the "l1"/"l2" metric/event token.
func levelToken(pt packet.PDUType) string {
	if pt == packet.PDUTypeL2LSP {
		return "l2"
	}
	return "l1"
}

// adjToLSDBLevel maps an adjacency.Level to the lsdb.Level.
func adjToLSDBLevel(l adjacency.Level) lsdb.Level {
	if l == adjacency.Level2 {
		return lsdb.Level2
	}
	return lsdb.Level1
}
