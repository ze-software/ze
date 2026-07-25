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
// Prefix-SID it carries. A malformed body is counted and skipped, never fatal.
func (e *engine) v6ReceivedPrefixSIDs() []v6ReceivedPrefixSID {
	if e.lsdb == nil {
		return nil
	}
	var out []v6ReceivedPrefixSID
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
			for i := range ext.TLVs {
				pfx, ps, ok := v6PrefixSIDFromTLV(ext.TLVs[i])
				if !ok {
					continue
				}
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
	return out
}

// v6PrefixSIDFromTLV extracts the prefix and its Prefix-SID from one top-level Extended-LSA
// TLV, honoring both the Intra/Inter/External Prefix TLV (nested Prefix-SID sub-TLV) and
// the Extended Prefix Range TLV (starting Prefix-SID). A TLV that carries no Prefix-SID, or
// is malformed, returns false without panicking.
func v6PrefixSIDFromTLV(tlv ospfv3packet.ExtendedTLV) (netip.Prefix, sr.PrefixSID, bool) {
	switch tlv.Type {
	case extTLVExtPrefixRange:
		rng, err := sr.DecodeExtPrefixRangeValueV6(tlv.Value)
		if err != nil || rng.AF != 1 || len(rng.PrefixSIDs) == 0 {
			return netip.Prefix{}, sr.PrefixSID{}, false
		}
		pfx, ok := v6PrefixFromWords(rng.PrefixLength, rng.AddressV6)
		if !ok {
			return netip.Prefix{}, sr.PrefixSID{}, false
		}
		return pfx, rng.PrefixSIDs[0], true
	case extTLVIntraAreaPrefix, extTLVInterAreaPrefix, extTLVExternalPrefix:
		return v6PrefixSIDFromPrefixTLV(tlv.Value)
	default:
		return netip.Prefix{}, sr.PrefixSID{}, false
	}
}

// v6PrefixSIDFromPrefixTLV parses an RFC 8362 §3.11 Intra/Inter/External Prefix TLV value:
// Metric(4) PrefixLength(1) PrefixOptions(1) Reserved(2) AddressPrefix(words) Sub-TLVs, and
// returns the prefix plus the first Prefix-SID sub-TLV (type 4). Bound-checked throughout.
func v6PrefixSIDFromPrefixTLV(value []byte) (netip.Prefix, sr.PrefixSID, bool) {
	if len(value) < 8 {
		return netip.Prefix{}, sr.PrefixSID{}, false
	}
	plen := value[4]
	words := v6PrefixTLVWordBytes(plen)
	if len(value) < 8+words {
		return netip.Prefix{}, sr.PrefixSID{}, false
	}
	pfx, ok := v6PrefixFromWords(plen, value[8:8+words])
	if !ok {
		return netip.Prefix{}, sr.PrefixSID{}, false
	}
	subs, err := ospfv3packet.SubTLVsAt(value, 8+words)
	if err != nil {
		return netip.Prefix{}, sr.PrefixSID{}, false
	}
	for i := range subs {
		if subs[i].Type != sr.V6TypePrefixSID {
			continue
		}
		ps, derr := sr.DecodePrefixSIDValueV6(subs[i].Value)
		if derr != nil {
			return netip.Prefix{}, sr.PrefixSID{}, false
		}
		return pfx, ps, true
	}
	return netip.Prefix{}, sr.PrefixSID{}, false
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
// map keyed by prefix. Two advertisements of the same prefix carrying the SAME SID (an ABR
// re-advertising an intra-area Prefix-SID inter-area, RFC 8666 §8.2) are not a conflict;
// two DIFFERENT SIDs for one prefix are a duplicate and all are ignored (RFC 8666 §6).
func (e *engine) srRemotePrefixSIDsV6() map[netip.Prefix]srRemotePrefixSID {
	out := make(map[netip.Prefix]srRemotePrefixSID)
	for _, r := range e.v6ReceivedPrefixSIDs() {
		ex, seen := out[r.Prefix]
		if !seen {
			out[r.Prefix] = srRemotePrefixSID{Originator: r.Originator, SID: r.SID}
			continue
		}
		if ex.Duplicate || v6PrefixSIDEqual(ex.SID, r.SID) {
			continue
		}
		ex.Duplicate = true
		out[r.Prefix] = ex
	}
	return out
}

// v6PrefixSIDEqual reports whether two Prefix-SIDs bind the same SID (index/label +
// algorithm) to a prefix, so a propagated re-advertisement is not mistaken for a conflict.
func v6PrefixSIDEqual(a, b sr.PrefixSID) bool {
	return a.IsLabel == b.IsLabel && a.Index == b.Index && a.Label == b.Label && a.Algorithm == b.Algorithm
}
