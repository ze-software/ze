// Design: docs/architecture/core-design.md — BGP CLI commands
// Overview: decode.go — top-level decode dispatch
// Related: decode_mp.go — MP_REACH/MP_UNREACH parsing
// Related: decode_extcomm.go — the four community attribute renderers
// RFC: rfc/short/rfc4271.md — path attribute header, flags and the base attribute codes
// RFC: rfc/short/rfc7606.md — Section 3.g keep-first for a repeated attribute code
// RFC: rfc/short/rfc4760.md — MP_REACH_NLRI and MP_UNREACH_NLRI (codes 14, 15)
// RFC: rfc/short/rfc7752.md — BGP-LS attribute (code 29)

package cli

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/ls"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// decodeUpdateMessage decodes a BGP UPDATE message and returns Ze format.
func decodeUpdateMessage(data []byte, _ string, hasHeader bool) (map[string]any, error) {
	body := data
	if hasHeader {
		if len(data) < message.HeaderLen {
			return nil, errDataTooShortForHeader
		}
		body = data[message.HeaderLen:]
	}

	update, err := message.UnpackUpdate(body)
	if err != nil {
		return nil, fmt.Errorf("unpack update: %w", err)
	}

	// Build Ze format update content
	updateContent := map[string]any{}

	// Parse path attributes - Ze format uses "attr" key
	attrs, mpReach, mpUnreach := parsePathAttributesZe(update.PathAttributes)

	// Extract and remove internal next-hop field (used for NLRI operations)
	nextHop := "0.0.0.0"
	if nh, ok := attrs["_next-hop"].(string); ok {
		nextHop = nh
		delete(attrs, "_next-hop")
	}

	if len(attrs) > 0 {
		updateContent["attr"] = attrs
	}

	// Ze format: family is direct key under update (no "nlri" wrapper)
	// Handle MP_REACH_NLRI (announcements)
	if mpReach != nil {
		fam, ops := buildMPReachZe(mpReach)
		if fam != "" && len(ops) > 0 {
			updateContent[fam] = ops
		}
	}

	// Handle MP_UNREACH_NLRI (withdrawals)
	if mpUnreach != nil {
		fam, ops := buildMPUnreachZe(mpUnreach)
		if fam != "" && len(ops) > 0 {
			if existing, ok := updateContent[fam].([]map[string]any); ok {
				updateContent[fam] = append(existing, ops...)
			} else {
				updateContent[fam] = ops
			}
		}
	}

	// Handle IPv4 withdrawn routes
	if len(update.WithdrawnRoutes) > 0 {
		prefixes := parseIPv4Prefixes(update.WithdrawnRoutes)
		if len(prefixes) > 0 {
			withdrawOp := map[string]any{"action": "del", "nlri": prefixes}
			if existing, ok := updateContent["ipv4/unicast"].([]map[string]any); ok {
				updateContent["ipv4/unicast"] = append(existing, withdrawOp)
			} else {
				updateContent["ipv4/unicast"] = []map[string]any{withdrawOp}
			}
		}
	}

	// Handle IPv4 NLRI (announcements)
	if len(update.NLRI) > 0 {
		prefixes := parseIPv4Prefixes(update.NLRI)
		if len(prefixes) > 0 {
			announceOp := map[string]any{"next-hop": nextHop, "action": "add", "nlri": prefixes}
			if existing, ok := updateContent["ipv4/unicast"].([]map[string]any); ok {
				updateContent["ipv4/unicast"] = append(existing, announceOp)
			} else {
				updateContent["ipv4/unicast"] = []map[string]any{announceOp}
			}
		}
	}

	return map[string]any{"update": updateContent}, nil
}

// parsePathAttributesZe parses path attributes for Ze format (uses simple AS_PATH array).
// Each attribute value is wrapped with RFC 4271 flag booleans from the wire header.
func parsePathAttributesZe(data []byte) (attrs map[string]any, mpReach, mpUnreach []byte) {
	attrs = make(map[string]any)
	offset := 0

	// RFC 7606 Section 3.g keep-first: a repeated attribute code is decoded once (first
	// occurrence wins). This mirrors the session's enforceRFC7606 duplicate strip so that
	// `ze bgp decode` of a malformed peer's UPDATE shows the same attributes an established
	// session would keep, rather than last-write-wins from the map overwrite (D-4b).
	var seen [256]bool

	for offset < len(data) {
		if offset+2 > len(data) {
			break
		}

		flags := attribute.AttributeFlags(data[offset])
		code := data[offset+1]

		hdrLen := 3
		var valueLen int
		if flags.IsExtLength() {
			if offset+4 > len(data) {
				break
			}
			valueLen = int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
			hdrLen = 4
		} else {
			if offset+3 > len(data) {
				break
			}
			valueLen = int(data[offset+2])
		}

		if offset+hdrLen+valueLen > len(data) {
			break
		}

		if seen[code] {
			offset += hdrLen + valueLen
			continue
		}
		seen[code] = true

		value := data[offset+hdrLen : offset+hdrLen+valueLen]
		wf := wireFlags(flags)

		// Three codes are consumed rather than rendered under "attr". Every
		// other code goes through renderAttributeZe, and one whose octets it
		// cannot read is filed under its raw form rather than dropped.
		switch code {
		case 3: // NEXT_HOP (RFC 4271 Section 5.1.3)
			// Carried into the announce operation, not into "attr". A length
			// other than 4 is not a next hop, so it takes the raw form.
			if len(value) == 4 {
				var b textbuf.Buffer
				attrs["_next-hop"] = b.Reset().Uint8(value[0]).Byte('.').Uint8(value[1]).Byte('.').Uint8(value[2]).Byte('.').Uint8(value[3]).String()
				break
			}
			attrs[rawAttrKey(code)] = wf.wrap(hex.EncodeToString(value))
		case 14: // MP_REACH_NLRI
			mpReach = value
		case 15: // MP_UNREACH_NLRI
			mpUnreach = value
		default:
			key, rendered, understood := renderAttributeZe(code, value)
			switch {
			case !understood:
				attrs[rawAttrKey(code)] = wf.wrap(hex.EncodeToString(value))
			case key != "":
				attrs[key] = wf.wrap(rendered)
			}
		}

		offset += hdrLen + valueLen
	}

	return attrs, mpReach, mpUnreach
}

// renderAttributeZe renders one path attribute for `ze bgp decode` JSON. It
// returns the JSON key to file the attribute under, its value, and whether the
// octets were the shape the attribute requires.
//
// Three outcomes, and the third is what this decoder was missing. Octets it
// understood give a key and a value. Octets it understood that hold nothing to
// show give no key, and the attribute stays out of the output, which is how a
// route the local speaker originated keeps its empty AS_PATH out of "attr".
// Octets it did NOT understand, and every code with no arm here, give
// understood=false, and the caller files the attribute under its raw form.
//
// Codes 8, 25 and 32 reached no arm of this switch and no default until
// 2026-08-19, so each one vanished from the output in silence: an operator
// reading a capture was told nothing about a community the peer had sent.
func renderAttributeZe(code byte, value []byte) (key string, rendered any, understood bool) {
	switch code {
	case 1: // ORIGIN
		origins := []string{"igp", "egp", "incomplete"}
		if len(value) >= 1 && int(value[0]) < len(origins) {
			return "origin", origins[value[0]], true
		}
	case 2: // AS_PATH - Ze format uses simple array
		if asPath := parseASPathZe(value); len(asPath) > 0 {
			return "as-path", asPath, true
		}
		// RFC 4271 Section 5.1.2: a route the local speaker originated
		// carries an empty AS_PATH, and there is nothing to show for one.
		if len(value) == 0 {
			return "", nil, true
		}
	case 4: // MED
		if len(value) == 4 {
			return "med", binary.BigEndian.Uint32(value), true
		}
	case 5: // LOCAL_PREF
		if len(value) == 4 {
			return "local-preference", binary.BigEndian.Uint32(value), true
		}
	case 6: // ATOMIC_AGGREGATE
		return "atomic-aggregate", true, true
	case 7: // AGGREGATOR
		if len(value) == 6 {
			var b textbuf.Buffer
			ip := b.Reset().Uint8(value[2]).Byte('.').Uint8(value[3]).Byte('.').Uint8(value[4]).Byte('.').Uint8(value[5]).String()
			var b2 textbuf.Buffer
			return "aggregator", b2.Reset().Uint16(binary.BigEndian.Uint16(value[0:2])).Byte(':').Str(ip).String(), true
		}
		if len(value) == 8 {
			var b textbuf.Buffer
			ip := b.Reset().Uint8(value[4]).Byte('.').Uint8(value[5]).Byte('.').Uint8(value[6]).Byte('.').Uint8(value[7]).String()
			var b2 textbuf.Buffer
			return "aggregator", b2.Reset().Uint32(binary.BigEndian.Uint32(value[0:4])).Byte(':').Str(ip).String(), true
		}
	case 8: // COMMUNITIES (RFC 1997)
		if comms := parseCommunities(value); len(comms) > 0 {
			return "community", comms, true
		}
		if len(value) == 0 {
			return "", nil, true
		}
	case 9: // ORIGINATOR_ID
		if len(value) == 4 {
			var b textbuf.Buffer
			return "originator-id", b.Reset().Uint8(value[0]).Byte('.').Uint8(value[1]).Byte('.').Uint8(value[2]).Byte('.').Uint8(value[3]).String(), true
		}
	case 10: // CLUSTER_LIST
		if len(value)%4 != 0 {
			break
		}
		clusters := make([]string, 0, len(value)/4)
		for i := 0; i+4 <= len(value); i += 4 {
			var b textbuf.Buffer
			clusters = append(clusters, b.Reset().Uint8(value[i]).Byte('.').Uint8(value[i+1]).Byte('.').Uint8(value[i+2]).Byte('.').Uint8(value[i+3]).String())
		}
		if len(clusters) > 0 {
			return "cluster-list", clusters, true
		}
		return "", nil, true
	case 16: // EXTENDED_COMMUNITIES (RFC 4360)
		if extComms := parseExtendedCommunities(value); len(extComms) > 0 {
			return "extended-community", extComms, true
		}
		if len(value) == 0 {
			return "", nil, true
		}
	case 25: // IPV6_EXTENDED_COMMUNITIES (RFC 5701)
		if extComms := parseIPv6ExtendedCommunities(value); len(extComms) > 0 {
			return "ipv6-extended-community", extComms, true
		}
		if len(value) == 0 {
			return "", nil, true
		}
	case 29: // BGP-LS Attribute (RFC 7752 Section 3.3)
		if bgplsAttr := ls.AttrTLVsToJSON(value); len(bgplsAttr) > 0 {
			return "bgp-ls", bgplsAttr, true
		}
	case 32: // LARGE_COMMUNITIES (RFC 8092)
		if comms := parseLargeCommunities(value); len(comms) > 0 {
			return "large-community", comms, true
		}
		if len(value) == 0 {
			return "", nil, true
		}
	}

	return "", nil, false
}

// rawAttrKey names an attribute Ze does not render as "attr-<code>", the same
// spelling appendAttributeJSON (internal/component/bgp/format/text_json.go)
// gives it on the receive path. Its value is the attribute's octets as
// lowercase hex, which is what that function writes too, so an operator meets
// one form on both surfaces.
func rawAttrKey(code byte) string {
	var b textbuf.Buffer
	return b.Reset().Str("attr-").Uint8(code).String()
}

// wireFlags wraps an AttributeFlags for building flag-annotated attribute maps.
type wireFlags attribute.AttributeFlags

func (wf wireFlags) wrap(value any) map[string]any {
	f := attribute.AttributeFlags(wf)
	return map[string]any{
		"value":      value,
		"optional":   f.IsOptional(),
		"transitive": f.IsTransitive(),
		"partial":    f.IsPartial(),
	}
}

// parseASPathZe parses AS_PATH attribute value into Ze format (simple array).
func parseASPathZe(data []byte) []uint32 {
	var result []uint32
	offset := 0

	for offset < len(data) {
		if offset+2 > len(data) {
			break
		}

		segLen := int(data[offset+1])
		offset += 2

		// Try 4-byte ASNs first, then 2-byte
		asnSize := 4
		if offset+segLen*4 > len(data) {
			asnSize = 2
		}
		if offset+segLen*asnSize > len(data) {
			break
		}

		for range segLen {
			var asn uint32
			if asnSize == 4 {
				asn = binary.BigEndian.Uint32(data[offset : offset+4])
			} else {
				asn = uint32(binary.BigEndian.Uint16(data[offset : offset+2]))
			}
			result = append(result, asn)
			offset += asnSize
		}
	}

	return result
}
