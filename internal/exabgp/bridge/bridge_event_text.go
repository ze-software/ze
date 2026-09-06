// Design: docs/architecture/exabgp-bridge.md -- the ExaBGP TEXT event encoder
// Overview: bridge_event.go -- the JSON encoder and the ze event reader
// Related: bridge_encoder.go -- the encoder a process declares
//
// ExaBGP writes one event per LINE when a process declares `encoder text`. This
// file is that format, ported from ExaBGP's own encoder,
// src/exabgp/reactor/api/response/text.py, class Text. The v4 encoder in
// response/v4/text.py answers byte-identical lines, so there is one format
// rather than two.
//
// One line per event is the contract a consumer relies on, so every value a
// PEER chose passes through appendOneline before it is written. A hostname of
// "a\nneighbor 1.2.3.4 down - forged" would otherwise forge a whole event, which
// is the CWE-116 that ExaBGP's own oneline() exists to close.

package bridge

import (
	"encoding/json"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// TextForm says what the text encoder made of one event.
//
// "Nothing was written" is two different answers and they are named apart.
// ExaBGP's text encoder legitimately writes nothing for `negotiated`, `fsm` and
// `signal`, and a kind the bridge does not know is a gap. One empty buffer for
// both would make the gap unreachable to a caller and invisible in a log
// (`ai/rules/principles.md`).
type TextForm uint8

const (
	// TextNone is an event ExaBGP's text encoder writes no line for.
	TextNone TextForm = iota
	// TextWritten is an event that was rendered into the buffer.
	TextWritten
	// TextUnknown is an event kind this encoder does not render.
	TextUnknown
)

// BGP version 4. RFC 4271 Section 4.2: "This 1-octet unsigned integer indicates
// the protocol version number of the message. The current BGP version number is
// 4." ze speaks BGP-4 alone and its OPEN event carries no version field, so the
// text line states the one version the message can have carried.
const textOpenVersion = "4"

// AppendText renders the event as ExaBGP text and appends it to buf. Every line
// it writes is terminated by '\n', so a caller writes the answer once.
//
// The caller owns the buffer.
func (e Event) AppendText(buf []byte) ([]byte, TextForm) {
	switch e.Kind {
	case msgTypeState:
		return e.appendStateText(buf)
	case msgTypeUpdate:
		return e.appendUpdateText(buf), TextWritten
	case msgTypeOpen:
		return e.appendOpenText(buf), TextWritten
	case msgTypeKeepalive:
		buf = e.appendPrefix(buf, true)
		buf = append(buf, " keepalive\n"...)
		return buf, TextWritten
	case msgTypeNotification:
		return e.appendNotificationText(buf), TextWritten
	case msgTypeRefresh:
		return e.appendRefreshText(buf), TextWritten
	case msgTypeNegotiated, msgTypeFSM, msgTypeSignal:
		// Text.negotiated, Text.fsm and Text.signal each answer None, so
		// ExaBGP writes no line for them. Ze subscribes to every event, so it
		// meets all three and drops them here on purpose.
		return buf, TextNone
	default:
		return buf, TextUnknown
	}
}

// appendPrefix writes `neighbor <address>`, and the direction after it when the
// line carries one. Every text line starts this way.
func (e Event) appendPrefix(buf []byte, withDirection bool) []byte {
	buf = append(buf, "neighbor "...)
	buf = appendOneline(buf, e.Peer)
	if withDirection {
		buf = append(buf, ' ')
		buf = append(buf, e.Direction...)
	}
	return buf
}

// appendStateText writes Text.up, Text.connected or Text.down.
func (e Event) appendStateText(buf []byte) ([]byte, TextForm) {
	state, _ := e.Payload["state"].(string)
	switch state {
	case "up", "connected":
		buf = e.appendPrefix(buf, false)
		buf = append(buf, ' ')
		buf = append(buf, state...)
		return append(buf, '\n'), TextWritten
	case "down":
		reason, _ := e.Payload["reason"].(string)
		buf = e.appendPrefix(buf, false)
		buf = append(buf, " down - "...)
		buf = appendOneline(buf, reason)
		return append(buf, '\n'), TextWritten
	default:
		// json-format.md declares three state values. A fourth is a ze change
		// this encoder has not been taught, and it is reported rather than
		// dropped.
		return buf, TextUnknown
	}
}

// appendUpdateText writes Text.update: a start line, one line for each
// announced and each withdrawn NLRI, and an end line.
//
// ExaBGP writes the attributes ONCE per announced NLRI, after the next hop, and
// writes none on a withdrawn line.
func (e Event) appendUpdateText(buf []byte) []byte {
	prefixAt := len(buf)
	buf = e.appendPrefix(buf, true)
	buf = append(buf, " update"...)
	prefix := string(buf[prefixAt:])

	buf = append(buf, " start\n"...)

	attributes := appendTextAttributes(nil, e.Data)

	nlri, _ := e.Data[bridgeUpdateNLRI].(map[string]any)
	for _, family := range sortedKeys(nlri) {
		entries, ok := nlri[family].([]any)
		if !ok {
			continue
		}
		for _, entry := range entries {
			operation, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			buf = appendUpdateOperation(buf, prefix, operation, attributes)
		}
	}

	buf = append(buf, prefix...)
	return append(buf, " end\n"...)
}

// appendUpdateOperation writes one announce or withdraw group of an UPDATE.
func appendUpdateOperation(buf []byte, prefix string, operation map[string]any, attributes []byte) []byte {
	action, _ := operation["action"].(string)
	nextHop, _ := operation["next-hop"].(string)
	values, _ := operation[bridgeUpdateNLRI].([]any)

	for _, value := range values {
		buf = append(buf, prefix...)
		switch action {
		case "add":
			buf = append(buf, " announced "...)
			buf = appendNLRIText(buf, value)
			if nextHop != "" {
				buf = append(buf, " next-hop "...)
				buf = appendOneline(buf, nextHop)
			}
			buf = append(buf, attributes...)
		case "del":
			buf = append(buf, " withdrawn "...)
			buf = appendNLRIText(buf, value)
		default:
			// Neither action: write the NLRI on a route line, which is the
			// shape ExaBGP uses for an End-of-RIB.
			buf = append(buf, " route "...)
			buf = appendNLRIText(buf, value)
		}
		buf = append(buf, '\n')
	}
	return buf
}

// appendNLRIText writes one NLRI as ExaBGP's `extensive()` writes it.
//
// ze reports a prefix family as a string and a structured family (mvpn, evpn,
// flow) as an object. ExaBGP's extensive() is per family and is not reachable
// from the object ze sends, so an object is written as its own `nlri` field
// when it has one and as compact JSON when it does not. A reader then still
// sees what arrived, on one line.
func appendNLRIText(buf []byte, value any) []byte {
	switch typed := value.(type) {
	case string:
		return appendOneline(buf, typed)
	case map[string]any:
		if text, ok := typed[bridgeUpdateNLRI].(string); ok {
			return appendOneline(buf, text)
		}
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return append(buf, "unrenderable"...)
	}
	return appendOneline(buf, string(encoded))
}

// textAttribute is one path attribute in ExaBGP's text rendering: the code it
// sorts by and the word it writes.
//
// Source: ExaBGP src/exabgp/bgp/message/update/attribute/collection.py, the
// AttributeCollection.representation table and _generate_text, which sorts by
// attribute code and writes ` <name> <value>`.
type textAttribute struct {
	// Code is the IANA path-attribute code. ExaBGP orders the attributes of one
	// UPDATE by it.
	Code uint8
	// Name is ExaBGP's own word. It differs from ze's JSON key wherever ExaBGP
	// writes the singular, which is why this column exists.
	Name string
}

// textAttributes maps ze's event JSON key to ExaBGP's text rendering. The keys
// are the ones internal/core/bgp/attribute registers through
// RegisterJSONFormatter, plus the plugin-registered community keys.
//
// NEXT_HOP is absent on purpose: ExaBGP marks it NO_GENERATION and writes it
// beside the NLRI instead of among the attributes.
var textAttributes = map[string]textAttribute{
	bridgeAttrOrigin:            {Code: 1, Name: bridgeAttrOrigin},
	bridgeAttrASPath:            {Code: 2, Name: bridgeAttrASPath},
	bridgeAttrMED:               {Code: 4, Name: bridgeAttrMED},
	bridgeAttrLocalPreference:   {Code: 5, Name: bridgeAttrLocalPreference},
	bridgeAttrAtomicAggregate:   {Code: 6, Name: bridgeAttrAtomicAggregate},
	bridgeAttrAggregator:        {Code: 7, Name: bridgeAttrAggregator},
	"communities":               {Code: 8, Name: bridgeAttrCommunity},
	bridgeAttrOriginatorID:      {Code: 9, Name: bridgeAttrOriginatorID},
	bridgeAttrClusterList:       {Code: 10, Name: bridgeAttrClusterList},
	"extended-communities":      {Code: 16, Name: bridgeAttrExtCommunity},
	"ipv6-extended-communities": {Code: 25, Name: bridgeAttrExtCommunityIPv6},
	bridgeAttrAIGP:              {Code: 26, Name: bridgeAttrAIGP},
	"large-communities":         {Code: 32, Name: bridgeAttrLargeCommunity},
}

// appendTextAttributes renders an UPDATE's attributes as ExaBGP writes them:
// each one preceded by a space, in attribute-code order.
//
// An attribute whose value is an EMPTY list is written NOT AT ALL, which is
// what ExaBGP does with one: `_generate_text` renders a list attribute through
// `str(attribute)` and skips it when that answers the empty string, and
// `ASPath.string` answers the empty string for a path with no segments
// (src/exabgp/bgp/message/update/attribute/aspath.py). A route this speaker
// originated has an empty AS_PATH and ze reports it as `"as-path": []`, so
// without the skip every announced line carried an ` as-path [ ]` ExaBGP never
// writes. api-check reads the whole line and exits 1 on the first one that is
// not the announce it waits for, so that one token failed the case.
//
// An attribute ze reports under a key this table does not name is written in
// ExaBGP's generic form, ` attribute [ 0xCC <value> ]`. ExaBGP puts the flag
// byte between the code and the value; the ze event carries no flags on this
// path, so the bridge writes the code and the value alone. That is the one
// place where a text line is narrower than ExaBGP's.
func appendTextAttributes(buf []byte, data map[string]any) []byte {
	attributes, ok := data[bridgeUpdateAttr].(map[string]any)
	if !ok || len(attributes) == 0 {
		return buf
	}

	keys := sortedKeys(attributes)
	sort.SliceStable(keys, func(i, j int) bool {
		return textAttributeCode(keys[i]) < textAttributeCode(keys[j])
	})

	for _, key := range keys {
		if key == bridgeAttrNextHop {
			continue
		}
		if emptyAttributeList(attributes[key]) {
			continue
		}
		named, known := textAttributes[key]
		if !known {
			buf = append(buf, " attribute [ 0x"...)
			buf = appendHexByte(buf, textAttributeCode(key))
			buf = append(buf, ' ')
			buf = appendAttributeValueText(buf, attributes[key])
			buf = append(buf, " ]"...)
			continue
		}
		if named.Name == bridgeAttrAtomicAggregate {
			// ATOMIC_AGGREGATE is a flag: ExaBGP writes the word alone.
			buf = append(buf, ' ')
			buf = append(buf, named.Name...)
			continue
		}
		buf = append(buf, ' ')
		buf = append(buf, named.Name...)
		buf = append(buf, ' ')
		buf = appendAttributeValueText(buf, attributes[key])
	}
	return buf
}

// emptyAttributeList reports a JSON value that is a list with no members. ze
// delivers every list-valued attribute as a JSON array, so this is the shape
// ExaBGP's own `if value:` test refuses.
func emptyAttributeList(value any) bool {
	list, ok := value.([]any)
	return ok && len(list) == 0
}

// textAttributeCode answers the attribute code a ze event key carries, so the
// attributes sort as ExaBGP sorts them. A key the table does not name is read
// as `attr-<code>`, which is how ze reports an attribute with no registered
// JSON formatter (internal/component/bgp/format/text_json.go,
// appendAttributeJSON). A key that is neither sorts last.
func textAttributeCode(key string) uint8 {
	if named, ok := textAttributes[key]; ok {
		return named.Code
	}
	if code, ok := strings.CutPrefix(key, "attr-"); ok {
		if value, err := strconv.ParseUint(code, 10, 8); err == nil {
			return uint8(value)
		}
	}
	return 255
}

// appendAttributeValueText writes one attribute value as ExaBGP writes it: a
// scalar bare, and a list inside `[ ]`.
func appendAttributeValueText(buf []byte, value any) []byte {
	switch typed := value.(type) {
	case string:
		return appendOneline(buf, typed)
	case float64:
		return strconv.AppendFloat(buf, typed, 'f', -1, 64)
	case bool:
		return strconv.AppendBool(buf, typed)
	case []any:
		buf = append(buf, "[ "...)
		for _, member := range typed {
			buf = appendAttributeMemberText(buf, member)
			buf = append(buf, ' ')
		}
		return append(buf, ']')
	default:
		return appendAttributeMemberText(buf, value)
	}
}

// appendAttributeMemberText writes one member of a list-valued attribute.
//
// ze sends an extended community as an object carrying both its numeric value
// and the `target:1.2.3.4:5` text an operator wrote. ExaBGP writes that text, so
// the object is read for its `string` field.
func appendAttributeMemberText(buf []byte, member any) []byte {
	switch typed := member.(type) {
	case string:
		return appendOneline(buf, typed)
	case float64:
		return strconv.AppendFloat(buf, typed, 'f', -1, 64)
	case map[string]any:
		if text, ok := typed["string"].(string); ok {
			return appendOneline(buf, text)
		}
	}
	encoded, err := json.Marshal(member)
	if err != nil {
		return append(buf, "unrenderable"...)
	}
	return appendOneline(buf, string(encoded))
}

// appendOpenText writes Text.open.
//
// ExaBGP writes `capabilities [...]` as the lowercased text of its own
// Capabilities object. The ze event carries a code, a name and a value for each
// capability instead, so the bridge writes `<name> <value>` for each of them,
// separated by a space. The set is the same; the spelling inside the brackets is
// ze's, and nothing in the compatibility corpus parses it.
func (e Event) appendOpenText(buf []byte) []byte {
	buf = e.appendPrefix(buf, true)
	buf = append(buf, " open version "...)
	buf = append(buf, textOpenVersion...)
	buf = append(buf, " asn "...)
	buf = appendJSONNumber(buf, e.Data["asn"])
	buf = append(buf, " hold_time "...)
	buf = appendJSONNumber(buf, e.Data["hold-time"])
	buf = append(buf, " router_id "...)
	routerID, _ := e.Data["router-id"].(string)
	buf = appendOneline(buf, routerID)
	buf = append(buf, " capabilities ["...)

	capabilities, _ := e.Data["capabilities"].([]any)
	for _, entry := range capabilities {
		capability, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		name, _ := capability["name"].(string)
		buf = append(buf, ' ')
		buf = appendLowerOneline(buf, name)
		if value, ok := capability["value"].(string); ok && value != "" {
			buf = append(buf, ' ')
			buf = appendLowerOneline(buf, value)
		}
	}
	return append(buf, "]\n"...)
}

// appendNotificationText writes Text.notification.
func (e Event) appendNotificationText(buf []byte) []byte {
	buf = e.appendPrefix(buf, true)
	buf = append(buf, " notification code "...)
	buf = appendJSONNumber(buf, e.Data["code"])
	buf = append(buf, " subcode "...)
	buf = appendJSONNumber(buf, e.Data["subcode"])
	buf = append(buf, " data "...)
	data, _ := e.Data["data"].(string)
	buf = appendOneline(buf, data)
	return append(buf, '\n')
}

// appendRefreshText writes Text.refresh.
func (e Event) appendRefreshText(buf []byte) []byte {
	buf = e.appendPrefix(buf, true)
	buf = append(buf, " route-refresh afi "...)
	buf = appendJSONNumber(buf, e.Data["afi"])
	buf = append(buf, " safi "...)
	buf = appendJSONNumber(buf, e.Data["safi"])
	buf = append(buf, ' ')
	buf = appendJSONNumber(buf, e.Data["subtype"])
	return append(buf, '\n')
}

// appendJSONNumber writes a value JSON delivered as a number, and writes 0 for
// a field the event did not carry. ExaBGP writes the field on every one of
// these lines, so a missing one keeps the line's token count rather than
// shortening it and moving every field after it.
func appendJSONNumber(buf []byte, value any) []byte {
	switch typed := value.(type) {
	case float64:
		return strconv.AppendFloat(buf, typed, 'f', -1, 64)
	case string:
		return appendOneline(buf, typed)
	default:
		return append(buf, '0')
	}
}

// appendHexByte writes one byte as two uppercase hex digits.
func appendHexByte(buf []byte, value uint8) []byte {
	const digits = "0123456789ABCDEF"
	return append(buf, digits[value>>4], digits[value&0x0F])
}

// appendOneline writes a value a PEER may have chosen without letting it end
// the line. A control character is ESCAPED rather than dropped, so what the peer
// sent is still visible to whoever reads the stream, and still on one line.
//
// Source: ExaBGP src/exabgp/reactor/api/response/text.py, oneline().
func appendOneline(buf []byte, text string) []byte {
	for index := 0; index < len(text); {
		character, width := utf8.DecodeRuneInString(text[index:])
		index += width
		if character == ' ' || unicode.IsPrint(character) {
			buf = utf8.AppendRune(buf, character)
			continue
		}
		buf = appendEscapedRune(buf, character)
	}
	return buf
}

// appendLowerOneline is appendOneline over the lowercased text, which is what
// ExaBGP writes for a capability string.
func appendLowerOneline(buf []byte, text string) []byte {
	return appendOneline(buf, strings.ToLower(text))
}

// appendEscapedRune writes one non-printing character the way Python's repr
// writes it, which is the escape ExaBGP's oneline() produces.
func appendEscapedRune(buf []byte, character rune) []byte {
	switch character {
	case '\n':
		return append(buf, `\n`...)
	case '\r':
		return append(buf, `\r`...)
	case '\t':
		return append(buf, `\t`...)
	}
	if character < 0x100 {
		buf = append(buf, `\x`...)
		return appendLowerHexByte(buf, uint8(character))
	}
	buf = append(buf, `\u`...)
	buf = appendLowerHexByte(buf, uint8(character>>8))
	return appendLowerHexByte(buf, uint8(character))
}

// appendLowerHexByte writes one byte as two lowercase hex digits, which is what
// a Python escape carries.
func appendLowerHexByte(buf []byte, value uint8) []byte {
	const digits = "0123456789abcdef"
	return append(buf, digits[value>>4], digits[value&0x0F])
}

// sortedKeys answers a map's keys in a stable order. A JSON object has none,
// and a script reading the stream needs the same line every run.
func sortedKeys(entries map[string]any) []string {
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
