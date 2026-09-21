// Design: docs/architecture/exabgp-bridge.md -- the ExaBGP JSON the bridge writes
// Related: bridge_event.go -- ZebgpToExabgpJSON, which this feeds
//
// The file carries the BGP gate because it is the one place the bridge reads an
// UPDATE's own octets: it names internal/component/bgp/types, and that package
// is listed under ze_bgp so an always-on importer cannot pin the BGP engine
// into a build compiled without it (feature-gates.txt, the ze_bgp section).

//go:build ze_bgp

package bridge

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// SessionFacts is what a renderer cannot read off the wire: the two ends of the
// session and the identifier this speaker presents.
//
// ExaBGP states all of it on every line (_neighbor,
// src/exabgp/reactor/api/response/json.py), and none of it is in an UPDATE's
// bytes, so a caller comparing a rendered frame against a fixture MUST supply
// what that fixture's own configuration set.
type SessionFacts struct {
	Local    netip.Addr
	Peer     netip.Addr
	LocalAS  uint32
	PeerAS   uint32
	RouterID uint32

	// Encoding is what the session NEGOTIATED, and it decides how the UPDATE's
	// own bytes are read rather than how they are described.
	//
	// RFC 7911 Section 3 puts a 4-octet Path Identifier ahead of each NLRI when
	// ADD-PATH was negotiated for that family, and nothing in the message says
	// so: a decoder that does not know reads the identifier's octets as prefix
	// bytes and produces routes the peer never sent. A nil value means no
	// capability beyond four-octet AS numbers, which is what an API-originated
	// message carries.
	Encoding *capability.EncodingCaps
}

// WireUpdateToExabgpJSON renders one UPDATE's wire body as the ExaBGP JSON
// document a process attached to that peer would receive.
//
// It exists so a fixture's `:json:` expectation can be checked against the frame
// the speaker actually sent. Until it did, those expectations were read and
// dropped (readExaBGPCase, internal/le/interoplab/bgp/exabgp_server.go), which
// is why two defects in this file's own output reached a verification sweep
// unseen.
//
// The payload is the UPDATE without its 19-octet header, which is what a `.ci`
// raw line carries after its length and type fields.
func WireUpdateToExabgpJSON(payload []byte, facts SessionFacts, direction rpc.MessageDirection) (map[string]any, error) {
	// The API context, not a zero ContextID. An unregistered id resolves to no
	// EncodingContext, so the attribute block decodes to nothing and the render
	// comes out with no attributes and a null next hop -- which looks like a
	// defect in the renderer's subject rather than in its setup.
	contextID, err := sessionContextID(facts)
	if err != nil {
		return nil, err
	}
	var wire wireu.WireUpdate
	wireu.InitWireUpdate(&wire, payload, contextID)
	attrs, attrErr := wire.Attrs()
	if attrErr != nil {
		return nil, fmt.Errorf("exabgp-bridge: parse update attributes: %w", attrErr)
	}

	peer := &plugin.PeerInfo{
		Address:         facts.Peer,
		LocalAddress:    facts.Local,
		AddressStr:      facts.Peer.String(),
		LocalAddressStr: facts.Local.String(),
		LocalAS:         facts.LocalAS,
		PeerAS:          facts.PeerAS,
		RouterID:        facts.RouterID,
	}
	message := bgptypes.RawMessage{
		Type:       msgtype.TypeUPDATE,
		RawBytes:   payload,
		Direction:  direction,
		WireUpdate: &wire,
		AttrsWire:  attrs,
	}

	// The ze-native document first, in the one encoding ZebgpToExabgpJSON reads.
	rendered := format.AppendMessage(nil, peer, message, bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   plugin.FormatParsed,
	})
	var zebgp map[string]any
	if err := json.Unmarshal(rendered, &zebgp); err != nil {
		return nil, fmt.Errorf("exabgp-bridge: the ze document does not parse: %w", err)
	}
	document := ZebgpToExabgpJSON(zebgp)

	// Two members are rebuilt from the PARSED attributes rather than from the
	// document above, because ze's own JSON is lossy for them and the bridge
	// cannot recover what it never received. Reading them here costs nothing:
	// this function already holds the attributes the wire was parsed into.
	//
	// Fixing ze's document instead would change a contract 21 .ci fixtures, 38
	// test files and the looking glass, the CLI decoder, the RIB formatter and
	// the path filters all read. That is a migration, not a rendering fix, and
	// it is not owed here: ze's flattened form is right for a ze reader.
	//
	// A withdraw-only UPDATE carries no attribute section at all, and Attrs()
	// answers (nil, nil) for one: "Returns (nil, nil) if empty"
	// (WireUpdate.Attrs, component/bgp/wireu/wire_update.go). There is nothing
	// to overlay, and reading one panics. Checking the ERROR alone is what let
	// that through, which is the nil a caller cannot tell from an answer
	// (ai/rules/principles.md).
	if attrs == nil {
		return document, nil
	}
	overlayASPath(document, attrs)
	overlayExtendedCommunities(document, attrs)
	overlayUnknownAttributes(document, attrs)
	return document, nil
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
func overlayUnknownAttributes(document map[string]any, attrs *attribute.AttributesWire) {
	update, ok := exabgpUpdateObject(document)
	if !ok {
		return
	}
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

// sessionContextID answers the registered EncodingContext for what the session
// negotiated, and the API context when it negotiated nothing.
//
// The direction is SEND, read from the speaker whose message this is: RFC 7911
// Section 4 makes ADD-PATH asymmetric, so the question the decoder needs
// answered is whether that speaker SENDS path identifiers for the family, not
// whether it would accept them.
func sessionContextID(facts SessionFacts) (bgpctx.ContextID, error) {
	if facts.Encoding == nil {
		return bgpctx.APIContextID, nil
	}
	id, err := bgpctx.Registry.Register(
		bgpctx.NewEncodingContext(nil, facts.Encoding, bgpctx.DirectionSend),
	)
	if err != nil {
		return bgpctx.APIContextID, fmt.Errorf("exabgp-bridge: register session context: %w", err)
	}
	return id, nil
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
// {"0": {"element": "as-sequence", "value": [65533]}}.
func overlayASPath(document map[string]any, attrs *attribute.AttributesWire) {
	update, ok := exabgpUpdateObject(document)
	if !ok {
		return
	}
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
		segments[strconv.Itoa(index)] = map[string]any{"element": element, "value": asns}
	}
	if len(segments) == 0 {
		return
	}
	attributeOf(update)["as-path"] = segments
}

// overlayExtendedCommunities restores the numeric value beside each community.
//
// ExaBGP states both halves -- {"string": "target:72:1", "value": 563259191066625}
// -- and ze's document carries the display text alone. The number is the octets
// as they sit on the wire, so it is read from the attribute rather than
// reconstructed from the text: a display form is not a wire form, and parsing
// one back is the lossy re-derivation ai/rules/principles.md warns about.
//
// Both widths are folded, because ExaBGP writes the same pair for RFC 4360's
// 8-octet communities and RFC 5701's 20-octet IPv6 ones. The 20-octet value
// does not fit a uint64, so it is accumulated as a float the way JSON will
// carry it either way.
func overlayExtendedCommunities(document map[string]any, attrs *attribute.AttributesWire) {
	update, ok := exabgpUpdateObject(document)
	if !ok {
		return
	}
	overlayCommunityWidth(update, attrs, attribute.AttrExtCommunity, "extended-community", 8)
	overlayCommunityWidth(update, attrs, attribute.AttrIPv6ExtCommunity, "extended-community-ipv6", 20)
}

// overlayCommunityWidth rebuilds one community list as the {string, value}
// members ExaBGP writes, pairing each rendered text with the octets it came
// from by position.
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
		member := map[string]any{"value": communityValue(raw[offset : offset+width])}
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
	for _, spelling := range exabgpCommunitySpellings {
		if rest, found := strings.CutPrefix(text, spelling[0]); found {
			var folded textbuf.Buffer
			return folded.Str(spelling[1]).Str(rest).String()
		}
	}
	return text
}

// exabgpUpdateObject answers the `update` object of a rendered document, and
// false when the document states none, which every non-UPDATE message does.
func exabgpUpdateObject(document map[string]any) (map[string]any, bool) {
	neighbor, ok := document["neighbor"].(map[string]any)
	if !ok {
		return nil, false
	}
	message, ok := neighbor["message"].(map[string]any)
	if !ok {
		return nil, false
	}
	update, ok := message["update"].(map[string]any)
	return update, ok
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

// exabgpVolatileMembers are the top-level members that differ between two
// correct runs, so a comparison drops them rather than pinning them.
//
// It is upstream's own list: qa/bin/functional's _cleanup removes exactly these
// before it compares two ExaBGP documents. `counter` is theirs too and ze writes
// none; it is listed so a future line carrying one is dropped rather than newly
// mismatched.
var exabgpVolatileMembers = []string{"exabgp", "time", "host", "pid", "ppid", "counter"}

// DropVolatile removes the members no two runs agree on, in place, and answers
// the same map so a caller can chain it.
//
// It also folds `direction`, which is NOT volatile but is stated in two
// vocabularies by one project. The ExaBGP DAEMON writes `receive` and `send`
// (src/exabgp/reactor/protocol.py) and so does ze; the `in` and `out` that
// appear in test/exabgp-compat fixtures come from upstream's own test harness
// (qa/bin/test_json, which calls encoder.update(neighbor, 'in', ...)). Folding
// them is what lets a fixture be compared at all, and it is the honest
// direction to fold: teaching ze to write `in` would make it disagree with the
// daemon it exists to be compatible with.
func DropVolatile(document map[string]any) map[string]any {
	for _, member := range exabgpVolatileMembers {
		delete(document, member)
	}
	neighbor, ok := document["neighbor"].(map[string]any)
	if !ok {
		return document
	}
	switch neighbor["direction"] {
	case "in":
		neighbor["direction"] = "receive"
	case "out":
		neighbor["direction"] = "send"
	}
	return document
}
