// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing reception: which SR
// TLV or sub-TLV length condemns the whole carrying LSA, and how received Prefix-SIDs are
// judged duplicate. Every reader of an Extended Prefix, Extended Link or OSPFv3
// Extended-LSA (Prefix-SID install, TI-LFA Adj-SID lookup, BGP-LS export) asks these
// predicates before it uses anything from the LSA.
// Related: sr_install.go -- IPv4 Prefix-SID reception and install
// Related: sr_reception_v6.go -- IPv6 Prefix-SID reception
// Related: sr_tilfa.go -- remote Adj-SID lookup
// Related: bgpls_export.go -- BGP-LS export of the SR sub-TLVs
// RFC: rfc/short/rfc8665.md (§5, §9); rfc/short/rfc8666.md (§6, §10)
package ospf

import (
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
)

// srExtPrefixLengthInvalid reports whether an IPv4 Extended Prefix Opaque LSA carries an
// RFC 8665 TLV or sub-TLV (an Extended Prefix Range TLV, a Prefix-SID sub-TLV) whose length
// is invalid. A true answer means the whole LSA is ignored, not only the bad sub-TLV.
// RFC 8665 Section 9: "For any new TLVs/sub-TLVs defined in this document, if the length
// is invalid, the LSA in which it is advertised is considered malformed and MUST be
// ignored."
// A V/L-Flag error is not a length error and condemns the sub-TLV alone.
func srExtPrefixLengthInvalid(lsa *packet.ExtPrefixLSA) bool {
	for i := range lsa.Ranges {
		if _, err := sr.DecodeExtPrefixRangeValueV4(lsa.Ranges[i].Value); errors.Is(err, sr.ErrLength) {
			return true
		}
	}
	for i := range lsa.Prefixes {
		for _, sub := range lsa.Prefixes[i].SubTLVs {
			if sub.Type != sr.V4TypePrefixSID {
				continue
			}
			if _, err := sr.DecodePrefixSIDValue(sub.Value); errors.Is(err, sr.ErrLength) {
				return true
			}
		}
	}
	return false
}

// srExtLinkLengthInvalid reports whether the Extended Link TLV of an IPv4 Extended Link
// Opaque LSA carries an Adj-SID or LAN Adj-SID sub-TLV whose length is invalid, which
// makes the whole LSA ignored.
// RFC 8665 Section 9: "if the length is invalid, the LSA in which it is advertised is
// considered malformed and MUST be ignored."
// A V/L-Flag error is not a length error and condemns the sub-TLV alone.
func srExtLinkLengthInvalid(link *packet.ExtLinkTLV) bool {
	for _, sub := range link.SubTLVs {
		var err error
		switch sub.Type {
		case sr.V4TypeAdjSID:
			_, err = sr.DecodeAdjSIDValue(sub.Value)
		case sr.V4TypeLANAdjSID:
			_, err = sr.DecodeLANAdjSIDValue(sub.Value)
		default:
			continue // not an RFC 8665 sub-TLV
		}
		if errors.Is(err, sr.ErrLength) {
			return true
		}
	}
	return false
}

// srV3ExtendedLengthInvalid reports whether the top-level TLVs of an OSPFv3 Extended-LSA
// carry an RFC 8666 TLV or sub-TLV (an Extended Prefix Range TLV; a Prefix-SID, Adj-SID or
// LAN Adj-SID sub-TLV) whose length is invalid, which makes the whole LSA ignored. The
// Extended-LSA sub-TLV types are one registry across TLVs (RFC 8362 Section 3), so each
// type is judged by its own layout wherever it appears.
// RFC 8666 Section 10: "For any new TLVs/sub-TLVs defined in this document, if the length
// is invalid, the LSA in which it is advertised is considered malformed and MUST be
// ignored."
// A V/L-Flag error is not a length error and condemns the sub-TLV alone.
func srV3ExtendedLengthInvalid(tlvs []ospfv3packet.ExtendedTLV) bool {
	for i := range tlvs {
		tlv := &tlvs[i]
		switch tlv.Type {
		case extTLVExtPrefixRange:
			if _, err := sr.DecodeExtPrefixRangeValueV6(tlv.Value); errors.Is(err, sr.ErrLength) {
				return true
			}
		case extTLVIntraAreaPrefix, extTLVInterAreaPrefix, extTLVExternalPrefix:
			if srV3SubTLVLengthInvalid(v6PrefixTLVSubTLVs(tlv.Value)) {
				return true
			}
		case extTLVRouterLink:
			// RFC 8362 Section 3.2: the Router-Link TLV keeps the 16-octet Router-LSA
			// link layout before its sub-TLVs.
			if len(tlv.Value) <= 16 {
				continue
			}
			subs, err := ospfv3packet.SubTLVsAt(tlv.Value, 16)
			if err != nil {
				continue // sub-TLV framing: an RFC 8362 error, judged by the LSA decoder
			}
			if srV3SubTLVLengthInvalid(subs) {
				return true
			}
		}
	}
	return false
}

// srV3SubTLVLengthInvalid reports whether any RFC 8666 Prefix-SID, Adj-SID or LAN Adj-SID
// sub-TLV in subs has an invalid length.
func srV3SubTLVLengthInvalid(subs []ospfv3packet.ExtendedTLV) bool {
	for i := range subs {
		var err error
		switch subs[i].Type {
		case sr.V6TypePrefixSID:
			_, err = sr.DecodePrefixSIDValueV6(subs[i].Value)
		case sr.V6TypeAdjSID:
			_, err = sr.DecodeAdjSIDValueV6(subs[i].Value)
		case sr.V6TypeLANAdjSID:
			_, err = sr.DecodeLANAdjSIDValueV6(subs[i].Value)
		default:
			continue // not an RFC 8666 sub-TLV
		}
		if errors.Is(err, sr.ErrLength) {
			return true
		}
	}
	return false
}

// srPrefixSIDKey is the scope in which received Prefix-SIDs are judged duplicate: one
// prefix, one topology (the MT-ID; OSPFv3 carries none, so always 0) and one algorithm.
type srPrefixSIDKey struct {
	prefix    netip.Prefix
	topology  uint8
	algorithm uint8
}

// srPrefixSIDAdvertiser is one advertising router within one area. The per-router rule
// is judged per area, because an ABR advertises a prefix once into each area it joins
// (RFC 8665 Section 7.2, RFC 8666 Section 8.2), and each area holds one advertisement.
type srPrefixSIDAdvertiser struct {
	area   types.AreaID
	router types.RouterID
}

// srPrefixSIDScope holds every Prefix-SID received for one srPrefixSIDKey, per
// advertiser, in reception order.
type srPrefixSIDScope struct {
	first   srRemotePrefixSID
	count   map[srPrefixSIDAdvertiser]int
	sid     map[srPrefixSIDAdvertiser]srRemotePrefixSID
	ordered []srPrefixSIDAdvertiser
}

// srPrefixSIDTable aggregates received Prefix-SIDs per srPrefixSIDKey, in reception order.
// Not safe for concurrent use: one reader builds it and drops it.
type srPrefixSIDTable struct {
	scopes map[srPrefixSIDKey]*srPrefixSIDScope
	order  []srPrefixSIDKey
}

func newSRPrefixSIDTable() *srPrefixSIDTable {
	return &srPrefixSIDTable{scopes: make(map[srPrefixSIDKey]*srPrefixSIDScope)}
}

// add records one Prefix-SID received for prefix from received.Originator in area.
func (t *srPrefixSIDTable) add(received srRemotePrefixSID, prefix netip.Prefix, area types.AreaID) {
	key := srPrefixSIDKey{prefix: prefix, topology: received.SID.MTID, algorithm: received.SID.Algorithm}
	scope, seen := t.scopes[key]
	if !seen {
		scope = &srPrefixSIDScope{
			first: received,
			count: make(map[srPrefixSIDAdvertiser]int),
			sid:   make(map[srPrefixSIDAdvertiser]srRemotePrefixSID),
		}
		t.scopes[key] = scope
		t.order = append(t.order, key)
	}
	adv := srPrefixSIDAdvertiser{area: area, router: received.Originator}
	if scope.count[adv] == 0 {
		scope.sid[adv] = received
		scope.ordered = append(scope.ordered, adv)
	}
	scope.count[adv]++
}

// resolve returns the Prefix-SID the scope yields, or its first Prefix-SID marked
// Duplicate when none survives.
//
// RFC 8665 Section 5: "If an OSPF router advertises multiple Prefix-SIDs for the same
// prefix, topology, and algorithm, all of them MUST be ignored."
// RFC 8666 Section 6: "If an OSPFv3 router advertises multiple Prefix-SIDs for the same
// prefix, topology, and algorithm, all of them MUST be ignored."
// The rule is per advertising router and holds whatever the SID values: a router that
// advertises two Prefix-SIDs in one scope, equal or not, has all of them ignored.
//
// Across routers the RFCs are silent, and the rest is Ze's policy: routers binding the
// SAME SID (an anycast prefix, RFC 8660 Section 2.5, or ABRs propagating one Prefix-SID,
// RFC 8665 Section 7.1) are one binding and it is used; routers binding DIFFERENT SIDs
// leave no binding to prefer, so every one of them is ignored.
func (s *srPrefixSIDScope) resolve() srRemotePrefixSID {
	var chosen srRemotePrefixSID
	survivors := 0
	for _, adv := range s.ordered {
		if s.count[adv] > 1 {
			continue // RFC 8665 Section 5 / RFC 8666 Section 6
		}
		received := s.sid[adv]
		if survivors > 0 && !srPrefixSIDEqual(chosen.SID, received.SID) {
			chosen.Duplicate = true // Ze policy: differing SIDs from different routers
			return chosen
		}
		if survivors == 0 {
			chosen = received
		}
		survivors++
	}
	if survivors == 0 {
		dup := s.first
		dup.Duplicate = true
		return dup
	}
	return chosen
}

// byPrefix returns the one Prefix-SID per prefix the installer reads: the topology 0,
// algorithm 0 scope when the prefix has one, because it is the only one Ze computes paths
// for; otherwise the first scope received, so the installer still counts a Prefix-SID it
// cannot install.
func (t *srPrefixSIDTable) byPrefix() map[netip.Prefix]srRemotePrefixSID {
	out := make(map[netip.Prefix]srRemotePrefixSID, len(t.order))
	for _, key := range t.order {
		if _, seen := out[key.prefix]; seen && (key.topology != 0 || key.algorithm != 0) {
			continue
		}
		out[key.prefix] = t.scopes[key].resolve()
	}
	return out
}

// srPrefixSIDEqual reports whether two Prefix-SIDs bind the same SID (index or label,
// under the same algorithm) to a prefix, so a propagated re-advertisement is not taken
// for a conflict.
func srPrefixSIDEqual(a, b sr.PrefixSID) bool {
	return a.IsLabel == b.IsLabel && a.Index == b.Index && a.Label == b.Label && a.Algorithm == b.Algorithm
}
