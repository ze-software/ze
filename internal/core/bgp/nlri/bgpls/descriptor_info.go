// Design: docs/architecture/wire/nlri-bgpls.md — NLRI descriptor TLV walkers for the JSON writers
// RFC: rfc/short/rfc7752.md
// Related: json.go — the AppendJSON writers that call these walkers

package bgpls

import (
	"encoding/binary"
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// jsonKeyRouterID is the IGP Router-ID of a node descriptor (TLV 515).
	jsonKeyRouterID = "router-id"
)

// BGPLSNLRITypeString returns the NLRI type string.
// RFC 7752 Section 3.2, Table 1 defines NLRI types 1-4, RFC 9514 defines type 6.
func BGPLSNLRITypeString(nlriType uint16) string {
	switch nlriType {
	case 1:
		return "bgpls-node"
	case 2:
		return "bgpls-link"
	case 3:
		return "bgpls-prefix-v4"
	case 4:
		return "bgpls-prefix-v6"
	case 6:
		return "bgpls-srv6-sid"
	}
	// RFC 7752: Unknown NLRI types are valid and should be labeled generically.
	// This is forward-compatibility, not silent ignore.
	var b textbuf.Buffer
	return b.Reset().Str("bgpls-type-").Uint16(nlriType).String()
}

// PrefixDescriptorInfo holds parsed prefix descriptor information.
type PrefixDescriptorInfo struct {
	Prefix string
	MTIDs  []any
}

// ParseBGPLSPrefixTLVs parses a Prefix NLRI wire format to extract all fields.
// RFC 7752 Section 3.2.3 defines the Prefix NLRI format.
func ParseBGPLSPrefixTLVs(data []byte, nlriType BGPLSNLRIType) (nodeDescs []any, info PrefixDescriptorInfo) {
	info.MTIDs = []any{}

	// Minimum: Type(2) + Len(2) + ProtoID(1) + Identifier(8) = 13 bytes
	if len(data) < 13 {
		return nodeDescs, info
	}

	offset := 13

	for offset+4 <= len(data) {
		tlvType := binary.BigEndian.Uint16(data[offset : offset+2])
		tlvLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))

		if offset+4+tlvLen > len(data) {
			break
		}

		value := data[offset+4 : offset+4+tlvLen]

		switch tlvType {
		case 256: // Local Node Descriptors
			nodeDescs = parseNodeDescriptorSubTLVs(value)
		case 263: // Multi-Topology ID
			for i := 0; i+2 <= len(value); i += 2 {
				mtID := binary.BigEndian.Uint16(value[i:i+2]) & 0x0FFF
				info.MTIDs = append(info.MTIDs, int(mtID))
			}
		case 265: // IP Reachability Information
			info.Prefix = formatIPReachability(value, nlriType)
		}
		// RFC 7752: Unknown TLVs are skipped (forward-compatibility).

		offset += 4 + tlvLen
	}

	return nodeDescs, info
}

// ParseBGPLSNodeTLVs parses a Node NLRI wire format to extract node descriptors.
// RFC 7752 Section 3.2.1 defines the Node NLRI format.
func ParseBGPLSNodeTLVs(data []byte) []any {
	// Minimum: Type(2) + Len(2) + ProtoID(1) + Identifier(8) = 13 bytes
	if len(data) < 13 {
		return []any{}
	}

	offset := 13

	for offset+4 <= len(data) {
		tlvType := binary.BigEndian.Uint16(data[offset : offset+2])
		tlvLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))

		if offset+4+tlvLen > len(data) {
			break
		}

		value := data[offset+4 : offset+4+tlvLen]

		if tlvType == 256 { // Local Node Descriptors
			return parseNodeDescriptorSubTLVs(value)
		}
		// RFC 7752: Unknown TLVs are skipped (forward-compatibility).

		offset += 4 + tlvLen
	}

	return []any{}
}

// LinkDescriptorInfo holds parsed link descriptor information.
type LinkDescriptorInfo struct {
	IfAddrs    []any // IPv4/IPv6 interface addresses
	NeighAddrs []any // IPv4/IPv6 neighbor addresses
	MTIDs      []any // Multi-topology IDs
	LinkIDs    []any // Link local/remote identifiers
}

// ParseBGPLSLinkTLVs parses a Link NLRI wire format to extract all TLVs.
// RFC 7752 Section 3.2.2 defines the Link NLRI format.
func ParseBGPLSLinkTLVs(data []byte) (localDescs, remoteDescs []any, info LinkDescriptorInfo) {
	info = LinkDescriptorInfo{
		IfAddrs:    []any{},
		NeighAddrs: []any{},
		MTIDs:      []any{},
		LinkIDs:    []any{},
	}

	// Minimum: Type(2) + Len(2) + ProtoID(1) + Identifier(8) = 13 bytes
	if len(data) < 13 {
		return localDescs, remoteDescs, info
	}

	offset := 13

	for offset+4 <= len(data) {
		tlvType := binary.BigEndian.Uint16(data[offset : offset+2])
		tlvLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))

		if offset+4+tlvLen > len(data) {
			break
		}

		value := data[offset+4 : offset+4+tlvLen]

		switch tlvType {
		case 256: // Local Node Descriptors
			localDescs = parseNodeDescriptorSubTLVs(value)
		case 257: // Remote Node Descriptors
			remoteDescs = parseNodeDescriptorSubTLVs(value)
		case 258: // Link Local/Remote Identifiers (8 bytes: local(4) + remote(4))
			if len(value) >= 8 {
				localID := binary.BigEndian.Uint32(value[0:4])
				remoteID := binary.BigEndian.Uint32(value[4:8])
				info.LinkIDs = append(info.LinkIDs, map[string]any{
					"link-local-id":  localID,
					"link-remote-id": remoteID,
				})
			}
		case 259: // IPv4 Interface Address
			if len(value) == 4 {
				info.IfAddrs = append(info.IfAddrs,
					textbuf.StringAddr(netip.AddrFrom4([4]byte(value[:4]))))
			}
		case 260: // IPv4 Neighbor Address
			if len(value) == 4 {
				info.NeighAddrs = append(info.NeighAddrs,
					textbuf.StringAddr(netip.AddrFrom4([4]byte(value[:4]))))
			}
		case 261: // IPv6 Interface Address
			if len(value) == 16 {
				info.IfAddrs = append(info.IfAddrs, FormatIPv6Compressed(value))
			}
		case 262: // IPv6 Neighbor Address
			if len(value) == 16 {
				info.NeighAddrs = append(info.NeighAddrs, FormatIPv6Compressed(value))
			}
		case 263: // Multi-Topology ID
			for i := 0; i+2 <= len(value); i += 2 {
				mtID := binary.BigEndian.Uint16(value[i:i+2]) & 0x0FFF
				info.MTIDs = append(info.MTIDs, int(mtID))
			}
		}
		// RFC 7752: Unknown TLVs are skipped (forward-compatibility).

		offset += 4 + tlvLen
	}

	return localDescs, remoteDescs, info
}

// parseNodeDescriptorSubTLVs parses node descriptor sub-TLVs into array format.
// RFC 7752 Section 3.2.1.4 defines the sub-TLV types.
func parseNodeDescriptorSubTLVs(data []byte) []any {
	var descs []any
	offset := 0

	for offset+4 <= len(data) {
		tlvType := binary.BigEndian.Uint16(data[offset : offset+2])
		tlvLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))

		if offset+4+tlvLen > len(data) {
			break
		}

		value := data[offset+4 : offset+4+tlvLen]

		switch tlvType {
		case 512: // Autonomous System
			if len(value) >= 4 {
				descs = append(descs, map[string]any{
					"autonomous-system": binary.BigEndian.Uint32(value),
				})
			}
		case 513: // BGP-LS Identifier
			if len(value) >= 4 {
				descs = append(descs, map[string]any{
					"bgp-ls-identifier": textbuf.StringUint32(binary.BigEndian.Uint32(value)),
				})
			}
		case 514: // OSPF Area-ID
			if len(value) >= 4 {
				descs = append(descs, map[string]any{
					"ospf-area-id": textbuf.StringAddr(netip.AddrFrom4([4]byte(value[:4]))),
				})
			}
		case 515: // IGP Router-ID
			routerID := formatRouterID(value)
			// Check for pseudonode (7-8 bytes) and add designated-router-id
			switch len(value) {
			case 8:
				// OSPF pseudonode: Router-ID + DR interface
				descs = append(descs, map[string]any{
					jsonKeyRouterID:        textbuf.StringAddr(netip.AddrFrom4([4]byte(value[:4]))),
					"designated-router-id": textbuf.StringAddr(netip.AddrFrom4([4]byte(value[4:8]))),
				})
			case 7:
				// IS-IS pseudonode: System-ID + PSN
				descs = append(descs, map[string]any{
					jsonKeyRouterID: routerID,
					"psn":           textbuf.StringUint8(value[6]),
				})
			// RFC 7752: 4-byte (OSPF) and 6-byte (IS-IS) are standard lengths.
			case 4, 6:
				descs = append(descs, map[string]any{jsonKeyRouterID: routerID})
			}
			// Unknown Router-ID lengths are silently skipped per RFC 7752 forward-compatibility.
		}
		// RFC 7752: Unknown sub-TLVs are skipped (forward-compatibility).

		offset += 4 + tlvLen
	}

	return descs
}

// formatRouterID formats the IGP Router-ID based on its length.
// - 4 bytes: OSPF Router-ID as dotted decimal
// - 6 bytes: IS-IS System ID as hex
// - 7 bytes: IS-IS pseudonode (6-byte System ID + 1-byte PSN)
// - 8 bytes: OSPF pseudonode (4-byte Router-ID + 4-byte interface).
func formatRouterID(id []byte) string {
	switch len(id) {
	case 4:
		return textbuf.StringAddr(netip.AddrFrom4([4]byte(id[:4])))
	case 6:
		return textbuf.StringHex(id)
	case 7:
		return textbuf.StringHex(id)
	case 8:
		var b textbuf.Buffer
		return b.Reset().Addr(netip.AddrFrom4([4]byte(id[:4]))).Byte(',').Addr(netip.AddrFrom4([4]byte(id[4:8]))).String()
	}
	// Unknown length: return hex (forward-compatibility).
	return strings.ToUpper(textbuf.StringHex(id))
}

// formatIPReachability formats the IP Reachability Information TLV.
// RFC 7752 Section 3.2.3.2 defines the format: prefix-length + prefix-bytes.
func formatIPReachability(data []byte, nlriType BGPLSNLRIType) string {
	if len(data) < 1 {
		return ""
	}

	prefixLen := int(data[0])
	byteLen := (prefixLen + 7) / 8

	if len(data) < 1+byteLen {
		return ""
	}

	prefixBytes := data[1 : 1+byteLen]

	if nlriType == BGPLSPrefixV6NLRI {
		addr := make([]byte, 16)
		copy(addr, prefixBytes)
		var b textbuf.Buffer
		return b.Reset().Str(FormatIPv6Compressed(addr)).Byte('/').Int(int64(prefixLen)).String()
	}

	var b4 [4]byte
	copy(b4[:], prefixBytes)
	var b textbuf.Buffer
	return b.Reset().Addr(netip.AddrFrom4(b4)).Byte('/').Int(int64(prefixLen)).String()
}

// FormatIPv6Compressed formats a 16-byte IPv6 address with zero compression.
func FormatIPv6Compressed(addr []byte) string {
	if len(addr) != 16 {
		return strings.ToUpper(textbuf.StringHex(addr))
	}
	ip := netip.AddrFrom16([16]byte(addr))
	return ip.String()
}
