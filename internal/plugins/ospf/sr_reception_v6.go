// Design: docs/architecture/wire/ospfv3.md -- OSPF Segment Routing IPv6 reception.
// Parses the Prefix-SID sub-TLVs (RFC 8666 §6) carried in received RFC 8362 Extended
// prefix LSAs (E-Intra-Area-Prefix 0x2029, E-Inter-Area-Prefix 0x2023, E-AS-External
// 0x4025, E-Type-7 0x2027) and returns them keyed by prefix so the shared reception->
// install driver (sr_install.go) computes labels and programs mpls-fib exactly like the
// IPv4 opaque Extended-Prefix path. Two carriages are honored: the Intra/Inter/External
// Prefix TLV (types 6/3/5) with a nested Prefix-SID sub-TLV, and the Extended Prefix Range
// TLV (type 9) whose Prefix-SID is the starting value. Every read is bound-checked over
// the RFC 8362 TLV iterator; a malformed body never panics (RFC 8666 §11).
// RFC: rfc/short/rfc8666.md (§5 Ext-Prefix-Range, §6 Prefix-SID, §8.2 inter-area)

package ospf

import (
	"net/netip"

	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// v6ReceivedPrefixSID is one Prefix-SID parsed from a received Extended prefix LSA, with
// enough context (originator, source area, LS type) for both the install map and the
// inter-area propagation decision (RFC 8666 §8.2).
type v6ReceivedPrefixSID struct {
	Prefix     netip.Prefix
	SID        sr.PrefixSID
	Originator types.RouterID
	Area       types.AreaID
	LSType     ospfv3types.LSType
}

// v6EPrefixLSTypes are the RFC 8362 Extended prefix LSA types that may carry a Prefix-SID
// (RFC 8666 §6). E-Intra-Area-Prefix carries a 12-byte referenced header before its TLVs;
// the others start with TLVs directly.
var v6EPrefixLSTypes = []ospfv3types.LSType{
	ospfv3types.LSTypeEIntraAreaPrefix,
	ospfv3types.LSTypeEInterAreaPrefix,
	ospfv3types.LSTypeEASExternal,
	ospfv3types.LSTypeEType7,
}

// v6EPrefixHeaderLen returns the fixed body prefix (before the TLV stream) for an Extended
// prefix LSA type: 12 octets for E-Intra-Area-Prefix (RFC 8362 §3.5 referenced fields), 0
// for E-Inter-Area-Prefix / E-AS-External / E-Type-7 (their bodies are TLVs directly).
func v6EPrefixHeaderLen(t ospfv3types.LSType) int {
	if t == ospfv3types.LSTypeEIntraAreaPrefix {
		return eIntraPrefixHeaderLen
	}
	return 0
}

// v6ReceivedPrefixSIDs reads every Extended prefix LSA in the LSDB and returns each
// Prefix-SID it carries, every algorithm included. A malformed body is counted and
// skipped, never fatal.
func (e *engine) v6ReceivedPrefixSIDs() []v6ReceivedPrefixSID {
	if e.lsdb == nil {
		return nil
	}
	var out []v6ReceivedPrefixSID
	var sids []sr.PrefixSID
	for _, lt := range v6EPrefixLSTypes {
		hdr := v6EPrefixHeaderLen(lt)
		for _, v := range e.lsdb.LSAViewsByType(types.LSType(lt)) {
			if len(v.Body) < hdr {
				continue
			}
			ext, err := ospfv3packet.DecodeExtendedLSABody(v.Body[hdr:])
			if err != nil {
				srMetrics.Load().observeMalformed(interfaceFamilyIPv6, "e-prefix")
				continue
			}
			// RFC 8666 Section 10: "if the length is invalid, the LSA in which it is
			// advertised is considered malformed and MUST be ignored."
			if srV3ExtendedLengthInvalid(ext.TLVs) {
				srMetrics.Load().observeMalformed(interfaceFamilyIPv6, "prefix-sid")
				continue
			}
			for i := range ext.TLVs {
				var pfx netip.Prefix
				pfx, sids = v6PrefixSIDsFromTLV(ext.TLVs[i], sids[:0])
				for _, ps := range sids {
					out = append(out, v6ReceivedPrefixSID{
						Prefix:     pfx,
						SID:        ps,
						Originator: v.AdvertisingRouter,
						Area:       v.Area,
						LSType:     lt,
					})
				}
			}
		}
	}
	return out
}

// v6PrefixSIDsFromTLV appends to sids every Prefix-SID one top-level Extended-LSA TLV
// carries and returns the TLV's prefix, honoring both the Intra/Inter/External Prefix TLV
// (nested Prefix-SID sub-TLVs) and the Extended Prefix Range TLV (starting Prefix-SIDs).
// A TLV that carries no Prefix-SID, or is malformed, appends nothing and never panics.
// A Prefix-SID sub-TLV MAY appear more than once, one per algorithm, so every one is kept
// for the duplicate judgement (srPrefixSIDTable) rather than the first one winning.
func v6PrefixSIDsFromTLV(tlv ospfv3packet.ExtendedTLV, sids []sr.PrefixSID) (netip.Prefix, []sr.PrefixSID) {
	switch tlv.Type {
	case extTLVExtPrefixRange:
		rng, err := sr.DecodeExtPrefixRangeValueV6(tlv.Value)
		if err != nil {
			return netip.Prefix{}, sids
		}
		if rng.AF != 1 {
			return netip.Prefix{}, sids // RFC 8666 Section 5: AF 1 is IPv6 unicast
		}
		pfx, ok := v6PrefixFromWords(rng.PrefixLength, rng.AddressV6)
		if !ok {
			return netip.Prefix{}, sids
		}
		return pfx, append(sids, rng.PrefixSIDs...)
	case extTLVIntraAreaPrefix, extTLVInterAreaPrefix, extTLVExternalPrefix:
		return v6PrefixSIDsFromPrefixTLV(tlv.Value, sids)
	default:
		return netip.Prefix{}, sids
	}
}

// v6PrefixSIDsFromPrefixTLV parses an RFC 8362 §3.11 Intra/Inter/External Prefix TLV value:
// Metric(4) PrefixLength(1) PrefixOptions(1) Reserved(2) AddressPrefix(words) Sub-TLVs, and
// appends every Prefix-SID sub-TLV (type 4) to sids. Bound-checked throughout. A Prefix-SID
// with an invalid V/L-Flag combination is dropped alone (RFC 8666 Section 6: "any SID
// Advertisement received with an invalid setting for V- and L-Flags MUST be ignored").
func v6PrefixSIDsFromPrefixTLV(value []byte, sids []sr.PrefixSID) (netip.Prefix, []sr.PrefixSID) {
	if len(value) < 8 {
		return netip.Prefix{}, sids
	}
	plen := value[4]
	words := v6PrefixTLVWordBytes(plen)
	if len(value) < 8+words {
		return netip.Prefix{}, sids
	}
	pfx, ok := v6PrefixFromWords(plen, value[8:8+words])
	if !ok {
		return netip.Prefix{}, sids
	}
	for _, sub := range v6PrefixTLVSubTLVs(value) {
		if sub.Type != sr.V6TypePrefixSID {
			continue
		}
		ps, err := sr.DecodePrefixSIDValueV6(sub.Value)
		if err != nil {
			continue
		}
		sids = append(sids, ps)
	}
	return pfx, sids
}

// v6PrefixTLVSubTLVs returns the sub-TLVs of an Intra/Inter/External Prefix TLV value, or
// nil when the fixed part or the sub-TLV framing is malformed.
func v6PrefixTLVSubTLVs(value []byte) []ospfv3packet.ExtendedTLV {
	if len(value) < 8 {
		return nil
	}
	words := v6PrefixTLVWordBytes(value[4])
	if len(value) < 8+words {
		return nil
	}
	subs, err := ospfv3packet.SubTLVsAt(value, 8+words)
	if err != nil {
		return nil
	}
	return subs
}

// v6PrefixFromWords reconstructs an IPv6 netip.Prefix from a padded ((PrefixLength+31)/32)
// 32-bit-word address field (RFC 5340 App A.4.1). It rejects a word count that would exceed
// a 128-bit address.
func v6PrefixFromWords(prefixLen uint8, words []byte) (netip.Prefix, bool) {
	if prefixLen > 128 {
		return netip.Prefix{}, false
	}
	var addr [16]byte
	n := min(len(words), 16)
	copy(addr[:n], words[:n])
	return netip.PrefixFrom(netip.AddrFrom16(addr), int(prefixLen)), true
}

// srRemotePrefixSIDsV6 aggregates the received IPv6 Prefix-SIDs into the shared install
// map keyed by prefix. The duplicate judgement is per prefix, topology and algorithm,
// and per advertising router in each area (srPrefixSIDScope.resolve, RFC 8666 Section 6).
func (e *engine) srRemotePrefixSIDsV6() map[netip.Prefix]srRemotePrefixSID {
	table := newSRPrefixSIDTable()
	for _, r := range e.v6ReceivedPrefixSIDs() {
		table.add(srRemotePrefixSID{Originator: r.Originator, SID: r.SID}, r.Prefix, r.Area)
	}
	return table.byPrefix()
}
