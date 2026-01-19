package plugin

import (
	"fmt"
	"net/netip"

	"codeberg.org/thomas-mangin/zebgp/internal/bgp/capability"
	"codeberg.org/thomas-mangin/zebgp/internal/bgp/message"
)

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

// DecodedRouteRefresh holds parsed ROUTE-REFRESH message data.
// RFC 2918 (base) and RFC 7313 (Enhanced Route Refresh with subtypes).
type DecodedRouteRefresh struct {
	AFI         uint16 // Address Family Identifier
	SAFI        uint8  // Subsequent AFI
	Subtype     uint8  // RFC 7313: 0=Normal, 1=BoRR, 2=EoRR
	SubtypeName string // "refresh", "borr", or "eorr"
	Family      string // Combined family name (e.g., "ipv4/unicast")
}

// DecodeRouteRefresh parses raw ROUTE-REFRESH message bytes.
// Returns zero-value DecodedRouteRefresh on invalid input (never panics).
// RFC 7313 Section 3.2: Message Subtype field values.
func DecodeRouteRefresh(body []byte) DecodedRouteRefresh {
	rr, err := message.UnpackRouteRefresh(body)
	if err != nil {
		return DecodedRouteRefresh{}
	}

	var subtypeName string
	switch rr.Subtype {
	case message.RouteRefreshNormal:
		subtypeName = "refresh"
	case message.RouteRefreshBoRR:
		subtypeName = "borr"
	case message.RouteRefreshEoRR:
		subtypeName = "eorr"
	default:
		subtypeName = fmt.Sprintf("unknown(%d)", rr.Subtype)
	}

	return DecodedRouteRefresh{
		AFI:         uint16(rr.AFI),
		SAFI:        uint8(rr.SAFI),
		Subtype:     uint8(rr.Subtype),
		SubtypeName: subtypeName,
		Family:      afiSafiToFamily(uint16(rr.AFI), uint8(rr.SAFI)),
	}
}

// DecodedNegotiated holds negotiated capabilities for API formatting.
// Sent after OPEN exchange to inform plugins of negotiated capabilities.
type DecodedNegotiated struct {
	// MessageSize is max message size (4096 or 65535 if ExtendedMessage).
	MessageSize int
	// HoldTime is negotiated hold time in seconds.
	HoldTime uint16
	// ASN4 is true if 4-byte ASN capability negotiated.
	ASN4 bool
	// RouteRefresh indicates route-refresh support: "absent", "normal", or "enhanced".
	RouteRefresh string
	// Families is list of negotiated address families (e.g., ["ipv4/unicast"]).
	Families []string
	// AddPathSend is list of families where ADD-PATH send is negotiated.
	AddPathSend []string
	// AddPathReceive is list of families where ADD-PATH receive is negotiated.
	AddPathReceive []string
	// ExtendedNextHop maps family to nexthop AFI (e.g., {"ipv4/unicast": "ipv6"}).
	ExtendedNextHop map[string]string
}

// NegotiatedToDecoded converts capability.Negotiated to DecodedNegotiated.
func NegotiatedToDecoded(neg *capability.Negotiated) DecodedNegotiated {
	if neg == nil {
		return DecodedNegotiated{}
	}

	// Determine message size
	msgSize := 4096
	if neg.ExtendedMessage {
		msgSize = 65535
	}

	// Determine route-refresh capability
	refresh := "absent"
	if neg.EnhancedRouteRefresh {
		refresh = "enhanced"
	} else if neg.RouteRefresh {
		refresh = "normal"
	}

	// Convert families
	families := neg.Families()
	familyStrs := make([]string, 0, len(families))
	for _, f := range families {
		familyStrs = append(familyStrs, afiSafiToFamily(uint16(f.AFI), uint8(f.SAFI)))
	}

	// Convert ADD-PATH (separate send/receive)
	var addPathSend, addPathRecv []string
	for _, f := range families {
		mode := neg.AddPathMode(f)
		famStr := afiSafiToFamily(uint16(f.AFI), uint8(f.SAFI))
		if mode == capability.AddPathSend || mode == capability.AddPathBoth {
			addPathSend = append(addPathSend, famStr)
		}
		if mode == capability.AddPathReceive || mode == capability.AddPathBoth {
			addPathRecv = append(addPathRecv, famStr)
		}
	}

	// Convert Extended Next Hop
	var extNH map[string]string
	for _, f := range families {
		nhAFI := neg.ExtendedNextHopAFI(f)
		if nhAFI != 0 {
			if extNH == nil {
				extNH = make(map[string]string)
			}
			famStr := afiSafiToFamily(uint16(f.AFI), uint8(f.SAFI))
			extNH[famStr] = afiToString(nhAFI)
		}
	}

	return DecodedNegotiated{
		MessageSize:     msgSize,
		HoldTime:        neg.HoldTime,
		ASN4:            neg.ASN4,
		RouteRefresh:    refresh,
		Families:        familyStrs,
		AddPathSend:     addPathSend,
		AddPathReceive:  addPathRecv,
		ExtendedNextHop: extNH,
	}
}

// afiToString converts AFI to string name.
func afiToString(afi capability.AFI) string {
	switch afi {
	case 1:
		return "ipv4"
	case 2:
		return "ipv6"
	default:
		return fmt.Sprintf("afi(%d)", afi)
	}
}

// afiSafiToFamily converts AFI/SAFI to family string.
func afiSafiToFamily(afi uint16, safi uint8) string {
	var afiName string
	switch afi {
	case 1:
		afiName = "ipv4"
	case 2:
		afiName = "ipv6"
	default:
		afiName = fmt.Sprintf("afi(%d)", afi)
	}

	var safiName string
	switch safi {
	case 1:
		safiName = SAFINameUnicast
	case 2:
		safiName = SAFINameMulticast
	case 4:
		safiName = "mpls-labels"
	case 128:
		safiName = SAFINameMPLSVPN
	default:
		safiName = fmt.Sprintf("safi(%d)", safi)
	}

	return afiName + "/" + safiName
}
