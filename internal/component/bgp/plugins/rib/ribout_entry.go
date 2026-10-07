// Design: docs/architecture/pool-architecture.md — Adj-RIB-Out compact storage
// RFC: rfc/short/rfc4271.md -- AS_PATH / community wire formats reconstructed for replay
// Related: rib_replay.go — collectGroupedRibOutRoutes, collectGroupedRibOutRoutesForFamily
//
// ribOutEntry replaces *Route in ribOut maps. Wire attribute bytes are
// deduplicated in pool.RibOut; full Route is reconstructed on demand for
// replay, show, and refresh operations.

package rib

import (
	"encoding/binary"
	"encoding/hex"
	"net"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// ribOutKey keeps CIDR identities allocation-free and native identities in their
// registered semantic form. Native excludes the separately stored path identifier.
type ribOutKey struct {
	Prefix netip.Prefix
	PathID uint32
	Native string
}

// ribOutEntry is a compact per-peer per-route record for Adj-RIB-Out.
// Wire attribute bytes live in pool.RibOut (shared across peers).
// NativeNLRI owns the exact advertised bytes, including any ADD-PATH identifier;
// it is empty for CIDR entries, whose wire representation is derived on replay.
// SourcePeer is authoritative for this destination, not for other peers carrying
// the same route key. Replay feedback preserves it; a new advertisement replaces it.
type ribOutEntry struct {
	MsgID        uint64
	AttrHandle   attrpool.Handle
	StaleLevel   uint8
	ConfigStatic bool
	AddPath      bool
	NativeNLRI   string
	NextHop      string
	SourcePeer   string
	// SourceMessageID identifies the received generation, independently of the
	// destination's ADD-PATH identifier and this sent event's MsgID.
	SourceMessageID uint64
	SourceID        uint32
	SourceOwner     uint64
	SourcePath      uint32
	SourceAddPath   bool
	LocalOrigin     bool
}

// ribOutRouteKey identifies one framed announcement from the registered splitter.
// Callers MUST pass the complete native NLRI, including ADD-PATH when addPath is true.
func ribOutRouteKey(fam family.Family, raw []byte, addPath bool) (ribOutKey, bool) {
	return ribOutRouteKeyForAction(fam, raw, addPath, false)
}

// ribOutRouteKeyForAction also accepts withdrawal-only compatibility fields.
func ribOutRouteKeyForAction(fam family.Family, raw []byte, addPath, withdraw bool) (ribOutKey, bool) {
	var key ribOutKey
	if addPath {
		if len(raw) < 4 {
			return key, false
		}
		key.PathID = binary.BigEndian.Uint32(raw)
		raw = raw[4:]
	}
	if len(raw) == 0 {
		return key, false
	}
	// Labeled unicast is CIDR in received storage, but its advertised labels
	// belong to the native wire representation retained by Adj-RIB-Out.
	if storage.IsCIDRFamily(fam) && fam.SAFI != family.SAFIMPLSLabel {
		var ok bool
		key.Prefix, ok = nlri.WirePrefixToKey(raw, fam)
		if !ok {
			return key, false
		}
		key.Prefix = key.Prefix.Masked()
		return key, key.Prefix.IsValid()
	}
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	native, err := nlrisplit.GetPrefixKey(fam)(raw, scratch[:], withdraw)
	if err != nil {
		return key, false
	}
	key.Native = string(native)
	return key, true
}

// release decrements the pool reference. Safe to call on zero-value entries.
func (e ribOutEntry) release() {
	if e.AttrHandle.IsValid() {
		_ = pool.RibOut.Release(e.AttrHandle)
	}
}

// reconstructRoute rebuilds a full *Route from the compact entry, the pool,
// and the map keys. Called only on infrequent paths (replay, show, refresh).
func reconstructRoute(entry ribOutEntry, fam family.Family, key ribOutKey) *Route {
	route := &Route{
		MsgID:      entry.MsgID,
		Family:     fam,
		AddPath:    entry.AddPath,
		PathID:     key.PathID,
		StaleLevel: entry.StaleLevel,
		SourcePeer: entry.SourcePeer,
		NextHop:    entry.NextHop,
	}
	if key.Prefix.IsValid() {
		route.Prefix = key.Prefix.String()
	} else {
		route.RawNLRI = hex.EncodeToString([]byte(entry.NativeNLRI))
		route.Prefix = route.RawNLRI
	}

	wireBytes, err := pool.RibOut.Get(entry.AttrHandle)
	if err != nil || len(wireBytes) == 0 {
		return route
	}
	route.RawAttrs = hex.EncodeToString(wireBytes)

	iter := attribute.NewAttrIterator(wireBytes)
	for typeCode, _, value, ok := iter.Next(); ok; typeCode, _, value, ok = iter.Next() {
		//exhaustive:ignore // Project display fields only; RawAttrs retains every attribute for replay.
		switch typeCode {
		case attribute.AttrOrigin:
			if len(value) >= 1 {
				o := attribute.Origin(value[0])
				route.Origin = &o
			}
		case attribute.AttrASPath:
			// Route.ASPath is []uint32 (flat); AS_SET segments are flattened.
			route.ASPath = parseASPathWire(value)
		case attribute.AttrNextHop:
			if len(value) == 4 {
				route.NextHop = net.IP(value).String()
			}
		case attribute.AttrMED:
			if len(value) == 4 {
				v := binary.BigEndian.Uint32(value)
				route.MED = &v
			}
		case attribute.AttrLocalPref:
			if len(value) == 4 {
				v := binary.BigEndian.Uint32(value)
				route.LocalPreference = &v
			}
		case attribute.AttrCommunity:
			route.Communities = parseCommunityWire(value)
		case attribute.AttrExtCommunity:
			route.ExtendedCommunities = parseExtCommunityWire(value)
		case attribute.AttrLargeCommunity:
			route.LargeCommunities = parseLargeCommunityWire(value)
		case attribute.AttrMPReachNLRI:
			if nh := extractNextHopFromMPReach(value); nh != "" {
				route.NextHop = nh
			}
		}
	}
	if entry.NextHop != "" {
		route.NextHop = entry.NextHop
	}

	return route
}

// parseASPathWire extracts ASN sequence from wire AS_PATH value (4-byte ASNs).
// All segment types (AS_SEQUENCE, AS_SET) are flattened into a single slice
// because Route.ASPath is []uint32 and cannot represent segment boundaries.
func parseASPathWire(value []byte) []uint32 {
	if len(value) < 2 {
		return nil
	}
	var result []uint32
	offset := 0
	for offset+2 <= len(value) {
		segLen := int(value[offset+1])
		offset += 2
		for i := 0; i < segLen && offset+4 <= len(value); i++ {
			result = append(result, binary.BigEndian.Uint32(value[offset:]))
			offset += 4
		}
	}
	return result
}

// parseCommunityWire extracts communities from wire bytes (4 bytes each).
func parseCommunityWire(value []byte) []attribute.Community {
	if len(value) < 4 {
		return nil
	}
	result := make([]attribute.Community, 0, len(value)/4)
	for i := 0; i+4 <= len(value); i += 4 {
		result = append(result, attribute.Community(binary.BigEndian.Uint32(value[i:])))
	}
	return result
}

// parseExtCommunityWire extracts extended communities from wire bytes (8 bytes each).
func parseExtCommunityWire(value []byte) []attribute.ExtendedCommunity {
	if len(value) < 8 {
		return nil
	}
	result := make([]attribute.ExtendedCommunity, 0, len(value)/8)
	for i := 0; i+8 <= len(value); i += 8 {
		var ec attribute.ExtendedCommunity
		copy(ec[:], value[i:i+8])
		result = append(result, ec)
	}
	return result
}

// parseLargeCommunityWire extracts large communities from wire bytes (12 bytes each).
func parseLargeCommunityWire(value []byte) []attribute.LargeCommunity {
	if len(value) < 12 {
		return nil
	}
	result := make([]attribute.LargeCommunity, 0, len(value)/12)
	for i := 0; i+12 <= len(value); i += 12 {
		result = append(result, attribute.LargeCommunity{
			GlobalAdmin: binary.BigEndian.Uint32(value[i:]),
			LocalData1:  binary.BigEndian.Uint32(value[i+4:]),
			LocalData2:  binary.BigEndian.Uint32(value[i+8:]),
		})
	}
	return result
}

// extractNextHopFromMPReach extracts next-hop from MP_REACH_NLRI wire value.
// Wire format: AFI(2) + SAFI(1) + NH-Len(1) + NextHop(N) + Reserved(1) + NLRI...
func extractNextHopFromMPReach(value []byte) string {
	if nextHop := wireu.MPReachWire(value).NextHop(); nextHop.IsValid() {
		return nextHop.String()
	}
	return ""
}

// collectRibOutRoutes reconstructs Routes from ribOut entries for a peer+family.
func (r *RIBManager) collectRibOutRoutes(peerAddr netip.Addr, fam family.Family) []*Route {
	familyRoutes := r.ribOut[peerAddr][fam]
	if familyRoutes == nil {
		return nil
	}
	routes := make([]*Route, 0, len(familyRoutes))
	for key, entry := range familyRoutes {
		if !replaySourceEligible(fam, key, entry.SourcePeer) {
			continue
		}
		routes = append(routes, reconstructRoute(entry, fam, key))
	}
	return routes
}

// packEventAttrs builds wire attribute bytes from parsed JSON event fields.
// Used when the event carries parsed fields instead of raw wire bytes.
func packEventAttrs(event *Event, nextHop string) []byte {
	var buf []byte

	if event.Origin != "" {
		var o attribute.Origin
		_ = o.UnmarshalText([]byte(event.Origin))
		buf = appendAttr(buf, byte(attribute.AttrOrigin), 0x40, []byte{byte(o)})
	}

	if len(event.ASPath) > 0 {
		buf = appendASPathAttr(buf, event.ASPath)
	}

	if nextHop != "" {
		if ip := net.ParseIP(nextHop); ip != nil {
			if ip4 := ip.To4(); ip4 != nil {
				buf = appendAttr(buf, byte(attribute.AttrNextHop), 0x40, ip4)
			}
		}
	}

	if event.MED != nil {
		val := make([]byte, 4)
		binary.BigEndian.PutUint32(val, *event.MED)
		buf = appendAttr(buf, byte(attribute.AttrMED), 0x80, val)
	}

	if event.LocalPreference != nil {
		val := make([]byte, 4)
		binary.BigEndian.PutUint32(val, *event.LocalPreference)
		buf = appendAttr(buf, byte(attribute.AttrLocalPref), 0x40, val)
	}

	// Guard on what PARSED, not on what arrived. Each parser drops a string it
	// cannot read and returns the rest, so a slice of strings none of which parse
	// yields an empty value -- and appendAttr writes a well-formed header with
	// length 0 over it. RFC 4271 Section 4.3 defines COMMUNITIES and its siblings
	// as a set of fixed-width members, so a zero-length one is malformed and
	// RFC 7606 makes the peer treat-as-withdraw or send a NOTIFICATION.
	if comms := parseCommunityStrings(event.Communities); len(comms) > 0 {
		val := make([]byte, 4*len(comms))
		for i, c := range comms {
			binary.BigEndian.PutUint32(val[4*i:], uint32(c))
		}
		buf = appendAttr(buf, byte(attribute.AttrCommunity), 0xC0, val)
	}

	if ecs := parseExtCommunityStrings(event.ExtendedCommunities); len(ecs) > 0 {
		val := make([]byte, 8*len(ecs))
		for i, ec := range ecs {
			copy(val[8*i:], ec[:])
		}
		buf = appendAttr(buf, byte(attribute.AttrExtCommunity), 0xC0, val)
	}

	if lcs := parseLargeCommunityStrings(event.LargeCommunities); len(lcs) > 0 {
		val := make([]byte, 12*len(lcs))
		for i, lc := range lcs {
			binary.BigEndian.PutUint32(val[12*i:], lc.GlobalAdmin)
			binary.BigEndian.PutUint32(val[12*i+4:], lc.LocalData1)
			binary.BigEndian.PutUint32(val[12*i+8:], lc.LocalData2)
		}
		buf = appendAttr(buf, byte(attribute.AttrLargeCommunity), 0xC0, val)
	}

	return buf
}

// appendASPathAttr encodes AS_PATH with multi-segment support for >255 ASNs.
// RFC 4271: each AS_SEQUENCE segment holds at most 255 entries.
func appendASPathAttr(buf []byte, asns []uint32) []byte {
	var val []byte
	remaining := asns
	for len(remaining) > 0 {
		segLen := min(len(remaining), 255)
		seg := remaining[:segLen]
		remaining = remaining[segLen:]
		segBytes := make([]byte, 2+4*segLen)
		segBytes[0] = 2 // AS_SEQUENCE
		segBytes[1] = byte(segLen)
		for i, asn := range seg {
			binary.BigEndian.PutUint32(segBytes[2+4*i:], asn)
		}
		val = append(val, segBytes...)
	}
	return appendAttr(buf, byte(attribute.AttrASPath), 0x40, val)
}

// firstAddNextHop returns the NextHop from the first Add operation in an event.
// In a BGP UPDATE, all NLRIs share the same path attributes including next-hop.
func firstAddNextHop(event *Event) string {
	for _, ops := range event.FamilyOps {
		for _, op := range ops {
			if op.Action == routeaction.Add && op.NextHop != "" {
				return op.NextHop
			}
		}
	}
	return ""
}

// appendAttr appends a single BGP path attribute TLV.
func appendAttr(buf []byte, typeCode, flags byte, value []byte) []byte {
	if len(value) > 255 {
		flags |= 0x10 // extended length
		buf = append(buf, flags, typeCode, byte(len(value)>>8), byte(len(value)))
	} else {
		buf = append(buf, flags, typeCode, byte(len(value)))
	}
	return append(buf, value...)
}
