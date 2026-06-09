// Design: docs/architecture/plugin/rib-storage-design.md — NLRI wire format helpers
// Overview: rib.go — RIB plugin core types and event handlers
// Related: rib_commands.go — command handling and JSON responses
// Related: rib_attr_format.go — attribute formatting for show enrichment
// Related: rib_pipeline.go — iterator pipeline for show commands
package rib

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

var (
	errTruncatedAddPathNlri = errors.New("truncated ADD-PATH NLRI")
	errTruncatedNlri        = errors.New("truncated NLRI")
	errTruncatedNlriPrefix  = errors.New("truncated NLRI prefix")
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
			return "", 0, errTruncatedAddPathNlri
		}
		pathID = uint32(wire[0])<<24 | uint32(wire[1])<<16 | uint32(wire[2])<<8 | uint32(wire[3])
		offset = 4
	}

	if offset >= len(wire) {
		return "", 0, errTruncatedNlri
	}

	prefixLen := int(wire[offset])
	prefixBytes := (prefixLen + 7) / 8

	if offset+1+prefixBytes > len(wire) {
		return "", 0, errTruncatedNlriPrefix
	}

	var addr netip.Addr
	if fam.AFI == family.AFIIPv4 {
		var b4 [4]byte
		copy(b4[:], wire[offset+1:offset+1+prefixBytes])
		addr = netip.AddrFrom4(b4)
	} else {
		var b16 [16]byte
		copy(b16[:], wire[offset+1:offset+1+prefixBytes])
		addr = netip.AddrFrom16(b16)
	}

	tb := textbuf.Get()
	defer tb.Release()
	return tb.Addr(addr).Byte('/').Int(int64(prefixLen)).String(), pathID, nil
}

// formatNLRIAsPrefix converts wire NLRI bytes to human-readable prefix string.
// Handles IPv4/IPv6 with and without ADD-PATH path IDs.
// Returns hex encoding for unrecognized families.
func formatNLRIAsPrefix(fam family.Family, nlriBytes []byte, addPath ...bool) string {
	if len(nlriBytes) == 0 {
		return ""
	}
	ap := len(addPath) > 0 && addPath[0]
	prefix, pathID, err := wireToPrefix(fam, nlriBytes, ap)
	if err != nil {
		var tb textbuf.Buffer
		return tb.Reset().Str("hex:").Hex(nlriBytes).String()
	}
	if ap && pathID != 0 {
		var tb textbuf.Buffer
		return tb.Reset().Str(prefix).Str(" [pathID=").Uint32(pathID).Byte(']').String()
	}
	return prefix
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
		var tb textbuf.Buffer
		afi = tb.Reset().Str("afi-").Uint16(uint16(fam.AFI)).String()
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
		var tb textbuf.Buffer
		safi = tb.Reset().Str("safi-").Uint16(uint16(fam.SAFI)).String()
	}

	var tb2 textbuf.Buffer
	return tb2.Str(afi).Byte('/').Str(safi).String()
}

// formatNextHop formats NEXT_HOP attribute bytes as an IP address string.
func formatNextHop(data []byte) string {
	switch len(data) {
	case 4:
		return textbuf.StringAddr(netip.AddrFrom4([4]byte{data[0], data[1], data[2], data[3]}))
	case 16:
		var b16 [16]byte
		copy(b16[:], data)
		return textbuf.StringAddr(netip.AddrFrom16(b16))
	default:
		return textbuf.StringHex(data)
	}
}
