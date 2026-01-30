package plugin

import (
	"fmt"
	"net/netip"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/plugin/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/plugin/evpn"
	"codeberg.org/thomas-mangin/ze/internal/plugin/flowspec"
)

// Encoding constants for process output formatting.
const (
	EncodingJSON = "json"
	EncodingText = "text"
)

// FormatMessage formats a RawMessage based on ContentConfig.
// Uses lazy parsing via AttrsWire when available for optimal performance.
// Handles encoding (json/text), format (parsed/raw/full), and attribute filtering.
// If overrideDir is non-empty, it overrides msg.Direction for formatting.
func FormatMessage(peer PeerInfo, msg RawMessage, content ContentConfig, overrideDir string) string {
	content = content.WithDefaults()

	// Compute effective direction
	direction := msg.Direction
	if overrideDir != "" {
		direction = overrideDir
	}

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

		return formatFromFilterResult(peer, msg, content, result, encCtx, direction)
	}

	// Non-UPDATE messages: format as raw
	return formatNonUpdate(peer, msg, content, direction)
}

// formatEmptyUpdate formats an empty UPDATE message.
// IPC 2.0 format: {"type":"bgp","bgp":{"type":"update",...}}.
func formatEmptyUpdate(peer PeerInfo, content ContentConfig) string {
	if content.Encoding == EncodingJSON {
		return fmt.Sprintf(`{"type":"bgp","bgp":{"type":"update","peer":{"address":"%s","asn":%d},"nlri":{}}}`+"\n",
			peer.Address, peer.PeerAS)
	}
	return fmt.Sprintf("peer %s update\n", peer.Address)
}

// formatNonUpdate formats non-UPDATE messages (OPEN, NOTIFICATION, KEEPALIVE).
// Routes to dedicated formatters for parsed output, falls back to raw for unknown types.
//
// NOTE: For PARSED format, this function ignores content.Encoding and always returns TEXT.
// For RAW format, it respects Encoding (JSON or text with raw hex).
// For structured JSON output of non-UPDATE messages, use Server.formatMessage()
// which has access to the shared JSONEncoder with proper counter semantics.
func formatNonUpdate(peer PeerInfo, msg RawMessage, content ContentConfig, direction string) string {
	// For parsed format, use dedicated text formatters
	if content.Format != FormatRaw {
		switch msg.Type { //nolint:exhaustive // only specific types have dedicated formatters
		case message.TypeOPEN:
			decoded := DecodeOpen(msg.RawBytes)
			return FormatOpen(peer, decoded, direction, msg.MessageID)
		case message.TypeNOTIFICATION:
			decoded := DecodeNotification(msg.RawBytes)
			return FormatNotification(peer, decoded, direction, msg.MessageID)
		case message.TypeKEEPALIVE:
			return FormatKeepalive(peer, direction, msg.MessageID)
		}
	}

	// Raw format or unknown type
	rawHex := fmt.Sprintf("%x", msg.RawBytes)

	if content.Encoding == EncodingJSON {
		// IPC 2.0 format: {"type":"bgp","bgp":{"type":"...","peer":{...},"raw":{...}}}
		return fmt.Sprintf(`{"type":"bgp","bgp":{"type":"%s","peer":{"address":"%s","asn":%d},"raw":{"message":"%s"}}}`+"\n",
			strings.ToLower(msg.Type.String()), peer.Address, peer.PeerAS, rawHex)
	}
	return fmt.Sprintf("peer %s %s raw %s\n",
		peer.Address, strings.ToLower(msg.Type.String()), rawHex)
}

// formatFromFilterResult formats UPDATE using lazy-parsed FilterResult.
// This is the optimized path that only parses requested attributes.
// ctx provides ADD-PATH state per family (nil means no ADD-PATH).
func formatFromFilterResult(peer PeerInfo, msg RawMessage, content ContentConfig, result FilterResult, ctx *bgpctx.EncodingContext, direction string) string {
	switch content.Format {
	case FormatRaw:
		return formatRawFromResult(peer, msg, content, direction)
	case FormatFull:
		return formatFullFromResult(peer, msg, content, result, ctx, direction)
	default: // FormatParsed
		return formatParsedFromResult(peer, msg, content, result, ctx, direction)
	}
}

// formatRawFromResult formats raw hex (doesn't need FilterResult attributes).
// IPC 2.0 format: {"type":"bgp","bgp":{"type":"update","message":{...},...,"raw":{...}}}.
func formatRawFromResult(peer PeerInfo, msg RawMessage, content ContentConfig, direction string) string {
	rawHex := fmt.Sprintf("%x", msg.RawBytes)
	if content.Encoding == EncodingJSON {
		var msgPart string
		if direction != "" {
			msgPart = fmt.Sprintf(`,"message":{"direction":"%s"}`, direction)
		}
		return fmt.Sprintf(`{"type":"bgp","bgp":{"type":"update"%s,"peer":{"address":"%s","asn":%d},"raw":{"update":"%s"}}}`+"\n",
			msgPart, peer.Address, peer.PeerAS, rawHex)
	}
	return fmt.Sprintf("peer %s %s update raw %s\n", peer.Address, direction, rawHex)
}

// formatParsedFromResult formats parsed UPDATE using FilterResult.
// ctx provides ADD-PATH state per family.
func formatParsedFromResult(peer PeerInfo, msg RawMessage, content ContentConfig, result FilterResult, ctx *bgpctx.EncodingContext, direction string) string {
	if content.Encoding == EncodingJSON {
		return formatFilterResultJSON(peer, result, msg.MessageID, direction, ctx)
	}
	return formatFilterResultText(peer, result, msg.MessageID, direction, ctx)
}

// formatFullFromResult formats both parsed content AND raw hex (IPC Protocol 2.0).
// ctx provides ADD-PATH state per family.
// Includes raw bytes nested under "raw" object: attributes, nlri, withdrawn.
func formatFullFromResult(peer PeerInfo, msg RawMessage, content ContentConfig, result FilterResult, ctx *bgpctx.EncodingContext, direction string) string {
	rawHex := fmt.Sprintf("%x", msg.RawBytes)
	parsed := formatParsedFromResult(peer, msg, content, result, ctx, direction)

	if content.Encoding == EncodingJSON {
		// Build raw object for pool-based storage
		var rawObj strings.Builder
		rawObj.WriteString(`"raw":{`)

		hasContent := false

		// Extract raw components if WireUpdate available
		if msg.WireUpdate != nil {
			rawComps, err := ExtractRawComponents(msg.WireUpdate)
			if err == nil && rawComps != nil {
				// attributes: raw bytes without MP_REACH/MP_UNREACH
				if len(rawComps.Attributes) > 0 {
					if hasContent {
						rawObj.WriteString(",")
					}
					rawObj.WriteString(fmt.Sprintf(`"attributes":"%x"`, rawComps.Attributes))
					hasContent = true
				}

				// nlri: per-family raw bytes
				if len(rawComps.NLRI) > 0 {
					if hasContent {
						rawObj.WriteString(",")
					}
					rawObj.WriteString(`"nlri":{`)
					first := true
					for family, nlriBytes := range rawComps.NLRI {
						if !first {
							rawObj.WriteString(",")
						}
						first = false
						rawObj.WriteString(fmt.Sprintf(`"%s":"%x"`, family.String(), nlriBytes))
					}
					rawObj.WriteString(`}`)
					hasContent = true
				}

				// withdrawn: per-family raw bytes
				if len(rawComps.Withdrawn) > 0 {
					if hasContent {
						rawObj.WriteString(",")
					}
					rawObj.WriteString(`"withdrawn":{`)
					first := true
					for family, wdBytes := range rawComps.Withdrawn {
						if !first {
							rawObj.WriteString(",")
						}
						first = false
						rawObj.WriteString(fmt.Sprintf(`"%s":"%x"`, family.String(), wdBytes))
					}
					rawObj.WriteString(`}`)
					hasContent = true
				}
			}
		}

		// Add full update bytes
		if hasContent {
			rawObj.WriteString(",")
		}
		rawObj.WriteString(fmt.Sprintf(`"update":"%s"`, rawHex))

		rawObj.WriteString(`}`)

		// IPC 2.0: inject raw into bgp object (ends with "}}\n")
		// Replace trailing "}}\n" with ","raw":{...}}}\n"
		if strings.HasSuffix(parsed, "}}\n") {
			return parsed[:len(parsed)-3] + "," + rawObj.String() + "}}\n"
		}
		return parsed
	}

	// For text, append raw line
	return parsed + fmt.Sprintf("peer %s %s update raw %s\n", peer.Address, direction, rawHex)
}

// formatFilterResultJSON formats FilterResult as JSON (IPC Protocol 2.0).
// Uses AnnouncedByFamily()/WithdrawnByFamily() for RFC 4760-correct next-hop per family.
// ctx provides ADD-PATH state per family.
//
// RFC 4271 Section 5.1.3: NEXT_HOP defines the IP address of the router
// that SHOULD be used as next hop to the destinations.
// RFC 4760 Section 3: Each MP_REACH_NLRI has its own next-hop field.
//
// IPC 2.0 format:
//
//	{
//	  "type": "bgp",
//	  "bgp": {
//	    "type": "update",
//	    "message": {"id": 123, "direction": "received"},
//	    "peer": {"address": "...", "asn": ...},
//	    "attributes": {"origin": "igp", ...},
//	    "nlri": {
//	      "ipv4/unicast": [{"next-hop": "...", "action": "add", "nlri": [...]}]
//	    }
//	  }
//	}
func formatFilterResultJSON(peer PeerInfo, result FilterResult, msgID uint64, direction string, ctx *bgpctx.EncodingContext) string {
	var sb strings.Builder

	// IPC 2.0 outer wrapper
	sb.WriteString(`{"type":"bgp","bgp":{`)

	// Event type
	sb.WriteString(`"type":"update"`)

	// Message metadata (id, direction) - only if present
	if msgID > 0 || direction != "" {
		sb.WriteString(`,"message":{`)
		needComma := false
		if msgID > 0 {
			sb.WriteString(fmt.Sprintf(`"id":%d`, msgID))
			needComma = true
		}
		if direction != "" {
			if needComma {
				sb.WriteString(",")
			}
			sb.WriteString(`"direction":"`)
			sb.WriteString(direction)
			sb.WriteString(`"`)
		}
		sb.WriteString(`}`)
	}

	// Peer info
	sb.WriteString(`,"peer":{"address":"`)
	sb.WriteString(peer.Address.String())
	sb.WriteString(`","asn":`)
	sb.WriteString(fmt.Sprintf("%d", peer.PeerAS))
	sb.WriteString(`}`)

	// Attributes nested under "attributes"
	if len(result.Attributes) > 0 {
		sb.WriteString(`,"attributes":{`)
		formatAttributesJSON(&sb, result)
		sb.WriteString(`}`)
	}

	// Collect operations by family
	// Map: family -> list of operations (each op has action, next-hop, nlris)
	familyOps := make(map[string][]familyOperation)

	// Add announced routes (action: add)
	announced := result.AnnouncedByFamily(ctx)
	for _, fam := range announced {
		op := familyOperation{
			Action:  "add",
			NextHop: fam.NextHop.String(),
			NLRIs:   fam.NLRIs,
		}
		familyOps[fam.Family] = append(familyOps[fam.Family], op)
	}

	// Add withdrawn routes (action: del)
	withdrawn := result.WithdrawnByFamily(ctx)
	for _, fam := range withdrawn {
		op := familyOperation{
			Action: "del",
			NLRIs:  fam.NLRIs,
		}
		familyOps[fam.Family] = append(familyOps[fam.Family], op)
	}

	// NLRIs nested under "nlri"
	sb.WriteString(`,"nlri":{`)
	first := true
	for family, ops := range familyOps {
		if !first {
			sb.WriteString(",")
		}
		first = false
		sb.WriteString(`"`)
		sb.WriteString(family)
		sb.WriteString(`":[`)
		for i, op := range ops {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(`{`)
			// next-hop only for add operations (and only if valid)
			if op.Action == "add" && op.NextHop != "" && op.NextHop != "invalid IP" {
				sb.WriteString(`"next-hop":"`)
				sb.WriteString(op.NextHop)
				sb.WriteString(`",`)
			}
			sb.WriteString(`"action":"`)
			sb.WriteString(op.Action)
			sb.WriteString(`","nlri":[`)
			for j, n := range op.NLRIs {
				if j > 0 {
					sb.WriteString(",")
				}
				formatNLRIJSONValue(&sb, n)
			}
			sb.WriteString(`]}`)
		}
		sb.WriteString(`]`)
	}
	sb.WriteString(`}`)

	// Close bgp object and outer wrapper
	sb.WriteString("}}\n")
	return sb.String()
}

// familyOperation represents a single operation (add/del) for a family.
type familyOperation struct {
	Action  string      // "add" or "del"
	NextHop string      // Only for "add" operations
	NLRIs   []nlri.NLRI // NLRIs in this operation
}

// formatNLRIJSONValue formats a single NLRI as JSON value.
// Simple prefixes without path-id are output as strings: "10.0.0.0/24".
// Complex NLRIs (ADD-PATH, VPN, EVPN, FlowSpec) are output as objects with structured fields.
//
// RFC 4364: VPN NLRI includes RD and labels.
// RFC 7432: EVPN NLRI includes route-type, ESI, etc.
// RFC 8277: Labeled Unicast NLRI includes labels.
// RFC 8955: FlowSpec NLRI includes match components.
func formatNLRIJSONValue(sb *strings.Builder, n nlri.NLRI) {
	// Check for complex NLRI types that need structured output
	switch v := n.(type) {
	case *nlri.IPVPN:
		formatIPVPNJSON(sb, v)
		return
	case *nlri.LabeledUnicast:
		formatLabeledUnicastJSON(sb, v)
		return
	case *flowspec.FlowSpecVPN:
		formatFlowSpecVPNJSON(sb, v)
		return
	case *evpn.EVPNType2:
		formatEVPNType2JSON(sb, v)
		return
	}

	pathID := n.PathID()

	// Simple prefix without path-id: output as string
	if pathID == 0 {
		if p, ok := n.(prefixer); ok {
			sb.WriteString(`"`)
			sb.WriteString(p.Prefix().String())
			sb.WriteString(`"`)
			return
		}
	}

	// Complex NLRI (has path-id or not a simple prefix): output as object
	formatNLRIJSON(sb, n)
}

// formatIPVPNJSON formats an IPVPN NLRI as structured JSON.
// RFC 4364: {"prefix":"10.0.0.0/24", "rd":"0:65000:1", "labels":[100]}.
func formatIPVPNJSON(sb *strings.Builder, v *nlri.IPVPN) {
	sb.WriteString(`{"prefix":"`)
	sb.WriteString(v.Prefix().String())
	sb.WriteString(`","rd":"`)
	sb.WriteString(v.RD().String())
	sb.WriteString(`"`)

	if labels := v.Labels(); len(labels) > 0 {
		sb.WriteString(`,"labels":[`)
		for i, l := range labels {
			if i > 0 {
				sb.WriteString(",")
			}
			fmt.Fprintf(sb, "%d", l)
		}
		sb.WriteString(`]`)
	}

	if pathID := v.PathID(); pathID != 0 {
		fmt.Fprintf(sb, `,"path-id":%d`, pathID)
	}

	sb.WriteString(`}`)
}

// formatLabeledUnicastJSON formats a LabeledUnicast NLRI as structured JSON.
// RFC 8277: {"prefix":"10.0.0.0/24", "labels":[100]}.
func formatLabeledUnicastJSON(sb *strings.Builder, v *nlri.LabeledUnicast) {
	sb.WriteString(`{"prefix":"`)
	sb.WriteString(v.Prefix().String())
	sb.WriteString(`"`)

	if labels := v.Labels(); len(labels) > 0 {
		sb.WriteString(`,"labels":[`)
		for i, l := range labels {
			if i > 0 {
				sb.WriteString(",")
			}
			fmt.Fprintf(sb, "%d", l)
		}
		sb.WriteString(`]`)
	}

	if pathID := v.PathID(); pathID != 0 {
		fmt.Fprintf(sb, `,"path-id":%d`, pathID)
	}

	sb.WriteString(`}`)
}

// formatEVPNType2JSON formats an EVPN Type 2 (MAC/IP Advertisement) NLRI as structured JSON.
// RFC 7432 Section 7.2: {"route-type":"mac-ip-advertisement","rd":"0:65000:1","esi":"00:...","mac":"...", ...}.
func formatEVPNType2JSON(sb *strings.Builder, v *evpn.EVPNType2) {
	sb.WriteString(`{"route-type":"`)
	sb.WriteString(v.RouteType().String())
	sb.WriteString(`","rd":"`)
	sb.WriteString(v.RD().String())
	sb.WriteString(`","esi":"`)
	sb.WriteString(v.ESI().String())
	sb.WriteString(`"`)

	// Ethernet Tag
	fmt.Fprintf(sb, `,"ethernet-tag":%d`, v.EthernetTag())

	// MAC
	mac := v.MAC()
	fmt.Fprintf(sb, `,"mac":"%02x:%02x:%02x:%02x:%02x:%02x"`,
		mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])

	// IP (optional)
	if ip := v.IP(); ip.IsValid() {
		sb.WriteString(`,"ip":"`)
		sb.WriteString(ip.String())
		sb.WriteString(`"`)
	}

	// Labels
	if labels := v.Labels(); len(labels) > 0 {
		sb.WriteString(`,"labels":[`)
		for i, l := range labels {
			if i > 0 {
				sb.WriteString(",")
			}
			fmt.Fprintf(sb, "%d", l)
		}
		sb.WriteString(`]`)
	}

	// Path ID
	if pathID := v.PathID(); pathID != 0 {
		fmt.Fprintf(sb, `,"path-id":%d`, pathID)
	}

	sb.WriteString(`}`)
}

// formatFlowSpecVPNJSON formats a FlowSpecVPN NLRI as structured JSON.
// RFC 8955: {"rd":"0:65000:1", ...flowspec components...}.
func formatFlowSpecVPNJSON(sb *strings.Builder, v *flowspec.FlowSpecVPN) {
	sb.WriteString(`{"rd":"`)
	sb.WriteString(v.RD().String())
	sb.WriteString(`"`)

	// FlowSpec components are complex - use String() for now
	// TODO: Add structured component output
	sb.WriteString(`,"spec":"`)
	writeJSONEscapedString(sb, v.FlowSpec().String())
	sb.WriteString(`"`)

	if pathID := v.PathID(); pathID != 0 {
		fmt.Fprintf(sb, `,"path-id":%d`, pathID)
	}

	sb.WriteString(`}`)
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

// FormatRouteRefresh formats a ROUTE-REFRESH message as text output.
// RFC 7313: Type is "refresh" (subtype 0), "borr" (subtype 1), or "eorr" (subtype 2).
// Format: peer <ip> <direction> <type> <msg-id> family <family>.
func FormatRouteRefresh(peer PeerInfo, decoded DecodedRouteRefresh, direction string, msgID uint64) string {
	return fmt.Sprintf("peer %s %s %s %d family %s\n",
		peer.Address, direction, decoded.SubtypeName, msgID, decoded.Family)
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
	// IPC 2.0 format: {"type":"bgp","bgp":{"type":"state",...}}
	return fmt.Sprintf(`{"type":"bgp","bgp":{"type":"state","peer":{"address":"%s","asn":%d},"state":"%s"}}`+"\n",
		peer.Address, peer.PeerAS, state)
}

func formatStateChangeText(peer PeerInfo, state string) string {
	return fmt.Sprintf("peer %s asn %d state %s\n", peer.Address, peer.PeerAS, state)
}

// FormatNegotiated formats negotiated capabilities event.
// Sent after OPEN exchange to inform plugins of negotiated capabilities.
func FormatNegotiated(peer PeerInfo, neg DecodedNegotiated, encoder *JSONEncoder) string {
	// Always use JSON for negotiated - too complex for text format
	return encoder.Negotiated(peer, neg)
}

// FormatSentMessage formats a sent UPDATE message.
// Uses "type":"sent" instead of "type":"update" to distinguish from received messages.
// For text format, uses "sent update" instead of "received update".
func FormatSentMessage(peer PeerInfo, msg RawMessage, content ContentConfig) string {
	// Format with direction override (no mutation of msg)
	output := FormatMessage(peer, msg, content, "sent")

	// Replace type indicator for JSON (text format uses direction field)
	// IPC 2.0 format has type in bgp payload: {"type":"bgp","bgp":{"type":"update"...
	// We want to change the inner type to "sent"
	if content.Encoding == EncodingJSON {
		// Replace the second occurrence of "type" (the one inside bgp payload)
		// Pattern: {"type":"bgp","bgp":{"type":"update" -> {"type":"bgp","bgp":{"type":"sent"
		output = strings.Replace(output, `"bgp":{"type":"update"`, `"bgp":{"type":"sent"`, 1)
	}

	return output
}
