// Design: docs/architecture/config/syntax.md — BGP route type conversion
// Overview: loader.go — reactor loading and creation
// Related: peers.go — peer extraction that calls these converters

package bgpconfig

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// FlowSpec action names.
const flowSpecRedirectNextHop = "redirect-to-nexthop"

// flowSpecConfigToPlugin converts a legacy flow{} route config into a generic
// PluginRouteConfig by routing it through the bgp-nlri-flowspec plugin's config
// route parser -- the same path used for the native update{} nlri form. This
// keeps all FlowSpec wire building (NLRI + actions) inside the plugin.
func flowSpecConfigToPlugin(fr FlowSpecRouteConfig) (PluginRouteConfig, error) {
	famName := "ipv4/flow"
	switch {
	case fr.IsIPv6 && fr.RD != "":
		famName = "ipv6/flow-vpn"
	case fr.IsIPv6:
		famName = "ipv6/flow"
	case fr.RD != "":
		famName = "ipv4/flow-vpn"
	}
	parser := registry.ConfigRouteParserByFamily(famName)
	if parser == nil {
		return PluginRouteConfig{}, fmt.Errorf("no config route parser for %s", famName)
	}

	// Reconstruct the NLRI content tokens the plugin parser expects.
	content := make([]string, 0, 4+2*len(fr.NLRI))
	if fr.RD != "" {
		content = append(content, "rd", fr.RD)
	}
	content = append(content, "add")
	for key, vals := range fr.NLRI {
		switch len(vals) {
		case 0:
		case 1:
			content = append(content, key, vals[0])
		default:
			content = append(content, key, "[")
			content = append(content, vals...)
			content = append(content, "]")
		}
	}

	// Pre-parse the generic attributes from the route config strings.
	var extComm []byte
	if fr.ExtendedCommunity != "" {
		ec, err := ParseExtendedCommunity(fr.ExtendedCommunity)
		if err != nil {
			return PluginRouteConfig{}, fmt.Errorf("parse extended-community: %w", err)
		}
		extComm = ec.Bytes
	}
	var community []uint32
	if fr.Community != "" {
		comm, err := ParseCommunity(fr.Community)
		if err != nil {
			return PluginRouteConfig{}, fmt.Errorf("parse community: %w", err)
		}
		community = comm.Values
	}
	ipv6Ext := buildIPv6ExtCommunityFromString(fr.ExtendedCommunity)
	if fr.Attribute != "" {
		rawAttr, err := ParseRawAttribute(fr.Attribute)
		if err != nil {
			return PluginRouteConfig{}, fmt.Errorf("parse raw attribute: %w", err)
		}
		if rawAttr.Code == 25 { // IPv6 Extended Communities (RFC 5701)
			ipv6Ext = append(ipv6Ext, rawAttr.Value...)
		}
	}

	pr, err := parser(registry.ConfigRouteRequest{
		Content:          content,
		NextHop:          fr.NextHop,
		IsIPv6:           fr.IsIPv6,
		ExtCommunity:     extComm,
		IPv6ExtCommunity: ipv6Ext,
		Community:        community,
	})
	if err != nil {
		return PluginRouteConfig{}, fmt.Errorf("%s: %w", famName, err)
	}
	return pluginRouteFromRegistry(famName, pr), nil
}

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

// buildIPv6ExtCommunityFromString builds IPv6 Extended Communities (attribute 25, RFC 5701)
// from an extended community string. Only extracts redirect-to-nexthop with IPv6 addresses.
// RFC 7674 Section 3.2 defines the Redirect to IPv6 action (subtype 0x000c).
func buildIPv6ExtCommunityFromString(ec string) []byte {
	var result []byte
	parts := strings.Fields(ec)

	for i := 0; i < len(parts); i++ {
		if parts[i] == flowSpecRedirectNextHop && i+1 < len(parts) {
			// Check if next part is an IPv6 address
			if ip, err := netip.ParseAddr(parts[i+1]); err == nil && ip.Is6() {
				// RFC 5701: IPv6 Extended Community = subtype(2) + IPv6(16) + copy_flag(2) = 20 bytes
				ipBytes := ip.As16()
				result = append(result, 0x00, 0x0c) // Subtype 0x000c = redirect to IP
				result = append(result, ipBytes[:]...)
				result = append(result, 0x00, 0x00) // Copy flag = 0
			}
			i++ // Skip the IP address part
		}
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
