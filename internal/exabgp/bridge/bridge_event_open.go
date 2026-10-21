// Design: docs/architecture/exabgp-bridge.md -- the OPEN an ExaBGP JSON script reads
// Overview: bridge_event.go -- the JSON encoder
// Related: bridge_event_notification.go -- the NOTIFICATION an ExaBGP script reads
//
// RFC: rfc/short/rfc4724.md
// RFC: rfc/short/rfc6793.md
// RFC: rfc/short/rfc8277.md
// RFC: rfc/short/rfc8654.md
// RFC: rfc/short/rfc9234.md

package bridge

import (
	"encoding/hex"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// openVersion is the BGP version an OPEN carried. RFC 4271 Section 4.2: "The
// current BGP version number is 4." ze speaks BGP-4 alone and its OPEN event
// carries no version field.
const openVersion = 4

// asTrans is the AS a NEW speaker puts in the OPEN's two-octet My Autonomous
// System field when its own AS needs four. RFC 6793 Section 4.1: "the NEW BGP
// speaker MUST use AS_TRANS (23456) in the 'My Autonomous System' field".
const asTrans = 23456

// asTwoOctetMax is the largest AS the My Autonomous System field holds.
const asTwoOctetMax = 65535

// capNameRouteRefresh is the name both route-refresh variants are shown by.
const capNameRouteRefresh = "route-refresh"

// ExaBGP's capability codes, as src/exabgp/bgp/message/open/capability/
// capability.py (CapabilityCode) names them.
const (
	capMultiprotocol        = 0x01
	capRouteRefresh         = 0x02
	capNextHop              = 0x05
	capExtendedMessage      = 0x06
	capMultipleLabels       = 0x08
	capRole                 = 0x09
	capGracefulRestart      = 0x40
	capASN4                 = 0x41
	capMultisession         = 0x44
	capAddPath              = 0x45
	capEnhancedRouteRefresh = 0x46
	capHostname             = 0x49
	capSoftwareVersion      = 0x4B
	capPathsLimit           = 0x4C
	capLinkLocalNextHop     = 0x4D
	capRouteRefreshCisco    = 0x80
	capMultisessionCisco    = 0x83
	capOperational          = 0xB9
)

// exabgpCapabilityNames is CapabilityCode.names: the key an API 6 helper reads
// a capability under. A code it does not name is `unknown capability 0x<hex>`
// (CapabilityCode.name).
var exabgpCapabilityNames = map[int]string{
	0x00:                    "reserved",
	capMultiprotocol:        "multiprotocol",
	capRouteRefresh:         capNameRouteRefresh,
	0x03:                    "outbound-route-filtering",
	0x04:                    "multiple-routes",
	capNextHop:              "nexthop",
	capExtendedMessage:      "extended-message",
	capRole:                 "role",
	capGracefulRestart:      "graceful-restart",
	capASN4:                 "asn4",
	0x43:                    "dynamic-capability",
	capMultisession:         "multi-session",
	capAddPath:              "add-path",
	capEnhancedRouteRefresh: "enhanced-route-refresh",
	capPathsLimit:           "paths-limit",
	capMultipleLabels:       "multiple-labels",
	capLinkLocalNextHop:     "link-local-nexthop",
	capOperational:          "operational",
	capRouteRefreshCisco:    "cisco-route-refresh",
	capMultisessionCisco:    "cisco-multi-sesion",
	capHostname:             "hostname",
	capSoftwareVersion:      "software-version",
}

// exabgpCapabilityName answers CapabilityCode.name for one code.
func exabgpCapabilityName(code int) string {
	if name, ok := exabgpCapabilityNames[code]; ok {
		return name
	}
	return string(strconv.AppendInt([]byte("unknown capability 0x"), int64(code), 16))
}

// openJSON renders the `open` member of an OPEN event, as ExaBGP's JSON.open
// (src/exabgp/reactor/api/response/json.py) writes it.
//
// ze's OPEN event (internal/component/bgp/format/json.go, Open) carries the
// capabilities as a list of code, name and value, so they reach the bridge
// already: ReadEvent puts the `open` object in Data and nothing more is
// carried. The bodies are rebuilt per code from ze's value, which is
// structured for the six capabilities ze's formatCapability knows and the
// payload in hex for every other one.
func (e Event) openJSON(format Format) map[string]any {
	timer, _ := e.Data["timer"].(map[string]any)
	return map[string]any{
		"version":      openVersion,
		"asn":          openMyAS(e.Data["asn"]),
		"hold_time":    timer["hold-time"],
		"router_id":    e.Data["router-id"],
		"capabilities": openCapabilitiesJSON(e.Data["capabilities"], format),
	}
}

// openMyAS answers the OPEN's My Autonomous System field, which is what
// ExaBGP's Open.asn holds. ze's event states the speaker's AS instead, taken
// from the Four-Octet AS capability when there is one, so an AS too large for
// the field went out as AS_TRANS.
func openMyAS(asn any) any {
	value, ok := asn.(float64)
	if !ok {
		return asn
	}
	if value > asTwoOctetMax {
		return asTrans
	}
	return value
}

// openCapabilitiesJSON groups ze's list by code, as ExaBGP's Capabilities is a
// dict keyed by code (two Multiprotocol TLVs are one entry with two families),
// and files each body as the format asks: an API 4 helper under the code,
// "1", with the body as it is; an API 6 helper under the name, with "code"
// inside (JSON._json_kv and JSON._with_code).
func openCapabilitiesJSON(list any, format Format) map[string]any {
	entries, _ := list.([]any)
	byCode := make(map[int][]string, len(entries))
	for _, entry := range entries {
		capability, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		code, ok := capability["code"].(float64)
		if !ok {
			continue
		}
		value, _ := capability["value"].(string)
		byCode[int(code)] = append(byCode[int(code)], value)
	}

	result := make(map[string]any, len(byCode))
	for code, values := range byCode {
		body := capabilityBody(code, values)
		if format == FormatV4JSON {
			result[strconv.Itoa(code)] = body
			continue
		}
		body["code"] = code
		result[exabgpCapabilityName(code)] = body
	}
	return result
}

// capabilityBody answers what ExaBGP's capability class writes from json() for
// one code (src/exabgp/bgp/message/open/capability/*.py). values holds ze's
// value of each TLV of that code, in order. A payload ExaBGP would refuse with
// a NOTIFICATION never reaches its API; ze still renders the capability, as
// ExaBGP renders one it has no class for.
func capabilityBody(code int, values []string) map[string]any {
	switch code {
	case capMultiprotocol:
		families := make([]string, 0, len(values))
		for _, value := range values {
			families = append(families, exabgpCapabilityFamily(value))
		}
		return map[string]any{"name": "multiprotocol", "families": families}
	case capRouteRefresh:
		return map[string]any{"name": capNameRouteRefresh, "variant": "RFC"}
	case capRouteRefreshCisco:
		return map[string]any{"name": capNameRouteRefresh, "variant": "Cisco"}
	case capEnhancedRouteRefresh:
		return map[string]any{"name": "enhanced-route-refresh"}
	case capLinkLocalNextHop:
		return map[string]any{"name": "link-local-nexthop"}
	case capOperational:
		return map[string]any{"name": "operational"}
	case capExtendedMessage:
		// ExtendedMessage.EXTENDED_SIZE. RFC 8654 Section 2: "the maximum
		// message size is 65535 octets".
		return map[string]any{"name": "extended-message", "size": 65535}
	case capASN4:
		if number, err := asn.Parse(values[0]); err == nil {
			return map[string]any{"name": "asn4", "asn4": number}
		}
	case capNextHop:
		return map[string]any{"name": "nexthop", "conversion": nextHopConversions(values)}
	case capAddPath:
		return familyKeyedBody("addpath", values, addPathMode)
	case capPathsLimit:
		return familyKeyedBody("paths-limit", values, pathsLimitValue)
	}
	payload, err := hex.DecodeString(values[0])
	if err != nil {
		return unknownCapabilityBody(code, nil)
	}
	if body := capabilityBodyFromWire(code, payload); body != nil {
		return body
	}
	return unknownCapabilityBody(code, payload)
}

// capabilityBodyFromWire renders a capability ze hands over as its payload.
// It answers nil for a code it has no ExaBGP class for, and for a payload the
// class would refuse.
func capabilityBodyFromWire(code int, payload []byte) map[string]any {
	switch code {
	case capGracefulRestart:
		return gracefulRestartBody(payload)
	case capHostname:
		return hostnameBody(payload)
	case capSoftwareVersion:
		return softwareVersionBody(payload)
	case capRole:
		return roleBody(payload)
	case capMultipleLabels:
		return multipleLabelsBody(payload)
	case capMultisession, capMultisessionCisco:
		return multisessionBody(code, payload)
	}
	return nil
}

// unknownCapabilityBody is UnknownCapability.json.
func unknownCapabilityBody(code int, payload []byte) map[string]any {
	iana := "unknown"
	switch {
	case code >= 128: // CapabilityCodes.reserved, range(128, 256)
		iana = "reserved"
	case code >= 70: // CapabilityCodes.unassigned, range(70, 128)
		iana = "unassigned"
	}
	return map[string]any{
		"name": "unknown", "iana": iana, "value": code,
		"raw": strings.ToUpper(hex.EncodeToString(payload)),
	}
}

// exabgpCapabilityFamily spells a ze family as ExaBGP's capabilities do,
// `str(afi)/str(safi)`: ExaBGP's SAFI name, a slash between the two.
func exabgpCapabilityFamily(zeFamily string) string {
	afi, safi, ok := strings.Cut(zeFamily, "/")
	if !ok {
		return zeFamily
	}
	var tb textbuf.Buffer
	return tb.Str(afi).Byte('/').Str(exabgpSAFIName(safi)).String()
}

// wireCapabilityFamily spells an AFI and a SAFI read off a payload.
func wireCapabilityFamily(afi uint16, safi uint8) string {
	return exabgpCapabilityFamily(family.Family{AFI: family.AFI(afi), SAFI: family.SAFI(safi)}.String())
}

// nextHopConversions is NextHop.json: `<afi>/<safi>/<next-hop afi>`. ze writes
// each entry as `<family> <next-hop afi>`.
func nextHopConversions(values []string) []string {
	conversions := make([]string, 0, len(values))
	for _, value := range values {
		zeFamily, nextHopAFI, _ := strings.Cut(value, " ")
		var tb textbuf.Buffer
		conversions = append(conversions, tb.Str(exabgpCapabilityFamily(zeFamily)).Byte('/').Str(nextHopAFI).String())
	}
	return conversions
}

// familyKeyedBody is the body of AddPath.json and PathsLimit.json: the name,
// then one member per family. ze writes each entry as `<family> <value>`.
func familyKeyedBody(name string, values []string, member func(string) any) map[string]any {
	body := map[string]any{"name": name}
	for _, entry := range values {
		zeFamily, value, _ := strings.Cut(entry, " ")
		body[exabgpCapabilityFamily(zeFamily)] = member(value)
	}
	return body
}

// addPathMode is AddPath.named. ze's AddPathMode.Label writes the both-ways
// mode `send-receive` where ExaBGP writes `send/receive`. ze's event omits a
// family whose mode RFC 7911 does not define, which ExaBGP shows as
// `disabled` or `invalid (<n>)`: the bridge cannot show what it never reads.
func addPathMode(label string) any {
	if label == "send-receive" {
		return "send/receive"
	}
	return label
}

// pathsLimitValue is a PathsLimit member: the limit as a number.
func pathsLimitValue(value string) any {
	if limit, err := strconv.ParseUint(value, 10, 16); err == nil {
		return limit
	}
	return value
}

// Graceful Restart flags (src/exabgp/bgp/message/open/capability/graceful.py).
// RFC 4724 Section 3: "Restart State (R): The most significant bit is defined
// as the Restart State bit" and "Forwarding State (F): The most significant
// bit is defined as the Forwarding State (F) bit".
const (
	gracefulRestartState    = 0x08 // in the four-bit Restart Flags
	gracefulForwardingState = 0x80 // in the per-family Flags octet
	gracefulTimeMask        = 0x0FFF
)

// gracefulRestartBody is Graceful.json, from the payload. RFC 4724 Section 3:
// Restart Flags (4 bits) and Restart Time (12 bits), then for each family an
// AFI (2 octets), a SAFI (1 octet) and its Flags (1 octet).
//
//	0                   1
//	0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	| Flags |   Restart Time (s)    |   offset 0
//	+-------------------------------+
//	|  AFI (2)  | SAFI | AF Flags   |   offset 2, repeated
//	+-------------------------------+
func gracefulRestartBody(payload []byte) map[string]any {
	if len(payload) < 2 {
		return nil
	}
	if (len(payload)-2)%4 != 0 {
		return nil
	}
	restart := uint16(payload[0])<<8 | uint16(payload[1])
	families := make(map[string]any, (len(payload)-2)/4)
	for offset := 2; offset < len(payload); offset += 4 {
		afi := uint16(payload[offset])<<8 | uint16(payload[offset+1])
		flags := []string{}
		if payload[offset+3]&gracefulForwardingState != 0 {
			flags = append(flags, "forwarding")
		}
		families[wireCapabilityFamily(afi, payload[offset+2])] = flags
	}
	restartFlags := []string{}
	if (restart>>12)&gracefulRestartState != 0 {
		restartFlags = append(restartFlags, "restart")
	}
	return map[string]any{
		"name":                 "graceful restart",
		"time":                 restart & gracefulTimeMask,
		"address-family-flags": families,
		"restart-flags":        restartFlags,
	}
}

// hostnameBody is HostName.json: a length and the host name, then a length
// and the domain name (draft-walton-bgp-hostname-capability-02 Section 3).
// ExaBGP writes no "name" member for it.
func hostnameBody(payload []byte) map[string]any {
	if len(payload) < 1 {
		return nil
	}
	hostLength := int(payload[0])
	if len(payload) < hostLength+2 {
		return nil
	}
	host := payload[1 : hostLength+1]
	domainLength := int(payload[hostLength+1])
	if len(payload) < hostLength+2+domainLength {
		return nil
	}
	domain := payload[hostLength+2 : hostLength+2+domainLength]
	if !utf8.Valid(host) {
		return nil
	}
	if !utf8.Valid(domain) {
		return nil
	}
	return map[string]any{"host-name": string(host), "domain-name": string(domain)}
}

// softwareOldLengthUnambiguous is Software.OLD_LENGTH_UNAMBIGUOUS: below it a
// leading octet that counts the rest is revision 00's length octet.
const softwareOldLengthUnambiguous = 0x20

// softwareVersionBody is Software.json. ExaBGP reads the payload as the
// version, or as a length octet and the version when the first octet counts
// the rest and is a control character; an empty or invalid version is an
// UnknownCapability.
func softwareVersionBody(payload []byte) map[string]any {
	version := payload
	if len(payload) > 0 && int(payload[0]) == len(payload)-1 && payload[0] < softwareOldLengthUnambiguous {
		version = payload[1:]
	}
	if len(version) == 0 {
		return nil
	}
	if !utf8.Valid(version) {
		return nil
	}
	return map[string]any{"software": string(version)}
}

// exabgpRoleNames is role.py _NAMES. RFC 9234 Section 4.2 assigns 0 to 4.
var exabgpRoleNames = []string{"provider", "rs", "rs-client", "customer", "peer"}

// roleBody is Role.json: one octet, a name, or `unassigned` for any other.
func roleBody(payload []byte) map[string]any {
	if len(payload) != 1 {
		return nil
	}
	role := "unassigned"
	if int(payload[0]) < len(exabgpRoleNames) {
		role = exabgpRoleNames[payload[0]]
	}
	return map[string]any{"name": "role", "role": role}
}

// multipleLabelsBody is MultipleLabels.json. RFC 8277 Section 2.1: AFI (2
// octets), SAFI (1 octet) and Count (1 octet), repeated.
func multipleLabelsBody(payload []byte) map[string]any {
	if len(payload)%4 != 0 {
		return nil
	}
	body := map[string]any{"name": "multiple-labels"}
	for offset := 0; offset < len(payload); offset += 4 {
		afi := uint16(payload[offset])<<8 | uint16(payload[offset+1])
		body[wireCapabilityFamily(afi, payload[offset+2])] = payload[offset+3]
	}
	return body
}

// multisessionBody is MultiSession.json: a flags octet, then the codes of the
// capabilities the session carries, each shown by its name.
func multisessionBody(code int, payload []byte) map[string]any {
	if len(payload) < 1 {
		return nil
	}
	variant := "Cisco"
	if code == capMultisession {
		variant = "RFC"
	}
	names := make([]string, 0, len(payload)-1)
	for _, carried := range payload[1:] {
		names = append(names, exabgpCapabilityName(int(carried)))
	}
	return map[string]any{"name": "multisession", "variant": variant, "capabilities": names}
}
