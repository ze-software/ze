// Design: docs/architecture/wire/ospfv3.md -- OSPF Segment Routing IPv6 inter-area
// Prefix-SID propagation (RFC 8666 §8.2). When this router is an ABR, it re-advertises the
// Prefix-SID of each intra-area prefix it learned into the OTHER active areas inside an
// E-Inter-Area-Prefix-LSA (0x2023). The propagated Prefix-SID sets NP and clears E (a
// prefix reached through the ABR is never directly attached to it, so the far egress must
// not PHP at this ABR); a Prefix-SID for a prefix directly attached to the ABR keeps its
// originated flags. The prefix carriage is the Extended Prefix Range TLV (type 9, Range
// Size 1), whose value codec is fully shared with the IPv4 range advertisement.
// RFC: rfc/short/rfc8666.md (§8.2 inter-area propagation, §6 NP/E on propagated prefixes)

package ospf

import (
	"net/netip"
	"sort"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// v6InterAreaPrefixSID is one prefix and its Prefix-SID chosen for inter-area propagation,
// with the source area it must NOT be re-originated back into.
type v6InterAreaPrefixSID struct {
	prefix     netip.Prefix
	sid        sr.PrefixSID
	sourceArea types.AreaID
}

// v6InterAreaPrefixSIDRule applies RFC 8666 §8.2 / §6 to a propagated Prefix-SID: NP is set
// and E cleared unless the prefix is directly attached to this ABR (then the originated
// flags are kept). V/L/index are carried through unchanged.
func v6InterAreaPrefixSIDRule(src sr.PrefixSID, directlyAttached bool) sr.PrefixSID {
	out := src
	if directlyAttached {
		return out
	}
	out.Flags.NP = true
	out.Flags.E = false
	return out
}

// v6EInterAreaPrefixBody builds an E-Inter-Area-Prefix-LSA body (RFC 8362 §3.6: TLVs with
// no fixed prefix) carrying a single-prefix Extended Prefix Range TLV (RFC 8666 §5, Range
// Size 1) with the propagated Prefix-SID.
func v6EInterAreaPrefixBody(prefix netip.Prefix, sid sr.PrefixSID) []byte {
	addr := prefix.Addr().As16()
	rangeVal := sr.EncodeExtPrefixRangeValueV6(uint8(prefix.Bits()), addr[:], 1, sid)
	return ospfv3packet.EncodeExtendedLSABody(ospfv3packet.ExtendedLSA{
		TLVs: []ospfv3packet.ExtendedTLV{{Type: extTLVExtPrefixRange, Value: rangeVal}},
	})
}

// v6OriginateInterAreaSR re-advertises learned intra-area Prefix-SIDs into the other active
// areas (RFC 8666 §8.2). It reads the received intra-area E-prefix LSAs, forces NP set / E
// clear, and originates one E-Inter-Area-Prefix-LSA per (destination area, prefix), keyed by
// a stable per-area sequential Link State ID. Returns the number of LSAs (re)originated.
func (e *engine) v6OriginateInterAreaSR(router types.RouterID, activeAreas []types.AreaID, _ []sr.PrefixSIDConfig, keep map[ospflsdb.SelfLSARef]struct{}) int {
	props := e.v6InterAreaPropagationSet(router)
	if len(props) == 0 {
		return 0
	}
	count := 0
	for _, area := range activeAreas {
		var lsid uint32
		for _, p := range props {
			if p.sourceArea == area {
				continue // never re-originate into the source area
			}
			lsid++
			id := v6SummaryLSID(lsid)
			key := types.LSAKey{Type: types.LSType(ospfv3types.LSTypeEInterAreaPrefix), LinkStateID: id, AdvertisingRouter: router}
			body := v6EInterAreaPrefixBody(p.prefix, p.sid)
			if _, orig := e.lsdb.OriginateSelf(area, key, body, v6SelfExtEncoder(ospfv3types.LSTypeEInterAreaPrefix, ospfv3types.LinkStateID(id), router, body)); orig {
				count++
			}
			keep[ospflsdb.SelfLSARef{Area: area, Key: key}] = struct{}{}
		}
	}
	return count
}

// v6InterAreaPropagationSet gathers the intra-area Prefix-SIDs this ABR propagates, sorted
// by prefix for a stable Link State ID assignment across re-origination (R-9). It excludes
// this router's own advertisements (its node Prefix-SIDs originate intra-area in every area
// already) and applies the NP-set/E-clear propagation rule.
func (e *engine) v6InterAreaPropagationSet(router types.RouterID) []v6InterAreaPrefixSID {
	var props []v6InterAreaPrefixSID
	for _, r := range e.v6ReceivedPrefixSIDs() {
		if r.LSType != ospfv3types.LSTypeEIntraAreaPrefix || r.Originator == router {
			continue
		}
		props = append(props, v6InterAreaPrefixSID{
			prefix:     r.Prefix,
			sid:        v6InterAreaPrefixSIDRule(r.SID, false),
			sourceArea: r.Area,
		})
	}
	sort.Slice(props, func(i, j int) bool { return props[i].prefix.String() < props[j].prefix.String() })
	return props
}
