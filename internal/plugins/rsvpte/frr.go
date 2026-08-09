// Design: docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md -- RSVP-TE Fast Reroute
// RFC: rfc/short/rfc4090.md
// Related: wire.go -- base object codecs and the protection-flag constants
// Related: build.go -- composes FAST_REROUTE/SESSION_ATTRIBUTE into PATH
// Related: engine.go -- PLR arming, local repair, head-end re-optimization
//
// RFC 4090 Fast Reroute extensions: the FAST_REROUTE (class 205) and DETOUR
// (class 63) object codecs, the SESSION_ATTRIBUTE (class 207) codec that carries
// the protection-desired flags, and the PLR/Merge-Point local-protection logic.
// Facility backup (Section 3.2) is the primary mode; one-to-one detour backup
// (Section 3.1) is secondary. The protection-flag constants (FRRFlag*,
// SessAttr*, RROFlag*) live in wire.go next to the other wire constants.
package rsvpte

import (
	"encoding/binary"
	"math"
	"net/netip"
)

// fastReroute is the FAST_REROUTE object (RFC 4090 Section 4.1). The head-end
// adds it to the PATH to request local protection; transit PLRs read its Flags
// (facility vs one-to-one) and Hop-limit. ze does not track resource affinities,
// so IncludeAny/ExcludeAny/IncludeAll are carried as zero.
type fastReroute struct {
	SetupPrio  uint8
	HoldPrio   uint8
	HopLimit   uint8
	Flags      uint8
	Bandwidth  float32
	IncludeAny uint32
	ExcludeAny uint32
	IncludeAll uint32
}

// frrBodyLen is the FAST_REROUTE body: Setup/Hold/Hop-limit/Flags (4) + Bandwidth
// (4) + Include-any/Exclude-any/Include-all (12). Object Length adds objHdrLen.
const frrBodyLen = 20

// encodeFastReroute writes a FAST_REROUTE object. Returns bytes written.
func encodeFastReroute(buf []byte, fr fastReroute) int {
	objLen := uint16(objHdrLen + frrBodyLen)
	encodeObjectHeader(buf, objectHeader{Length: objLen, ClassNum: ClassFastReroute, CType: CTypeFastReroute})
	buf[4] = fr.SetupPrio
	buf[5] = fr.HoldPrio
	buf[6] = fr.HopLimit
	buf[7] = fr.Flags
	binary.BigEndian.PutUint32(buf[8:12], math.Float32bits(fr.Bandwidth))
	binary.BigEndian.PutUint32(buf[12:16], fr.IncludeAny)
	binary.BigEndian.PutUint32(buf[16:20], fr.ExcludeAny)
	binary.BigEndian.PutUint32(buf[20:24], fr.IncludeAll)
	return int(objLen)
}

// decodeFastReroute reads a FAST_REROUTE object body (after the object header).
func decodeFastReroute(body []byte) (fastReroute, error) {
	if len(body) < frrBodyLen {
		return fastReroute{}, errShortObject
	}
	return fastReroute{
		SetupPrio:  body[0],
		HoldPrio:   body[1],
		HopLimit:   body[2],
		Flags:      body[3],
		Bandwidth:  math.Float32frombits(binary.BigEndian.Uint32(body[4:8])),
		IncludeAny: binary.BigEndian.Uint32(body[8:12]),
		ExcludeAny: binary.BigEndian.Uint32(body[12:16]),
		IncludeAll: binary.BigEndian.Uint32(body[16:20]),
	}, nil
}

// maxSessionName bounds the SESSION_ATTRIBUTE display name so a malicious or
// malformed object cannot grow the encode past the message buffer; it also keeps
// the name well inside a single subobject. RFC 3209 does not cap it.
const maxSessionName = 64

// sessionAttribute is the SESSION_ATTRIBUTE object (RFC 3209 Section 4.7.2,
// C-Type 7 LSP_TUNNEL without resource affinities). The Flags byte carries the
// RFC 4090 Section 4.3 local/node/bandwidth protection-desired bits.
type sessionAttribute struct {
	SetupPrio uint8
	HoldPrio  uint8
	Flags     uint8
	Name      string
}

// encodeSessionAttr writes a SESSION_ATTRIBUTE object (C-Type 7). The Session
// Name is null-padded to a 4-byte boundary (RFC 3209 Section 4.7.2). Returns
// bytes written. copy(dst []byte, src string) does not allocate.
func encodeSessionAttr(buf []byte, sa sessionAttribute) int {
	name := sa.Name
	if len(name) > maxSessionName {
		name = name[:maxSessionName]
	}
	nameLen := len(name)
	padded := (nameLen + 3) &^ 3 // round up to a 4-byte boundary
	objLen := uint16(objHdrLen + 4 + padded)
	encodeObjectHeader(buf, objectHeader{Length: objLen, ClassNum: ClassSessionAttr, CType: CTypeSessionAttr})
	buf[4] = sa.SetupPrio
	buf[5] = sa.HoldPrio
	buf[6] = sa.Flags
	buf[7] = uint8(nameLen)
	for i := 8; i < 8+padded; i++ {
		buf[i] = 0
	}
	copy(buf[8:8+nameLen], name)
	return int(objLen)
}

// decodeSessionAttr reads a SESSION_ATTRIBUTE object body (after the header) for
// the given C-Type. C-Type 1 (LSP_TUNNEL_RA) prefixes the priorities with three
// 4-byte resource-affinity masks (Exclude-any / Include-any / Include-all, RFC
// 3209 Section 4.7.1) that ze does not use; C-Type 7 (LSP_TUNNEL) has no prefix.
// Without this the protection Flags byte would be misread for a C-Type 1 peer.
func decodeSessionAttr(body []byte, cType uint8) (sessionAttribute, error) {
	off := 0
	if cType == CTypeSessionAttrRA {
		off = 12 // skip Exclude-any / Include-any / Include-all
	}
	if len(body) < off+4 {
		return sessionAttribute{}, errShortObject
	}
	sa := sessionAttribute{
		SetupPrio: body[off],
		HoldPrio:  body[off+1],
		Flags:     body[off+2],
	}
	nameLen := int(body[off+3])
	if nameLen > 0 {
		if off+4+nameLen > len(body) {
			return sessionAttribute{}, errShortObject
		}
		sa.Name = string(body[off+4 : off+4+nameLen])
	}
	return sa, nil
}

// protectionRequest holds the head-end's configured local-protection parameters
// for an LSP (RFC 4090). It drives the SESSION_ATTRIBUTE protection flags and the
// FAST_REROUTE object added to the PATH, and a transit PLR reconstructs it from a
// received PATH to learn what protection to arm. A nil pathStateBlock.Protection
// means no protection is requested.
type protectionRequest struct {
	Facility            bool  // facility backup (Section 3.2) vs one-to-one (Section 3.1)
	NodeProtection      bool  // request NNHOP (node) protection rather than NHOP (link)
	BandwidthProtection bool  // request a backup that guarantees the reserved bandwidth
	HopLimit            uint8 // bound on the backup path length
	Bandwidth           float32
	SetupPrio           uint8
	HoldPrio            uint8
	Name                string // tunnel name, carried in SESSION_ATTRIBUTE for display
}

// sessionAttr builds the SESSION_ATTRIBUTE object carrying the protection-desired
// flags (RFC 4090 Section 4.3): always local-protection-desired, plus node and
// bandwidth protection when requested.
func (pr *protectionRequest) sessionAttr() sessionAttribute {
	// RFC 3209 Section 4.4.3 / RFC 4090 Section 6.4.2: request label recording so
	// the RESV RRO carries each hop's label. The PLR needs the merge point's label
	// to build the backup forwarding (the NNHOP label for node protection).
	// RFC 4090 Section 4.3: also advertise SE style so the head-end's
	// make-before-break re-optimization shares bandwidth on common links (the
	// reroute already reserves with the SE style).
	flags := SessAttrLocalProtection | SessAttrLabelRecording | SessAttrSEStyle
	if pr.NodeProtection {
		flags |= SessAttrNodeProtection
	}
	if pr.BandwidthProtection {
		flags |= SessAttrBandwidthProtection
	}
	return sessionAttribute{SetupPrio: pr.SetupPrio, HoldPrio: pr.HoldPrio, Flags: flags, Name: pr.Name}
}

// fastReroute builds the FAST_REROUTE object (RFC 4090 Section 4.1) requesting
// either facility or one-to-one backup.
func (pr *protectionRequest) fastReroute() fastReroute {
	flags := FRRFlagFacilityBackup
	if !pr.Facility {
		flags = FRRFlagOneToOneBackup
	}
	return fastReroute{
		SetupPrio: pr.SetupPrio,
		HoldPrio:  pr.HoldPrio,
		HopLimit:  pr.HopLimit,
		Flags:     flags,
		Bandwidth: pr.Bandwidth,
	}
}

// protectionFromPath reconstructs a protectionRequest from a received PATH so a
// transit node can learn what local protection the head-end wants. It returns nil
// when the PATH requests no protection (no FAST_REROUTE and no SESSION_ATTRIBUTE
// local-protection flag), so a non-protected LSP carries no protection state.
func protectionFromPath(msg *ParsedMessage) *protectionRequest {
	saProtect := msg.HasSessionAttr && msg.SessionAttr.Flags&SessAttrLocalProtection != 0
	if !msg.HasFastReroute && !saProtect {
		return nil
	}
	pr := &protectionRequest{Facility: true}
	if msg.HasFastReroute {
		pr.Facility = msg.FastReroute.Flags&FRRFlagFacilityBackup != 0 || msg.FastReroute.Flags&FRRFlagOneToOneBackup == 0
		pr.HopLimit = msg.FastReroute.HopLimit
		pr.Bandwidth = msg.FastReroute.Bandwidth
		pr.SetupPrio = msg.FastReroute.SetupPrio
		pr.HoldPrio = msg.FastReroute.HoldPrio
	}
	if msg.HasSessionAttr {
		pr.NodeProtection = msg.SessionAttr.Flags&SessAttrNodeProtection != 0
		pr.BandwidthProtection = msg.SessionAttr.Flags&SessAttrBandwidthProtection != 0
		pr.Name = msg.SessionAttr.Name
	}
	return pr
}

// bypassTunnelIDBase reserves the top 4096 tunnel-ids (0xF000-0xFFFF) for
// configured bypass LSPs so a bypass never shares an lspKey with a protected
// tunnel to the same destination (A-3: bypasses live in the same lspTable, keyed
// distinctly). parseConfig rejects a configured tunnel-id in this range.
const bypassTunnelIDBase uint16 = 0xF000

// bypassNameHash derives a stable 12-bit id from the bypass name (FNV-1a). The
// bypass lspKey is keyed by this, not by the slice index, so a bypass keeps the
// same key across a config reload that reorders the bypass list (the index is not
// stable; the name is). Collisions within the 4096-slot space are detected at
// config time (parseConfig).
func bypassNameHash(name string) uint16 {
	const offset, prime = uint32(2166136261), uint32(16777619)
	h := offset
	for i := range len(name) {
		h = (h ^ uint32(name[i])) * prime
	}
	return uint16(h) & 0x0FFF
}

// bypassKey is the lspKey a configured bypass signals under: this PLR as the
// sender, the bypass merge point as the tunnel endpoint, and a reserved tunnel-id
// derived (stably) from the bypass name.
func bypassKey(bc bypassConfig, routerID netip.Addr) lspKey {
	return lspKey{
		TunnelEndpoint: bc.MergePoint,
		TunnelID:       bypassTunnelIDBase | bypassNameHash(bc.Name),
		ExtTunnelID:    addrToUint32(routerID),
		SenderAddr:     routerID,
		LSPID:          1,
	}
}

// selectBypass finds the configured bypass that protects the protected LSP's
// downstream resource: its merge point is the NHOP for link protection, or the
// NNHOP for node protection (RFC 4090 Section 3.2). rem is the remaining ERO at
// this transit node (rem[0] is the NHOP, rem[1] the NNHOP). It returns the bypass
// LSP key and true on a match.
func (e *engine) selectBypass(rem []eroHop, pr *protectionRequest) (lspKey, bool) {
	if len(rem) == 0 {
		return lspKey{}, false
	}
	mp := rem[0].Address.Addr() // NHOP (link protection)
	if pr.NodeProtection {
		if len(rem) < 2 {
			return lspKey{}, false // no NNHOP in the ERO: cannot node-protect here
		}
		mp = rem[1].Address.Addr() // NNHOP (node protection)
	}
	cfg := e.cfg() // single consistent snapshot (a reload mid-scan must not mix configs)
	for _, bc := range cfg.Bypasses {
		if bc.MergePoint != mp {
			continue
		}
		// Node protection needs a bypass that merges at the NNHOP; a link-only
		// bypass (merging at the NHOP) does not protect against the node failing.
		if pr.NodeProtection && !bc.NodeProtection {
			continue
		}
		return bypassKey(bc, cfg.RouterID), true
	}
	return lspKey{}, false
}

// rroProtectionFlags computes the RFC 4090 RRO subobject flags a PLR records for
// a protected transit LSP: protection-available once a bypass is armed,
// protection-in-use once local repair has switched traffic onto it, plus the
// node/bandwidth protection bits the head-end requested. Callers hold lsp.mu.
func rroProtectionFlags(lsp *LSP) uint8 {
	if lsp.Bypass == nil {
		return 0
	}
	flags := RROFlagProtectionAvailable
	if lsp.ProtectionInUse {
		flags |= RROFlagProtectionInUse
	}
	if lsp.PSB != nil && lsp.PSB.Protection != nil {
		if lsp.PSB.Protection.NodeProtection {
			flags |= RROFlagNodeProtection
		}
		if lsp.PSB.Protection.BandwidthProtection {
			flags |= RROFlagBandwidthProtection
		}
	}
	return flags
}

// tryLocalRepair performs RFC 4090 facility-backup local repair for a protected
// transit LSP whose downstream link failed: it redirects the LSP onto its armed
// bypass by programming a 2-label swap -- the bypass label pushed over the
// (swapped) protected label, forwarded via the bypass next hop -- and marks
// protection in use. It returns true when the LSP is now protected and MUST be
// retained (the caller skips teardown), and false when no usable bypass is
// available (the caller falls back to the base tear-down behavior). Idempotent:
// a repeated link-down event for an already-repaired LSP keeps the repair.
func (e *engine) tryLocalRepair(lsp *LSP, key lspKey) bool {
	lsp.mu.Lock()
	bypass := lsp.Bypass
	inLabel := lsp.InLabel
	// The inner label the PLR pushes under the bypass is the merge point's label
	// for the protected LSP: the NHOP's advertised label for link protection, or
	// the NNHOP's recorded label for node protection (RFC 4090 Section 6.4.2).
	// handleResvTransit resolves it into BackupLabel; 0 means "unresolved" (node
	// protection whose NNHOP label was never recorded), which must NOT be repaired
	// even if a PATH refresh re-armed the bypass.
	protectedOut := lsp.BackupLabel
	already := lsp.ProtectionInUse
	lsp.mu.Unlock()
	if already {
		return true
	}
	// No bypass armed, no forwarding state yet (a transit LSP still in PathReceived
	// with no swap entry), or no resolved merge-point label: nothing to safely
	// redirect, so fall back to the base tear-down rather than claim a repair the
	// data plane never made or would deliver to the wrong label.
	if bypass == nil || inLabel == 0 || protectedOut == 0 {
		return false
	}

	// The bypass must be up with a known out-label and next hop to carry traffic.
	bl, ok := e.table.Get(*bypass)
	if !ok {
		return false
	}
	bl.mu.Lock()
	bypassLabel := bl.OutLabel
	bypassNextHop := bl.NextHop
	bypassUp := bl.State == LSPStateUp
	bl.mu.Unlock()
	if !bypassUp || !bypassNextHop.IsValid() || bypassLabel == 0 {
		e.log.Warn("rsvp-te: bypass not ready, cannot locally repair", "lsp", key.String(), "bypass", bypass.String())
		return false
	}

	// RFC 4090 Section 3.2: push the bypass label on top of the swapped protected
	// label and forward over the bypass next hop. labels[0] is the outermost label
	// (the bypass label), so the merge point pops it and continues the protected
	// LSP. The kernel AF_MPLS swap accepts this multi-label stack directly.
	if e.fib != nil {
		if err := e.fib.programBackup(inLabel, []uint32{bypassLabel, protectedOut}, bypassNextHop); err != nil {
			e.log.Error("rsvp-te: local repair FIB program failed", "lsp", key.String(), "error", err)
			return false
		}
	}

	lsp.mu.Lock()
	lsp.ProtectionInUse = true
	lsp.mu.Unlock()
	if m := rsvpteMetricsPtr.Load(); m != nil {
		m.localRepairs.Inc()
	}
	e.log.Info("rsvp-te: local repair onto bypass", "lsp", key.String(), "bypass", bypass.String(),
		"in-label", inLabel, "bypass-label", bypassLabel, "protected-label", protectedOut, "next-hop", bypassNextHop)
	return true
}

// reoptimizeOnNotify handles a Notify ("Tunnel locally repaired", RFC 4090
// Section 6.5) at the head-end: it starts a make-before-break reroute of the
// ingress LSP onto a fresh path (its configured ERO), which tears the old,
// locally-repaired LSP once the replacement is up. It only acts on an ingress
// LSP this node head-ends, and skips if a reroute for the next LSP_ID is already
// in flight so repeated Notifies do not spawn a storm of replacements.
func (e *engine) reoptimizeOnNotify(key lspKey) {
	lsp, ok := e.table.Get(key)
	if !ok {
		return
	}
	lsp.mu.Lock()
	isIngress := lsp.Role == RoleIngress
	var ero []eroHop
	if lsp.PSB != nil {
		ero = lsp.PSB.ERO
	}
	lsp.mu.Unlock()
	if !isIngress {
		return
	}
	newKey := key
	newKey.LSPID = key.LSPID + 1
	if _, inFlight := e.table.Get(newKey); inFlight {
		return // a make-before-break reroute is already under way
	}
	if _, started := e.reroute(key, ero); started {
		e.log.Info("rsvp-te: head-end re-optimizing after local-repair Notify", "lsp", key.String())
	}
}

// clearBypassReferences clears the armed-bypass association on every protected
// LSP that pointed at the given (now-removed) bypass LSP, so they stop reporting
// "protection available"/"in use" for a bypass that no longer exists. The next
// PATH refresh re-arms (via selectBypass) if a matching bypass returns.
func (e *engine) clearBypassReferences(bypass lspKey) {
	for _, lsp := range e.table.All() {
		lsp.mu.Lock()
		if lsp.Bypass != nil && *lsp.Bypass == bypass {
			lsp.Bypass = nil
			lsp.ProtectionInUse = false
		}
		lsp.mu.Unlock()
	}
}

// updateFRRGauges recomputes the protected/bypass LSP gauges from the table.
// Called from the refresh loop and after a tunnel reconcile.
func updateFRRGauges(lspTable *lspTable) {
	m := rsvpteMetricsPtr.Load()
	if m == nil {
		return
	}
	var protected, bypass int
	for _, lsp := range lspTable.All() {
		lsp.mu.Lock()
		switch {
		case lsp.IsBypass:
			bypass++
		case lsp.Bypass != nil:
			protected++
		}
		lsp.mu.Unlock()
	}
	m.protectedLSPs.Set(float64(protected))
	m.bypassLSPs.Set(float64(bypass))
}
