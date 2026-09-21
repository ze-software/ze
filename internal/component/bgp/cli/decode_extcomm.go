// Design: docs/architecture/core-design.md — BGP CLI commands
// Overview: the four community attributes rendered for `ze bgp decode`
// Related: decode_update.go — renderAttributeZe files each one under its JSON key
// RFC: rfc/short/rfc1997.md — COMMUNITIES, four octets per community (code 8)
// RFC: rfc/short/rfc4360.md — EXTENDED_COMMUNITIES, eight octets per community (code 16)
// RFC: rfc/short/rfc5701.md — IPV6_EXTENDED_COMMUNITIES, twenty octets per community (code 25)
// RFC: rfc/short/rfc8092.md — LARGE_COMMUNITIES, twelve octets and the canonical form (code 32)

package cli

import (
	"encoding/binary"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// Each renderer below takes one community attribute's whole value and returns
// the text of every community in it. The per-community renderer is the SAME one
// the daemon's event JSON uses (appendCommunitiesJSON and its three siblings,
// internal/component/bgp/plugins/filter_community/json.go): Community.String,
// ExtendedCommunity.AppendDecoded, LargeCommunity.String, and
// IPv6ExtendedCommunity.String. So one community cannot read one way in a
// capture and another way in the event stream.
//
// A renderer returns nil when the wire length is not a whole number of
// communities. Nil is not a drop: renderAttributeZe (decode_update.go) then
// files the attribute under its raw form, so an attribute a peer sent always
// leaves a trace.

// parseCommunities renders a COMMUNITIES attribute value (RFC 1997) as one
// "asn:value" string per community, or the well-known name when the community
// registry carries one.
func parseCommunities(data []byte) []string {
	comms, err := attribute.ParseCommunities(data)
	if err != nil {
		return nil
	}

	text := make([]string, len(comms))
	for i, comm := range comms {
		text[i] = comm.String()
	}

	return text
}

// parseExtendedCommunities renders each 8-octet extended community (RFC 4360
// Section 2) for `ze bgp decode` JSON: the raw 64-bit value, and the named form.
//
// The naming is attribute.ExtendedCommunity.AppendDecoded, the same renderer the
// receive path uses for the plugin event JSON, so the CLI and the event stream
// cannot answer differently about one community.
//
// This is the one community attribute rendered as objects rather than as text.
// Its 8 octets are the only community width that fits a JSON number, so the
// reader gets the decoded name and the value a filter matches on. RFC 8092's 12
// octets and RFC 5701's 20 octets do not fit one, and RFC 1997's 4 octets are
// already legible in the name.
func parseExtendedCommunities(data []byte) []map[string]any {
	comms, err := attribute.ParseExtendedCommunities(data)
	if err != nil {
		return nil
	}

	var text [48]byte
	rendered := make([]map[string]any, len(comms))
	for i, comm := range comms {
		rendered[i] = map[string]any{
			"value":  binary.BigEndian.Uint64(comm[:]),
			"string": string(comm.AppendDecoded(text[:0])),
		}
	}

	return rendered
}

// parseIPv6ExtendedCommunities renders each 20-octet IPv6 extended community
// (RFC 5701 Section 2) in its named form.
//
// Through String(), which is AppendDecoded, and for the reason its neighbor
// parseLargeCommunities renders through String() too: this is the SECOND reader
// of these octets, and the first is appendIPv6ExtCommunitiesJSON, which the
// plugin feed reads. They answered differently until 2026-09-21 -- `ze bgp
// decode` wrote bare hex and the feed wrote the decoded form -- so one fact had
// two spellings and no reader of both could tell they were the same thing.
//
// The comment this replaces justified the hex by saying no IPv6 vocabulary
// exists, because every field offset the 8-octet one reads names something else
// here. That has not been true since AppendDecoded grew its redirect-to-nexthop
// arm, and RFC 5701 Section 3 names two more.
func parseIPv6ExtendedCommunities(data []byte) []string {
	comms, err := attribute.ParseIPv6ExtendedCommunities(data)
	if err != nil {
		return nil
	}

	text := make([]string, len(comms))
	for i, comm := range comms {
		text[i] = comm.String()
	}

	return text
}

// parseLargeCommunities renders each 12-octet large community (RFC 8092
// Section 3) in the canonical "global:local1:local2" form RFC 8092 Section 5
// states.
func parseLargeCommunities(data []byte) []string {
	comms, err := attribute.ParseLargeCommunities(data)
	if err != nil {
		return nil
	}

	text := make([]string, len(comms))
	for i, comm := range comms {
		text[i] = comm.String()
	}

	return text
}
