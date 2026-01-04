package api

import (
	"encoding/binary"
	"fmt"
	"net/netip"

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

// DecodedCapability holds structured capability data for API formatting.
// Used by both text and JSON encoders.
type DecodedCapability struct {
	Code  uint8  // Capability code (e.g., 1 for multiprotocol)
	Name  string // Capability name (e.g., "multiprotocol")
	Value string // Capability value (e.g., "ipv4/unicast"), empty if none
}

// DecodedOpen holds parsed OPEN message contents for API formatting.
type DecodedOpen struct {
	Version      uint8
	ASN          uint32 // 4-byte ASN (uses ASN4 capability if present)
	HoldTime     uint16
	RouterID     string              // Dotted-decimal format
	Capabilities []DecodedCapability // Structured capability data
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

// parseCapabilitiesFromOptParams extracts capabilities from OPEN optional parameters.
// RFC 3392: Optional Parameter Type 2 contains capabilities.
// Returns structured capabilities and ASN4 value (0 if not present).
func parseCapabilitiesFromOptParams(optParams []byte) ([]DecodedCapability, uint32) {
	if len(optParams) == 0 {
		return nil, 0
	}

	var caps []DecodedCapability
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
			parsed, err := capability.Parse(paramData)
			if err != nil {
				continue
			}
			for _, cap := range parsed {
				caps = append(caps, formatCapability(cap)...)
				// Extract ASN4 if present
				if asn4Cap, ok := cap.(*capability.ASN4); ok {
					asn4 = asn4Cap.ASN
				}
			}
		}
	}

	return caps, asn4
}

// formatCapability returns structured capability data.
// Most capabilities return a single entry, but AddPath/ExtendedNextHop return one per family.
func formatCapability(cap capability.Capability) []DecodedCapability {
	code := uint8(cap.Code())
	switch c := cap.(type) {
	case *capability.Multiprotocol:
		return []DecodedCapability{{Code: code, Name: "multiprotocol", Value: fmt.Sprintf("%s/%s", c.AFI, c.SAFI)}}
	case *capability.ASN4:
		return []DecodedCapability{{Code: code, Name: "asn4", Value: fmt.Sprintf("%d", c.ASN)}}
	case *capability.RouteRefresh:
		return []DecodedCapability{{Code: code, Name: "route-refresh"}}
	case *capability.ExtendedMessage:
		return []DecodedCapability{{Code: code, Name: "extended-message"}}
	case *capability.EnhancedRouteRefresh:
		return []DecodedCapability{{Code: code, Name: "enhanced-route-refresh"}}
	case *capability.AddPath:
		// Return one entry per family
		var results []DecodedCapability
		for _, f := range c.Families {
			var mode string
			switch f.Mode {
			case capability.AddPathNone:
				continue // Skip none mode
			case capability.AddPathReceive:
				mode = "receive"
			case capability.AddPathSend:
				mode = "send"
			case capability.AddPathBoth:
				mode = "send-receive"
			}
			results = append(results, DecodedCapability{
				Code:  code,
				Name:  "addpath",
				Value: fmt.Sprintf("%s/%s %s", f.AFI, f.SAFI, mode),
			})
		}
		return results
	case *capability.GracefulRestart:
		return []DecodedCapability{{Code: code, Name: "graceful-restart", Value: fmt.Sprintf("%d", c.RestartTime)}}
	case *capability.ExtendedNextHop:
		// Return one entry per family
		var results []DecodedCapability
		for _, f := range c.Families {
			results = append(results, DecodedCapability{
				Code:  code,
				Name:  "extended-nexthop",
				Value: fmt.Sprintf("%s/%s %s", f.NLRIAFI, f.NLRISAFI, f.NextHopAFI),
			})
		}
		return results
	case *capability.FQDN:
		if c.DomainName != "" {
			return []DecodedCapability{{Code: code, Name: "fqdn", Value: fmt.Sprintf("%s.%s", c.Hostname, c.DomainName)}}
		}
		return []DecodedCapability{{Code: code, Name: "fqdn", Value: c.Hostname}}
	case *capability.SoftwareVersion:
		return []DecodedCapability{{Code: code, Name: "software-version", Value: c.Version}}
	default:
		// Unknown capability: use "unknown-<code>" as name, hex data as value
		return []DecodedCapability{{Code: code, Name: fmt.Sprintf("unknown-%d", code), Value: fmt.Sprintf("%x", cap.Pack())}}
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
