// Design: docs/architecture/exabgp-bridge.md -- the ExaBGP TEXT event encoder
// Overview: bridge_event_text.go -- the attributes of a text UPDATE line
// Related: bridge_event_wire.go -- the same octets, read for the JSON line
//
// The community lists of a text UPDATE line, as ExaBGP's Communities.__repr__
// writes them (src/exabgp/bgp/message/update/attribute/community/initial/
// communities.py, 5.0.1):
//
//	if lc > 1: return '[ {} ]'.format(' '.join(repr(c) for c in sorted(self.communities)))
//	if lc == 1: return repr(self.communities[0])
//	return ''
//
// So a list of one is written bare, and a longer list is SORTED, by the
// community octets, whatever order the peer sent. ExtendedCommunities and
// LargeCommunities inherit the method. The order is a fact of the octets, so
// it is read off the UPDATE's own attribute bytes (Event.Packet); ze's text
// alone cannot sort `no-export` against `65000:1`.

package bridge

import (
	"bytes"
	"encoding/binary"
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// textCommunity is one community of a list: its octets, which order it, and
// the word ExaBGP writes for it.
type textCommunity struct {
	octets []byte
	text   string
}

// textCommunityKind is how one community attribute is read off the wire.
type textCommunityKind struct {
	code attribute.AttributeCode
	// width is the octet count of one community: 4 (RFC 1997), 8 (RFC 4360),
	// 12 (RFC 8092).
	width int
	// document is the member of the ExaBGP JSON attribute object that holds
	// the rendered text of each community, or "" when the octets alone say it.
	document string
}

// ze's event keys for the four community attributes, as
// internal/core/bgp/attribute registers their JSON formatters.
const (
	zeAttrCommunities         = "communities"
	zeAttrLargeCommunities    = "large-communities"
	zeAttrExtCommunities      = "extended-communities"
	zeAttrIPv6ExtCommunities  = "ipv6-extended-communities"
	exabgpAttrExtCommunity    = "extended-community"
	exabgpAttrIPv6ExtCommunty = "extended-community-ipv6"
)

// textCommunityKinds maps ze's event key to the attribute it renders.
var textCommunityKinds = map[string]textCommunityKind{
	zeAttrCommunities:        {code: attribute.AttrCommunity, width: 4},
	zeAttrLargeCommunities:   {code: attribute.AttrLargeCommunity, width: 12},
	zeAttrExtCommunities:     {code: attribute.AttrExtCommunity, width: 8, document: exabgpAttrExtCommunity},
	zeAttrIPv6ExtCommunities: {code: attribute.AttrIPv6ExtCommunity, width: 20, document: exabgpAttrIPv6ExtCommunty},
}

// textAttributeWire is what the text attributes read beside ze's event: the
// UPDATE's own attributes, and the ExaBGP JSON attribute object built from
// them. attrs is nil when the event carries no packet bytes, which is the
// named guard every reader checks: there is then nothing to sort by.
type textAttributeWire struct {
	attrs    *attribute.AttributesWire
	document map[string]any
}

// textWire answers the UPDATE's attribute octets and the ExaBGP document's
// attribute object for the text line of e.
func (e Event) textWire(message map[string]any) textAttributeWire {
	update, _ := message[msgTypeUpdate].(map[string]any)
	document, _ := update["attribute"].(map[string]any)
	section, ok := e.Packet.updateAttributes()
	if !ok {
		return textAttributeWire{document: document}
	}
	// Community octets are width-free, so the API context serves when the
	// runner has not yet seen the session's negotiation.
	contextID := e.attrContext
	if contextID == 0 {
		contextID = bgpctx.APIContextID
	}
	return textAttributeWire{attrs: attribute.NewAttributesWire(section, contextID), document: document}
}

// communities answers the communities of kind in ExaBGP's order, and false
// when the octets are not there to read, so the caller keeps ze's own list.
func (w textAttributeWire) communities(kind textCommunityKind) ([]textCommunity, bool) {
	if w.attrs == nil {
		return nil, false
	}
	raw, err := w.attrs.GetRaw(kind.code)
	if err != nil {
		return nil, false
	}
	if len(raw)%kind.width != 0 {
		return nil, false
	}
	documented, _ := w.document[kind.document].([]any)
	members := make([]textCommunity, 0, len(raw)/kind.width)
	for offset := 0; offset+kind.width <= len(raw); offset += kind.width {
		octets := raw[offset : offset+kind.width]
		members = append(members, textCommunity{octets: octets, text: communityText(kind, octets, documented, offset/kind.width)})
	}
	slices.SortStableFunc(members, func(a, b textCommunity) int { return bytes.Compare(a.octets, b.octets) })
	return members, true
}

// exabgpWellKnownCommunities are the RFC 1997, RFC 3765 and RFC 7999 values
// Community.__init__ names (community/initial/community.py, 5.0.1).
var exabgpWellKnownCommunities = map[uint32]string{
	0xFFFFFF01: "no-export",
	0xFFFFFF02: "no-advertise",
	0xFFFFFF03: "no-export-subconfed",
	0xFFFFFF04: "no-peer",
	0xFFFF029A: "blackhole",
}

// communityText answers the word ExaBGP writes for one community.
//
// A standard community is `%d:%d` of its two halves unless it is well known.
// A large community is its three 4-octet fields joined by colons. An extended
// community is the text the JSON document states for it (the `string` member,
// which ExtendedCommunity.json fills from the same __repr__ the text calls),
// and `0x%016X` when that member is empty: ExaBGP's repr of a community it has
// no class for.
func communityText(kind textCommunityKind, octets []byte, documented []any, index int) string {
	var text []byte
	switch kind.width {
	case 4:
		value := binary.BigEndian.Uint32(octets)
		if name, known := exabgpWellKnownCommunities[value]; known {
			return name
		}
		text = strconv.AppendUint(text, uint64(value>>16), 10)
		text = append(text, ':')
		text = strconv.AppendUint(text, uint64(value&0xFFFF), 10)
		return string(text)
	case 12:
		for field := range 3 {
			if field > 0 {
				text = append(text, ':')
			}
			text = strconv.AppendUint(text, uint64(binary.BigEndian.Uint32(octets[field*4:])), 10)
		}
		return string(text)
	}
	if index < len(documented) {
		if member, ok := documented[index].(map[string]any); ok {
			if rendered, ok := member["string"].(string); ok && rendered != "" {
				return rendered
			}
		}
	}
	text = append(text, "0x"...)
	for _, octet := range octets {
		text = appendHexByte(text, octet)
	}
	return string(text)
}

// appendCommunityListText writes a sorted list as Communities.__repr__ does:
// one member bare, more inside `[ ]`, none not at all.
func appendCommunityListText(buf []byte, members []textCommunity) []byte {
	if len(members) > 1 {
		buf = append(buf, "[ "...)
	}
	for index, member := range members {
		if index > 0 {
			buf = append(buf, ' ')
		}
		buf = appendOneline(buf, member.text)
	}
	if len(members) > 1 {
		buf = append(buf, " ]"...)
	}
	return buf
}
