// Design: docs/architecture/exabgp-bridge.md -- the members ExaBGP reads off an UPDATE's own bytes
// Related: bridge_event.go -- Event.ExabgpJSON, the one UPDATE renderer, which calls overlayWire
// Related: bridge_event_packets.go -- Packet, the bytes a `full` event carries

package bridge

import (
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// exabgpMemberValue is the member ExaBGP states a decoded value under, beside
// its `string` or its `element`.
const exabgpMemberValue = "value"

// overlayWire rewrites the members of an UPDATE line that ze's JSON states
// lossily, from the UPDATE's own path attributes: the AS_PATH segments, the
// numeric value beside each extended community, and the flag octet in the name
// of an attribute ze has no parser for.
//
// ze's document carries the display text alone for these, and the bridge
// cannot recover from text what the text dropped, so it reads the attribute
// octets the `full` event carries (Event.Packet). An event that carries no
// bytes keeps ze's members as they are: there is nothing to read them from.
//
// The AS_PATH needs the session's AS number width (RFC 6793 Section 4: a
// NEW BGP speaker talking to an OLD one sends two-octet AS numbers), so it is
// read only when the runner knows the session (Event.attrContext); every other
// member is width-free.
func (e Event) overlayWire(message map[string]any) {
	update, ok := message[msgTypeUpdate].(map[string]any)
	if !ok {
		return
	}
	section, ok := e.Packet.updateAttributes()
	if !ok {
		return
	}
	contextID := e.attrContext
	knowsWidth := contextID != 0
	if !knowsWidth {
		contextID = bgpctx.APIContextID
	}
	attrs := attribute.NewAttributesWire(section, contextID)
	if knowsWidth {
		overlayASPath(update, attrs)
	}
	overlayCommunityWidth(update, attrs, attribute.AttrExtCommunity, "extended-community", 8)
	overlayCommunityWidth(update, attrs, attribute.AttrIPv6ExtCommunity, "extended-community-ipv6", 20)
	overlayUnknownAttributes(update, attrs)
}

// updateAttributes answers the Path Attributes field of an UPDATE body, and
// false when the packet is not an UPDATE, carries no attribute, or its lengths
// do not fit the body (RFC 4271 Section 4.3: "Withdrawn Routes Length (2
// octets)", "Total Path Attribute Length (2 octets)"). The engine validated
// the message before it delivered it, so a misfit is not the engine's bytes
// and nothing is read from them.
func (p Packet) updateAttributes() ([]byte, bool) {
	if p.code != packetCodes[msgTypeUpdate] {
		return nil, false
	}
	body, err := hex.DecodeString(p.body)
	if err != nil {
		return nil, false
	}
	if len(body) < 4 {
		return nil, false
	}
	withdrawn := int(binary.BigEndian.Uint16(body))
	if len(body) < 4+withdrawn {
		return nil, false
	}
	length := int(binary.BigEndian.Uint16(body[2+withdrawn:]))
	start := 4 + withdrawn
	if length == 0 {
		return nil, false
	}
	if len(body) < start+length {
		return nil, false
	}
	return body[start : start+length], true
}

// overlayUnknownAttributes renames every attribute Ze has no parser for, from
// `attr-<code>` to the `attribute-0x<code>-0x<flags>` ExaBGP writes.
//
// The flags are the difference that matters. ExaBGP names an unknown attribute
// by its code AND the flag octet the peer set (_generate_json,
// bgp/message/update/attribute/collection.py), because two peers can send the
// same unknown code with different transitivity, and a script deciding whether
// to propagate it has to read RFC 4271 Section 4.3's Optional and Transitive
// bits. Ze's own document states the code alone unless a caller asks for the
// flag booleans, so the bridge reads them off the parsed attribute instead.
//
// The value gains the `0x` ExaBGP prefixes its hex with, so a reader can tell
// the payload from a decimal.
func overlayUnknownAttributes(update map[string]any, attrs *attribute.AttributesWire) {
	attrObject, ok := update["attribute"].(map[string]any)
	if !ok {
		return
	}
	for key, value := range attrObject {
		number, isUnknown := strings.CutPrefix(key, "attr-")
		if !isUnknown {
			continue
		}
		code, err := strconv.ParseUint(number, 10, 8)
		if err != nil {
			continue
		}
		parsed, err := attrs.Get(attribute.AttributeCode(code))
		if err != nil || parsed == nil {
			continue
		}
		text, isText := value.(string)
		if !isText {
			continue
		}
		delete(attrObject, key)
		var value textbuf.Buffer
		attrObject[exabgpUnknownAttributeName(uint8(code), parsed.Flags())] =
			value.Str("0x").Str(text).String()
	}
}

// exabgpUnknownAttributeName spells `attribute-0xCC-0xFF`.
//
// Both octets are UPPER case, because ExaBGP builds the name with `{:02X}`
// (_generate_json, bgp/message/update/attribute/collection.py) and a script
// keyed on its name finds nothing under a lower-case one.
func exabgpUnknownAttributeName(code uint8, flags attribute.AttributeFlags) string {
	var name textbuf.Buffer
	octets := [1]byte{code}
	name.Str("attribute-0x").HexUpper(octets[:])
	octets[0] = uint8(flags)
	return name.Str("-0x").HexUpper(octets[:]).String()
}

// exabgpASPathElements names each AS_PATH segment type the way ExaBGP does
// (src/exabgp/bgp/message/update/attribute/aspath.py).
var exabgpASPathElements = map[attribute.ASPathSegmentType]string{
	attribute.ASSet:            "as-set",
	attribute.ASSequence:       "as-sequence",
	attribute.ASConfedSequence: "as-confed-sequence",
	attribute.ASConfedSet:      "as-confed-set",
}

// overlayASPath replaces the flattened as-path with the SEGMENTS ExaBGP states.
//
// ze writes `[65533]`: every ASN of every segment, in order, with the segment
// boundaries and their types discarded (appendASPathJSON,
// internal/core/bgp/attribute/json.go). A reader of that cannot tell an AS_SET
// from an AS_SEQUENCE, which are different statements about the path: RFC 4271
// Section 4.3 makes a set UNORDERED. ExaBGP keeps both, keyed by segment index:
// {"0": {"element": "as-sequence", exabgpMemberValue: [65533]}}.
func overlayASPath(update map[string]any, attrs *attribute.AttributesWire) {
	value, err := attrs.Get(attribute.AttrASPath)
	if err != nil || value == nil {
		return
	}
	path, ok := value.(*attribute.ASPath)
	if !ok {
		return
	}
	segments := make(map[string]any, len(path.Segments))
	for index, segment := range path.Segments {
		asns := make([]any, 0, len(segment.ASNs))
		for _, asn := range segment.ASNs {
			asns = append(asns, float64(asn))
		}
		element, named := exabgpASPathElements[segment.Type]
		if !named {
			// An unnamed segment type is not guessed at: the index is kept so
			// the path's shape survives, and the type is stated as the number
			// the wire carried.
			var unnamed textbuf.Buffer
			element = unnamed.Str("type-").Uint8(uint8(segment.Type)).String()
		}
		segments[strconv.Itoa(index)] = map[string]any{"element": element, exabgpMemberValue: asns}
	}
	if len(segments) == 0 {
		return
	}
	attributeOf(update)["as-path"] = segments
}

// overlayCommunityWidth rebuilds one community list as the {string, value}
// members ExaBGP writes, pairing each rendered text with the octets it came
// from by position.
//
// ExaBGP states both halves -- {"string": "target:72:1", exabgpMemberValue: 563259191066625}
// -- and ze's document carries the display text alone. The number is the octets
// as they sit on the wire, so it is read from the attribute rather than
// reconstructed from the text. overlayWire calls it for RFC 4360's 8-octet
// communities and RFC 5701's 20-octet IPv6 ones; the 20-octet value does not
// fit a uint64, so it is carried as a float, the way JSON carries it either way.
func overlayCommunityWidth(
	update map[string]any,
	attrs *attribute.AttributesWire,
	code attribute.AttributeCode,
	key string,
	width int,
) {
	raw, err := attrs.GetRaw(code)
	if err != nil || len(raw) == 0 || len(raw)%width != 0 {
		return
	}
	// The attribute object is created only once there is something to put in
	// it. Asking for it first left `"attribute": {}` on an update whose
	// attributes the normaliser had emptied, and ExaBGP writes no member at all
	// there (reactor/api/response/json.py).
	attrObject := attributeOf(update)
	existing, _ := attrObject[key].([]any)
	rebuilt := make([]any, 0, len(raw)/width)
	for offset := 0; offset+width <= len(raw); offset += width {
		member := map[string]any{exabgpMemberValue: communityValue(raw[offset : offset+width])}
		// The string ze already rendered, in the order it rendered them, so the
		// two halves describe the same community.
		if index := offset / width; index < len(existing) {
			if text, isText := existing[index].(string); isText {
				folded, transitive, states := exabgpInterfaceSet(text)
				if states {
					member["string"] = folded
					member["transitive"] = transitive
				} else {
					member["string"] = exabgpCommunityText(text)
				}
			}
		}
		rebuilt = append(rebuilt, member)
	}
	attrObject[key] = rebuilt
}

// communityValue reads a community's octets as the one big-endian number
// ExaBGP prints beside its text.
//
// A float is what comes out, because JSON has one number type and both sides of
// any comparison land on a float64. The value is built EXACTLY first and
// rounded once: RFC 5701's community is 160 bits wide, and accumulating
// `value*256 + octet` in float rounds at every octet, which put this member two
// thousand off the expectation for an 8-octet community whose decimal literal
// rounds cleanly.
func communityValue(octets []byte) float64 {
	value, _ := new(big.Int).SetBytes(octets).Float64()
	return value
}

// exabgpCommunitySpellings are the FlowSpec actions the two projects write with
// different words, as a prefix of the rendered community and its replacement.
//
// The values below are matched as prefixes rather than whole strings because
// each carries an operand: a DSCP number, an address.
//
//   - ze writes `mark:10` and ExaBGP writes `mark 10` (TrafficMark.__repr__).
//     ze's colon form is load-bearing on its own side: the FlowSpec firewall
//     bridge keys on `mark:<n>` (extcomm_decoded.go), so the fold belongs here
//     and not at the renderer.
//   - both projects write `redirect-to-nexthop` and mean DIFFERENT communities
//     by it. ze's names draft-ietf-idr-flowspec-redirect-ip's 0x01:0x0c, which
//     carries an address; ExaBGP calls that one `redirect-to-nexthop-ietf` and
//     keeps the bare word for 0x08:0x00, the pre-draft form that carries none
//     (TrafficNextHopSimpson). Folding the address-carrying one is what makes
//     the two vocabularies agree on which community is which.
var exabgpCommunitySpellings = [][2]string{
	{"mark:", "mark "},
	{"traffic-action:", "action "},
	{"redirect-to-nexthop ", "redirect-to-nexthop-ietf "},
	{"copy-to-nexthop ", "copy-to-nexthop-ietf "},
}

// exabgpInterfaceSet splits ze's interface-set text into the two halves ExaBGP
// states separately, and answers false for anything that is not one.
//
// draft-ietf-idr-flowspec-interfaceset Section 5 defines the community twice,
// transitive and non-transitive, which differ by one bit of the type octet. ze
// writes that bit into the text, because every extended community it carries
// sits in one flat list and the text is the only place left to say it. ExaBGP
// has a `scope` block of its own, so it prints four fields and puts
// transitivity in a JSON member beside them (InterfaceSet.json, its
// flowspec_scope.py). It is the ONLY community ExaBGP writes that member for.
func exabgpInterfaceSet(text string) (folded string, transitive, states bool) {
	rest, isInterfaceSet := strings.CutPrefix(text, "interface-set:")
	if !isInterfaceSet {
		return "", false, false
	}
	transitivity, fields, split := strings.Cut(rest, ":")
	if !split {
		return "", false, false
	}
	switch transitivity {
	case "transitive":
		transitive = true
	case "non-transitive":
		transitive = false
	default:
		return "", false, false
	}
	var tb textbuf.Buffer
	return tb.Str("interface-set:").Str(fields).String(), transitive, true
}

// exabgpCommunityText restates one rendered extended community in ExaBGP's
// words, and answers the text unchanged when both projects already agree.
func exabgpCommunityText(text string) string {
	if word, isSimpson := exabgpSimpsonNextHop(text); isSimpson {
		return word
	}
	for _, spelling := range exabgpCommunitySpellings {
		if rest, found := strings.CutPrefix(text, spelling[0]); found {
			var folded textbuf.Buffer
			return folded.Str(spelling[1]).Str(rest).String()
		}
	}
	return text
}

// exabgpSimpsonNextHop answers ExaBGP's word for the 0x0800 redirect-to-next-hop
// community of draft-simpson-idr-flowspec-redirect, and false for any other.
//
// ze has no name for type 0x08 subtype 0x00 and writes it as its octets,
// `0x0800:` and the six value octets in hex. ExaBGP decodes it as
// TrafficNextHopSimpson (community/extended/traffic.py, 5.0.1):
//
//	(bit,) = unpack('!B', data[7:8])
//	return TrafficNextHopSimpson(bool(bit & 0x01), data[:8])
//	...
//	return 'copy-to-nexthop' if self.copy else 'redirect-to-nexthop'
//
// so the copy flag is the low bit of the last octet, the last hex digit here.
func exabgpSimpsonNextHop(text string) (string, bool) {
	value, isSimpson := strings.CutPrefix(text, "0x0800:")
	if !isSimpson {
		return "", false
	}
	if len(value) != 12 {
		return "", false
	}
	last, err := strconv.ParseUint(value[11:], 16, 8)
	if err != nil {
		return "", false
	}
	if last&0x01 == 0x01 {
		return "copy-to-nexthop", true
	}
	return "redirect-to-nexthop", true
}

// attributeOf answers an update's attribute object, creating it when the update
// carries none, so a caller can write into it unconditionally.
func attributeOf(update map[string]any) map[string]any {
	existing, ok := update["attribute"].(map[string]any)
	if ok {
		return existing
	}
	created := map[string]any{}
	update["attribute"] = created
	return created
}
