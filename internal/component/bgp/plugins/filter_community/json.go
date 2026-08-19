// Design: docs/architecture/api/json-format.md -- community attribute JSON rendering

package filter_community

import (
	"encoding/hex"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// The four parsers behind these formatters (ParseCommunities,
// ParseLargeCommunities, ParseExtendedCommunities and
// ParseIPv6ExtendedCommunities in internal/core/bgp/attribute/community.go)
// return VALUE types, and knownAttrParsers boxes that value into the Attribute
// interface. So each assertion below asks for the value, never a pointer: a
// pointer assertion never matches anything the receive path produces, and the
// nil return would send every community attribute to the "attr-N": "<hex>"
// fallback in appendAttributeJSON. Boxing a pointer instead would force a heap
// allocation on the wire receive path, which ai/rules/performance.md forbids.

// appendCommunitiesJSON renders COMMUNITIES (RFC 1997) as a JSON array of
// "asn:value" strings. It returns nil when attr is not a Communities value.
func appendCommunitiesJSON(buf []byte, attr attribute.Attribute) []byte {
	c, ok := attr.(attribute.Communities)
	if !ok {
		return nil
	}
	buf = append(buf, '[')
	for i, comm := range c {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '"')
		buf = append(buf, comm.String()...)
		buf = append(buf, '"')
	}
	return append(buf, ']')
}

// appendLargeCommunitiesJSON renders LARGE_COMMUNITIES (RFC 8092) as a JSON
// array of "ga:ld1:ld2" strings. It returns nil when attr is not a
// LargeCommunities value.
func appendLargeCommunitiesJSON(buf []byte, attr attribute.Attribute) []byte {
	lc, ok := attr.(attribute.LargeCommunities)
	if !ok {
		return nil
	}
	buf = append(buf, '[')
	for i, comm := range lc {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '"')
		buf = append(buf, comm.String()...)
		buf = append(buf, '"')
	}
	return append(buf, ']')
}

// appendExtCommunitiesJSON renders EXTENDED_COMMUNITIES (RFC 4360) as a JSON
// array of named strings: "target:65000:1", "rate-limit:0", "mark:46". It
// returns nil when attr is not an ExtendedCommunities value.
//
// The names come from attribute.ExtendedCommunity.AppendDecoded, the one
// renderer `ze bgp decode` uses too. A type it does not name keeps its octets
// as "0x<type><subtype>:<hex>". The FlowSpec firewall bridge matches these
// words on this event, so hex here left every FlowSpec action unreadable.
func appendExtCommunitiesJSON(buf []byte, attr attribute.Attribute) []byte {
	ec, ok := attr.(attribute.ExtendedCommunities)
	if !ok {
		return nil
	}
	buf = append(buf, '[')
	for i, comm := range ec {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '"')
		buf = comm.AppendDecoded(buf)
		buf = append(buf, '"')
	}
	return append(buf, ']')
}

// appendIPv6ExtCommunitiesJSON renders IPV6_EXTENDED_COMMUNITIES (RFC 5701) as
// a JSON array of 20-octet hex strings. It returns nil when attr is not an
// IPv6ExtendedCommunities value.
//
// Hex, not the named form its 8-octet sibling above renders. RFC 5701 Section 2
// puts a 16-octet IPv6 global administrator where RFC 4360 Section 3.1 puts a
// 2-octet AS, so every field offset the vocabulary reads names something else
// here, and no RFC 8955 traffic filtering action uses this attribute. Naming it
// needs its own spelling for an IPv6 global administrator, which no parser in
// Ze accepts on input yet.
func appendIPv6ExtCommunitiesJSON(buf []byte, attr attribute.Attribute) []byte {
	ec, ok := attr.(attribute.IPv6ExtendedCommunities)
	if !ok {
		return nil
	}
	buf = append(buf, '[')
	for i, comm := range ec {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '"')
		buf = hex.AppendEncode(buf, comm[:])
		buf = append(buf, '"')
	}
	return append(buf, ']')
}
