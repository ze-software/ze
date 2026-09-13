// Design: docs/architecture/config/syntax.md — BGP route type conversion
// Overview: loader.go — reactor loading and creation
// Related: peers.go — peer extraction that calls these converters

package bgpconfig

import (
	"fmt"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
)

// FlowSpec action names.
const flowSpecRedirectNextHop = "redirect-to-nexthop"

// sortExtCommunities sorts extended communities by type for RFC 4360 compliance.
// Each extended community is 8 bytes. Sorting by the 64-bit value puts lower
// type codes first (e.g., origin 0x0003 before redirect 0x8008).
// Trailing bytes that don't form a complete community are discarded.
func sortExtCommunities(data []byte) []byte {
	if len(data) < 16 { // Need at least 2 communities to sort
		return data
	}

	// Validate and truncate to complete communities only
	count := len(data) / 8
	if count*8 != len(data) {
		// Discard trailing bytes that don't form a complete community
		data = data[:count*8]
	}
	communities := make([]uint64, count)
	for i := range count {
		offset := i * 8
		communities[i] = uint64(data[offset])<<56 |
			uint64(data[offset+1])<<48 |
			uint64(data[offset+2])<<40 |
			uint64(data[offset+3])<<32 |
			uint64(data[offset+4])<<24 |
			uint64(data[offset+5])<<16 |
			uint64(data[offset+6])<<8 |
			uint64(data[offset+7])
	}

	// Sort by value (lower type codes first)
	slices.Sort(communities)

	// Rebuild byte slice
	result := make([]byte, len(data))
	for i, c := range communities {
		offset := i * 8
		result[offset] = byte(c >> 56)
		result[offset+1] = byte(c >> 48)
		result[offset+2] = byte(c >> 40)
		result[offset+3] = byte(c >> 32)
		result[offset+4] = byte(c >> 24)
		result[offset+5] = byte(c >> 16)
		result[offset+6] = byte(c >> 8)
		result[offset+7] = byte(c)
	}
	return result
}

// convertPluginRoute converts a PluginRouteConfig to a reactor PluginRoute.
func convertPluginRoute(pr PluginRouteConfig) (reactor.PluginRoute, error) {
	route := reactor.PluginRoute{
		Family:          pr.Family,
		IsIPv6:          pr.IsIPv6,
		NLRI:            pr.NLRI,
		ASPath:          pr.ASPath,
		LocalPreference: pr.LocalPreference,
		Group:           pr.Group,
		MapV4NextHop:    pr.MapV4NextHop,
	}

	if pr.NextHop != "" {
		ip, err := netip.ParseAddr(pr.NextHop)
		if err != nil {
			return route, fmt.Errorf("parse next-hop: %w", err)
		}
		route.NextHop = ip
	}

	for i := range pr.Attrs {
		a := &pr.Attrs[i]
		raw := buildPluginAttrWire(a.Flags, a.Code, a.Value)
		route.RawAttrs = append(route.RawAttrs, raw)
	}

	return route, nil
}

// buildPluginAttrWire builds the complete wire bytes for a path attribute.
func buildPluginAttrWire(flags, code uint8, value []byte) []byte {
	vlen := len(value)
	if vlen > 255 || (flags&0x10) != 0 {
		buf := make([]byte, 4+vlen)
		buf[0] = flags | 0x10
		buf[1] = code
		buf[2] = byte(vlen >> 8)
		buf[3] = byte(vlen)
		copy(buf[4:], value)
		return buf
	}
	buf := make([]byte, 3+vlen)
	buf[0] = flags
	buf[1] = code
	buf[2] = byte(vlen)
	copy(buf[3:], value)
	return buf
}
