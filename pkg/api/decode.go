package api

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"strings"

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/exa-networks/zebgp/pkg/bgp/capability"
	"github.com/exa-networks/zebgp/pkg/bgp/message"
)

// DecodedUpdate holds parsed UPDATE message contents.
type DecodedUpdate struct {
	Announced []ReceivedRoute // Announced routes (NLRI + MP_REACH_NLRI)
	Withdrawn []netip.Prefix  // Withdrawn prefixes (Withdrawn Routes + MP_UNREACH_NLRI)
}

// DecodeUpdate parses raw UPDATE bytes into announced and withdrawn routes.
// Handles both IPv4 unicast NLRI and MP_REACH_NLRI for IPv6.
func DecodeUpdate(body []byte) DecodedUpdate {
	result := DecodedUpdate{}

	if len(body) < 4 {
		return result
	}

	// Parse UPDATE structure: withdrawn_len (2) + withdrawn + attr_len (2) + attrs + nlri
	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	offset := 2

	// Parse withdrawn routes (IPv4)
	if withdrawnLen > 0 && offset+withdrawnLen <= len(body) {
		result.Withdrawn = parseIPv4Prefixes(body[offset : offset+withdrawnLen])
	}
	offset += withdrawnLen

	if offset+2 > len(body) {
		return result
	}

	attrLen := int(binary.BigEndian.Uint16(body[offset : offset+2]))
	offset += 2
	if offset+attrLen > len(body) {
		return result
	}

	pathAttrs := body[offset : offset+attrLen]
	nlriOffset := offset + attrLen
	nlriLen := len(body) - nlriOffset

	// Parse path attributes including MP extensions
	attrs := parsePathAttributes(pathAttrs)

	// Add MP_UNREACH_NLRI withdrawals
	result.Withdrawn = append(result.Withdrawn, attrs.mpWithdrawn...)

	// Parse IPv4 NLRI and add to announced
	if nlriLen > 0 {
		ipv4Routes := parseIPv4NLRI(body[nlriOffset:], attrs)
		result.Announced = append(result.Announced, ipv4Routes...)
	}

	// Add MP_REACH_NLRI announcements
	result.Announced = append(result.Announced, attrs.mpAnnounced...)

	return result
}

// DecodeUpdateRoutes parses raw UPDATE bytes into ReceivedRoute structs.
// This is used for on-demand parsing when format=parsed or format=full.
// For full UPDATE parsing including withdrawals, use DecodeUpdate instead.
func DecodeUpdateRoutes(body []byte) []ReceivedRoute {
	return DecodeUpdate(body).Announced
}

// parsedAttrs holds attributes extracted from UPDATE path attributes.
type parsedAttrs struct {
	origin      string
	localPref   uint32
	med         uint32
	nextHop     netip.Addr
	asPath      []uint32
	mpAnnounced []ReceivedRoute
	mpWithdrawn []netip.Prefix
}

// parsePathAttributes extracts path attributes from UPDATE.
func parsePathAttributes(pathAttrs []byte) parsedAttrs {
	attrs := parsedAttrs{
		origin:    "igp",
		localPref: 100, // Default for iBGP
	}

	for i := 0; i < len(pathAttrs); {
		if i+2 > len(pathAttrs) {
			break
		}
		flags := pathAttrs[i]
		typeCode := pathAttrs[i+1]
		attrLenBytes := 1
		if flags&0x10 != 0 { // Extended length
			attrLenBytes = 2
		}
		if i+2+attrLenBytes > len(pathAttrs) {
			break
		}
		var attrValueLen int
		if attrLenBytes == 1 {
			attrValueLen = int(pathAttrs[i+2])
			i += 3
		} else {
			attrValueLen = int(binary.BigEndian.Uint16(pathAttrs[i+2 : i+4]))
			i += 4
		}
		if i+attrValueLen > len(pathAttrs) {
			break
		}
		attrValue := pathAttrs[i : i+attrValueLen]
		i += attrValueLen

		switch typeCode {
		case 1: // ORIGIN
			if o, err := attribute.ParseOrigin(attrValue); err == nil {
				attrs.origin = o.String()
			}
		case 2: // AS_PATH
			if ap, err := attribute.ParseASPath(attrValue, true); err == nil {
				for _, seg := range ap.Segments {
					attrs.asPath = append(attrs.asPath, seg.ASNs...)
				}
			}
		case 3: // NEXT_HOP
			if nh, err := attribute.ParseNextHop(attrValue); err == nil {
				attrs.nextHop = nh.Addr
			}
		case 4: // MED
			if m, err := attribute.ParseMED(attrValue); err == nil {
				attrs.med = uint32(m)
			}
		case 5: // LOCAL_PREF
			if lp, err := attribute.ParseLocalPref(attrValue); err == nil {
				attrs.localPref = uint32(lp)
			}
		case 14: // MP_REACH_NLRI
			attrs.mpAnnounced = parseMPReachNLRI(attrValue, attrs)
		case 15: // MP_UNREACH_NLRI
			attrs.mpWithdrawn = parseMPUnreachNLRI(attrValue)
		}
	}

	return attrs
}

// parseIPv4Prefixes parses a sequence of IPv4 prefixes (used for withdrawn routes).
func parseIPv4Prefixes(data []byte) []netip.Prefix {
	var prefixes []netip.Prefix
	for i := 0; i < len(data); {
		if i >= len(data) {
			break
		}
		prefixLen := int(data[i])
		i++
		prefixBytes := (prefixLen + 7) / 8
		if i+prefixBytes > len(data) {
			break
		}
		var addrBytes [4]byte
		copy(addrBytes[:], data[i:i+prefixBytes])
		i += prefixBytes

		addr := netip.AddrFrom4(addrBytes)
		prefix := netip.PrefixFrom(addr, prefixLen)
		prefixes = append(prefixes, prefix)
	}
	return prefixes
}

// parseIPv4NLRI parses IPv4 NLRI into ReceivedRoute structs.
func parseIPv4NLRI(data []byte, attrs parsedAttrs) []ReceivedRoute {
	var routes []ReceivedRoute
	for i := 0; i < len(data); {
		if i >= len(data) {
			break
		}
		prefixLen := int(data[i])
		i++
		prefixBytes := (prefixLen + 7) / 8
		if i+prefixBytes > len(data) {
			break
		}
		var addrBytes [4]byte
		copy(addrBytes[:], data[i:i+prefixBytes])
		i += prefixBytes

		addr := netip.AddrFrom4(addrBytes)
		prefix := netip.PrefixFrom(addr, prefixLen)
		routes = append(routes, ReceivedRoute{
			Prefix:          prefix,
			NextHop:         attrs.nextHop,
			Origin:          attrs.origin,
			LocalPreference: attrs.localPref,
			MED:             attrs.med,
			ASPath:          attrs.asPath,
		})
	}
	return routes
}

// parseMPReachNLRI parses MP_REACH_NLRI attribute (RFC 4760).
// Handles IPv6 unicast announcements.
func parseMPReachNLRI(data []byte, attrs parsedAttrs) []ReceivedRoute {
	// MP_REACH_NLRI format:
	// AFI (2) + SAFI (1) + NH Length (1) + Next Hop + Reserved (1) + NLRI
	if len(data) < 5 {
		return nil
	}

	afi := binary.BigEndian.Uint16(data[0:2])
	safi := data[2]
	nhLen := int(data[3])

	if len(data) < 4+nhLen+1 {
		return nil
	}

	// Parse next hop (IPv6: 16 or 32 bytes for link-local)
	var nextHop netip.Addr
	if afi == 2 && nhLen >= 16 { // AFI_IPV6
		var addrBytes [16]byte
		copy(addrBytes[:], data[4:4+16])
		nextHop = netip.AddrFrom16(addrBytes)
	}

	// Skip to NLRI (after next hop + reserved byte)
	nlriOffset := 4 + nhLen + 1
	if nlriOffset >= len(data) {
		return nil
	}
	nlriData := data[nlriOffset:]

	// Only handle IPv6 unicast for now
	if afi != 2 || safi != 1 {
		return nil
	}

	return parseIPv6NLRI(nlriData, nextHop, attrs)
}

// parseMPUnreachNLRI parses MP_UNREACH_NLRI attribute (RFC 4760).
// Handles IPv6 unicast withdrawals.
func parseMPUnreachNLRI(data []byte) []netip.Prefix {
	// MP_UNREACH_NLRI format:
	// AFI (2) + SAFI (1) + Withdrawn Routes
	if len(data) < 3 {
		return nil
	}

	afi := binary.BigEndian.Uint16(data[0:2])
	safi := data[2]

	// Only handle IPv6 unicast for now
	if afi != 2 || safi != 1 {
		return nil
	}

	return parseIPv6Prefixes(data[3:])
}

// parseIPv6NLRI parses IPv6 NLRI into ReceivedRoute structs.
func parseIPv6NLRI(data []byte, nextHop netip.Addr, attrs parsedAttrs) []ReceivedRoute {
	var routes []ReceivedRoute
	for i := 0; i < len(data); {
		if i >= len(data) {
			break
		}
		prefixLen := int(data[i])
		i++
		prefixBytes := (prefixLen + 7) / 8
		if i+prefixBytes > len(data) {
			break
		}
		var addrBytes [16]byte
		copy(addrBytes[:], data[i:i+prefixBytes])
		i += prefixBytes

		addr := netip.AddrFrom16(addrBytes)
		prefix := netip.PrefixFrom(addr, prefixLen)
		routes = append(routes, ReceivedRoute{
			Prefix:          prefix,
			NextHop:         nextHop,
			Origin:          attrs.origin,
			LocalPreference: attrs.localPref,
			MED:             attrs.med,
			ASPath:          attrs.asPath,
		})
	}
	return routes
}

// parseIPv6Prefixes parses a sequence of IPv6 prefixes (used for MP_UNREACH).
func parseIPv6Prefixes(data []byte) []netip.Prefix {
	var prefixes []netip.Prefix
	for i := 0; i < len(data); {
		if i >= len(data) {
			break
		}
		prefixLen := int(data[i])
		i++
		prefixBytes := (prefixLen + 7) / 8
		if i+prefixBytes > len(data) {
			break
		}
		var addrBytes [16]byte
		copy(addrBytes[:], data[i:i+prefixBytes])
		i += prefixBytes

		addr := netip.AddrFrom16(addrBytes)
		prefix := netip.PrefixFrom(addr, prefixLen)
		prefixes = append(prefixes, prefix)
	}
	return prefixes
}

// DecodedOpen holds parsed OPEN message contents for API formatting.
type DecodedOpen struct {
	Version      uint8
	ASN          uint32 // 4-byte ASN (uses ASN4 capability if present)
	HoldTime     uint16
	RouterID     string   // Dotted-decimal format
	Capabilities []string // Human-readable capability names
}

// DecodeOpen parses raw OPEN message bytes into API-friendly struct.
// Returns zero-value DecodedOpen on invalid input (never panics).
func DecodeOpen(body []byte) DecodedOpen {
	open, err := message.UnpackOpen(body)
	if err != nil {
		return DecodedOpen{}
	}

	// Parse capabilities from OptionalParams
	caps, asn4 := parseCapabilitiesFromOptParams(open.OptionalParams)

	// Determine ASN: prefer ASN4 from capability, then Open.ASN4, then MyAS
	asn := uint32(open.MyAS)
	if asn4 > 0 {
		asn = asn4
	} else if open.ASN4 > 0 {
		asn = open.ASN4
	}

	return DecodedOpen{
		Version:      open.Version,
		ASN:          asn,
		HoldTime:     open.HoldTime,
		RouterID:     open.RouterID(),
		Capabilities: caps,
	}
}

// parseCapabilitiesFromOptParams extracts capability strings from OPEN optional parameters.
// RFC 3392: Optional Parameter Type 2 contains capabilities.
// Returns capability strings and ASN4 value (0 if not present).
func parseCapabilitiesFromOptParams(optParams []byte) ([]string, uint32) {
	if len(optParams) == 0 {
		return nil, 0
	}

	var capStrings []string
	var asn4 uint32
	offset := 0

	// Parse optional parameters TLV structure
	for offset < len(optParams) {
		if offset+2 > len(optParams) {
			break
		}

		paramType := optParams[offset]
		paramLen := int(optParams[offset+1])
		offset += 2

		if offset+paramLen > len(optParams) {
			break
		}

		paramData := optParams[offset : offset+paramLen]
		offset += paramLen

		// Type 2 = Capabilities Optional Parameter (RFC 3392/5492)
		if paramType == 2 {
			caps, err := capability.Parse(paramData)
			if err != nil {
				continue
			}
			for _, cap := range caps {
				capStrings = append(capStrings, formatCapability(cap))
				// Extract ASN4 if present
				if asn4Cap, ok := cap.(*capability.ASN4); ok {
					asn4 = asn4Cap.ASN
				}
			}
		}
	}

	return capStrings, asn4
}

// formatCapability returns human-readable string for a capability.
// Format matches ExaBGP text encoder (lowercased): capname(value) or capname.
func formatCapability(cap capability.Capability) string {
	switch c := cap.(type) {
	case *capability.Multiprotocol:
		// ExaBGP: "Multiprotocol(ipv4 unicast,ipv6 unicast)" -> "multiprotocol(ipv4 unicast)"
		return fmt.Sprintf("multiprotocol(%s %s)", c.AFI, c.SAFI)
	case *capability.ASN4:
		// ExaBGP: "ASN4(65536)" -> "asn4(65536)"
		return fmt.Sprintf("asn4(%d)", c.ASN)
	case *capability.RouteRefresh:
		// ExaBGP: "Route Refresh" -> "route refresh"
		return "route refresh"
	case *capability.ExtendedMessage:
		// ExaBGP: "Extended Message" -> "extended message"
		return "extended message"
	case *capability.EnhancedRouteRefresh:
		// ExaBGP: "Enhanced Route Refresh" -> "enhanced route refresh"
		return "enhanced route refresh"
	case *capability.AddPath:
		// ExaBGP: "AddPath(receive ipv4 unicast,send ipv6 unicast)" -> "addpath(...)"
		var parts []string
		for _, f := range c.Families {
			var mode string
			switch f.Mode {
			case capability.AddPathNone:
				mode = "none"
			case capability.AddPathReceive:
				mode = "receive"
			case capability.AddPathSend:
				mode = "send"
			case capability.AddPathBoth:
				mode = "send/receive"
			}
			parts = append(parts, fmt.Sprintf("%s %s %s", mode, f.AFI, f.SAFI))
		}
		return "addpath(" + strings.Join(parts, ",") + ")"
	case *capability.GracefulRestart:
		// ExaBGP: "Graceful Restart(120,ipv4 unicast preserved)" -> "graceful restart(...)"
		return fmt.Sprintf("graceful restart(%d)", c.RestartTime)
	case *capability.ExtendedNextHop:
		// ExaBGP: "Nexthop(ipv4/unicast/ipv6)" -> "nexthop(...)"
		var parts []string
		for _, f := range c.Families {
			parts = append(parts, fmt.Sprintf("%s/%s/%s", f.NLRIAFI, f.NLRISAFI, f.NextHopAFI))
		}
		return "nexthop(" + strings.Join(parts, ",") + ")"
	case *capability.FQDN:
		// ExaBGP: "Hostname(host,domain)" -> "hostname(...)"
		if c.DomainName != "" {
			return fmt.Sprintf("hostname(%s,%s)", c.Hostname, c.DomainName)
		}
		return fmt.Sprintf("hostname(%s)", c.Hostname)
	case *capability.SoftwareVersion:
		// ExaBGP: "Software(version)" -> "software(version)"
		return fmt.Sprintf("software(%s)", c.Version)
	default:
		return strings.ToLower(cap.Code().String())
	}
}

// DecodedNotification holds parsed NOTIFICATION message contents for API formatting.
type DecodedNotification struct {
	ErrorCode        uint8
	ErrorSubcode     uint8
	ErrorCodeName    string
	ErrorSubcodeName string
	ShutdownMessage  string // For Cease/Admin Shutdown or Admin Reset
	Data             []byte // Raw data field
}

// DecodeNotification parses raw NOTIFICATION message bytes into API-friendly struct.
// Returns zero-value DecodedNotification on invalid input (never panics).
func DecodeNotification(body []byte) DecodedNotification {
	notify, err := message.UnpackNotification(body)
	if err != nil {
		return DecodedNotification{}
	}

	decoded := DecodedNotification{
		ErrorCode:        uint8(notify.ErrorCode),
		ErrorSubcode:     notify.ErrorSubcode,
		ErrorCodeName:    notify.ErrorCode.String(),
		ErrorSubcodeName: notificationSubcodeString(notify.ErrorCode, notify.ErrorSubcode),
		Data:             notify.Data,
	}

	// Extract shutdown message if applicable
	if msg, err := notify.ShutdownMessage(); err == nil {
		decoded.ShutdownMessage = msg
	}

	return decoded
}

// subcodeUnspecific is the string for unspecific/unspecified subcodes.
const subcodeUnspecific = "Unspecific"

// notificationSubcodeString returns human-readable subcode name.
func notificationSubcodeString(code message.NotifyErrorCode, subcode uint8) string {
	switch code { //nolint:exhaustive // Only some codes have specific subcode strings
	case message.NotifyCease:
		return message.CeaseSubcodeString(subcode)
	case message.NotifyOpenMessage:
		return openSubcodeString(subcode)
	case message.NotifyUpdateMessage:
		return updateSubcodeString(subcode)
	case message.NotifyMessageHeader:
		return headerSubcodeString(subcode)
	case message.NotifyFSMError:
		return fsmSubcodeString(subcode)
	default:
		if subcode == 0 {
			return subcodeUnspecific
		}
		return fmt.Sprintf("Subcode(%d)", subcode)
	}
}

func openSubcodeString(subcode uint8) string {
	switch subcode {
	case 0:
		return subcodeUnspecific
	case message.NotifyOpenUnsupportedVersion:
		return "Unsupported Version Number"
	case message.NotifyOpenBadPeerAS:
		return "Bad Peer AS"
	case message.NotifyOpenBadBGPID:
		return "Bad BGP Identifier"
	case message.NotifyOpenUnsupportedOptParam:
		return "Unsupported Optional Parameter"
	case message.NotifyOpenUnacceptableHoldTime:
		return "Unacceptable Hold Time"
	case message.NotifyOpenUnsupportedCapability:
		return "Unsupported Capability"
	case message.NotifyOpenRoleMismatch:
		return "Role Mismatch"
	default:
		return fmt.Sprintf("Subcode(%d)", subcode)
	}
}

func updateSubcodeString(subcode uint8) string {
	switch subcode {
	case 0:
		return subcodeUnspecific
	case message.NotifyUpdateMalformedAttr:
		return "Malformed Attribute List"
	case message.NotifyUpdateUnrecognizedAttr:
		return "Unrecognized Well-known Attribute"
	case message.NotifyUpdateMissingAttr:
		return "Missing Well-known Attribute"
	case message.NotifyUpdateAttrFlags:
		return "Attribute Flags Error"
	case message.NotifyUpdateAttrLength:
		return "Attribute Length Error"
	case message.NotifyUpdateInvalidOrigin:
		return "Invalid ORIGIN Attribute"
	case message.NotifyUpdateInvalidNextHop:
		return "Invalid NEXT_HOP Attribute"
	case message.NotifyUpdateOptionalAttr:
		return "Optional Attribute Error"
	case message.NotifyUpdateInvalidNetwork:
		return "Invalid Network Field"
	case message.NotifyUpdateMalformedASPath:
		return "Malformed AS_PATH"
	default:
		return fmt.Sprintf("Subcode(%d)", subcode)
	}
}

func headerSubcodeString(subcode uint8) string {
	switch subcode {
	case 0:
		return subcodeUnspecific
	case message.NotifyHeaderConnectionNotSync:
		return "Connection Not Synchronized"
	case message.NotifyHeaderBadLength:
		return "Bad Message Length"
	case message.NotifyHeaderBadType:
		return "Bad Message Type"
	default:
		return fmt.Sprintf("Subcode(%d)", subcode)
	}
}

func fsmSubcodeString(subcode uint8) string {
	switch subcode {
	case message.NotifyFSMUnspecified:
		return "Unspecified Error"
	case message.NotifyFSMUnexpectedOpenSent:
		return "Receive Unexpected Message in OpenSent State"
	case message.NotifyFSMUnexpectedOpenConfirm:
		return "Receive Unexpected Message in OpenConfirm State"
	case message.NotifyFSMUnexpectedEstablished:
		return "Receive Unexpected Message in Established State"
	default:
		return fmt.Sprintf("Subcode(%d)", subcode)
	}
}
