// Design: docs/architecture/exabgp-bridge.md -- the OPEN an ExaBGP text script reads
// Overview: bridge_event_text.go -- the text encoder
// Related: bridge_event_open.go -- the same capabilities, as ExaBGP's JSON
//
// RFC: rfc/short/rfc4724.md

package bridge

import (
	"encoding/hex"
	"slices"
	"strconv"
	"strings"
)

// appendCapabilitiesText writes what ExaBGP's text encoder puts between the
// brackets of `capabilities [...]`: str(Capabilities), lowercased. Capabilities
// is a dict keyed by code and its __str__ joins str() of each value with ", ",
// in code order (src/exabgp/bgp/message/open/capability/capabilities.py,
// Capabilities.__str__, `for key in sorted(self.keys())`). Each value's words
// are its class's __str__, written by appendCapabilityText.
func appendCapabilitiesText(buf []byte, list any) []byte {
	byCode := capabilityValuesByCode(list)
	codes := make([]int, 0, len(byCode))
	for code := range byCode {
		codes = append(codes, code)
	}
	slices.Sort(codes)

	// The whole text is lowercased, the hostname and software version a peer
	// chose included, as `str(sent_open.capabilities).lower()` does.
	var words []byte
	for i, code := range codes {
		if i > 0 {
			words = append(words, ", "...)
		}
		words = appendCapabilityText(words, code, byCode[code])
	}
	return appendLowerOneline(buf, string(words))
}

// appendCapabilityText writes one capability class's __str__
// (src/exabgp/bgp/message/open/capability/*.py). values holds ze's value of
// each TLV of that code, in wire order. The body capabilityBody builds for the
// JSON encoder decides which class ExaBGP would have used, so a payload it
// would refuse is written as UnknownCapability here too.
func appendCapabilityText(buf []byte, code int, values []string) []byte {
	body := capabilityBody(code, values)
	if name, _ := body[exabgpMemberName].(string); name == capNameUnknown {
		// UnknownCapability.__str__: `Unknown`, `Unassigned` or `Reserved`,
		// then str() of the code, which is CapabilityCode.name.
		iana, _ := body["iana"].(string)
		buf = append(buf, iana...)
		buf = append(buf, ' ')
		return append(buf, exabgpCapabilityName(code)...)
	}
	switch code {
	case capMultiprotocol:
		// MultiProtocol.__str__: `Multiprotocol(<afi> <safi>,...)`.
		families, _ := body["families"].([]string)
		return appendTextList(append(buf, "multiprotocol("...), families, ",")
	case capRouteRefresh:
		return append(buf, "route refresh"...)
	case capRouteRefreshCisco:
		return append(buf, "cisco route refresh"...)
	case capEnhancedRouteRefresh:
		return append(buf, "enhanced route refresh"...)
	case capLinkLocalNextHop:
		return append(buf, "link-local nexthop"...)
	case capOperational:
		return append(buf, "operational"...)
	case capExtendedMessage:
		// ExtendedMessage.__str__ writes EXTENDED_SIZE. RFC 8654 Section 2:
		// "the maximum message size is 65535 octets".
		return append(buf, "extended message(65535)"...)
	case capASN4:
		// ASN4.__str__: `ASN4(<asn>)`. capabilityBody parsed ze's decimal
		// value, or answered unknown above.
		buf = append(buf, "asn4("...)
		buf = append(buf, values[0]...)
		return append(buf, ')')
	case capNextHop:
		// NextHop.__str__: `NextHop(<afi> <safi> <next-hop afi>,...)`.
		conversions, _ := body["conversion"].([]string)
		return appendTextList(append(buf, "nexthop("...), conversions, ",")
	case capAddPath:
		return appendAddPathText(buf, values)
	case capPathsLimit:
		return appendPathsLimitText(buf, values)
	case capGracefulRestart, capRole, capMultipleLabels:
		return appendCapabilityTextFromWire(buf, code, values[0])
	case capHostname:
		// HostName.__str__: `Hostname(<host> <domain>)`.
		buf = append(buf, "hostname("...)
		host, _ := body["host-name"].(string)
		domain, _ := body["domain-name"].(string)
		buf = append(buf, host...)
		buf = append(buf, ' ')
		buf = append(buf, domain...)
		return append(buf, ')')
	case capSoftwareVersion:
		version, _ := body["software"].(string)
		buf = append(buf, "software("...)
		buf = append(buf, version...)
		return append(buf, ')')
	case capMultisession, capMultisessionCisco:
		// MultiSession.__str__: `Multisession (RFC) <codes>` for the RFC code,
		// `Multisession <codes>` for Cisco's, a space before the codes even when
		// there is none.
		buf = append(buf, "multisession"...)
		if code == capMultisession {
			buf = append(buf, " (rfc)"...)
		}
		buf = append(buf, ' ')
		names, _ := body["capabilities"].([]string)
		return append(buf, strings.Join(names, " ")...)
	}
	// capabilityBody has a class for every code above and answers unknown
	// for every other one, which returned before the switch.
	panic("BUG: capabilityBody named a capability appendCapabilityText has no words for")
}

// appendTextList writes ExaBGP's `<afi>/<safi>` members as `<afi> <safi>`,
// joined by separator, and closes the parenthesis.
func appendTextList(buf []byte, members []string, separator string) []byte {
	for i, member := range members {
		if i > 0 {
			buf = append(buf, separator...)
		}
		buf = append(buf, strings.ReplaceAll(member, "/", " ")...)
	}
	return append(buf, ')')
}

// appendAddPathText is AddPath.__str__: `AddPath(<mode> <afi> <safi>,...)`,
// each family in the order the peer sent it. ze writes each entry as
// `<family> <mode>`.
func appendAddPathText(buf []byte, values []string) []byte {
	buf = append(buf, "addpath("...)
	for i, entry := range values {
		if i > 0 {
			buf = append(buf, ',')
		}
		zeFamily, mode, _ := strings.Cut(entry, " ")
		label, _ := addPathMode(mode).(string)
		buf = append(buf, label...)
		buf = append(buf, ' ')
		buf = append(buf, strings.ReplaceAll(exabgpCapabilityFamily(zeFamily), "/", " ")...)
	}
	return append(buf, ')')
}

// appendPathsLimitText is PathsLimit.__str__: `PathsLimit(<afi> <safi> <limit>, ...)`.
func appendPathsLimitText(buf []byte, values []string) []byte {
	buf = append(buf, "pathslimit("...)
	for i, entry := range values {
		if i > 0 {
			buf = append(buf, ", "...)
		}
		zeFamily, limit, _ := strings.Cut(entry, " ")
		buf = append(buf, strings.ReplaceAll(exabgpCapabilityFamily(zeFamily), "/", " ")...)
		buf = append(buf, ' ')
		buf = append(buf, limit...)
	}
	return append(buf, ')')
}

// appendCapabilityTextFromWire writes the classes whose words need the payload
// in wire order, which the JSON body's family-keyed map does not keep.
// capabilityBody accepted the payload, so its shape is already checked.
func appendCapabilityTextFromWire(buf []byte, code int, value string) []byte {
	payload, err := hex.DecodeString(value)
	if err != nil {
		panic("BUG: capabilityBody accepted a payload that is not hex")
	}
	switch code {
	case capGracefulRestart:
		return appendGracefulRestartText(buf, payload)
	case capRole:
		// Role.__str__: `Role(<name>)`, `Role(unassigned <octet>)` past the
		// five RFC 9234 Section 4.2 assigns.
		buf = append(buf, "role("...)
		if int(payload[0]) < len(exabgpRoleNames) {
			buf = append(buf, exabgpRoleNames[payload[0]]...)
			return append(buf, ')')
		}
		buf = append(buf, "unassigned "...)
		buf = strconv.AppendUint(buf, uint64(payload[0]), 10)
		return append(buf, ')')
	}
	// MultipleLabels.__str__: `MultipleLabels(<afi> <safi> <count>, ...)`.
	// RFC 8277 Section 2.1: AFI (2 octets), SAFI (1 octet), Count (1 octet).
	buf = append(buf, "multiplelabels("...)
	for offset := 0; offset < len(payload); offset += 4 {
		if offset > 0 {
			buf = append(buf, ", "...)
		}
		afi := uint16(payload[offset])<<8 | uint16(payload[offset+1])
		buf = append(buf, strings.ReplaceAll(wireCapabilityFamily(afi, payload[offset+2]), "/", " ")...)
		buf = append(buf, ' ')
		buf = strconv.AppendUint(buf, uint64(payload[offset+3]), 10)
	}
	return append(buf, ')')
}

// appendGracefulRestartText is Graceful.__str__:
// `Graceful Restart Flags <hex> Time <seconds> <afi>/<safi>=<hex> ...`. The
// flags are the four high bits of the first two octets, and each family's
// flags octet is kept to its Forwarding State bit (Graceful.set, `family_flag &
// Graceful.FORWARDING_STATE`). The families are a dict: a family the peer sent
// twice keeps its first place and its last flags. The layout is the diagram
// above gracefulRestartBody.
func appendGracefulRestartText(buf, payload []byte) []byte {
	restart := uint16(payload[0])<<8 | uint16(payload[1])
	type familyFlags struct {
		family string
		flags  uint8
	}
	families := make([]familyFlags, 0, (len(payload)-2)/4)
	for offset := 2; offset < len(payload); offset += 4 {
		afi := uint16(payload[offset])<<8 | uint16(payload[offset+1])
		entry := familyFlags{wireCapabilityFamily(afi, payload[offset+2]), payload[offset+3] & gracefulForwardingState}
		at := slices.IndexFunc(families, func(seen familyFlags) bool { return seen.family == entry.family })
		if at >= 0 {
			families[at].flags = entry.flags
			continue
		}
		families = append(families, entry)
	}

	buf = append(buf, "graceful restart flags 0x"...)
	buf = strconv.AppendUint(buf, uint64(restart>>12), 16)
	buf = append(buf, " time "...)
	buf = strconv.AppendUint(buf, uint64(restart&gracefulTimeMask), 10)
	buf = append(buf, ' ')
	for i, entry := range families {
		if i > 0 {
			buf = append(buf, ' ')
		}
		buf = append(buf, entry.family...)
		buf = append(buf, "=0x"...)
		buf = strconv.AppendUint(buf, uint64(entry.flags), 16)
	}
	return buf
}
