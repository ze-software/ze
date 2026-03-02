// Design: docs/architecture/config/syntax.md — BGP route type conversion
// Overview: loader.go — reactor loading and creation
// Related: peers.go — peer extraction that calls these converters

package config

import (
	"encoding/hex"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/reactor"
)

// FlowSpec action names.
const flowSpecRedirectNextHop = "redirect-to-nexthop"

// convertMVPNRoute converts config MVPN route to reactor MVPN route.
func convertMVPNRoute(mr MVPNRouteConfig) (reactor.MVPNRoute, error) {
	route := reactor.MVPNRoute{
		IsIPv6:          mr.IsIPv6,
		SourceAS:        mr.SourceAS,
		LocalPreference: mr.LocalPreference,
		MED:             mr.MED,
	}

	// Route type
	switch mr.RouteType {
	case "source-ad":
		route.RouteType = 5
	case "shared-join":
		route.RouteType = 6
	case "source-join":
		route.RouteType = 7
	default:
		return route, fmt.Errorf("unknown MVPN route type: %s", mr.RouteType)
	}

	// Origin
	route.Origin = parseOrigin(mr.Origin)

	// Parse RD
	if mr.RD != "" {
		rd, err := ParseRouteDistinguisher(mr.RD)
		if err != nil {
			return route, fmt.Errorf("parse RD: %w", err)
		}
		route.RD = rd.Bytes
	}

	// Parse Source/RP IP
	if mr.Source != "" {
		ip, err := netip.ParseAddr(mr.Source)
		if err != nil {
			return route, fmt.Errorf("parse source: %w", err)
		}
		route.Source = ip
	}

	// Parse Group IP
	if mr.Group != "" {
		ip, err := netip.ParseAddr(mr.Group)
		if err != nil {
			return route, fmt.Errorf("parse group: %w", err)
		}
		route.Group = ip
	}

	// Parse NextHop
	if mr.NextHop != "" {
		ip, err := netip.ParseAddr(mr.NextHop)
		if err != nil {
			return route, fmt.Errorf("parse next-hop: %w", err)
		}
		route.NextHop = ip
	}

	// Parse extended communities
	if mr.ExtendedCommunity != "" {
		ec, err := ParseExtendedCommunity(mr.ExtendedCommunity)
		if err != nil {
			return route, fmt.Errorf("parse extended-community: %w", err)
		}
		route.ExtCommunityBytes = ec.Bytes
	}

	// Parse originator-id (RFC 4456)
	if mr.OriginatorID != "" {
		ip, err := netip.ParseAddr(mr.OriginatorID)
		if err != nil {
			return route, fmt.Errorf("parse originator-id: %w", err)
		}
		if ip.Is4() {
			b := ip.As4()
			route.OriginatorID = uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
		}
	}

	// Parse cluster-list (RFC 4456, space-separated IPs)
	if mr.ClusterList != "" {
		parts := strings.FieldsSeq(mr.ClusterList)
		for p := range parts {
			p = strings.Trim(p, "[]")
			if p == "" {
				continue
			}
			ip, err := netip.ParseAddr(p)
			if err != nil {
				return route, fmt.Errorf("parse cluster-list: %w", err)
			}
			if ip.Is4() {
				b := ip.As4()
				route.ClusterList = append(route.ClusterList, uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]))
			}
		}
	}

	return route, nil
}

// convertVPLSRoute converts config VPLS route to reactor VPLS route.
func convertVPLSRoute(vr VPLSRouteConfig) (reactor.VPLSRoute, error) {
	route := reactor.VPLSRoute{
		Name:            vr.Name,
		Endpoint:        vr.Endpoint,
		Base:            vr.Base,
		Offset:          vr.Offset,
		Size:            vr.Size,
		LocalPreference: vr.LocalPreference,
		MED:             vr.MED,
	}

	// Origin
	route.Origin = parseOrigin(vr.Origin)

	// Parse RD
	if vr.RD != "" {
		rd, err := ParseRouteDistinguisher(vr.RD)
		if err != nil {
			return route, fmt.Errorf("parse RD: %w", err)
		}
		route.RD = rd.Bytes
	}

	// Parse NextHop
	if vr.NextHop != "" {
		ip, err := netip.ParseAddr(vr.NextHop)
		if err != nil {
			return route, fmt.Errorf("parse next-hop: %w", err)
		}
		route.NextHop = ip
	}

	// Parse AS Path
	if vr.ASPath != "" {
		asPath, err := parseASPathSimple(vr.ASPath)
		if err != nil {
			return route, fmt.Errorf("parse as-path: %w", err)
		}
		route.ASPath = asPath
	}

	// Parse communities
	if vr.Community != "" {
		comm, err := ParseCommunity(vr.Community)
		if err != nil {
			return route, fmt.Errorf("parse community: %w", err)
		}
		route.Communities = comm.Values
	}

	// Parse extended communities
	if vr.ExtendedCommunity != "" {
		ec, err := ParseExtendedCommunity(vr.ExtendedCommunity)
		if err != nil {
			return route, fmt.Errorf("parse extended-community: %w", err)
		}
		route.ExtCommunityBytes = sortExtCommunities(ec.Bytes)
	}

	// Parse originator-id
	if vr.OriginatorID != "" {
		ip, err := netip.ParseAddr(vr.OriginatorID)
		if err != nil {
			return route, fmt.Errorf("parse originator-id: %w", err)
		}
		if ip.Is4() {
			b := ip.As4()
			route.OriginatorID = uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
		}
	}

	// Parse cluster-list (space-separated IPs)
	if vr.ClusterList != "" {
		parts := strings.FieldsSeq(vr.ClusterList)
		for p := range parts {
			// Remove brackets
			p = strings.Trim(p, "[]")
			if p == "" {
				continue
			}
			ip, err := netip.ParseAddr(p)
			if err != nil {
				return route, fmt.Errorf("parse cluster-list: %w", err)
			}
			if ip.Is4() {
				b := ip.As4()
				route.ClusterList = append(route.ClusterList, uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]))
			}
		}
	}

	return route, nil
}

// convertFlowSpecRoute converts config FlowSpec route to reactor FlowSpec route.
// RFC 8955 Section 4 defines the FlowSpec NLRI format.
// RFC 8955 Section 7 defines the Traffic Filtering Actions (extended communities).
// RFC 8955 Section 8 defines the FlowSpec VPN variant (SAFI 134) with Route Distinguisher.
func convertFlowSpecRoute(fr FlowSpecRouteConfig) (reactor.FlowSpecRoute, error) {
	route := reactor.FlowSpecRoute{
		Name:   fr.Name,
		IsIPv6: fr.IsIPv6,
	}

	// Parse RD for flow-vpn
	if fr.RD != "" {
		rd, err := ParseRouteDistinguisher(fr.RD)
		if err != nil {
			return route, fmt.Errorf("parse RD: %w", err)
		}
		route.RD = rd.Bytes
	}

	// Parse NextHop
	if fr.NextHop != "" {
		ip, err := netip.ParseAddr(fr.NextHop)
		if err != nil {
			return route, fmt.Errorf("parse next-hop: %w", err)
		}
		route.NextHop = ip
	}

	// Build FlowSpec NLRI from match criteria (RFC 8955 Section 4)
	// For VPN routes, use component bytes (no length prefix - VPN adds its own)
	isVPN := fr.RD != ""
	flowFamily := "ipv4/flow"
	if fr.IsIPv6 {
		flowFamily = "ipv6/flow"
	}
	if builder := registry.ConfigNLRIBuilder(flowFamily); builder != nil {
		route.NLRI = builder(fr.NLRI, fr.IsIPv6, isVPN)
	}

	// Build communities (RFC 1997)
	if c := fr.Community; c != "" {
		comm, err := ParseCommunity(c)
		if err != nil {
			return route, fmt.Errorf("parse community: %w", err)
		}
		// Convert []uint32 to wire bytes (4 bytes each, big-endian)
		for _, v := range comm.Values {
			route.CommunityBytes = append(route.CommunityBytes,
				byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
		}
	}

	// Build extended communities (RFC 8955 Section 7)
	// Actions like discard, rate-limit, redirect are encoded as extended communities
	if ec := fr.ExtendedCommunity; ec != "" {
		extComm, err := ParseExtendedCommunity(ec)
		if err != nil {
			return route, fmt.Errorf("parse extended-community: %w", err)
		}
		route.ExtCommunityBytes = append(route.ExtCommunityBytes, extComm.Bytes...)

		// Build IPv6 Extended Communities (attribute 25) for redirect-to-nexthop with IPv6
		route.IPv6ExtCommunityBytes = buildIPv6ExtCommunityFromString(ec)
	}

	// Sort extended communities by type for RFC 4360 compliance
	route.ExtCommunityBytes = sortExtCommunities(route.ExtCommunityBytes)

	// Handle raw attributes (e.g., attribute 25 for IPv6 Extended Communities)
	if fr.Attribute != "" {
		rawAttr, err := ParseRawAttribute(fr.Attribute)
		if err != nil {
			return route, fmt.Errorf("parse raw attribute: %w", err)
		}
		// Attribute 25 = IPv6 Extended Communities (RFC 5701)
		if rawAttr.Code == 25 {
			route.IPv6ExtCommunityBytes = append(route.IPv6ExtCommunityBytes, rawAttr.Value...)
		}
	}

	return route, nil
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

// convertMUPRoute converts config MUP route to reactor MUP route.
func convertMUPRoute(mr MUPRouteConfig) (reactor.MUPRoute, error) {
	route := reactor.MUPRoute{
		IsIPv6: mr.IsIPv6,
	}

	// Route type
	switch mr.RouteType {
	case "mup-isd":
		route.RouteType = 1
	case "mup-dsd":
		route.RouteType = 2
	case "mup-t1st":
		route.RouteType = 3
	case "mup-t2st":
		route.RouteType = 4
	default:
		return route, fmt.Errorf("unknown MUP route type: %s", mr.RouteType)
	}

	// Parse NextHop
	if mr.NextHop != "" {
		ip, err := netip.ParseAddr(mr.NextHop)
		if err != nil {
			return route, fmt.Errorf("parse next-hop: %w", err)
		}
		route.NextHop = ip
	}

	// Parse extended communities
	if mr.ExtendedCommunity != "" {
		ec, err := ParseExtendedCommunity(mr.ExtendedCommunity)
		if err != nil {
			return route, fmt.Errorf("parse extended-community: %w", err)
		}
		route.ExtCommunityBytes = ec.Bytes
	}

	// Build MUP NLRI via registry (avoids direct plugin import)
	mupFamily := familyIPv4MUP
	if mr.IsIPv6 {
		mupFamily = familyIPv6MUP
	}
	mupArgs := mupRouteConfigToArgs(mr)
	nlriHex, err := registry.EncodeNLRIByFamily(mupFamily, mupArgs)
	if err != nil {
		return route, fmt.Errorf("build MUP NLRI: %w", err)
	}
	nlriBytes, err := hex.DecodeString(nlriHex)
	if err != nil {
		return route, fmt.Errorf("decode MUP NLRI hex: %w", err)
	}
	route.NLRI = nlriBytes

	// Parse SRv6 Prefix-SID if present
	if mr.PrefixSID != "" {
		sid, err := ParsePrefixSIDSRv6(mr.PrefixSID)
		if err != nil {
			return route, fmt.Errorf("parse prefix-sid-srv6: %w", err)
		}
		route.PrefixSID = sid.Bytes
	}

	return route, nil
}

// mupRouteConfigToArgs converts MUPRouteConfig to CLI-style args for registry.EncodeNLRIByFamily.
func mupRouteConfigToArgs(mr MUPRouteConfig) []string {
	var args []string
	if mr.RouteType != "" {
		args = append(args, "route-type", mr.RouteType)
	}
	if mr.RD != "" {
		args = append(args, "rd", mr.RD)
	}
	if mr.Prefix != "" {
		args = append(args, "prefix", mr.Prefix)
	}
	if mr.Address != "" {
		args = append(args, "address", mr.Address)
	}
	if mr.TEID != "" {
		args = append(args, "teid", mr.TEID)
	}
	if mr.QFI != 0 {
		args = append(args, "qfi", strconv.FormatUint(uint64(mr.QFI), 10))
	}
	if mr.Endpoint != "" {
		args = append(args, "endpoint", mr.Endpoint)
	}
	if mr.Source != "" {
		args = append(args, "source", mr.Source)
	}
	return args
}

// parseOrigin converts origin string to code.
// Empty or unset defaults to IGP (0).
func parseOrigin(s string) uint8 {
	switch strings.ToLower(s) {
	case "", originIGP:
		return 0 // IGP is default
	case originEGP:
		return 1
	default:
		return 2 // incomplete
	}
}

// parseASPathSimple parses an AS path string like "[ 30740 30740 ]" to []uint32.
func parseASPathSimple(s string) ([]uint32, error) {
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	result := make([]uint32, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		n, err := strconv.ParseUint(p, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid ASN: %s", p)
		}
		result = append(result, uint32(n))
	}
	return result, nil
}
