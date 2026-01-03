package api

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"strings"

	"github.com/exa-networks/zebgp/pkg/bgp/capability"
	"github.com/exa-networks/zebgp/pkg/bgp/message"
)

// ExtractAttributeBytes extracts the path attributes section from UPDATE body.
// Returns nil if body is malformed or has no attributes.
//
// UPDATE body format (RFC 4271 Section 4.3):
//   - Withdrawn Routes Length: 2 octets
//   - Withdrawn Routes: variable
//   - Total Path Attribute Length: 2 octets
//   - Path Attributes: variable
//   - NLRI: variable
func ExtractAttributeBytes(body []byte) []byte {
	if len(body) < 4 {
		return nil
	}

	// Skip withdrawn routes
	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	offset := 2 + withdrawnLen

	if offset+2 > len(body) {
		return nil
	}

	// Read attribute length
	attrLen := int(binary.BigEndian.Uint16(body[offset : offset+2]))
	offset += 2

	if attrLen == 0 || offset+attrLen > len(body) {
		return nil
	}

	return body[offset : offset+attrLen]
}

// parseIPv4Prefixes parses a sequence of IPv4 prefixes.
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

// parseIPv6Prefixes parses a sequence of IPv6 prefixes.
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
