// Design: docs/architecture/isis/isis-7-flooding.md -- CSNP/PSNP synchronization + per-circuit pending-request set.
// ISO/IEC 10589 clause 7.3.15-17, clause 9.10 (CSNP) / 9.11 (PSNP) / 9.14 (TLV 9).
//
// CSNP ("here is everything I have") and PSNP ("here is a partial list, used to
// acknowledge or request specific LSPs") synchronize two LSDBs. This file builds,
// sends, and receives them on the Flooder defined in flooding.go.
//
// The receive side has a subtlety the SRM/SSN model cannot express on its own: an
// SSN flag lives on an EXISTING LSDB entry, so it can only ACKNOWLEDGE an LSP we
// already hold. When a CSNP lists an LSP ID we do NOT hold (or hold only an older
// copy of), there is no entry to mark, so the request is recorded in a per-circuit
// PENDING-REQUEST set owned here, independent of the LSDB. A PSNP drains that set
// to request the missing LSPs; a pending entry is cleared when the requested LSP
// arrives and is stored (flooding.go ReceiveLSP -> clearPending).

package lsdb

import (
	"sort"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// maxLSPEntriesPerSNP is the number of TLV 9 (LSP Entries) records that fit one
// SNP TLV: the TLV value field is 1 octet (max 255), each entry is
// packet.LSPEntryLen (16) octets, so 255/16 = 15 entries. A CSNP/PSNP needing
// more than one TLV 9 splits across PDUs (CSNP) or carries several TLV 9s up to
// the PDU budget (PSNP). isis-2 enforces one TLV 9 per 15 entries.
const maxLSPEntriesPerSNP = packet.MaxTLVValueLen / packet.LSPEntryLen

// maxPendingPerCircuit bounds the per-circuit pending-request set so a hostile
// CSNP advertising a flood of distinct LSP IDs we do not hold cannot grow the set
// without bound (security review: resource exhaustion). The set is deduplicated
// by LSP ID; once full, a new wanted LSP ID is dropped (the neighbor re-lists it
// in its next CSNP, so a genuine gap is re-discovered once the set drains).
const maxPendingPerCircuit = 4096

// pendingReq records that we want a specific LSP (identified by LSP ID) at a
// known-or-higher sequence, learned from a neighbor's CSNP, but do NOT yet hold
// it (or hold only an older copy). It is cleared when the LSP arrives and is
// stored. The sequence is the value the neighbor advertised, so a later CSNP
// that bumps it updates the request.
type pendingReq struct {
	seq      types.SequenceNumber
	lifetime types.RemainingLifetime
	checksum uint16
}

// recordPending adds (or refreshes) a pending-request entry for id on circuit cid
// at level. Bounded by maxPendingPerCircuit (a full set drops the new id).
func (f *Flooder) recordPending(cid CircuitID, level Level, id types.LSPID, req pendingReq) {
	f.pendMu.Lock()
	defer f.pendMu.Unlock()
	byLevel := f.pending[cid]
	if byLevel == nil {
		byLevel = make(map[Level]map[types.LSPID]pendingReq)
		f.pending[cid] = byLevel
	}
	set := byLevel[level]
	if set == nil {
		set = make(map[types.LSPID]pendingReq)
		byLevel[level] = set
	}
	if _, exists := set[id]; !exists && len(set) >= maxPendingPerCircuit {
		return // set full: drop; the neighbor re-lists it later.
	}
	set[id] = req
}

// recordAckOnly queues an ACKNOWLEDGEMENT of an LSP this node received but does
// not hold, so the next PSNP on circuit cid carries it (ISO/IEC 10589 clause
// 7.3.16.4 a-1). The entry echoes the ARRIVED values, which is what makes it an
// acknowledgement rather than a request: a request goes out at sequence 0 so the
// holder reads it as older and supplies the LSP (pendingFor), while an entry at
// the sender's own sequence is read as an ack and clears its SRM.
//
// It is bounded by maxPendingPerCircuit for the same reason the request set is:
// a peer that floods LSPs for LSP IDs this node does not hold must not grow it
// without bound. A full set drops the ack; the sender retransmits.
func (f *Flooder) recordAckOnly(cid CircuitID, level Level, id types.LSPID, e packet.LSPEntry) {
	f.pendMu.Lock()
	defer f.pendMu.Unlock()
	byLevel := f.ackOnly[cid]
	if byLevel == nil {
		byLevel = make(map[Level]map[types.LSPID]packet.LSPEntry)
		f.ackOnly[cid] = byLevel
	}
	set := byLevel[level]
	if set == nil {
		set = make(map[types.LSPID]packet.LSPEntry)
		byLevel[level] = set
	}
	if _, exists := set[id]; !exists && len(set) >= maxPendingPerCircuit {
		return
	}
	set[id] = e
}

// drainAckOnly removes and returns the queued acknowledgements for circuit cid at
// level. Unlike a pending REQUEST, which stays until the LSP arrives, an
// acknowledgement is sent once: clause 7.3.16.4 a-2 has this node retain nothing
// about the LSP after the acknowledgement goes out.
func (f *Flooder) drainAckOnly(cid CircuitID, level Level) []packet.LSPEntry {
	f.pendMu.Lock()
	set := f.ackOnly[cid][level]
	out := make([]packet.LSPEntry, 0, len(set))
	for id, e := range set {
		out = append(out, e)
		delete(set, id)
	}
	f.pendMu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].LSPID.Less(out[j].LSPID) })
	return out
}

// clearPending removes a pending-request entry for id on circuit cid at level
// (called when the requested LSP arrives and is stored, AC-15). A no-op when
// absent.
func (f *Flooder) clearPending(cid CircuitID, level Level, id types.LSPID) {
	f.pendMu.Lock()
	defer f.pendMu.Unlock()
	if set := f.pending[cid][level]; set != nil {
		delete(set, id)
	}
}

// pendingFor returns a stable, LSP-ID-ordered snapshot of the pending requests
// for circuit cid at level, encoded as the standard "send me this LSP" PSNP
// request form: each entry carries the LSP ID at SEQUENCE 0 / lifetime 0 /
// checksum 0 (ISO/IEC 10589 clause 7.3.15.3). A request MUST NOT echo the
// sequence we learned from the neighbor's CSNP: an entry at the holder's current
// sequence is indistinguishable from an ACKNOWLEDGEMENT, so the holder would
// clear SRM (ReceivePSNP cmp >= 0) and never supply the LSP -- the requester
// would wait forever. At sequence 0 the holder always compares the request as
// older than its copy (cmp < 0) and sets SRM to supply it (AC-10). The order is
// deterministic (CSNP/PSNP order) so the emitted PSNP is reproducible.
func (f *Flooder) pendingFor(cid CircuitID, level Level) []packet.LSPEntry {
	f.pendMu.Lock()
	set := f.pending[cid][level]
	out := make([]packet.LSPEntry, 0, len(set))
	for id := range set {
		out = append(out, packet.LSPEntry{
			RemainingLifetime: 0,
			LSPID:             id,
			SequenceNumber:    0,
			Checksum:          0,
		})
	}
	f.pendMu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].LSPID.Less(out[j].LSPID) })
	return out
}

// PendingCount returns the number of pending-request entries for circuit cid at
// level (exposed for tests and diagnostics).
func (f *Flooder) PendingCount(cid CircuitID, level Level) int {
	f.pendMu.Lock()
	defer f.pendMu.Unlock()
	return len(f.pending[cid][level])
}

// ---- CSNP build / send ----

// buildCSNPs builds the Complete Sequence Numbers PDU(s) for level from the
// current LSDB, sourced from srcID. ISO/IEC 10589 clause 9.10: a CSNP carries a
// Start and End LSP ID bounding the range it fully describes, plus TLV 9 entries
// for every LSP in that range. The common case fits one CSNP spanning the whole
// LSP-ID space (Start 0000.0000.0000.00-00, End ffff.ffff.ffff.ff-ff); a database
// larger than one PDU is split into ordered, contiguous ranges (AC-13, A-4) so
// each PDU's Start/End exactly bound the entries it carries and the ranges tile
// the space with no gap. Returns the encoded PDU byte slices ready for tx.
func (f *Flooder) buildCSNPs(level Level, srcID types.SourceID) [][]byte {
	entries := f.lspEntries(level)

	pt := packet.PDUTypeL1CSNP
	if level == Level2 {
		pt = packet.PDUTypeL2CSNP
	}

	if len(entries) == 0 {
		// Empty database: one CSNP spanning the whole space with no entries tells a
		// neighbor we hold nothing in the range, so it floods us everything it has.
		c := packet.CSNP{
			PDUType:    pt,
			SourceID:   srcID,
			StartLSPID: minLSPID(),
			EndLSPID:   maxLSPID(),
		}
		return [][]byte{f.signSNP(encodeCSNP(&c))}
	}

	var out [][]byte
	for start := 0; start < len(entries); start += maxLSPEntriesPerSNP {
		end := min(start+maxLSPEntriesPerSNP, len(entries))
		chunk := entries[start:end]

		// The range Start is the whole-space minimum for the first PDU, else this
		// chunk's first LSP ID; the range End is the whole-space maximum for the
		// last PDU, else this chunk's last LSP ID. This tiles the LSP-ID space so a
		// receiver sees a contiguous, gap-free cover (clause 7.3.15.2).
		startID := chunk[0].LSPID
		if start == 0 {
			startID = minLSPID()
		}
		endID := chunk[len(chunk)-1].LSPID
		if end == len(entries) {
			endID = maxLSPID()
		}

		c := packet.CSNP{
			PDUType:    pt,
			SourceID:   srcID,
			StartLSPID: startID,
			EndLSPID:   endID,
			TLVs:       []packet.TLV{lspEntriesTLV(chunk)},
		}
		out = append(out, f.signSNP(encodeCSNP(&c)))
	}
	return out
}

// SendCSNP builds and transmits the CSNP(s) for level on circuit c, sourced from
// srcID. Used by the periodic CSNP cadence and the P2P initial CSNP (AC-11).
// Increments ze_isis_csnp_sent_total per PDU sent.
func (f *Flooder) SendCSNP(c FloodCircuit, level Level, srcID types.SourceID) {
	if !c.formsLevel(level) || c.Passive {
		return
	}
	for _, pdu := range f.buildCSNPs(level, srcID) {
		if err := f.transmit(c.Name, level, pdu); err != nil {
			continue
		}
		f.metricSet().csnpSent.With(level.String()).Inc()
	}
}

// InitialCSNP sends the one-shot CSNP a point-to-point circuit emits the moment
// its adjacency reaches Up, to synchronize the two LSDBs fast (ISO/IEC 10589
// clause 7.3.15.2 / spec AC-11, R-5). The engine calls it from the P2P adjacency
// Up hook. A non-P2P circuit is ignored (LAN periodic CSNP is DIS-sourced,
// isis-8). srcID is the node's own Source ID.
func (f *Flooder) InitialCSNP(c FloodCircuit, level Level, srcID types.SourceID) {
	if !c.P2P {
		return
	}
	f.SendCSNP(c, level, srcID)
}

// ---- CSNP receive ----

// ReceiveCSNP processes a received CSNP on circuit cid and reconciles its TLV 9
// entries against our LSDB (ISO/IEC 10589 clause 7.3.15.2). For each listed LSP:
//
//   - We hold it at the SAME sequence (and checksum): the neighbor confirms it
//     has our copy, so clear SRM on this circuit (an implicit ack) and clear any
//     matching pending-request entry (AC-13 equal case).
//   - The neighbor is NEWER (higher seq, or equal seq with a purge it advertises
//     and we do not): we either hold an older copy or none. Either way the
//     authoritative request is a per-circuit pending-request entry so a PSNP can
//     request it (AC-7, AC-15). When we DO hold a (stale) entry we also set SSN on
//     it (a held entry can carry the flag); when we hold nothing there is no entry
//     to flag, which is exactly why the pending-request set exists.
//   - We are NEWER (we hold a higher seq than listed): set SRM on this circuit to
//     send our copy (AC-8).
//
// An LSP we hold that is ABSENT from a CSNP whose range covers it is handled by
// reconcileCSNPRange below (set SRM to send it). The level comes from the PDU
// type.
func (f *Flooder) ReceiveCSNP(cid CircuitID, csnp *packet.CSNP) {
	level, ok := levelOf(csnp.PDUType)
	if !ok {
		f.metricSet().dropped.With(Level1.String(), "wrong-pdu-type").Inc()
		return
	}
	f.metricSet().csnpRecv.With(level.String()).Inc()

	listed := decodeLSPEntries(csnp.TLVs)
	listedSet := make(map[types.LSPID]struct{}, len(listed))
	for _, e := range listed {
		listedSet[e.LSPID] = struct{}{}
		f.reconcileCSNPEntry(cid, level, e)
	}
	// LSPs we hold within [Start, End] but the CSNP did not list: the neighbor is
	// missing them, so flood them (SRM on this circuit). ISO/IEC 10589 7.3.15.2.
	f.reconcileCSNPRange(cid, level, csnp.StartLSPID, csnp.EndLSPID, listedSet)
}

// reconcileCSNPEntry compares one CSNP TLV-9 entry to our LSDB and sets the
// appropriate flag / pending-request (see ReceiveCSNP).
func (f *Flooder) reconcileCSNPEntry(cid CircuitID, level Level, e packet.LSPEntry) {
	held := f.db.Lookup(level, e.LSPID)
	if held == nil {
		// We do not hold this LSP at all: no entry to flag with SSN. Record a
		// pending-request so a PSNP asks for it (AC-15).
		f.recordPending(cid, level, e.LSPID, pendingReq{seq: e.SequenceNumber, lifetime: e.RemainingLifetime, checksum: e.Checksum})
		return
	}
	cmp := compareSNPEntry(e, held)
	switch {
	case cmp > 0:
		// Neighbor newer: request via the pending-request set (authoritative) and
		// also set SSN on the held stale entry (AC-7).
		f.recordPending(cid, level, e.LSPID, pendingReq{seq: e.SequenceNumber, lifetime: e.RemainingLifetime, checksum: e.Checksum})
		f.db.SetSSN(level, e.LSPID, cid)
	case cmp < 0:
		// We are newer: send our copy (AC-8).
		f.db.SetSRM(level, e.LSPID, cid)
	default:
		// Equal: implicit ack. Clear SRM on this circuit and drop any pending entry
		// (the neighbor confirms it holds our copy). AC-13 equal case, R-2.
		f.db.ClearSRM(level, e.LSPID, cid)
		f.clearPending(cid, level, e.LSPID)
	}
}

// reconcileCSNPRange sets SRM on circuit cid for every LSP we hold whose LSP ID
// falls within [start, end] (inclusive) but is absent from the CSNP's entry list:
// the neighbor does not have it, so we must flood it (ISO/IEC 10589 clause
// 7.3.15.2). A purged entry is still flooded (the neighbor needs the purge).
func (f *Flooder) reconcileCSNPRange(cid CircuitID, level Level, start, end types.LSPID, listed map[types.LSPID]struct{}) {
	for _, id := range f.db.LSPIDs(level) {
		if id.Compare(start) < 0 || id.Compare(end) > 0 {
			continue // outside the range this CSNP describes
		}
		if _, ok := listed[id]; ok {
			continue // the neighbor listed it; handled per-entry above
		}
		f.db.SetSRM(level, id, cid)
	}
}

// ---- PSNP build / send ----

// buildPSNP builds the Partial Sequence Numbers PDU(s) for level on circuit cid,
// sourced from srcID, from two sources (ISO/IEC 10589 clause 7.3.15.3):
//
//   - ACK list: every LSP we hold with SSN set on this circuit. The TLV 9 entry
//     carries our held sequence/lifetime/checksum (acknowledging the LSP). The
//     SSN flag is cleared as it is added (a PSNP send clears SSN, clause 7.3.16).
//   - REQUEST list: every per-circuit pending-request entry (LSPs we do NOT yet
//     hold), encoded at sequence 0 / lifetime 0 / checksum 0 -- the standard
//     "send me this LSP" request form (clause 7.3.15.3). The request MUST NOT echo
//     the sequence learned from the neighbor's CSNP: an entry at the holder's
//     current sequence is indistinguishable from an acknowledgement, so the holder
//     would clear SRM and never supply the LSP. The entry stays pending until the
//     LSP arrives and is stored.
//
// Returns the encoded PDU byte slices. Several TLV 9s are packed per PDU up to
// the entry budget; a very large list splits across PDUs.
func (f *Flooder) buildPSNP(cid CircuitID, level Level, srcID types.SourceID) [][]byte {
	var entries []packet.LSPEntry

	// ACK list: SSN-set held LSPs. Clear SSN as we add (PSNP send clears SSN).
	for _, id := range f.db.LSPIDs(level) {
		if !f.db.SSN(level, id, cid) {
			continue
		}
		held := f.db.Lookup(level, id)
		if held == nil {
			f.db.ClearSSN(level, id, cid)
			continue
		}
		entries = append(entries, packet.LSPEntry{
			RemainingLifetime: held.Lifetime(),
			LSPID:             id,
			SequenceNumber:    held.Sequence(),
			Checksum:          held.Checksum(),
		})
		f.db.ClearSSN(level, id, cid)
	}

	// REQUEST list: pending-request entries (LSPs we do not yet hold). These stay
	// pending until the LSP arrives (clearPending in ReceiveLSP).
	entries = append(entries, f.pendingFor(cid, level)...)

	// ACK-ONLY list: LSPs this node received, refused to retain, and owes an
	// acknowledgement for (ISO/IEC 10589 clause 7.3.16.4 a). Drained, not held:
	// a-2 leaves nothing retained once the acknowledgement is sent.
	entries = append(entries, f.drainAckOnly(cid, level)...)

	if len(entries) == 0 {
		return nil
	}

	pt := packet.PDUTypeL1PSNP
	if level == Level2 {
		pt = packet.PDUTypeL2PSNP
	}

	var out [][]byte
	for start := 0; start < len(entries); start += maxLSPEntriesPerSNP {
		end := min(start+maxLSPEntriesPerSNP, len(entries))
		p := packet.PSNP{
			PDUType:  pt,
			SourceID: srcID,
			TLVs:     []packet.TLV{lspEntriesTLV(entries[start:end])},
		}
		out = append(out, f.signSNP(encodePSNP(&p)))
	}
	return out
}

// SendPSNP builds and transmits the PSNP(s) for level on circuit c (ack + request
// list), sourced from srcID. Increments ze_isis_psnp_sent_total per PDU sent.
// Used by the periodic PSNP cadence; clears SSN for the acknowledged LSPs.
func (f *Flooder) SendPSNP(c FloodCircuit, level Level, srcID types.SourceID) {
	if !c.formsLevel(level) || c.Passive {
		return
	}
	for _, pdu := range f.buildPSNP(c.ID, level, srcID) {
		if err := f.transmit(c.Name, level, pdu); err != nil {
			continue
		}
		f.metricSet().psnpSent.With(level.String()).Inc()
	}
}

// ---- PSNP receive ----

// ReceivePSNP processes a received PSNP on circuit cid (ISO/IEC 10589 clause
// 7.3.15.3). For each TLV 9 entry:
//
//   - The neighbor acknowledges our LSP at OUR sequence (or newer): clear SRM on
//     this circuit -- the flood is confirmed delivered (AC-9).
//   - The neighbor lists an LSP it does NOT have, or has an OLDER copy of (its
//     entry sequence is lower than ours, including a zero-sequence "please send"
//     request): set SRM on this circuit to supply it (AC-10).
func (f *Flooder) ReceivePSNP(cid CircuitID, psnp *packet.PSNP) {
	level, ok := levelOf(psnp.PDUType)
	if !ok {
		f.metricSet().dropped.With(Level1.String(), "wrong-pdu-type").Inc()
		return
	}
	f.metricSet().psnpRecv.With(level.String()).Inc()

	for _, e := range decodeLSPEntries(psnp.TLVs) {
		held := f.db.Lookup(level, e.LSPID)
		if held == nil {
			// We do not hold the listed LSP: nothing to ack or supply.
			continue
		}
		cmp := compareSNPEntry(e, held)
		switch {
		case cmp >= 0:
			// Acked at our sequence (or the neighbor has newer; either way our
			// flood obligation on this circuit is discharged). Clear SRM (AC-9).
			f.db.ClearSRM(level, e.LSPID, cid)
		default:
			// The neighbor is behind (older copy or a request): supply ours (AC-10).
			f.db.SetSRM(level, e.LSPID, cid)
		}
	}
}

// clearSRMByPSNP is a thin alias used by tests/diagnostics to express that a PSNP
// ack clears SRM; the real path is ReceivePSNP. Kept unexported and small.

// ---- helpers ----

// compareSNPEntry compares an SNP (CSNP/PSNP) TLV-9 entry against a held LSDB
// entry and returns +1 when the entry (neighbor's view) is newer, -1 when ours
// is newer, 0 when they are the same version. The order matches LSDB freshness
// (ISO/IEC 10589 clause 7.3.15): sequence first, then the purge tiebreak at equal
// sequence (a purge -- remaining lifetime 0 -- is newer than a non-zero copy),
// then equal. A differing checksum at equal sequence and equal purge-class is
// treated as the SAME version for SNP purposes (the LSP-level freshness compare,
// not the SNP layer, decides corruption; the SNP layer only drives
// request/supply by sequence).
func compareSNPEntry(e packet.LSPEntry, held *Entry) int {
	switch {
	case e.SequenceNumber > held.Sequence():
		return 1
	case e.SequenceNumber < held.Sequence():
		return -1
	}
	ePurge := e.RemainingLifetime.IsPurge()
	hPurge := held.Lifetime().IsPurge()
	switch {
	case ePurge && !hPurge:
		return 1
	case !ePurge && hPurge:
		return -1
	default:
		return 0
	}
}

// lspEntries returns the LSP-Entries (TLV 9 records) for every LSP at level,
// ordered by LSP ID (the CSNP range order). It delegates to LSDB.LSPEntries,
// which builds the records directly from the typed entry metadata under a single
// read lock -- no Snapshot-then-ParseLSPID string round-trip on the CSNP cadence.
func (f *Flooder) lspEntries(level Level) []packet.LSPEntry {
	return f.db.LSPEntries(level)
}

// decodeLSPEntries flattens the TLV 9 (LSP Entries) records carried in an SNP's
// TLV list. Non-TLV-9 TLVs (e.g. authentication TLV 10) are skipped. A malformed
// TLV 9 value is skipped (isis-2 already length-validated the PDU on decode).
func decodeLSPEntries(tlvs []packet.TLV) []packet.LSPEntry {
	var out []packet.LSPEntry
	for _, t := range tlvs {
		if t.Type != packet.TLVLSPEntries {
			continue
		}
		dec, err := packet.DecodeLSPEntriesTLV(t.Value)
		if err != nil {
			continue
		}
		out = append(out, dec.Entries...)
	}
	return out
}

// lspEntriesTLV builds one TLV 9 (LSP Entries) from up to maxLSPEntriesPerSNP
// records using the isis-2 encoder. The caller chunks so len(entries) never
// exceeds the per-TLV limit.
func lspEntriesTLV(entries []packet.LSPEntry) packet.TLV {
	in := packet.LSPEntriesTLV{Entries: entries}
	buf := make([]byte, in.EncodedLen())
	n := packet.WriteLSPEntriesTLV(buf, 0, in)
	return packet.TLV{Type: packet.TLVLSPEntries, Value: buf[packet.TLVHeaderLen:n]}
}

// encodeCSNP serializes a CSNP to a fresh byte slice (final bytes for tx; CSNP
// carries no Fletcher checksum, clause 9.10). Authentication TLV 10 (isis-10) is
// inserted by the engine's send path before framing; this codec emits whatever
// TLVs are present.
func encodeCSNP(c *packet.CSNP) []byte {
	buf := make([]byte, c.EncodedLen())
	n := c.WriteTo(buf, 0)
	return buf[:n]
}

// encodePSNP serializes a PSNP to a fresh byte slice (final bytes for tx; PSNP
// carries no Fletcher checksum, clause 9.11).
func encodePSNP(p *packet.PSNP) []byte {
	buf := make([]byte, p.EncodedLen())
	n := p.WriteTo(buf, 0)
	return buf[:n]
}

// minLSPID is the all-zero LSP ID (the start of the CSNP range space).
func minLSPID() types.LSPID { return types.LSPID{} }

// maxLSPID is the all-ones LSP ID (the end of the CSNP range space). ISO/IEC
// 10589 clause 9.10: a CSNP spanning [0..0, ff..ff] describes the entire space.
func maxLSPID() types.LSPID {
	var id types.LSPID
	for i := range id {
		id[i] = 0xff
	}
	return id
}
