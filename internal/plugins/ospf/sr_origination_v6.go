// Design: docs/architecture/wire/ospfv3.md -- OSPF Segment Routing IPv6 origination.
// When SR is enabled on an OSPFv3 (IPv6) engine, this file originates the SR-bearing
// RFC 8362 Extended LSAs off the shared v6OriginateSelf seam: the E-Router-LSA (0x2021)
// carrying an Adj-SID/LAN-Adj-SID sub-TLV per adjacency under each Router-Link TLV, and
// the E-Intra-Area-Prefix-LSA (0x2029) carrying a Prefix-SID sub-TLV under an Intra-Area
// Prefix TLV for each configured node prefix (typically the IPv6 loopback). The RFC 8362
// TLV framing comes from v3/packet/lsa_extended.go; the SR sub-TLV VALUE codecs from
// sr/codec_v6.go. Inter-area propagation (AC-15) lives in sr_interarea_v6.go. The E-LSA
// types are in v6ManagedSelfTypes so the stale-flush refreshes/withdraws them like every
// other self-LSA (SR off -> nothing originated -> the flush MaxAges any residue).
// RFC: rfc/short/rfc8666.md (§6 Prefix-SID, §7 Adj-SID); RFC 8362 §3 (Extended-LSA TLVs)

package ospf

import (
	"encoding/binary"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// RFC 8362 §7.1 OSPFv3 Extended-LSA top-level TLV type codes the SR subset rides on.
// These are the Extended-LSA registry values, NOT SR sub-TLV codes (those live in
// sr/codec_v6.go). Kept in the SR consumer so no SR/RFC-8362 spelling leaks into the
// base v3 codec beyond the generic TLV framing.
const (
	extTLVRouterLink      uint16 = 1 // Router-Link TLV (E-Router-LSA)
	extTLVInterAreaPrefix uint16 = 3 // Inter-Area-Prefix TLV (E-Inter-Area-Prefix-LSA)
	extTLVExternalPrefix  uint16 = 5 // External-Prefix TLV (E-AS-External / E-Type-7 LSA)
	extTLVIntraAreaPrefix uint16 = 6 // Intra-Area-Prefix TLV (E-Intra-Area-Prefix-LSA)
	// extTLVExtPrefixRange is the RFC 8666 §5 Extended Prefix Range TLV (type 9); its
	// value codec is sr.EncodeExtPrefixRangeValueV6 / sr.DecodeExtPrefixRangeValueV6.
	extTLVExtPrefixRange = sr.V6TypeExtPrefixRange
)

// eRouterHeaderLen is the E-Router / E-Network-LSA fixed prefix (RouterLSA flags +
// 24-bit options) before the TLV stream (RFC 8362 §3.2). SR carries no routing in it,
// so it is zeroed: the base Router-LSA (0x2001) remains the sole SPF vertex (AC-22).
const eRouterHeaderLen = 4

// eIntraPrefixHeaderLen is the E-Intra-Area-Prefix-LSA fixed prefix before the TLV
// stream (RFC 8362 §3.5): Reserved(2) + Referenced LS Type(2) + Referenced Link State
// ID(4) + Referenced Advertising Router(4).
const eIntraPrefixHeaderLen = 12

// v6ERouterKey is the LSDB key for this router's OSPFv3 E-Router-LSA (Link State ID 0,
// the single fragment; RFC 8362 §3.2 mirrors the base Router-LSA).
func v6ERouterKey(router types.RouterID) types.LSAKey {
	return types.LSAKey{Type: types.LSType(ospfv3types.LSTypeERouter), AdvertisingRouter: router}
}

// v6EIntraAreaPrefixKey is the LSDB key for this router's E-Intra-Area-Prefix-LSA.
func v6EIntraAreaPrefixKey(router types.RouterID) types.LSAKey {
	return types.LSAKey{Type: types.LSType(ospfv3types.LSTypeEIntraAreaPrefix), AdvertisingRouter: router}
}

// v6OriginateSR (re)originates this router's SR-bearing Extended LSAs for every active
// area, adding each key to keep so v6OriginateSelf's stale flush retains them while SR is
// enabled and MaxAge-purges them when SR is disabled or a SID goes away (AC-13/AC-20). It
// returns the number of LSAs (re)originated. SR disabled originates nothing; the caller's
// flush then withdraws any residue.
func (e *engine) v6OriginateSR(router types.RouterID, byArea map[types.AreaID][]ospflsdb.InterfaceInfo, abr bool, activeAreas []types.AreaID, keep map[ospflsdb.SelfLSARef]struct{}) int {
	if e.lsdb == nil {
		return 0
	}
	// Read the IPv6 family's own SR config: when both address families configured SR the
	// shared config is the IPv4 block, whose Prefixes are IPv4-only, so the IPv6 family
	// must consult its own override to originate its IPv6 node Prefix-SIDs (RFC 8666 §6).
	cfg, ok := srWire.getAF(router, true)
	if !ok || !cfg.Enabled {
		return 0
	}
	nodeSIDs := v6NodePrefixSIDs(cfg)
	count := 0
	for area, ifaces := range byArea {
		if body, has := e.v6BuildERouterBody(ifaces); has {
			key := v6ERouterKey(router)
			if _, orig := e.lsdb.OriginateSelf(area, key, body, v6SelfExtEncoder(ospfv3types.LSTypeERouter, ospfv3types.LinkStateID{}, router, body)); orig {
				count++
			}
			keep[ospflsdb.SelfLSARef{Area: area, Key: key}] = struct{}{}
		}
		if len(nodeSIDs) > 0 {
			body := v6EIntraAreaPrefixBody(router, nodeSIDs)
			key := v6EIntraAreaPrefixKey(router)
			if _, orig := e.lsdb.OriginateSelf(area, key, body, v6SelfExtEncoder(ospfv3types.LSTypeEIntraAreaPrefix, ospfv3types.LinkStateID{}, router, body)); orig {
				count++
			}
			keep[ospflsdb.SelfLSARef{Area: area, Key: key}] = struct{}{}
		}
	}
	if abr {
		count += e.v6OriginateInterAreaSR(router, activeAreas, nodeSIDs, keep)
	}
	return count
}

// v6NodePrefixSIDs returns the configured node Prefix-SIDs whose prefix is an IPv6
// prefix (the IPv6 family advertises only its IPv6 prefixes; the shared srWire store may
// also hold IPv4 prefix-SIDs when the operator configured both families).
func v6NodePrefixSIDs(cfg sr.SRConfig) []sr.PrefixSIDConfig {
	var out []sr.PrefixSIDConfig
	for _, p := range cfg.Prefixes {
		if p.Prefix.Addr().Is6() {
			out = append(out, p)
		}
	}
	return out
}

// v6BuildERouterBody builds the E-Router-LSA body (RFC 8362 §3.2): the 4-byte fixed
// prefix followed by one Router-Link TLV per adjacency that has an allocated Adj-SID,
// each carrying an Adj-SID (type 5) or LAN-Adj-SID (type 6) sub-TLV. It returns false
// when no adjacency carries an Adj-SID, so an empty E-Router-LSA is never originated.
func (e *engine) v6BuildERouterBody(ifaces []ospflsdb.InterfaceInfo) ([]byte, bool) {
	if e.srAdj == nil {
		return nil, false
	}
	var tlvs []ospfv3packet.ExtendedTLV
	for idx := range ifaces {
		iface := &ifaces[idx]
		linkType := v6ERouterLinkType(iface.NetworkType)
		if linkType == 0 {
			continue
		}
		metric := iface.Cost
		if metric == 0 {
			metric = 1
		}
		for _, nbr := range iface.Neighbors {
			if nbr.State != ospflsdb.NeighborStateFull {
				continue
			}
			adj, has := e.srAdj.adjFor(iface.Name, nbr.RouterID)
			if !has {
				continue
			}
			fixed := make([]byte, 16)
			fixed[0] = linkType
			binary.BigEndian.PutUint16(fixed[2:], metric)
			binary.BigEndian.PutUint32(fixed[4:], iface.InterfaceID)
			binary.BigEndian.PutUint32(fixed[8:], nbr.InterfaceID)
			copy(fixed[12:16], nbr.RouterID[:])
			value := ospfv3packet.AppendSubTLVs(fixed, []ospfv3packet.ExtendedTLV{v6AdjSubTLV(adj)})
			tlvs = append(tlvs, ospfv3packet.ExtendedTLV{Type: extTLVRouterLink, Value: value})
		}
	}
	if len(tlvs) == 0 {
		return nil, false
	}
	ext := ospfv3packet.EncodeExtendedLSABody(ospfv3packet.ExtendedLSA{TLVs: tlvs})
	body := make([]byte, eRouterHeaderLen+len(ext))
	copy(body[eRouterHeaderLen:], ext)
	return body, true
}

// v6ERouterLinkType maps an OSPF network type to the RFC 8362 Router-Link TLV link type.
// SR advertises Adj-SIDs on point-to-point and broadcast/NBMA (transit) links; it returns
// 0 for a link type SR does not advertise an Adj-SID on.
func v6ERouterLinkType(networkType string) byte {
	switch networkType {
	case ospflsdb.NetworkPointToPoint, ospflsdb.NetworkPointToMultipoint:
		return ospfv3packet.RouterLinkTypeP2P
	case ospflsdb.NetworkBroadcast, ospflsdb.NetworkNBMA:
		return ospfv3packet.RouterLinkTypeTransit
	default:
		return 0
	}
}

// v6AdjSubTLV frames an Adj-SID (RFC 8666 §7.1, type 5) or LAN-Adj-SID (§7.2, type 6)
// sub-TLV for one adjacency.
func v6AdjSubTLV(a sr.AdjSID) ospfv3packet.ExtendedTLV {
	if a.IsLAN {
		return ospfv3packet.ExtendedTLV{Type: sr.V6TypeLANAdjSID, Value: sr.EncodeLANAdjSIDValueV6(a)}
	}
	return ospfv3packet.ExtendedTLV{Type: sr.V6TypeAdjSID, Value: sr.EncodeAdjSIDValueV6(a)}
}

// v6EIntraAreaPrefixBody builds the E-Intra-Area-Prefix-LSA body (RFC 8362 §3.5): the
// 12-byte referenced header (pointing at this router's Router-LSA) followed by one
// Intra-Area-Prefix TLV per configured node Prefix-SID.
func v6EIntraAreaPrefixBody(router types.RouterID, sids []sr.PrefixSIDConfig) []byte {
	tlvs := make([]ospfv3packet.ExtendedTLV, 0, len(sids))
	for _, p := range sids {
		if tlv, ok := v6IntraAreaPrefixTLV(p); ok {
			tlvs = append(tlvs, tlv)
		}
	}
	ext := ospfv3packet.EncodeExtendedLSABody(ospfv3packet.ExtendedLSA{TLVs: tlvs})
	body := make([]byte, eIntraPrefixHeaderLen+len(ext))
	// [0:2] reserved zero; [2:4] Referenced LS Type = Router-LSA; [4:8] Referenced Link
	// State ID zero (the router fragment); [8:12] Referenced Advertising Router.
	binary.BigEndian.PutUint16(body[2:], uint16(ospfv3types.LSTypeRouter))
	copy(body[8:12], router[:])
	copy(body[eIntraPrefixHeaderLen:], ext)
	return body
}

// v6IntraAreaPrefixTLV builds one RFC 8362 §3.11 Intra-Area-Prefix TLV (type 6) carrying
// an IPv6 prefix and a nested Prefix-SID sub-TLV (RFC 8666 §6). The prefix is padded to
// ((PrefixLength+31)/32) 32-bit words. NP/E come from the node config (a directly-attached
// loopback keeps the configured flags). It returns false for a non-IPv6 prefix.
func v6IntraAreaPrefixTLV(p sr.PrefixSIDConfig) (ospfv3packet.ExtendedTLV, bool) {
	if !p.Prefix.Addr().Is6() {
		return ospfv3packet.ExtendedTLV{}, false
	}
	plen := uint8(p.Prefix.Bits())
	words := v6PrefixTLVWordBytes(plen)
	fixed := make([]byte, 8+words)
	// Metric(4) zero for a host prefix; PrefixLength(1); PrefixOptions(1); Reserved(2).
	fixed[4] = plen
	addr := p.Prefix.Addr().As16()
	copy(fixed[8:8+words], addr[:min(words, len(addr))])
	sid := sr.PrefixSID{Flags: sr.SIDFlags{NP: p.NoPHP, E: p.ExplicitNull}, Algorithm: 0, Index: p.Index}
	sub := ospfv3packet.ExtendedTLV{Type: sr.V6TypePrefixSID, Value: sr.EncodePrefixSIDValueV6(sid)}
	value := ospfv3packet.AppendSubTLVs(fixed, []ospfv3packet.ExtendedTLV{sub})
	return ospfv3packet.ExtendedTLV{Type: extTLVIntraAreaPrefix, Value: value}, true
}

// v6PrefixTLVWordBytes returns the address-prefix byte count for a prefix length:
// ((PrefixLength+31)/32) 32-bit words (RFC 8362 §3.11 / RFC 5340 App A.4.1).
func v6PrefixTLVWordBytes(prefixLen uint8) int { return ((int(prefixLen) + 31) / 32) * 4 }

// v6SelfExtEncoder returns the SelfLSAEncoder for a raw-body OSPFv3 Extended LSA: the
// header carries the scope-encoded LS Type and the body rides verbatim (no typed body),
// so LSA.WriteTo copies it and finalizes Length + Fletcher checksum.
func v6SelfExtEncoder(lsType ospfv3types.LSType, lsid ospfv3types.LinkStateID, router types.RouterID, body []byte) ospflsdb.SelfLSAEncoder {
	return func(seq types.LSSequenceNumber, purge bool) packet.LSA {
		return v6SelfLSA(ospfv3packet.LSA{
			Header: v6OriginHeader(lsType, lsid, router, seq, purge),
			Body:   body,
		})
	}
}
