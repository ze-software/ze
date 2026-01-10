package plugin

import (
	"fmt"
	"net/netip"
	"strings"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/zebgp/pkg/bgp/context"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/message"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
)

// Encoding constants for process output formatting.
const (
	EncodingJSON = "json"
	EncodingText = "text"
)

// FormatMessage formats a RawMessage based on ContentConfig.
// Uses lazy parsing via AttrsWire when available for optimal performance.
// Handles encoding (json/text), format (parsed/raw/full), and attribute filtering.
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

		// Get encoding context for ADD-PATH state
		var encCtx *bgpctx.EncodingContext
		if msg.AttrsWire != nil {
			encCtx = bgpctx.Registry.Get(msg.AttrsWire.SourceContext())
		}

		return formatFromFilterResult(peer, msg, content, result, encCtx)
	}

	// Non-UPDATE messages: format as raw
	return formatNonUpdate(peer, msg, content)
}

// formatEmptyUpdate formats an empty UPDATE message.
func formatEmptyUpdate(peer PeerInfo, content ContentConfig) string {
	if content.Encoding == EncodingJSON {
		return fmt.Sprintf(`{"message":{"type":"update"},"peer":{"address":"%s","asn":%d},"announce":{}}`+"\n",
			peer.Address, peer.PeerAS)
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
			return FormatOpen(peer, decoded, msg.Direction, msg.MessageID)
		case message.TypeNOTIFICATION:
			decoded := DecodeNotification(msg.RawBytes)
			return FormatNotification(peer, decoded, msg.Direction, msg.MessageID)
		case message.TypeKEEPALIVE:
			return FormatKeepalive(peer, msg.Direction, msg.MessageID)
		}
	}

	// Raw format or unknown type
	rawHex := fmt.Sprintf("%x", msg.RawBytes)

	if content.Encoding == EncodingJSON {
		return fmt.Sprintf(`{"message":{"type":"%s"},"peer":"%s","raw":"%s"}`+"\n",
			strings.ToLower(msg.Type.String()), peer.Address, rawHex)
	}
	return fmt.Sprintf("peer %s %s raw %s\n",
		peer.Address, strings.ToLower(msg.Type.String()), rawHex)
}

// formatFromFilterResult formats UPDATE using lazy-parsed FilterResult.
// This is the optimized path that only parses requested attributes.
// ctx provides ADD-PATH state per family (nil means no ADD-PATH).
func formatFromFilterResult(peer PeerInfo, msg RawMessage, content ContentConfig, result FilterResult, ctx *bgpctx.EncodingContext) string {
	switch content.Format {
	case FormatRaw:
		return formatRawFromResult(peer, msg, content)
	case FormatFull:
		return formatFullFromResult(peer, msg, content, result, ctx)
	default: // FormatParsed
		return formatParsedFromResult(peer, msg, content, result, ctx)
	}
}

// formatRawFromResult formats raw hex (doesn't need FilterResult attributes).
func formatRawFromResult(peer PeerInfo, msg RawMessage, content ContentConfig) string {
	rawHex := fmt.Sprintf("%x", msg.RawBytes)
	if content.Encoding == EncodingJSON {
		return fmt.Sprintf(`{"message":{"type":"update"},"direction":"%s","peer":{"address":"%s","asn":%d},"raw":"%s"}`+"\n",
			msg.Direction, peer.Address, peer.PeerAS, rawHex)
	}
	return fmt.Sprintf("peer %s %s update raw %s\n", peer.Address, msg.Direction, rawHex)
}

// formatParsedFromResult formats parsed UPDATE using FilterResult.
// ctx provides ADD-PATH state per family.
func formatParsedFromResult(peer PeerInfo, msg RawMessage, content ContentConfig, result FilterResult, ctx *bgpctx.EncodingContext) string {
	if content.Encoding == EncodingJSON {
		return formatFilterResultJSON(peer, result, msg.MessageID, msg.Direction, ctx)
	}
	return formatFilterResultText(peer, result, msg.MessageID, msg.Direction, ctx)
}

// formatFullFromResult formats both parsed content AND raw hex.
// ctx provides ADD-PATH state per family.
func formatFullFromResult(peer PeerInfo, msg RawMessage, content ContentConfig, result FilterResult, ctx *bgpctx.EncodingContext) string {
	rawHex := fmt.Sprintf("%x", msg.RawBytes)
	parsed := formatParsedFromResult(peer, msg, content, result, ctx)

	if content.Encoding == EncodingJSON {
		// Inject raw bytes into JSON: replace trailing "}\n" with ,"raw":"hex"}\n
		if strings.HasSuffix(parsed, "}\n") {
			return parsed[:len(parsed)-2] + fmt.Sprintf(`,"raw":"%s"}`+"\n", rawHex)
		}
		return parsed
	}

	// For text, append raw line
	return parsed + fmt.Sprintf("peer %s %s update raw %s\n", peer.Address, msg.Direction, rawHex)
}

// formatFilterResultJSON formats FilterResult as JSON.
// Uses AnnouncedByFamily()/WithdrawnByFamily() for RFC 4760-correct next-hop per family.
// ctx provides ADD-PATH state per family.
func formatFilterResultJSON(peer PeerInfo, result FilterResult, msgID uint64, direction string, ctx *bgpctx.EncodingContext) string {
	var sb strings.Builder

	// Message wrapper with type and optional id
	sb.WriteString(`{"message":{"type":"update"`)
	if msgID > 0 {
		sb.WriteString(fmt.Sprintf(`,"id":%d`, msgID))
	}
	sb.WriteString(`}`)

	// Include direction
	if direction != "" {
		sb.WriteString(`,"direction":"`)
		sb.WriteString(direction)
		sb.WriteString(`"`)
	}

	sb.WriteString(`,"peer":{"address":"`)
	sb.WriteString(peer.Address.String())
	sb.WriteString(`","asn":`)
	sb.WriteString(fmt.Sprintf("%d", peer.PeerAS))
	sb.WriteString(`}`)

	// Attributes at top level (not inside announce - that breaks JSON parsing for RIB plugin)
	if len(result.Attributes) > 0 {
		sb.WriteString(",")
		formatAttributesJSON(&sb, result)
	}

	// Announced routes - grouped by family with per-family next-hop
	announced := result.AnnouncedByFamily(ctx)
	if len(announced) > 0 {
		sb.WriteString(`,"announce":{`)

		first := true
		for _, fam := range announced {
			if !first {
				sb.WriteString(",")
			}
			// Family name (e.g., "ipv4/unicast")
			familyKey := fam.Family
			sb.WriteString(`"`)
			sb.WriteString(familyKey)
			sb.WriteString(`":{"`)
			sb.WriteString(fam.NextHop.String())
			sb.WriteString(`":[`)
			for i, n := range fam.NLRIs {
				if i > 0 {
					sb.WriteString(",")
				}
				formatNLRIJSON(&sb, n)
			}
			sb.WriteString("]}")
			first = false
		}

		sb.WriteString(`}`)
	}

	// Withdrawn routes - no attributes, just family -> [nlris]
	withdrawn := result.WithdrawnByFamily(ctx)
	if len(withdrawn) > 0 {
		sb.WriteString(`,"withdraw":{`)
		first := true
		for _, fam := range withdrawn {
			if !first {
				sb.WriteString(",")
			}
			familyKey := fam.Family
			sb.WriteString(`"`)
			sb.WriteString(familyKey)
			sb.WriteString(`":[`)
			for i, n := range fam.NLRIs {
				if i > 0 {
					sb.WriteString(",")
				}
				formatNLRIJSON(&sb, n)
			}
			sb.WriteString("]")
			first = false
		}
		sb.WriteString(`}`)
	}

	sb.WriteString("}\n")
	return sb.String()
}

// prefixer is implemented by NLRI types that have a Prefix() method.
type prefixer interface {
	Prefix() netip.Prefix
}

// formatNLRIJSON formats a single NLRI as JSON.
// RFC 7911: Outputs structured format with path-id when present.
// Format: {"prefix":"10.0.0.0/24"} or {"prefix":"10.0.0.0/24","path-id":1}.
func formatNLRIJSON(sb *strings.Builder, n nlri.NLRI) {
	sb.WriteString(`{"prefix":"`)

	// Use type assertion to get prefix cleanly
	if p, ok := n.(prefixer); ok {
		sb.WriteString(p.Prefix().String())
	} else {
		// Fallback for complex NLRI types (EVPN, FlowSpec, etc.)
		// Escape for JSON safety (handles quotes, backslashes, control chars)
		writeJSONEscapedString(sb, n.String())
	}
	sb.WriteString(`"`)

	if pathID := n.PathID(); pathID != 0 {
		fmt.Fprintf(sb, `,"path-id":%d`, pathID)
	}

	sb.WriteString(`}`)
}

// writeJSONEscapedString writes s to sb with JSON string escaping.
// Escapes: \ " and control characters (0x00-0x1F).
func writeJSONEscapedString(sb *strings.Builder, s string) {
	for _, r := range s {
		switch r {
		case '\\':
			sb.WriteString(`\\`)
		case '"':
			sb.WriteString(`\"`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			if r < 0x20 {
				// Control character - use \uXXXX
				fmt.Fprintf(sb, `\u%04x`, r)
			} else {
				sb.WriteRune(r)
			}
		}
	}
}

// formatAttributesJSON formats attributes from FilterResult for JSON.
// Returns true if any attributes were written (for comma handling).
func formatAttributesJSON(sb *strings.Builder, result FilterResult) bool {
	if len(result.Attributes) == 0 {
		return false
	}

	first := true
	for code, attr := range result.Attributes {
		if !first {
			sb.WriteString(",")
		}
		first = false
		formatAttributeJSON(sb, code, attr)
	}
	return true
}

// formatAttributeJSON formats a single attribute for JSON.
func formatAttributeJSON(sb *strings.Builder, code attribute.AttributeCode, attr attribute.Attribute) {
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
		attrBuf := make([]byte, attr.Len())
		attr.WriteTo(attrBuf, 0)
		fmt.Fprintf(sb, `"attr-%d":"%x"`, code, attrBuf)
	}
}

// formatFilterResultText formats FilterResult as text.
// Uses AnnouncedByFamily()/WithdrawnByFamily() for RFC 4760-correct next-hop per family.
// ctx provides ADD-PATH state per family.
func formatFilterResultText(peer PeerInfo, result FilterResult, msgID uint64, direction string, ctx *bgpctx.EncodingContext) string {
	var sb strings.Builder

	// Build prefix: peer <ip> <direction> update <id>
	prefix := fmt.Sprintf("peer %s %s update %d", peer.Address, direction, msgID)

	// Announced routes - grouped by family with per-family next-hop
	announced := result.AnnouncedByFamily(ctx)
	if len(announced) > 0 {
		sb.WriteString(prefix)
		sb.WriteString(" announce")

		// Attributes first (shared) - only what filter requested
		formatAttributesText(&sb, result)

		for _, fam := range announced {
			// Family name with space (e.g., "ipv4/unicast")
			familyKey := fam.Family
			sb.WriteString(" ")
			sb.WriteString(familyKey)
			sb.WriteString(" next-hop ")
			sb.WriteString(fam.NextHop.String())
			sb.WriteString(" nlri")
			for _, n := range fam.NLRIs {
				sb.WriteString(" ")
				sb.WriteString(n.String())
			}
		}

		sb.WriteString("\n")
	}

	// Withdrawn routes - no attributes
	withdrawn := result.WithdrawnByFamily(ctx)
	if len(withdrawn) > 0 {
		sb.WriteString(prefix)
		sb.WriteString(" withdraw")

		for _, fam := range withdrawn {
			familyKey := fam.Family
			sb.WriteString(" ")
			sb.WriteString(familyKey)
			sb.WriteString(" nlri")
			for _, n := range fam.NLRIs {
				sb.WriteString(" ")
				sb.WriteString(n.String())
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// formatAttributesText formats attributes from FilterResult for text output.
// Only outputs what's in result.Attributes (lazy parsing - filter controls what's parsed).
func formatAttributesText(sb *strings.Builder, result FilterResult) {
	for code, attr := range result.Attributes {
		sb.WriteString(" ")
		formatAttributeText(sb, code, attr)
	}
}

// formatAttributeText formats a single attribute for text output.
func formatAttributeText(sb *strings.Builder, code attribute.AttributeCode, attr attribute.Attribute) {
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
		attrBuf := make([]byte, attr.Len())
		attr.WriteTo(attrBuf, 0)
		fmt.Fprintf(sb, "attr-%d %x", code, attrBuf)
	}
}

// FormatOpen formats an OPEN message as text output.
// Format: peer <ip> <direction> open <msg-id> asn <asn> router-id <id> hold-time <t> [cap <code> <name> <value>]...
// ASN is the speaker's ASN (from the OPEN message).
// Capabilities use "cap <code> <name> <value>" format for easy parsing.
func FormatOpen(peer PeerInfo, open DecodedOpen, direction string, msgID uint64) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("peer %s %s open %d asn %d router-id %s hold-time %d",
		peer.Address, direction, msgID, open.ASN, open.RouterID, open.HoldTime))

	for _, cap := range open.Capabilities {
		if cap.Value != "" {
			fmt.Fprintf(&sb, " cap %d %s %s", cap.Code, cap.Name, cap.Value)
		} else {
			fmt.Fprintf(&sb, " cap %d %s", cap.Code, cap.Name)
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

// FormatNotification formats a NOTIFICATION message as text output.
// Format: peer <ip> <direction> notification <msg-id> code <n> subcode <n> code-name <name> subcode-name <name> data <hex>.
// Names are hyphenated for single-word parsing (e.g., "Administrative-Shutdown").
func FormatNotification(peer PeerInfo, notify DecodedNotification, direction string, msgID uint64) string {
	dataHex := ""
	if len(notify.Data) > 0 {
		dataHex = fmt.Sprintf("%x", notify.Data)
	}

	// Replace spaces with hyphens in names for easier parsing
	codeName := strings.ReplaceAll(notify.ErrorCodeName, " ", "-")
	subcodeName := strings.ReplaceAll(notify.ErrorSubcodeName, " ", "-")

	return fmt.Sprintf("peer %s %s notification %d code %d subcode %d code-name %s subcode-name %s data %s\n",
		peer.Address, direction, msgID, notify.ErrorCode, notify.ErrorSubcode,
		codeName, subcodeName, dataHex)
}

// FormatKeepalive formats a KEEPALIVE message as text output.
// Format: peer <ip> <direction> keepalive <msg-id>.
func FormatKeepalive(peer PeerInfo, direction string, msgID uint64) string {
	return fmt.Sprintf("peer %s %s keepalive %d\n", peer.Address, direction, msgID)
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
	// Manual JSON construction with message wrapper
	return fmt.Sprintf(`{"message":{"type":"state"},"peer":{"address":"%s","asn":%d},"state":"%s"}`+"\n",
		peer.Address, peer.PeerAS, state)
}

func formatStateChangeText(peer PeerInfo, state string) string {
	return fmt.Sprintf("peer %s asn %d state %s\n", peer.Address, peer.PeerAS, state)
}

// FormatSentMessage formats a sent UPDATE message.
// Uses "type":"sent" instead of "type":"update" to distinguish from received messages.
// For text format, uses "sent update" instead of "received update".
func FormatSentMessage(peer PeerInfo, msg RawMessage, content ContentConfig) string {
	// Force direction to "sent" for sent messages
	msg.Direction = "sent"

	// Format as regular update message
	output := FormatMessage(peer, msg, content)

	// Replace type indicator for JSON (text format uses direction field)
	// New format has type in message wrapper: {"message":{"type":"update"...
	if content.Encoding == EncodingJSON {
		output = strings.Replace(output, `"type":"update"`, `"type":"sent"`, 1)
	}

	return output
}
