// Design: plan/learned/1051-ospf-ext-6-ti-lfa.md -- the engine's SRResolver: the read-only
// seam the SPF package's TI-LFA repair-list builder uses to resolve ext-5's
// Segment Routing labels. PrefixSIDLabel resolves a P-node's node Prefix-SID
// through its advertised SRGB (RFC 8665 Section 3.2/5); AdjSIDLabel resolves the
// Adj-SID for a Q-segment across a specific adjacency (Section 6.1). ext-6 only
// READS these maps; it never parses SR TLVs.
// RFC: rfc/short/rfc8665.md (Section 5 Prefix-SID, Section 6.1 Adj-SID)

package ospf

import (
	"net/netip"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// srTILFAResolver adapts the engine's ext-5 SR state to the spf.SRResolver seam.
// Only the IPv4 family installs it (OSPFv3 SR carriage, RFC 8666, is out of scope).
type srTILFAResolver struct{ e *engine }

// PrefixSIDLabel returns the resolved 20-bit MPLS label toward router's node
// prefix. It scans the received Prefix-SIDs for one originated by router
// (preferring a /32 host/node prefix), and resolves an index form through
// router's advertised SRGB (RFC 8665 Section 3.2). An absolute (V=1/L=1) label is
// returned as-is.
func (r srTILFAResolver) PrefixSIDLabel(router types.RouterID) (uint32, bool) {
	if r.e == nil {
		return 0, false
	}
	caps, _ := r.e.srRemoteCapabilities()
	sids := r.e.srRemotePrefixSIDs()
	var (
		best    uint32
		bestPfx netip.Prefix
		found   bool
	)
	for pfx, rs := range sids {
		if rs.Originator != router || rs.Duplicate {
			continue
		}
		lbl, ok := resolvePrefixSIDLabel(rs, caps[router])
		if !ok {
			continue
		}
		// Prefer a /32 node prefix, then the numerically smallest, for determinism.
		if !found || preferPrefix(pfx, bestPfx) {
			best, bestPfx, found = lbl, pfx, true
		}
	}
	return best, found
}

func resolvePrefixSIDLabel(rs srRemotePrefixSID, srgb sr.SRGB) (uint32, bool) {
	if rs.SID.IsLabel {
		return rs.SID.Label, true
	}
	if srgb.Empty() {
		return 0, false
	}
	return srgb.Label(rs.SID.Index)
}

func preferPrefix(candidate, current netip.Prefix) bool {
	if candidate.Bits() == 32 && current.Bits() != 32 {
		return true
	}
	if candidate.Bits() != 32 && current.Bits() == 32 {
		return false
	}
	return candidate.Addr().Less(current.Addr())
}

// AdjSIDLabel returns the Adj-SID label for the adjacency from -> to (RFC 8665
// Section 6.1). A LOCAL adjacency (from == this router) resolves through the SRLB
// allocation this router made. A REMOTE-node Q-segment (from != self, the only case
// a reachable TI-LFA Adj-SID repair ever uses -- a directly S-adjacent Q-node is
// always a base LFA and preempts TI-LFA) resolves through the P-node's advertised
// Extended Link Adj-SID (srRemoteAdjSID), returning that router's own absolute local
// label so the repaired packet steers across the P-node -> Q-node adjacency verbatim.
func (r srTILFAResolver) AdjSIDLabel(from, to types.RouterID) (uint32, bool) {
	if r.e == nil {
		return 0, false
	}
	if from == r.e.cfg.RouterID {
		if r.e.srAdj == nil {
			return 0, false
		}
		return r.e.srAdj.adjLabelForRouter(to)
	}
	return r.e.srRemoteAdjSID(from, to)
}

// srRemoteAdjSID resolves the Adj-SID MPLS label advertised by advRouter for its
// adjacency toward neighbor, by scanning advRouter's RFC 7684 Extended Link Opaque
// LSAs (Opaque Type 8) for the RFC 8665 Section 6.1 Adj-SID / Section 6.2 LAN-Adj-SID
// sub-TLV whose adjacency points at neighbor. It is the remote-node counterpart of
// the local srAdj.adjLabelForRouter, used by the TI-LFA Q-segment when the P-node is
// a remote router. The scan is on-demand (mirroring srRemotePrefixSIDs): a purged LSA
// simply is not found, so no withdrawal bookkeeping is needed.
//
// Neighbor matching (a wrong link's Adj-SID would mis-forward the repaired packet):
//   - point-to-point link (Link Type 1): the Extended Link TLV's Link ID IS the
//     neighbor's Router ID (RFC 7684 Section 3 / RFC 2328 App A.4.2); match it and
//     read the Adj-SID (sub-TLV type 2, the ext-5 origination code).
//   - transit link (Link Type 2): the LAN-Adj-SID (sub-TLV type 3) carries the
//     neighbor's Router ID explicitly in its Neighbor ID field; match that.
//
// Only the absolute local-label form (V=1/L=1, RFC 8665 Section 6.1) is used: the
// label is advRouter's own MPLS local label and belongs in the repair stack verbatim.
// An index-form Adj-SID (rare) is skipped rather than resolved through an SRLB that is
// not read here, so no wrong label is ever emitted. The label is bounded to 20 bits.
func (e *engine) srRemoteAdjSID(advRouter, neighbor types.RouterID) (uint32, bool) {
	if e == nil || e.lsdb == nil {
		return 0, false
	}
	want := [4]byte(neighbor)
	for _, v := range e.lsdb.OpaqueLSAsByType(packet.ExtLinkOpaqueType) {
		if v.AdvertisingRouter != advRouter {
			continue
		}
		lsa, err := packet.DecodeExtLinkLSA(v.Body)
		if err != nil || !lsa.HasLink {
			continue
		}
		link := lsa.Link
		switch link.LinkType {
		case packet.RouterLinkTypeP2P:
			if link.LinkID != want {
				continue
			}
			for _, sub := range link.SubTLVs {
				if sub.Type != sr.V4TypeAdjSID {
					continue
				}
				if a, derr := sr.DecodeAdjSIDValue(sub.Value); derr == nil && a.IsLabel && a.Label <= sr.MaxLabel {
					return a.Label, true
				}
			}
		case packet.RouterLinkTypeTransit:
			for _, sub := range link.SubTLVs {
				if sub.Type != sr.V4TypeLANAdjSID {
					continue
				}
				if a, derr := sr.DecodeLANAdjSIDValue(sub.Value); derr == nil && a.NeighborID == want && a.IsLabel && a.Label <= sr.MaxLabel {
					return a.Label, true
				}
			}
		}
	}
	return 0, false
}

// adjLabelForRouter returns the local Adj-SID label allocated toward neighbor
// router, across any interface. Used by the TI-LFA resolver for a Q-segment whose
// P-node is this router.
func (m *srAdjManager) adjLabelForRouter(router types.RouterID) (uint32, bool) {
	if m == nil {
		return 0, false
	}
	for k, rec := range m.labels {
		if k.router == router {
			return rec.label, true
		}
	}
	return 0, false
}
