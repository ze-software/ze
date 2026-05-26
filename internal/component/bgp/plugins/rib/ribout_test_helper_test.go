// Design: docs/architecture/pool-architecture.md — test helpers for ribOut compact storage

package rib

import (
	"encoding/binary"
	"net"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/pool"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// testRibOutEntry creates a ribOutEntry with wire bytes interned from Route fields.
// Used in tests that pre-populate ribOut maps.
func testRibOutEntry(route *Route) ribOutEntry {
	wireBytes := packTestRouteAttrs(route)
	handle, _ := pool.RibOut.Intern(wireBytes)
	return ribOutEntry{
		MsgID:      route.MsgID,
		AttrHandle: handle,
		StaleLevel: route.StaleLevel,
	}
}

// testRibOutMap builds a ribOut family map from Route values.
func testRibOutMap(routes map[string]*Route) map[string]ribOutEntry {
	result := make(map[string]ribOutEntry, len(routes))
	for key, rt := range routes {
		result[key] = testRibOutEntry(rt)
	}
	return result
}

// testRibOutFamilyMap builds a ribOut peer map from family->key->Route values.
func testRibOutFamilyMap(routes map[family.Family]map[string]*Route) map[family.Family]map[string]ribOutEntry {
	result := make(map[family.Family]map[string]ribOutEntry, len(routes))
	for fam, familyRoutes := range routes {
		result[fam] = testRibOutMap(familyRoutes)
	}
	return result
}

// packTestRouteAttrs builds minimal wire attribute bytes from a Route for testing.
func packTestRouteAttrs(route *Route) []byte {
	var buf []byte

	if route.Origin != nil {
		buf = appendAttr(buf, byte(attribute.AttrOrigin), 0x40, []byte{byte(*route.Origin)})
	}

	if len(route.ASPath) > 0 {
		val := make([]byte, 2+4*len(route.ASPath))
		val[0] = 2 // AS_SEQUENCE
		val[1] = byte(len(route.ASPath))
		for i, asn := range route.ASPath {
			binary.BigEndian.PutUint32(val[2+4*i:], asn)
		}
		buf = appendAttr(buf, byte(attribute.AttrASPath), 0x40, val)
	}

	if route.NextHop != "" {
		ip := net.ParseIP(route.NextHop)
		if ip4 := ip.To4(); ip4 != nil {
			buf = appendAttr(buf, byte(attribute.AttrNextHop), 0x40, ip4)
		}
	}

	if route.MED != nil {
		val := make([]byte, 4)
		binary.BigEndian.PutUint32(val, *route.MED)
		buf = appendAttr(buf, byte(attribute.AttrMED), 0x80, val)
	}

	if route.LocalPreference != nil {
		val := make([]byte, 4)
		binary.BigEndian.PutUint32(val, *route.LocalPreference)
		buf = appendAttr(buf, byte(attribute.AttrLocalPref), 0x40, val)
	}

	if len(route.Communities) > 0 {
		val := make([]byte, 4*len(route.Communities))
		for i, c := range route.Communities {
			binary.BigEndian.PutUint32(val[4*i:], uint32(c))
		}
		buf = appendAttr(buf, byte(attribute.AttrCommunity), 0xC0, val)
	}

	if len(route.LargeCommunities) > 0 {
		val := make([]byte, 12*len(route.LargeCommunities))
		for i, lc := range route.LargeCommunities {
			binary.BigEndian.PutUint32(val[12*i:], lc.GlobalAdmin)
			binary.BigEndian.PutUint32(val[12*i+4:], lc.LocalData1)
			binary.BigEndian.PutUint32(val[12*i+8:], lc.LocalData2)
		}
		buf = appendAttr(buf, byte(attribute.AttrLargeCommunity), 0xC0, val)
	}

	if len(route.ExtendedCommunities) > 0 {
		val := make([]byte, 8*len(route.ExtendedCommunities))
		for i, ec := range route.ExtendedCommunities {
			copy(val[8*i:], ec[:])
		}
		buf = appendAttr(buf, byte(attribute.AttrExtCommunity), 0xC0, val)
	}

	if len(buf) == 0 {
		buf = appendAttr(buf, byte(attribute.AttrOrigin), 0x40, []byte{0})
	}

	return buf
}
