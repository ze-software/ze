// Design: plan/learned/932-isis-6-lsdb.md -- own-LSP origination from live state.
// ISO/IEC 10589 clause 7.3.12 (origination triggers), 7.3.3 (sequence numbers /
// wraparound), 9.8 (LSP type block / overload). Wide metrics only (RFC 5305).
//
// RFC: rfc/short/rfc1195.md -- TLV 129 (Protocols Supported), TLV 132 (IP Interface Address)
// RFC: rfc/short/rfc5305.md -- TLV 22 (Extended IS Reach, 24-bit metric), TLV 135 (Extended IP Reach, 32-bit metric)
// RFC: rfc/short/rfc5301.md -- TLV 137 (Dynamic Hostname)
// RFC: rfc/short/rfc3787.md -- overload bit in the non-pseudonode LSP fragment 0 only
// RFC: rfc/short/rfc3786.md -- the 256-fragment model (LSP number 0..255; fragment 0 special)
//
// Origination builds the node's own L1 and/or L2 LSP set by full regeneration
// (not incremental, matching bio-rd): TLV 1/129/132/137 + the overload bit in
// fragment 0, then TLV 22 (neighbors) and TLV 135 (connected/redistributed
// prefixes) packed across fragments 0..255 without splitting a single TLV value.
// Each fragment is a distinct LSP with its own sequence number and Fletcher
// checksum (computed by the isis-2 codec on WriteTo). The Originator assigns and
// increments sequence numbers, handles wraparound (purge then suspend the LSP ID
// for MaxAge + ZeroAgeLifetime before re-originating from 1), and stores the
// fragments into the LSDB. Flooding (isis-7) is signaled by the returned IDs.

package lsdb

import (
	"net/netip"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// DefaultMaxLSPSize is the default maximum LSP PDU size used for fragmentation
// when no circuit MTU is known (ISO/IEC 10589: typically 1492 for Ethernet,
// spec A-5). The originator splits its TLVs so no fragment exceeds this.
const DefaultMaxLSPSize = 1492

// minTLVBudget is the smallest per-fragment TLV-region budget the fragmenter
// will accept: enough for one largest single TLV entry plus its TLV header so an
// entry is NEVER dropped for want of room (a TLV 22 entry is 11 octets, a TLV
// 135 IPv4 entry up to 9; 64 leaves comfortable headroom). A caller passing a
// smaller derived max is clamped up to minLSPSize. (Realistic MTUs are >= 1492;
// this floor only guards a misconfigured/forced-tiny max in tests.)
const minTLVBudget = 64

// minLSPSize is the floor the originator clamps a configured/derived max to: the
// common header + the LSP fixed fields + minTLVBudget. Clamping here keeps every
// fragment able to carry at least one entry and fragment 0 valid (RFC 3786).
const minLSPSize = packet.CommonHeaderLen + lspBodyFixedLen + minTLVBudget

// lspBodyFixedLen is the LSP body fixed-field length before the TLVs: PDU length
// (2) + Remaining Lifetime (2) + LSP ID (8) + Sequence Number (4) + Checksum (2)
// + type block (1). Mirrors packet.lspFixedLen (kept local: the codec's copy is
// unexported, and this is the budget the fragmenter reserves per fragment).
const lspBodyFixedLen = 2 + types.LifetimeLen + types.LSPIDLen + types.SequenceNumberLen + 2 + 1

// maxFragments is the LSP-number space (ISO/IEC 10589: LSP number 0..255). When
// the node's state needs more than 256 fragments the originator stops (RFC 3786
// extended fragments via Additional System IDs are out of scope for v1).
const maxFragments = 256

// NodeInfo is the node's own identity and global LSP attributes for origination.
// It is a plain value the engine fills from the resolved config (isis-4) so the
// lsdb package does not import the config/circuit layer.
type NodeInfo struct {
	// SystemID is the node's 6-octet System ID (the Source ID of its own LSPs,
	// pseudonode 0).
	SystemID types.SystemID
	// Areas are the node's area addresses (TLV 1), originated in fragment 0.
	Areas []types.AreaID
	// Hostname is the node's dynamic hostname (TLV 137, RFC 5301); empty omits it.
	Hostname string
	// AdvertiseIPv4 adds NLPID 0xCC to TLV 129 (always true for an IPv4 node).
	AdvertiseIPv4 bool
	// AdvertiseIPv6 adds NLPID 0x8E to TLV 129 (dual-stack). The IPv6 reachability
	// TLV 236 itself is originated by isis-12; this only advertises the protocol.
	AdvertiseIPv6 bool
	// Overload, when set, sets the LSP-database-overload (OL) bit in the
	// non-pseudonode LSP fragment 0 (RFC 3787 sec 4): the node stays reachable but
	// is not used as a transit router (SPF honors it in isis-9).
	Overload bool
	// MaxLifetime is the Remaining Lifetime stamped on a freshly originated LSP
	// (MaxAge; the lsp-lifetime leaf, default DefaultMaxAge). Zero defaults to
	// DefaultMaxAge in seconds.
	MaxLifetime uint16
	// MaxLSPSize is the maximum LSP PDU size for fragmentation (derived from the
	// circuit MTU, spec A-5). Zero defaults to DefaultMaxLSPSize.
	MaxLSPSize int
}

// AdjacencyInfo is one IS neighbor the node advertises in TLV 22 (Extended IS
// Reachability). The engine fills it from the adjacency tables (isis-5) for the
// circuits forming the level being originated.
type AdjacencyInfo struct {
	// Neighbor is the neighbor's Source ID (System ID + pseudonode). For a P2P
	// or LAN router adjacency the pseudonode is 0; the DIS pseudonode (isis-8)
	// uses a non-zero pseudonode.
	Neighbor types.SourceID
	// Metric is the wide IS-reachability metric to the neighbor (24-bit,
	// RFC 5305 sec 3).
	Metric types.Metric
}

// PrefixInfo is one IPv4 prefix the node advertises in TLV 135 (Extended IP
// Reachability). Connected and redistributed prefixes (isis-11) become these.
type PrefixInfo struct {
	// Prefix is the IPv4 prefix.
	Prefix netip.Prefix
	// Metric is the 32-bit prefix metric (RFC 5305 sec 4).
	Metric types.PrefixMetric
	// UpDown is the up/down bit (RFC 2966): set when an L1L2 node leaks an
	// L2-derived prefix into L1 (applied in isis-9; the originator carries it).
	UpDown bool
}

// PrefixInfoV6 is one IPv6 prefix the node advertises in TLV 236 (IPv6
// Reachability, RFC 5308 sec 2). Connected and redistributed IPv6 prefixes
// (isis-12) become these. The origination layer (isis-12) filters out
// link-local (fe80::/10) prefixes before building these (RFC 5308 sec 2:
// link-local prefixes MUST NOT be advertised in TLV 236).
type PrefixInfoV6 struct {
	// Prefix is the (non-link-local) IPv6 prefix.
	Prefix netip.Prefix
	// Metric is the 32-bit prefix metric (RFC 5308 sec 2, same width as TLV 135).
	Metric types.PrefixMetric
	// UpDown is the up/down bit (RFC 2966 applied to IPv6, RFC 5308 sec 5): set
	// when an L1L2 node leaks an L2-derived prefix into L1.
	UpDown bool
	// External is the X bit (RFC 5308 sec 2): set when the prefix was distributed
	// into IS-IS from another protocol (redistribution).
	External bool
}

// LevelState is the live state the node advertises at one level: its neighbors
// (TLV 22), its connected/redistributed prefixes (TLV 135), and its own IPv4
// interface addresses (TLV 132). The engine builds one per level from the
// adjacency tables and the prefix sources.
type LevelState struct {
	Neighbors []AdjacencyInfo
	Prefixes  []PrefixInfo
	// InterfaceAddrs are the node's own IPv4 interface addresses (TLV 132,
	// RFC 1195): peers use them as the SPF next-hop. De-duplicated by the engine.
	InterfaceAddrs []netip.Addr
	// PrefixesV6 are the node's own IPv6 prefixes (TLV 236, RFC 5308 sec 2),
	// originated only when the node advertises IPv6 (isis-12). Link-local prefixes
	// are excluded by the engine before they reach here (RFC 5308 sec 2).
	PrefixesV6 []PrefixInfoV6
	// InterfaceAddrsV6 are the node's own NON-LINK-LOCAL IPv6 interface addresses
	// (TLV 232 in an LSP, RFC 5308 sec 3: an LSP carries only non-link-local
	// addresses; the link-local addresses go in the IIH TLV 232, owned by the
	// circuit layer). De-duplicated by the engine.
	InterfaceAddrsV6 []netip.Addr
}

// Originator builds and stores the node's own LSPs. It holds the LSDB it writes
// into and the per-LSP-ID sequence-number state (so a regeneration increments
// monotonically) plus the wraparound suspension deadlines. It is safe for the
// engine to call Originate from its origination goroutine; the sequence/suspend
// state is guarded by its own mutex and the LSDB locks itself.
type Originator struct {
	lsdb *LSDB
	now  func() time.Time

	// sign, when set, signs a fully-encoded LSP (inserts the TLV 10 authentication
	// value as the first TLV and recomputes the Fletcher checksum) before the bytes
	// are stored and flooded (spec-isis-10). It is the engine's per-level signer:
	// an LSP is signed at ORIGINATION, not at flood time, because the LSDB stores
	// and re-floods the raw bytes verbatim, so the stored bytes must already carry
	// valid authentication (RFC 5304 sec 2). nil leaves the LSP unsigned
	// (unauthenticated operation, the default). Set via SetSigner.
	sign func(pdu []byte) []byte

	mu sync.Mutex
	// lastSeq records the last sequence number assigned to each own LSP ID so the
	// next origination increments it (clause 7.3.3). An ID absent here originates
	// at FirstSequenceNumber (1).
	lastSeq map[types.LSPID]types.SequenceNumber
	// suspendUntil records, per own LSP ID, a time before which the ID must NOT
	// be re-originated after a sequence wraparound (clause 7.3.3: purge then wait
	// MaxAge + ZeroAgeLifetime). An ID absent here is not suspended.
	suspendUntil map[types.LSPID]time.Time
}

// SetSigner installs the per-level LSP signer (spec-isis-10). The signer takes a
// fully-encoded LSP (Fletcher checksum already computed) and returns the signed
// bytes (TLV 10 inserted first, checksum recomputed). nil disables signing. Safe
// to call before any origination; the originator holds it under its own mutex.
func (o *Originator) SetSigner(sign func(pdu []byte) []byte) {
	o.mu.Lock()
	o.sign = sign
	o.mu.Unlock()
}

// encodeAndSign encodes lsp and, when a signer is installed, signs it (the caller
// holds o.mu, so o.sign is read safely). The signer re-inserts TLV 10 first and
// recomputes the Fletcher checksum (spec-isis-10), so the stored/flooded bytes
// carry valid authentication (RFC 5304 sec 2).
func (o *Originator) encodeAndSign(lsp *packet.LSP) []byte {
	raw := encodeLSP(lsp)
	if o.sign != nil {
		raw = o.sign(raw)
	}
	return raw
}

// NewOriginator constructs an Originator writing into lsdb. now may be nil
// (defaults to time.Now).
func NewOriginator(lsdb *LSDB, now func() time.Time) *Originator {
	if now == nil {
		now = time.Now
	}
	return &Originator{
		lsdb:         lsdb,
		now:          now,
		lastSeq:      make(map[types.LSPID]types.SequenceNumber),
		suspendUntil: make(map[types.LSPID]time.Time),
	}
}

// OriginateResult reports the outcome of an origination so the engine can flood
// (isis-7 sets SRM on the listed fragments) and emit LSP-change events.
type OriginateResult struct {
	// Originated lists the own LSP IDs (re)written this pass, in fragment order.
	Originated []types.LSPID
	// Purged lists own LSP IDs flooded as a purge this pass: fragments that are
	// no longer needed (state shrank) and any LSP ID that hit sequence wraparound.
	Purged []types.LSPID
	// Wrapped is true when at least one LSP ID hit sequence wraparound (the
	// engine logs it; the suspension is internal).
	Wrapped bool
}

// Originate regenerates the node's own LSP set for level from the live state and
// stores it in the LSDB (ISO/IEC 10589 clause 7.3.12: full regeneration on a
// topology change). It:
//
//   - builds fragment 0 with the non-fragmentable TLVs (area 1, protocols 129,
//     interface addresses 132, hostname 137) and the overload bit, then packs
//     TLV 22 neighbor entries and TLV 135 prefix entries across fragments 0..N
//     without splitting a single entry (spec AC-5, R-3);
//   - assigns each fragment a monotonically increasing sequence number, skipping
//     any LSP ID currently suspended after a wraparound, and triggers wraparound
//     handling at 0xFFFFFFFF (purge + suspend + re-originate-from-1, clause
//     7.3.3, spec AC-4);
//   - stamps MaxLifetime and lets the codec compute the Fletcher checksum on
//     encode (clause 7.3.11), then stores the fragment;
//   - purges any previously-originated fragment that this pass no longer needs
//     (state shrank), so a stale fragment does not linger (spec AC-5).
//
// It returns the affected LSP IDs for the engine to flood and emit events.
func (o *Originator) Originate(level Level, node NodeInfo, state LevelState) OriginateResult {
	o.mu.Lock()
	defer o.mu.Unlock()

	maxSize := node.MaxLSPSize
	if maxSize <= 0 {
		maxSize = DefaultMaxLSPSize
	}
	if maxSize < minLSPSize {
		maxSize = minLSPSize
	}
	lifetime := node.MaxLifetime
	if lifetime == 0 {
		lifetime = uint16(DefaultMaxAge / time.Second)
	}

	pt := lspPDUType(level)
	src := types.NewSourceID(node.SystemID, 0)

	// Build the ordered TLV list for fragment 0 (non-fragmentable) and the
	// fragmentable TLV 22 / TLV 135 entry streams.
	fixedTLVs := o.fixedTLVs(node, level)
	fragments := o.fragmentTLVs(maxSize, fixedTLVs, state)

	var res OriginateResult
	now := o.now()

	// Originate each fragment in number order.
	for num, tlvs := range fragments {
		id := types.NewLSPID(src, uint8(num))
		typeBlock := lspTypeBlock(level, node.Overload, num == 0)
		wrote, wrapped := o.originateFragmentLocked(level, pt, id, typeBlock, lifetime, tlvs, now)
		if wrapped {
			res.Wrapped = true
			res.Purged = append(res.Purged, id)
			// A wrapped fragment is purged and suspended this pass; it will be
			// re-originated from sequence 1 after the suspension window by a later
			// Originate call (the engine re-triggers on the suspend deadline).
			continue
		}
		if wrote {
			res.Originated = append(res.Originated, id)
		}
	}

	// Purge own fragments that exist in the LSDB but are no longer produced this
	// pass (the state shrank to fewer fragments). They are flooded as purges and
	// retained for the grace period (clause 7.3.16/17).
	purged := o.purgeStaleFragmentsLocked(level, src, len(fragments))
	res.Purged = append(res.Purged, purged...)

	if len(res.Originated) > 0 || len(res.Purged) > 0 {
		o.lsdb.incOriginations(level)
	}
	return res
}

// originateFragmentLocked assigns the next sequence number to id, builds the LSP
// with the codec (which computes the Fletcher checksum on WriteTo), and stores
// it. It returns whether it wrote the fragment and whether the sequence wrapped.
// On wrap it purges the LSP ID and suspends re-origination for MaxAge +
// ZeroAgeLifetime (clause 7.3.3). The caller holds o.mu.
func (o *Originator) originateFragmentLocked(level Level, pt packet.PDUType, id types.LSPID, typeBlock uint8, lifetime uint16, tlvs []packet.TLV, now time.Time) (wrote, wrapped bool) {
	// Honor an active wraparound suspension: do not re-originate the ID until
	// the window elapses (clause 7.3.3).
	if until, ok := o.suspendUntil[id]; ok {
		if now.Before(until) {
			return false, false
		}
		// Window elapsed: clear the suspension and re-originate from 1.
		delete(o.suspendUntil, id)
		delete(o.lastSeq, id)
	}

	prev := o.lastSeq[id] // zero (reserved) when first originated -> Next yields 1
	next, didWrap := prev.NextChecked()
	if didWrap {
		// Sequence reached 0xFFFFFFFF: purge at the max sequence and suspend the
		// ID for MaxAge + ZeroAgeLifetime before re-originating from 1 (clause
		// 7.3.3, spec AC-4).
		o.purgeFragmentLocked(level, pt, id, typeBlock, types.MaxSequenceNumber)
		o.suspendUntil[id] = now.Add(DefaultMaxAge + ZeroAgeLifetime)
		delete(o.lastSeq, id)
		o.lsdb.incWraps(level)
		return false, true
	}

	lsp := &packet.LSP{
		PDUType:           pt,
		RemainingLifetime: types.RemainingLifetime(lifetime),
		LSPID:             id,
		SequenceNumber:    next,
		TypeBlock:         typeBlock,
		TLVs:              tlvs,
	}
	raw := o.encodeAndSign(lsp)
	o.lsdb.Insert(level, lsp, raw)
	o.lastSeq[id] = next
	return true, false
}

// purgeFragmentLocked originates a purge for id: an empty LSP at the given
// sequence with Remaining Lifetime 0, stored so the aging path floods and
// retains it for the grace period (clause 7.3.16/17). Used on wraparound and for
// stale fragments. The caller holds o.mu.
func (o *Originator) purgeFragmentLocked(level Level, pt packet.PDUType, id types.LSPID, typeBlock uint8, seq types.SequenceNumber) {
	// RFC 5304 sec 2: an IS that initiates an LSP purge MUST remove the body of
	// the LSP and add the authentication TLV. StripPurgeBody is the canonical
	// body-stripping helper (Remaining Lifetime 0, no TLVs); routing every purge
	// through it keeps that canonicalization in one place so a purge can never
	// carry a stray body. The signer then inserts TLV 10 as the only TLV.
	lsp := packet.StripPurgeBody(&packet.LSP{
		PDUType:        pt,
		LSPID:          id,
		SequenceNumber: seq,
		TypeBlock:      typeBlock,
	})
	raw := o.encodeAndSign(lsp)
	o.lsdb.Insert(level, lsp, raw)
}

// purgeStaleFragmentsLocked purges own fragments numbered >= produced that still
// exist in the LSDB (the state shrank). Returns the purged IDs. The caller holds
// o.mu. It walks numbers from produced..255 and stops at the first absent one
// (fragments are contiguous from 0). Each purge re-uses the last sequence + 1 so
// peers accept it as newer.
func (o *Originator) purgeStaleFragmentsLocked(level Level, src types.SourceID, produced int) []types.LSPID {
	var purged []types.LSPID
	pt := lspPDUType(level)
	for num := produced; num < maxFragments; num++ {
		id := types.NewLSPID(src, uint8(num))
		if o.lsdb.Lookup(level, id) == nil {
			break
		}
		// Bump the sequence so the purge supersedes the live fragment.
		prev := o.lastSeq[id]
		next, didWrap := prev.NextChecked()
		if didWrap {
			next = types.MaxSequenceNumber
		}
		typeBlock := lspTypeBlock(level, false, num == 0)
		o.purgeFragmentLocked(level, pt, id, typeBlock, next)
		o.lastSeq[id] = next
		purged = append(purged, id)
	}
	return purged
}

// fixedTLVs builds the non-fragmentable TLVs that MUST appear in fragment 0
// (RFC 1195 / 5301): TLV 1 (areas), TLV 129 (protocols supported), TLV 132 (own
// IPv4 interface addresses), TLV 137 (hostname). The list is encoded into opaque
// packet.TLV values so the fragmenter can size and place them uniformly.
func (o *Originator) fixedTLVs(node NodeInfo, _ Level) []packet.TLV {
	var out []packet.TLV
	if len(node.Areas) > 0 {
		out = append(out, encodeAreaTLV(node.Areas))
	}
	if nlpids := protocolNLPIDs(node); len(nlpids) > 0 {
		out = append(out, packet.TLV{Type: packet.TLVProtocolsSupported, Value: nlpids})
	}
	if name := node.Hostname; name != "" {
		out = append(out, hostnameTLV(name))
	}
	return out
}

// fragmentTLVs packs the fixed (fragment-0) TLVs and the fragmentable TLV 22 /
// TLV 135 entry streams into per-fragment TLV lists so no fragment exceeds
// maxSize and no single TLV entry is split (spec AC-5, R-3). Fragment 0 always
// exists (it carries the fixed fields and is valid, RFC 3786). Returns one TLV
// slice per fragment, indexed by LSP number.
func (o *Originator) fragmentTLVs(maxSize int, fixed []packet.TLV, state LevelState) [][]packet.TLV {
	// Per-fragment TLV budget: the PDU max minus the common header and the LSP
	// fixed fields. Each TLV also costs its 2-octet header.
	budget := maxSize - packet.CommonHeaderLen - lspBodyFixedLen

	frags := newFragmentPacker(budget)

	// Fragment 0 starts with the fixed TLVs.
	for _, t := range fixed {
		frags.addWholeTLV(t)
	}

	// Interface-address TLV 132: list of 4-octet addresses, fragment 0 (RFC 1195
	// keeps it with the node's fixed info). Packed as whole TLVs (split across
	// several TLV 132s only if the address list overflows one TLV).
	for _, t := range interfaceAddrTLVs(state.InterfaceAddrs) {
		frags.addWholeTLV(t)
	}

	// Interface-address TLV 232 (IPv6, RFC 5308 sec 3): the node's own
	// non-link-local IPv6 addresses, fragment 0 alongside TLV 132. Whole TLVs
	// (split across several TLV 232s if the list overflows one TLV value).
	for _, t := range interfaceAddrV6TLVs(state.InterfaceAddrsV6) {
		frags.addWholeTLV(t)
	}

	// TLV 22 neighbor entries (RFC 5305 sec 3): pack entries into TLV 22s,
	// starting a new TLV (and fragment if needed) so no entry is split.
	for _, n := range state.Neighbors {
		frags.addEntry(packet.TLVExtendedISReach, extISReachEntryBytes(n))
	}
	// TLV 135 prefix entries (RFC 5305 sec 4).
	for _, p := range state.Prefixes {
		frags.addEntry(packet.TLVExtendedIPReach, extIPReachEntryBytes(p))
	}
	// TLV 236 IPv6 prefix entries (RFC 5308 sec 2): packed exactly like TLV 135
	// (no entry split). Only present when the node advertises IPv6 (isis-12).
	for _, p := range state.PrefixesV6 {
		frags.addEntry(packet.TLVIPv6Reachability, extIPv6ReachEntryBytes(p))
	}

	return frags.fragments()
}

// lspPDUType maps a database level to the LSP PDU type.
func lspPDUType(level Level) packet.PDUType {
	if level == Level2 {
		return packet.PDUTypeL2LSP
	}
	return packet.PDUTypeL1LSP
}

// lspTypeBlock builds the LSP type-block octet (ISO/IEC 10589 clause 9.8): the
// IS-type bits for the level, and the overload (OL) bit ONLY on fragment 0 of
// the non-pseudonode LSP (RFC 3787 sec 4: the OL bit lives in LSP number zero;
// an overloaded node sets it there, not in every fragment). The P and ATT bits
// are left zero by this spec (ATT is set by an L1L2 router in its L1 LSP, isis-9).
func lspTypeBlock(level Level, overload, fragmentZero bool) uint8 {
	var tb uint8
	if level == Level2 {
		tb |= packet.LSPFlagISTypeL2
	} else {
		tb |= packet.LSPFlagISTypeL1
	}
	if overload && fragmentZero {
		tb |= packet.LSPFlagOverload
	}
	return tb
}

// encodeLSP serializes an LSP into a freshly allocated, entry-sized buffer and
// returns it. The codec backfills the PDU length and the Fletcher checksum
// (ISO/IEC 10589 clause 7.3.11) on WriteTo. The result is a build buffer the
// LSDB copies once into the entry (entry.go); using make here is the
// result-copy-to-caller case explicitly allowed by ai/rules/performance.md
// ("cached encoding, result copies to callers").
func encodeLSP(lsp *packet.LSP) []byte {
	buf := make([]byte, lsp.EncodedLen())
	n := lsp.WriteTo(buf, 0)
	return buf[:n]
}
