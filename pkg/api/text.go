package api

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/exa-networks/zebgp/pkg/bgp/message"
)

// Encoding constants for process output formatting.
const (
	EncodingJSON = "json"
	EncodingText = "text"
)

// FormatMessage formats a RawMessage based on ContentConfig.
// Uses lazy parsing via AttrsWire when available for optimal performance.
// Handles encoding (json/text), format (parsed/raw/full), version (6/7), and attribute filtering.
func FormatMessage(peer PeerInfo, msg RawMessage, content ContentConfig) string {
	content = content.WithDefaults()

	// Get attribute filter (nil means all)
	filter := content.Attributes
	if filter == nil {
		all := NewFilterAll()
		filter = &all
	}

	// Get NLRI filter (nil means all)
	nlriFilter := content.NLRI
	if nlriFilter == nil {
		all := NewNLRIFilterAll()
		nlriFilter = &all
	}

	// For UPDATE messages, build FilterResult and use unified formatter
	if msg.Type == message.TypeUPDATE {
		// AttrsWire required for attribute parsing (needs valid context ID)
		// If nil, we can only extract NLRI from body structure
		result, err := filter.ApplyToUpdate(msg.AttrsWire, msg.RawBytes, *nlriFilter)
		if err != nil {
			return formatEmptyUpdate(peer, content)
		}

		return formatFromFilterResult(peer, msg, content, result)
	}

	// Non-UPDATE messages: format as raw
	return formatNonUpdate(peer, msg, content)
}

// formatEmptyUpdate formats an empty UPDATE message.
func formatEmptyUpdate(peer PeerInfo, content ContentConfig) string {
	if content.Encoding == EncodingJSON {
		if content.Version == APIVersionLegacy {
			return fmt.Sprintf(`{"type":"update","peer":{"address":{"peer":"%s"}},"announce":{}}`, peer.Address)
		}
		return fmt.Sprintf(`{"type":"update","peer":{"address":"%s","asn":%d},"announce":{}}`+"\n",
			peer.Address, peer.PeerAS)
	}
	if content.Version == APIVersionLegacy {
		return fmt.Sprintf("neighbor %s receive update\n", peer.Address)
	}
	return fmt.Sprintf("peer %s update\n", peer.Address)
}

// formatNonUpdate formats non-UPDATE messages (OPEN, NOTIFICATION, KEEPALIVE).
// Routes to dedicated formatters for parsed output, falls back to raw for unknown types.
func formatNonUpdate(peer PeerInfo, msg RawMessage, content ContentConfig) string {
	// For parsed format, use dedicated formatters
	if content.Format != FormatRaw {
		switch msg.Type { //nolint:exhaustive // only specific types have dedicated formatters
		case message.TypeOPEN:
			decoded := DecodeOpen(msg.RawBytes)
			return FormatOpen(peer.Address, decoded)
		case message.TypeNOTIFICATION:
			decoded := DecodeNotification(msg.RawBytes)
			return FormatNotification(peer.Address, decoded)
		case message.TypeKEEPALIVE:
			return FormatKeepalive(peer.Address)
		}
	}

	// Raw format or unknown type
	rawHex := fmt.Sprintf("%x", msg.RawBytes)

	if content.Encoding == EncodingJSON {
		return fmt.Sprintf(`{"type":"%s","peer":"%s","raw":"%s"}`+"\n",
			strings.ToLower(msg.Type.String()), peer.Address, rawHex)
	}
	if content.Version == APIVersionLegacy {
		return fmt.Sprintf("neighbor %s receive %s raw %s\n",
			peer.Address, strings.ToLower(msg.Type.String()), rawHex)
	}
	return fmt.Sprintf("peer %s %s raw %s\n",
		peer.Address, strings.ToLower(msg.Type.String()), rawHex)
}

// formatFromFilterResult formats UPDATE using lazy-parsed FilterResult.
// This is the optimized path that only parses requested attributes.
func formatFromFilterResult(peer PeerInfo, msg RawMessage, content ContentConfig, result FilterResult) string {
	switch content.Format {
	case FormatRaw:
		return formatRawFromResult(peer, msg, content)
	case FormatFull:
		return formatFullFromResult(peer, msg, content, result)
	default: // FormatParsed
		return formatParsedFromResult(peer, msg, content, result)
	}
}

// formatRawFromResult formats raw hex (doesn't need FilterResult attributes).
func formatRawFromResult(peer PeerInfo, msg RawMessage, content ContentConfig) string {
	rawHex := fmt.Sprintf("%x", msg.RawBytes)
	if content.Encoding == EncodingJSON {
		if content.Version == APIVersionLegacy {
			return fmt.Sprintf(`{"type":"%s","raw":"%s","peer":"%s"}`,
				msg.Type.String(), rawHex, peer.Address)
		}
		return fmt.Sprintf(`{"type":"update","peer":{"address":"%s","asn":%d},"raw":"%s"}`+"\n",
			peer.Address, peer.PeerAS, rawHex)
	}
	if content.Version == APIVersionLegacy {
		return fmt.Sprintf("neighbor %s receive raw %s %s\n",
			peer.Address, msg.Type.String(), rawHex)
	}
	return fmt.Sprintf("peer %s update raw %s\n", peer.Address, rawHex)
}

// formatParsedFromResult formats parsed UPDATE using FilterResult.
func formatParsedFromResult(peer PeerInfo, msg RawMessage, content ContentConfig, result FilterResult) string {
	if content.Encoding == EncodingJSON {
		return formatFilterResultJSON(peer, content, result, msg.UpdateID)
	}
	return formatFilterResultText(peer, content, result, msg.UpdateID)
}

// formatFullFromResult formats both parsed content AND raw hex.
func formatFullFromResult(peer PeerInfo, msg RawMessage, content ContentConfig, result FilterResult) string {
	rawHex := fmt.Sprintf("%x", msg.RawBytes)
	parsed := formatParsedFromResult(peer, msg, content, result)

	if content.Encoding == EncodingJSON {
		// Inject raw bytes into JSON: replace trailing "}\n" with ,"raw":"hex"}\n
		if strings.HasSuffix(parsed, "}\n") {
			return parsed[:len(parsed)-2] + fmt.Sprintf(`,"raw":"%s"}`+"\n", rawHex)
		}
		return parsed
	}

	// For text, append raw line
	if content.Version == APIVersionLegacy {
		return parsed + fmt.Sprintf("neighbor %s receive update raw %s\n", peer.Address, rawHex)
	}
	return parsed + fmt.Sprintf("peer %s update raw %s\n", peer.Address, rawHex)
}

// formatFilterResultJSON formats FilterResult as JSON.
func formatFilterResultJSON(peer PeerInfo, content ContentConfig, result FilterResult, updateID uint64) string {
	if content.Version == APIVersionLegacy {
		// V6 still uses extracted values for backward compat
		origin := getOriginFromResult(result)
		asPath := getASPathFromResult(result)
		localPref := getLocalPrefFromResult(result)
		med := getMEDFromResult(result)
		return formatFilterResultJSONV6(peer, result, origin, asPath, localPref, med)
	}
	return formatFilterResultJSONV7(peer, result, updateID)
}

// formatFilterResultJSONV6 formats FilterResult as v6 JSON.
func formatFilterResultJSONV6(peer PeerInfo, result FilterResult, origin string, asPath []uint32, localPref, med uint32) string {
	var sb strings.Builder
	sb.WriteString(`{"type":"update","peer":{"address":{"peer":"`)
	sb.WriteString(peer.Address.String())
	sb.WriteString(`"}}`)

	// Announced routes
	if len(result.Announced) > 0 {
		sb.WriteString(`,"announce":{`)
		// Group by family
		ipv4, ipv6 := groupPrefixesByFamily(result.Announced)
		first := true
		if len(ipv4) > 0 {
			sb.WriteString(`"ipv4 unicast":{`)
			sb.WriteString(formatPrefixesJSON(ipv4, result.NextHopIPv4, origin, asPath, localPref, med))
			sb.WriteString(`}`)
			first = false
		}
		if len(ipv6) > 0 {
			if !first {
				sb.WriteString(`,`)
			}
			sb.WriteString(`"ipv6 unicast":{`)
			sb.WriteString(formatPrefixesJSON(ipv6, result.NextHopIPv6, origin, asPath, localPref, med))
			sb.WriteString(`}`)
		}
		sb.WriteString(`}`)
	}

	// Withdrawn routes
	if len(result.Withdrawn) > 0 {
		sb.WriteString(`,"withdraw":{`)
		ipv4, ipv6 := groupPrefixesByFamily(result.Withdrawn)
		first := true
		if len(ipv4) > 0 {
			sb.WriteString(`"ipv4 unicast":["`)
			sb.WriteString(joinPrefixesQuoted(ipv4))
			sb.WriteString(`"]`)
			first = false
		}
		if len(ipv6) > 0 {
			if !first {
				sb.WriteString(`,`)
			}
			sb.WriteString(`"ipv6 unicast":["`)
			sb.WriteString(joinPrefixesQuoted(ipv6))
			sb.WriteString(`"]`)
		}
		sb.WriteString(`}`)
	}

	sb.WriteString("}\n")
	return sb.String()
}

// formatFilterResultJSONV7 formats FilterResult as v7 JSON.
func formatFilterResultJSONV7(peer PeerInfo, result FilterResult, updateID uint64) string {
	var sb strings.Builder
	sb.WriteString(`{"type":"update"`)

	// Include update-id if set
	if updateID > 0 {
		sb.WriteString(fmt.Sprintf(`,"update-id":%d`, updateID))
	}

	sb.WriteString(`,"peer":{"address":"`)
	sb.WriteString(peer.Address.String())
	sb.WriteString(`","asn":`)
	sb.WriteString(fmt.Sprintf("%d", peer.PeerAS))
	sb.WriteString(`}`)

	// Announced routes
	if len(result.Announced) > 0 {
		sb.WriteString(`,"announce":{`)

		// Attributes first (only what filter requested)
		needComma := formatAttributesJSONV7(&sb, result)

		// Group prefixes by family, indexed by next-hop
		ipv4, ipv6 := groupPrefixesByFamily(result.Announced)

		if len(ipv4) > 0 {
			if needComma {
				sb.WriteString(",")
			}
			sb.WriteString(`"ipv4 unicast":{"`)
			sb.WriteString(result.NextHopIPv4.String())
			sb.WriteString(`":[`)
			for i, p := range ipv4 {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(`"`)
				sb.WriteString(p.String())
				sb.WriteString(`"`)
			}
			sb.WriteString("]}")
			needComma = true
		}

		if len(ipv6) > 0 {
			if needComma {
				sb.WriteString(",")
			}
			sb.WriteString(`"ipv6 unicast":{"`)
			sb.WriteString(result.NextHopIPv6.String())
			sb.WriteString(`":[`)
			for i, p := range ipv6 {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(`"`)
				sb.WriteString(p.String())
				sb.WriteString(`"`)
			}
			sb.WriteString("]}")
		}

		sb.WriteString(`}`)
	}

	// Withdrawn routes - no attributes, just family -> [prefixes]
	if len(result.Withdrawn) > 0 {
		sb.WriteString(`,"withdraw":{`)
		ipv4, ipv6 := groupPrefixesByFamily(result.Withdrawn)
		first := true
		if len(ipv4) > 0 {
			sb.WriteString(`"ipv4 unicast":[`)
			for i, p := range ipv4 {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(`"`)
				sb.WriteString(p.String())
				sb.WriteString(`"`)
			}
			sb.WriteString("]")
			first = false
		}
		if len(ipv6) > 0 {
			if !first {
				sb.WriteString(",")
			}
			sb.WriteString(`"ipv6 unicast":[`)
			for i, p := range ipv6 {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(`"`)
				sb.WriteString(p.String())
				sb.WriteString(`"`)
			}
			sb.WriteString("]")
		}
		sb.WriteString(`}`)
	}

	sb.WriteString("}\n")
	return sb.String()
}

// formatAttributesJSONV7 formats attributes from FilterResult for v7 JSON.
// Returns true if any attributes were written (for comma handling).
func formatAttributesJSONV7(sb *strings.Builder, result FilterResult) bool {
	if len(result.Attributes) == 0 {
		return false
	}

	first := true
	for code, attr := range result.Attributes {
		if !first {
			sb.WriteString(",")
		}
		first = false
		formatAttributeJSONV7(sb, code, attr)
	}
	return true
}

// formatAttributeJSONV7 formats a single attribute for v7 JSON.
func formatAttributeJSONV7(sb *strings.Builder, code attribute.AttributeCode, attr attribute.Attribute) {
	switch code { //nolint:exhaustive // default handles unknown codes as attr-N
	case attribute.AttrOrigin:
		if o, ok := attr.(*attribute.Origin); ok {
			sb.WriteString(`"origin":"`)
			sb.WriteString(strings.ToLower(o.String()))
			sb.WriteString(`"`)
		} else if o, ok := attr.(attribute.Origin); ok {
			sb.WriteString(`"origin":"`)
			sb.WriteString(strings.ToLower(o.String()))
			sb.WriteString(`"`)
		}
	case attribute.AttrASPath:
		if ap, ok := attr.(*attribute.ASPath); ok {
			sb.WriteString(`"as-path":[`)
			first := true
			for _, seg := range ap.Segments {
				for _, asn := range seg.ASNs {
					if !first {
						sb.WriteString(",")
					}
					first = false
					fmt.Fprintf(sb, "%d", asn)
				}
			}
			sb.WriteString("]")
		}
	case attribute.AttrMED:
		if m, ok := attr.(*attribute.MED); ok {
			fmt.Fprintf(sb, `"med":%d`, uint32(*m))
		} else if m, ok := attr.(attribute.MED); ok {
			fmt.Fprintf(sb, `"med":%d`, uint32(m))
		}
	case attribute.AttrLocalPref:
		if lp, ok := attr.(*attribute.LocalPref); ok {
			fmt.Fprintf(sb, `"local-preference":%d`, uint32(*lp))
		} else if lp, ok := attr.(attribute.LocalPref); ok {
			fmt.Fprintf(sb, `"local-preference":%d`, uint32(lp))
		}
	case attribute.AttrCommunity:
		if c, ok := attr.(*attribute.Communities); ok {
			sb.WriteString(`"communities":[`)
			for i, comm := range *c {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(`"`)
				sb.WriteString(comm.String())
				sb.WriteString(`"`)
			}
			sb.WriteString("]")
		}
	case attribute.AttrLargeCommunity:
		if lc, ok := attr.(*attribute.LargeCommunities); ok {
			sb.WriteString(`"large-communities":[`)
			for i, comm := range *lc {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(`"`)
				sb.WriteString(comm.String())
				sb.WriteString(`"`)
			}
			sb.WriteString("]")
		}
	case attribute.AttrExtCommunity:
		if ec, ok := attr.(*attribute.ExtendedCommunities); ok {
			sb.WriteString(`"extended-communities":[`)
			for i, comm := range *ec {
				if i > 0 {
					sb.WriteString(",")
				}
				fmt.Fprintf(sb, `"%x"`, comm[:])
			}
			sb.WriteString("]")
		}
	default:
		// Unknown attribute: attr-N as hex
		fmt.Fprintf(sb, `"attr-%d":"%x"`, code, attr.Pack())
	}
}

// formatFilterResultText formats FilterResult as text.
func formatFilterResultText(peer PeerInfo, content ContentConfig, result FilterResult, updateID uint64) string {
	if content.Version == APIVersionLegacy {
		// V6 still uses extracted values for backward compat
		origin := getOriginFromResult(result)
		asPath := getASPathFromResult(result)
		localPref := getLocalPrefFromResult(result)
		med := getMEDFromResult(result)
		return formatFilterResultTextV6(peer, result, origin, asPath, localPref, med)
	}
	return formatFilterResultTextV7(peer, result, updateID)
}

// formatFilterResultTextV6 formats FilterResult as v6 text.
func formatFilterResultTextV6(peer PeerInfo, result FilterResult, origin string, asPath []uint32, localPref, med uint32) string {
	var sb strings.Builder
	prefix := fmt.Sprintf("neighbor %s receive update", peer.Address)

	sb.WriteString(prefix)
	sb.WriteString(" start\n")

	// Announced routes
	for _, p := range result.Announced {
		sb.WriteString(prefix)
		sb.WriteString(" announced ")
		sb.WriteString(p.String())
		sb.WriteString(" next-hop ")
		sb.WriteString(result.NextHopFor(p).String())
		formatAttributesText(&sb, origin, asPath, localPref, med)
		sb.WriteString("\n")
	}

	// Withdrawn routes
	for _, p := range result.Withdrawn {
		sb.WriteString(prefix)
		sb.WriteString(" withdrawn ")
		sb.WriteString(p.String())
		sb.WriteString("\n")
	}

	sb.WriteString(prefix)
	sb.WriteString(" end\n")

	return sb.String()
}

// formatFilterResultTextV7 formats FilterResult as v7 text.
func formatFilterResultTextV7(peer PeerInfo, result FilterResult, updateID uint64) string {
	var sb strings.Builder

	// Build prefix: peer <ip> asn <asn> update <id>
	prefix := fmt.Sprintf("peer %s asn %d update %d", peer.Address, peer.PeerAS, updateID)

	// Announced routes - group by family and next-hop
	if len(result.Announced) > 0 {
		sb.WriteString(prefix)
		sb.WriteString(" announce")

		// Attributes first (shared) - only what filter requested
		formatAttributesTextV7(&sb, result)

		// Group prefixes by family
		ipv4, ipv6 := groupPrefixesByFamily(result.Announced)

		if len(ipv4) > 0 {
			sb.WriteString(" ipv4 unicast next-hop ")
			sb.WriteString(result.NextHopIPv4.String())
			sb.WriteString(" nlri")
			for _, p := range ipv4 {
				sb.WriteString(" ")
				sb.WriteString(p.String())
			}
		}

		if len(ipv6) > 0 {
			sb.WriteString(" ipv6 unicast next-hop ")
			sb.WriteString(result.NextHopIPv6.String())
			sb.WriteString(" nlri")
			for _, p := range ipv6 {
				sb.WriteString(" ")
				sb.WriteString(p.String())
			}
		}

		sb.WriteString("\n")
	}

	// Withdrawn routes - no attributes
	if len(result.Withdrawn) > 0 {
		sb.WriteString(prefix)
		sb.WriteString(" withdraw")

		// Group prefixes by family
		ipv4, ipv6 := groupPrefixesByFamily(result.Withdrawn)

		if len(ipv4) > 0 {
			sb.WriteString(" ipv4 unicast nlri")
			for _, p := range ipv4 {
				sb.WriteString(" ")
				sb.WriteString(p.String())
			}
		}

		if len(ipv6) > 0 {
			sb.WriteString(" ipv6 unicast nlri")
			for _, p := range ipv6 {
				sb.WriteString(" ")
				sb.WriteString(p.String())
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// formatAttributesTextV7 formats attributes from FilterResult for v7 text.
// Only outputs what's in result.Attributes (lazy parsing - filter controls what's parsed).
func formatAttributesTextV7(sb *strings.Builder, result FilterResult) {
	for code, attr := range result.Attributes {
		sb.WriteString(" ")
		formatAttributeTextV7(sb, code, attr)
	}
}

// formatAttributeTextV7 formats a single attribute for v7 text.
func formatAttributeTextV7(sb *strings.Builder, code attribute.AttributeCode, attr attribute.Attribute) {
	switch code { //nolint:exhaustive // default handles unknown codes as attr-N
	case attribute.AttrOrigin:
		if o, ok := attr.(*attribute.Origin); ok {
			sb.WriteString("origin ")
			sb.WriteString(strings.ToLower(o.String()))
		} else if o, ok := attr.(attribute.Origin); ok {
			sb.WriteString("origin ")
			sb.WriteString(strings.ToLower(o.String()))
		}
	case attribute.AttrASPath:
		if ap, ok := attr.(*attribute.ASPath); ok {
			sb.WriteString("as-path")
			for _, seg := range ap.Segments {
				for _, asn := range seg.ASNs {
					fmt.Fprintf(sb, " %d", asn)
				}
			}
		}
	case attribute.AttrNextHop:
		if nh, ok := attr.(*attribute.NextHop); ok {
			sb.WriteString("next-hop ")
			sb.WriteString(nh.Addr.String())
		}
	case attribute.AttrMED:
		if m, ok := attr.(*attribute.MED); ok {
			fmt.Fprintf(sb, "med %d", uint32(*m))
		} else if m, ok := attr.(attribute.MED); ok {
			fmt.Fprintf(sb, "med %d", uint32(m))
		}
	case attribute.AttrLocalPref:
		if lp, ok := attr.(*attribute.LocalPref); ok {
			fmt.Fprintf(sb, "local-preference %d", uint32(*lp))
		} else if lp, ok := attr.(attribute.LocalPref); ok {
			fmt.Fprintf(sb, "local-preference %d", uint32(lp))
		}
	case attribute.AttrCommunity:
		if c, ok := attr.(*attribute.Communities); ok {
			sb.WriteString("community [")
			for i, comm := range *c {
				if i > 0 {
					sb.WriteString(" ")
				}
				sb.WriteString(comm.String())
			}
			sb.WriteString("]")
		}
	case attribute.AttrLargeCommunity:
		if lc, ok := attr.(*attribute.LargeCommunities); ok {
			sb.WriteString("large-community [")
			for i, comm := range *lc {
				if i > 0 {
					sb.WriteString(" ")
				}
				sb.WriteString(comm.String())
			}
			sb.WriteString("]")
		}
	case attribute.AttrExtCommunity:
		if ec, ok := attr.(*attribute.ExtendedCommunities); ok {
			sb.WriteString("extended-community [")
			for i, comm := range *ec {
				if i > 0 {
					sb.WriteString(" ")
				}
				fmt.Fprintf(sb, "%x", comm[:])
			}
			sb.WriteString("]")
		}
	default:
		// Unknown attribute: attr-N hex
		fmt.Fprintf(sb, "attr-%d %x", code, attr.Pack())
	}
}

// Helper functions for extracting attributes from FilterResult

func getOriginFromResult(result FilterResult) string {
	if attr, ok := result.Attributes[attribute.AttrOrigin]; ok {
		// Handle both pointer and value types
		switch o := attr.(type) {
		case *attribute.Origin:
			return strings.ToLower(o.String())
		case attribute.Origin:
			return strings.ToLower(o.String())
		}
	}
	return ""
}

func getASPathFromResult(result FilterResult) []uint32 {
	if attr, ok := result.Attributes[attribute.AttrASPath]; ok {
		if ap, ok := attr.(*attribute.ASPath); ok {
			var path []uint32
			for _, seg := range ap.Segments {
				path = append(path, seg.ASNs...)
			}
			return path
		}
	}
	return nil
}

func getLocalPrefFromResult(result FilterResult) uint32 {
	if attr, ok := result.Attributes[attribute.AttrLocalPref]; ok {
		// Handle both pointer and value types
		switch lp := attr.(type) {
		case *attribute.LocalPref:
			return uint32(*lp)
		case attribute.LocalPref:
			return uint32(lp)
		}
	}
	return 0
}

func getMEDFromResult(result FilterResult) uint32 {
	if attr, ok := result.Attributes[attribute.AttrMED]; ok {
		// Handle both pointer and value types
		switch m := attr.(type) {
		case *attribute.MED:
			return uint32(*m)
		case attribute.MED:
			return uint32(m)
		}
	}
	return 0
}

// formatAttributesText appends attribute text to builder.
func formatAttributesText(sb *strings.Builder, origin string, asPath []uint32, localPref, med uint32) {
	if origin != "" {
		sb.WriteString(" origin ")
		sb.WriteString(origin)
	}
	if localPref > 0 {
		fmt.Fprintf(sb, " local-preference %d", localPref)
	}
	if med > 0 {
		fmt.Fprintf(sb, " med %d", med)
	}
	if len(asPath) > 0 {
		sb.WriteString(" as-path [")
		for i, asn := range asPath {
			if i > 0 {
				sb.WriteString(" ")
			}
			fmt.Fprintf(sb, "%d", asn)
		}
		sb.WriteString("]")
	}
}

// groupPrefixesByFamily separates prefixes into IPv4 and IPv6.
func groupPrefixesByFamily(prefixes []netip.Prefix) (ipv4, ipv6 []netip.Prefix) {
	for _, p := range prefixes {
		if p.Addr().Is4() {
			ipv4 = append(ipv4, p)
		} else {
			ipv6 = append(ipv6, p)
		}
	}
	return ipv4, ipv6
}

// joinPrefixesQuoted joins prefixes as quoted JSON strings.
func joinPrefixesQuoted(prefixes []netip.Prefix) string {
	strs := make([]string, len(prefixes))
	for i, p := range prefixes {
		strs[i] = p.String()
	}
	return strings.Join(strs, `","`)
}

// formatPrefixesJSON formats prefixes with attributes as JSON object.
func formatPrefixesJSON(prefixes []netip.Prefix, nextHop netip.Addr, origin string, asPath []uint32, localPref, med uint32) string {
	var sb strings.Builder

	// Format as: "prefix": {"next-hop": "x", "origin": "y", ...}
	for i, p := range prefixes {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`"`)
		sb.WriteString(p.String())
		sb.WriteString(`":{"next-hop":"`)
		sb.WriteString(nextHop.String())
		sb.WriteString(`"`)

		if origin != "" {
			sb.WriteString(`,"origin":"`)
			sb.WriteString(origin)
			sb.WriteString(`"`)
		}
		if len(asPath) > 0 {
			sb.WriteString(`,"as-path":[`)
			for j, asn := range asPath {
				if j > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(fmt.Sprintf("%d", asn))
			}
			sb.WriteString("]")
		}
		if localPref > 0 {
			sb.WriteString(fmt.Sprintf(`,"local-preference":%d`, localPref))
		}
		if med > 0 {
			sb.WriteString(fmt.Sprintf(`,"med":%d`, med))
		}
		sb.WriteString("}")
	}

	return sb.String()
}

// FormatOpen formats an OPEN message as ExaBGP text encoder output.
// Format: neighbor <ip> receive open version <v> asn <asn> hold_time <t> router_id <id> capabilities [...].
func FormatOpen(peerAddr netip.Addr, open DecodedOpen) string {
	capsStr := "[]"
	if len(open.Capabilities) > 0 {
		capsStr = "[" + strings.Join(open.Capabilities, ", ") + "]"
	}
	return fmt.Sprintf("neighbor %s receive open version %d asn %d hold_time %d router_id %s capabilities %s\n",
		peerAddr, open.Version, open.ASN, open.HoldTime, open.RouterID, capsStr)
}

// FormatNotification formats a NOTIFICATION message as ExaBGP text encoder output.
// Format: neighbor <ip> receive notification code <c> subcode <s> data <hex> [name].
func FormatNotification(peerAddr netip.Addr, notify DecodedNotification) string {
	// ExaBGP format: code {num} subcode {num} data {hex}
	// We add human-readable names at the end as extension
	dataHex := ""
	if len(notify.Data) > 0 {
		dataHex = fmt.Sprintf("%x", notify.Data)
	}

	base := fmt.Sprintf("neighbor %s receive notification code %d subcode %d data %s",
		peerAddr, notify.ErrorCode, notify.ErrorSubcode, dataHex)

	// Add human-readable names
	names := fmt.Sprintf(" [%s/%s]", notify.ErrorCodeName, notify.ErrorSubcodeName)

	return base + names + "\n"
}

// FormatKeepalive formats a KEEPALIVE message as ExaBGP text encoder output.
// Format: neighbor <ip> receive keepalive.
func FormatKeepalive(peerAddr netip.Addr) string {
	return fmt.Sprintf("neighbor %s receive keepalive\n", peerAddr)
}

// FormatStateChange formats a peer state change event.
// State events are separate from BGP protocol messages.
// Common states: "up", "down", "connected", "established".
func FormatStateChange(peer PeerInfo, state string, encoding string) string {
	if encoding == EncodingJSON {
		return formatStateChangeJSON(peer, state)
	}
	return formatStateChangeText(peer, state)
}

func formatStateChangeJSON(peer PeerInfo, state string) string {
	// Manual JSON construction to match ExaBGP format
	return fmt.Sprintf(`{"type":"state","peer":{"address":"%s","asn":%d},"state":"%s"}`+"\n",
		peer.Address, peer.PeerAS, state)
}

func formatStateChangeText(peer PeerInfo, state string) string {
	return fmt.Sprintf("neighbor %s state %s\n", peer.Address, state)
}
