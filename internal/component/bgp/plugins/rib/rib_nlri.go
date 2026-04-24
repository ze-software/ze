// Design: docs/architecture/plugin/rib-storage-design.md — NLRI wire format helpers
// Overview: rib.go — RIB plugin core types and event handlers
// Related: rib_commands.go — command handling and JSON responses
// Related: rib_attr_format.go — attribute formatting for show enrichment
// Related: rib_pipeline.go — iterator pipeline for show commands
package rib

import (
	"fmt"
	"net"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// parseFamily converts a family string like "ipv4/unicast" to family.Family.
// Returns false if the format is invalid.
func parseFamily(familyStr string) (family.Family, bool) {
	parts := strings.Split(familyStr, "/")
	if len(parts) != 2 {
		return family.Family{}, false
	}

	var afi family.AFI
	switch parts[0] {
	case "ipv4":
		afi = family.AFIIPv4
	case "ipv6":
		afi = family.AFIIPv6
	case "l2vpn":
		afi = family.AFIL2VPN
	default: // unknown AFI
		return family.Family{}, false
	}

	var safi family.SAFI
	switch parts[1] {
	case "unicast":
		safi = family.SAFIUnicast
	case "multicast":
		safi = family.SAFIMulticast
	case "mpls-vpn":
		safi = family.SAFIVPN
	case "mpls-label":
		safi = family.SAFIMPLSLabel
	case "evpn":
		safi = family.SAFIEVPN
	case "flowspec":
		safi = family.SAFIFlowSpec
	default: // unknown SAFI
		return family.Family{}, false
	}

	return family.Family{AFI: afi, SAFI: safi}, true
}

// isSimplePrefixFamily returns true for families with simple NLRI format.
// Only IPv4/IPv6 unicast and multicast use the standard [prefix-len][prefix-bytes] format.
// Other families (EVPN, VPN, FlowSpec, etc.) have complex NLRI structures.
func isSimplePrefixFamily(fam family.Family) bool {
	// Only unicast and multicast have simple [prefix-len][prefix-bytes] format
	if fam.SAFI != family.SAFIUnicast && fam.SAFI != family.SAFIMulticast {
		return false
	}
	return fam.AFI == family.AFIIPv4 || fam.AFI == family.AFIIPv6
}

// prefixToWire converts a text prefix to wire bytes.
// RFC 4271: NLRI format is [prefix-len:1][prefix-bytes].
// RFC 7911: ADD-PATH prepends [path-id:4].
//
// LIMITATION: Only works for IPv4/IPv6 unicast. Other families have different formats.
func prefixToWire(familyStr, prefix string, pathID uint32, addPath bool) ([]byte, error) {
	fam, ok := parseFamily(familyStr)
	if !ok {
		return nil, fmt.Errorf("unknown family: %s", familyStr)
	}

	_, ipnet, err := net.ParseCIDR(prefix)
	if err != nil {
		return nil, fmt.Errorf("parse prefix: %w", err)
	}

	prefixLen, _ := ipnet.Mask.Size()
	prefixBytes := (prefixLen + 7) / 8

	// Normalize IP based on AFI
	var ip net.IP
	if fam.AFI == family.AFIIPv4 {
		ip = ipnet.IP.To4()
	} else {
		ip = ipnet.IP.To16()
	}
	if ip == nil {
		return nil, fmt.Errorf("IP address mismatch for family %s", familyStr)
	}

	var wire []byte
	if addPath {
		wire = make([]byte, 4+1+prefixBytes)
		wire[0] = byte(pathID >> 24)
		wire[1] = byte(pathID >> 16)
		wire[2] = byte(pathID >> 8)
		wire[3] = byte(pathID)
		wire[4] = byte(prefixLen)
		copy(wire[5:], ip[:prefixBytes])
	} else {
		wire = make([]byte, 1+prefixBytes)
		wire[0] = byte(prefixLen)
		copy(wire[1:], ip[:prefixBytes])
	}

	return wire, nil
}

// wireToPrefix converts wire bytes to a text prefix.
// RFC 4271: NLRI format is [prefix-len:1][prefix-bytes].
// RFC 7911: ADD-PATH prepends [path-id:4].
//
// LIMITATION: Only works for IPv4/IPv6 unicast. Other families have different formats.
func wireToPrefix(fam family.Family, wire []byte, addPath bool) (string, uint32, error) {
	offset := 0
	var pathID uint32

	if addPath {
		if len(wire) < 5 {
			return "", 0, fmt.Errorf("truncated ADD-PATH NLRI")
		}
		pathID = uint32(wire[0])<<24 | uint32(wire[1])<<16 | uint32(wire[2])<<8 | uint32(wire[3])
		offset = 4
	}

	if offset >= len(wire) {
		return "", 0, fmt.Errorf("truncated NLRI")
	}

	prefixLen := int(wire[offset])
	prefixBytes := (prefixLen + 7) / 8

	if offset+1+prefixBytes > len(wire) {
		return "", 0, fmt.Errorf("truncated NLRI prefix")
	}

	// Reconstruct IP
	var ip net.IP
	if fam.AFI == family.AFIIPv4 {
		ip = make(net.IP, 4)
	} else {
		ip = make(net.IP, 16)
	}
	copy(ip, wire[offset+1:offset+1+prefixBytes])

	return fmt.Sprintf("%s/%d", ip.String(), prefixLen), pathID, nil
}

// formatNLRIAsPrefix converts wire NLRI bytes to human-readable prefix string.
// For IPv4: [24][10][0][0] -> "10.0.0.0/24".
// For IPv6: [64][...] -> "2001:db8::/64".
// Returns hex encoding for unrecognized formats.
//
// NOTE: Only handles IPv4/IPv6 unicast without ADD-PATH.
// TODO: ADD-PATH support requires path-id prefix handling.
// TODO: VPN/EVPN/FlowSpec have different NLRI structures.
func formatNLRIAsPrefix(fam family.Family, nlriBytes []byte) string {
	if len(nlriBytes) == 0 {
		return ""
	}

	prefixLen := int(nlriBytes[0])
	prefixBytes := nlriBytes[1:]

	switch fam.AFI { //nolint:exhaustive // Only IPv4/IPv6 have standard prefix format
	case family.AFIIPv4:
		// Pad to 4 bytes
		ip := make([]byte, 4)
		copy(ip, prefixBytes)
		return fmt.Sprintf("%d.%d.%d.%d/%d", ip[0], ip[1], ip[2], ip[3], prefixLen)

	case family.AFIIPv6:
		// Pad to 16 bytes
		ip := make([]byte, 16)
		copy(ip, prefixBytes)
		return fmt.Sprintf("%x:%x:%x:%x:%x:%x:%x:%x/%d",
			uint16(ip[0])<<8|uint16(ip[1]),
			uint16(ip[2])<<8|uint16(ip[3]),
			uint16(ip[4])<<8|uint16(ip[5]),
			uint16(ip[6])<<8|uint16(ip[7]),
			uint16(ip[8])<<8|uint16(ip[9]),
			uint16(ip[10])<<8|uint16(ip[11]),
			uint16(ip[12])<<8|uint16(ip[13]),
			uint16(ip[14])<<8|uint16(ip[15]),
			prefixLen)

	default: // unsupported family - return hex
		return fmt.Sprintf("hex:%x", nlriBytes)
	}
}

// formatFamily converts family.Family to string like "ipv4/unicast".
func formatFamily(fam family.Family) string {
	var afi, safi string

	switch fam.AFI { //nolint:exhaustive // Common families only, default handles rest
	case family.AFIIPv4:
		afi = "ipv4"
	case family.AFIIPv6:
		afi = "ipv6"
	case family.AFIL2VPN:
		afi = "l2vpn"
	case family.AFIBGPLS:
		afi = "bgp-ls"
	default: // numeric fallback for unknown AFI
		afi = fmt.Sprintf("afi-%d", fam.AFI)
	}

	switch fam.SAFI { //nolint:exhaustive // Common families only, default handles rest
	case family.SAFIUnicast:
		safi = "unicast"
	case family.SAFIMulticast:
		safi = "multicast"
	case family.SAFIVPN:
		safi = "mpls-vpn"
	case family.SAFIMPLSLabel:
		safi = "mpls-label"
	case family.SAFIEVPN:
		safi = "evpn"
	case family.SAFIFlowSpec:
		safi = "flowspec"
	case family.SAFIBGPLinkState:
		safi = "bgp-ls"
	default: // numeric fallback for unknown SAFI
		safi = fmt.Sprintf("safi-%d", fam.SAFI)
	}

	return afi + "/" + safi
}

// formatNextHop formats NEXT_HOP attribute bytes as an IP address string.
func formatNextHop(data []byte) string {
	switch len(data) {
	case 4:
		// IPv4.
		return fmt.Sprintf("%d.%d.%d.%d", data[0], data[1], data[2], data[3])
	case 16:
		// IPv6.
		return fmt.Sprintf("%x:%x:%x:%x:%x:%x:%x:%x",
			uint16(data[0])<<8|uint16(data[1]),
			uint16(data[2])<<8|uint16(data[3]),
			uint16(data[4])<<8|uint16(data[5]),
			uint16(data[6])<<8|uint16(data[7]),
			uint16(data[8])<<8|uint16(data[9]),
			uint16(data[10])<<8|uint16(data[11]),
			uint16(data[12])<<8|uint16(data[13]),
			uint16(data[14])<<8|uint16(data[15]))
	default: // unknown length - return hex
		return fmt.Sprintf("%x", data)
	}
}

func formatRouterID(id uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d", id>>24, (id>>16)&0xff, (id>>8)&0xff, id&0xff)
}
