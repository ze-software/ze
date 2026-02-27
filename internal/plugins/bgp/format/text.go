// Design: docs/architecture/api/json-format.md — message formatting
// Related: ../textparse/keywords.go — shared keyword constants and alias resolution

package format

import (
	"encoding/hex"
	"fmt"
	"net/netip"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/plugin/registry"
	labeled "codeberg.org/thomas-mangin/ze/internal/plugins/bgp-nlri-labeled"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/context"
	bgpfilter "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/filter"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/textparse"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/wireu"
)

// FormatMessage formats a RawMessage based on ContentConfig.
// Uses lazy parsing via AttrsWire when available for optimal performance.
// Handles encoding (json/text), format (parsed/raw/full), and attribute filtering.
// If overrideDir is non-empty, it overrides msg.Direction for formatting.
func FormatMessage(peer plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, overrideDir string) string {
	content = content.WithDefaults()

	// Compute effective direction
	direction := msg.Direction
	if overrideDir != "" {
		direction = overrideDir
	}

	// Summary format: lightweight NLRI metadata only (skip full attribute parsing).
	// Must short-circuit before filter setup for performance.
	if content.Format == plugin.FormatSummary && msg.Type == message.TypeUPDATE {
		return formatSummary(peer, msg.RawBytes, msg.MessageID, direction)
	}

	// Get attribute filter (nil means all)
	filter := content.Attributes
	if filter == nil {
		all := bgpfilter.NewFilterAll()
		filter = &all
	}

	// Get NLRI filter (nil means all)
	nlriFilter := content.NLRI
	if nlriFilter == nil {
		all := bgpfilter.NewNLRIFilterAll()
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
// ze-bgp JSON format: {"type":"bgp","bgp":{"message":{"type":"update"},...}}.
func formatEmptyUpdate(peer plugin.PeerInfo, content bgptypes.ContentConfig) string {
	if content.Encoding == plugin.EncodingJSON {
		return fmt.Sprintf(`{"type":"bgp","bgp":{"message":{"type":"update"},"peer":{"address":"%s","asn":%d},"nlri":{}}}`+"\n",
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
func formatNonUpdate(peer plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, direction string) string {
	// For parsed format, use dedicated text formatters
	if content.Format != plugin.FormatRaw {
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

	if content.Encoding == plugin.EncodingJSON {
		// ze-bgp JSON format: {"type":"bgp","bgp":{"message":{"type":"..."},...}}
		msgType := strings.ToLower(msg.Type.String())
		return fmt.Sprintf(`{"type":"bgp","bgp":{"message":{"type":"%s"},"peer":{"address":"%s","asn":%d},"raw":{"message":"%s"}}}`+"\n",
			msgType, peer.Address, peer.PeerAS, rawHex)
	}
	return fmt.Sprintf("peer %s %s raw %s\n",
		peer.Address, strings.ToLower(msg.Type.String()), rawHex)
}

// formatFromFilterResult formats UPDATE using lazy-parsed FilterResult.
// This is the optimized path that only parses requested attributes.
// ctx provides ADD-PATH state per family (nil means no ADD-PATH).
func formatFromFilterResult(peer plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, result bgpfilter.FilterResult, ctx *bgpctx.EncodingContext, direction string) string {
	switch content.Format {
	case plugin.FormatRaw, plugin.FormatHex:
		return formatRawFromResult(peer, msg, content, direction)
	case plugin.FormatFull:
		return formatFullFromResult(peer, msg, content, result, ctx, direction)
	}
	// FormatParsed (the common case)
	return formatParsedFromResult(peer, msg, content, result, ctx, direction)
}

// formatRawFromResult formats raw hex (doesn't need FilterResult attributes).
// ze-bgp JSON format: {"type":"bgp","bgp":{"message":{"type":"update",...},...}}.
func formatRawFromResult(peer plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, direction string) string {
	rawHex := fmt.Sprintf("%x", msg.RawBytes)
	if content.Encoding == plugin.EncodingJSON {
		var msgFields string
		if direction != "" {
			msgFields = fmt.Sprintf(`,"direction":%q`, direction)
		}
		return fmt.Sprintf(`{"type":"bgp","bgp":{"message":{"type":"update"%s},"peer":{"address":"%s","asn":%d},"raw":{"update":"%s"}}}`+"\n",
			msgFields, peer.Address, peer.PeerAS, rawHex)
	}
	return fmt.Sprintf("peer %s %s update raw %s\n", peer.Address, direction, rawHex)
}

// formatParsedFromResult formats parsed UPDATE using FilterResult.
// ctx provides ADD-PATH state per family.
func formatParsedFromResult(peer plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, result bgpfilter.FilterResult, ctx *bgpctx.EncodingContext, direction string) string {
	if content.Encoding == plugin.EncodingJSON {
		return formatFilterResultJSON(peer, result, msg.MessageID, direction, ctx)
	}
	return formatFilterResultText(peer, result, msg.MessageID, direction, ctx)
}

// formatFullFromResult formats both parsed content AND raw hex (ze-bgp JSON).
// ctx provides ADD-PATH state per family.
// Includes raw bytes nested under "raw" object: attributes, nlri, withdrawn.
func formatFullFromResult(peer plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, result bgpfilter.FilterResult, ctx *bgpctx.EncodingContext, direction string) string {
	rawHex := fmt.Sprintf("%x", msg.RawBytes)
	parsed := formatParsedFromResult(peer, msg, content, result, ctx, direction)

	if content.Encoding == plugin.EncodingJSON {
		// Build raw object for pool-based storage
		var rawObj strings.Builder
		rawObj.WriteString(`"raw":{`)

		hasContent := false

		// Extract raw components if WireUpdate available
		if msg.WireUpdate != nil {
			rawComps, err := wireu.ExtractRawComponents(msg.WireUpdate)
			if err == nil && rawComps != nil {
				// attributes: raw bytes without MP_REACH/MP_UNREACH
				if len(rawComps.Attributes) > 0 {
					if hasContent {
						rawObj.WriteString(",")
					}
					fmt.Fprintf(&rawObj, `"attributes":"%x"`, rawComps.Attributes)
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
						fmt.Fprintf(&rawObj, `"%s":"%x"`, family.String(), nlriBytes)
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
						fmt.Fprintf(&rawObj, `"%s":"%x"`, family.String(), wdBytes)
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
		fmt.Fprintf(&rawObj, `"update":"%s"`, rawHex)

		rawObj.WriteString(`}`)

		// ze-bgp JSON: inject raw into bgp object (ends with "}}\n")
		// Replace trailing "}}\n" with ","raw":{...}}}\n"
		if strings.HasSuffix(parsed, "}}\n") {
			return parsed[:len(parsed)-3] + "," + rawObj.String() + "}}\n"
		}
		return parsed
	}

	// For text, append raw line
	return parsed + fmt.Sprintf("peer %s %s update raw %s\n", peer.Address, direction, rawHex)
}

// formatFilterResultJSON formats FilterResult as JSON (ze-bgp JSON).
// Uses AnnouncedByFamily()/WithdrawnByFamily() for RFC 4760-correct next-hop per family.
// ctx provides ADD-PATH state per family.
//
// RFC 4271 Section 5.1.3: NEXT_HOP defines the IP address of the router
// that SHOULD be used as next hop to the destinations.
// RFC 4760 Section 3: Each MP_REACH_NLRI has its own next-hop field.
//
// ze-bgp JSON format:
//
//	{
//	  "type": "bgp",
//	  "bgp": {
//	    "message": {"type": "update", "id": 123, "direction": "received"},
//	    "peer": {"address": "...", "asn": ...},
//	    "update": {
//	      "attr": {"origin": "igp", ...},
//	      "nlri": {
//	        "ipv4/unicast": [{"next-hop": "...", "action": "add", "nlri": [...]}]
//	      }
//	    }
//	  }
//	}
func formatFilterResultJSON(peer plugin.PeerInfo, result bgpfilter.FilterResult, msgID uint64, direction string, ctx *bgpctx.EncodingContext) string {
	var sb strings.Builder

	// ze-bgp JSON outer wrapper
	sb.WriteString(`{"type":"bgp","bgp":{`)

	// Message metadata with type inside
	sb.WriteString(`"message":{"type":"update"`)
	if msgID > 0 {
		fmt.Fprintf(&sb, `,"id":%d`, msgID)
	}
	if direction != "" {
		sb.WriteString(`,"direction":"`)
		sb.WriteString(direction)
		sb.WriteString(`"`)
	}
	sb.WriteString(`}`)

	// Peer at bgp level
	sb.WriteString(`,"peer":{"address":"`)
	sb.WriteString(peer.Address.String())
	sb.WriteString(`","asn":`)
	fmt.Fprintf(&sb, "%d", peer.PeerAS)
	sb.WriteString(`}`)

	// Update container with attr and nlri inside
	sb.WriteString(`,"update":{`)

	// Attributes inside update
	if len(result.Attributes) > 0 {
		sb.WriteString(`"attr":{`)
		formatAttributesJSON(&sb, result)
		sb.WriteString(`},`)
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

	// NLRIs inside update
	sb.WriteString(`"nlri":{`)
	formatFamilyOpsJSON(&sb, familyOps)
	sb.WriteString(`}`)

	// Close update, bgp, and outer wrapper
	sb.WriteString("}}}\n")
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
// Plugin NLRI types (VPN, EVPN, FlowSpec) are decoded via registry.DecodeNLRIByFamily,
// which routes to the plugin's in-process decoder. This avoids direct plugin type imports.
// Core types (LabeledUnicast) are handled directly since they live in the nlri package.
//
// RFC 4364: VPN NLRI includes RD and labels.
// RFC 7432: EVPN NLRI includes route-type, ESI, etc.
// RFC 8277: Labeled Unicast NLRI includes labels.
// RFC 8955: FlowSpec NLRI includes match components.
func formatNLRIJSONValue(sb *strings.Builder, n nlri.NLRI) {
	// Core type: LabeledUnicast lives in nlri package, not a plugin
	if lu, ok := n.(*labeled.LabeledUnicast); ok {
		formatLabeledUnicastJSON(sb, lu)
		return
	}

	// Try registry-based decode for plugin NLRI types (VPN, EVPN, FlowSpec).
	// The registry routes to the plugin's InProcessNLRIDecoder by family.
	familyStr := n.Family().String()
	if registry.PluginForFamily(familyStr) != "" {
		hexData := hex.EncodeToString(n.Bytes())
		decoded, err := registry.DecodeNLRIByFamily(familyStr, hexData)
		if err == nil {
			sb.WriteString(decoded)
			return
		}
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

// formatLabeledUnicastJSON formats a LabeledUnicast NLRI as structured JSON.
// RFC 8277: {"prefix":"10.0.0.0/24", "labels":[100]}.
func formatLabeledUnicastJSON(sb *strings.Builder, v *labeled.LabeledUnicast) {
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
			continue
		case '"':
			sb.WriteString(`\"`)
			continue
		case '\n':
			sb.WriteString(`\n`)
			continue
		case '\r':
			sb.WriteString(`\r`)
			continue
		case '\t':
			sb.WriteString(`\t`)
			continue
		}
		if r < 0x20 {
			// Control character - use \uXXXX
			fmt.Fprintf(sb, `\u%04x`, r)
			continue
		}
		sb.WriteRune(r)
	}
}

// formatAttributesJSON formats attributes from FilterResult for JSON.
func formatAttributesJSON(sb *strings.Builder, result bgpfilter.FilterResult) {
	if len(result.Attributes) == 0 {
		return
	}

	first := true
	for code, attr := range result.Attributes {
		if !first {
			sb.WriteString(",")
		}
		first = false
		formatAttributeJSON(sb, code, attr)
	}
}

// formatFamilyOpsJSON writes family operations as JSON object entries.
// Shared by formatFilterResultJSON and FormatDecodeUpdateJSON.
func formatFamilyOpsJSON(sb *strings.Builder, familyOps map[string][]familyOperation) {
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
				formatNLRIJSONValue(sb, n)
			}
			sb.WriteString(`]}`)
		}
		sb.WriteString(`]`)
	}
}

// formatAttributeJSON formats a single attribute for JSON.
// Known attribute types are formatted with named keys; unknown types use "attr-N" with hex value.
func formatAttributeJSON(sb *strings.Builder, code attribute.AttributeCode, attr attribute.Attribute) {
	switch code { //nolint:exhaustive // common attributes; unknown handled after switch
	case attribute.AttrOrigin:
		switch o := attr.(type) {
		case *attribute.Origin:
			sb.WriteString(`"origin":"`)
			sb.WriteString(strings.ToLower(o.String()))
			sb.WriteString(`"`)
		case attribute.Origin:
			sb.WriteString(`"origin":"`)
			sb.WriteString(strings.ToLower(o.String()))
			sb.WriteString(`"`)
		}
		return
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
		return
	case attribute.AttrMED:
		switch m := attr.(type) {
		case *attribute.MED:
			fmt.Fprintf(sb, `"med":%d`, uint32(*m))
		case attribute.MED:
			fmt.Fprintf(sb, `"med":%d`, uint32(m))
		}
		return
	case attribute.AttrLocalPref:
		switch lp := attr.(type) {
		case *attribute.LocalPref:
			fmt.Fprintf(sb, `"local-preference":%d`, uint32(*lp))
		case attribute.LocalPref:
			fmt.Fprintf(sb, `"local-preference":%d`, uint32(lp))
		}
		return
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
		return
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
		return
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
		return
	}
	// Unknown attribute code — format as "attr-N": "hex"
	attrBuf := make([]byte, attr.Len())
	attr.WriteTo(attrBuf, 0)
	fmt.Fprintf(sb, `"attr-%d":"%x"`, code, attrBuf)
}

// formatFilterResultText formats FilterResult as text.
// Uses AnnouncedByFamily()/WithdrawnByFamily() for RFC 4760-correct next-hop per family.
// ctx provides ADD-PATH state per family.
func formatFilterResultText(peer plugin.PeerInfo, result bgpfilter.FilterResult, msgID uint64, direction string, ctx *bgpctx.EncodingContext) string {
	var sb strings.Builder

	// Uniform header: peer <ip> asn <asn> <direction> update <id>
	fmt.Fprintf(&sb, "peer %s asn %d %s update %d", peer.Address, peer.PeerAS, direction, msgID)

	announced := result.AnnouncedByFamily(ctx)
	withdrawn := result.WithdrawnByFamily(ctx)

	if len(announced) == 0 && len(withdrawn) == 0 {
		// Empty UPDATE (End-of-RIB marker or attribute-only): minimal text line
		sb.WriteString("\n")
		return sb.String()
	}

	// Attributes (shared across all families)
	formatAttributesText(&sb, result)

	// Announced routes: next <nh> nlri <fam> add <nlri>[,<nlri>]...
	for _, fam := range announced {
		sb.WriteString(" " + textparse.ShortNext + " ")
		sb.WriteString(fam.NextHop.String())
		sb.WriteString(" nlri ")
		sb.WriteString(fam.Family)
		sb.WriteString(" add ")
		writeNLRIList(&sb, fam.NLRIs)
	}

	// Withdrawn routes: nlri <fam> del <nlri>[,<nlri>]...
	for _, fam := range withdrawn {
		sb.WriteString(" nlri ")
		sb.WriteString(fam.Family)
		sb.WriteString(" del ")
		writeNLRIList(&sb, fam.NLRIs)
	}

	sb.WriteString("\n")
	return sb.String()
}

// writeNLRIList writes NLRIs in compact format.
// INET NLRIs (which implement Key()) use comma-separated CIDRs: prefix <a>,<b>.
// Other NLRIs use keyword boundary: <nlri1> <nlri2>.
func writeNLRIList(sb *strings.Builder, nlris []nlri.NLRI) {
	if len(nlris) == 0 {
		return
	}

	// Check if first NLRI supports compact Key() (INET).
	type keyer interface{ Key() string }
	_, useComma := nlris[0].(keyer)

	sb.WriteString(nlris[0].String())
	for _, n := range nlris[1:] {
		if useComma {
			sb.WriteByte(',')
			if k, ok := n.(keyer); ok {
				sb.WriteString(k.Key())
			} else {
				sb.WriteString(n.String())
			}
		} else {
			sb.WriteByte(' ')
			sb.WriteString(n.String())
		}
	}
}

// formatAttributesText formats attributes from FilterResult for text output.
// Only outputs what's in result.Attributes (lazy parsing - filter controls what's parsed).
func formatAttributesText(sb *strings.Builder, result bgpfilter.FilterResult) {
	for code, attr := range result.Attributes {
		sb.WriteString(" ")
		formatAttributeText(sb, code, attr)
	}
}

// formatAttributeText formats a single attribute for text output.
// Known attribute types are formatted with named keys (short aliases for API output);
// unknown types use "attr-N" with hex value.
// Short forms: next (next-hop), path (as-path), pref (local-preference),
// s-com (community), l-com (large-community), e-com (extended-community).
func formatAttributeText(sb *strings.Builder, code attribute.AttributeCode, attr attribute.Attribute) {
	switch code { //nolint:exhaustive // common attributes; unknown handled after switch
	case attribute.AttrOrigin:
		switch o := attr.(type) {
		case *attribute.Origin:
			sb.WriteString(textparse.KWOrigin + " ")
			sb.WriteString(strings.ToLower(o.String()))
		case attribute.Origin:
			sb.WriteString(textparse.KWOrigin + " ")
			sb.WriteString(strings.ToLower(o.String()))
		}
		return
	case attribute.AttrASPath:
		if ap, ok := attr.(*attribute.ASPath); ok {
			sb.WriteString(textparse.ShortPath + " ")
			first := true
			for _, seg := range ap.Segments {
				for _, asn := range seg.ASNs {
					if !first {
						sb.WriteString(",")
					}
					fmt.Fprintf(sb, "%d", asn)
					first = false
				}
			}
		}
		return
	case attribute.AttrNextHop:
		if nh, ok := attr.(*attribute.NextHop); ok {
			sb.WriteString(textparse.ShortNext + " ")
			sb.WriteString(nh.Addr.String())
		}
		return
	case attribute.AttrMED:
		switch m := attr.(type) {
		case *attribute.MED:
			fmt.Fprintf(sb, textparse.KWMED+" %d", uint32(*m))
		case attribute.MED:
			fmt.Fprintf(sb, textparse.KWMED+" %d", uint32(m))
		}
		return
	case attribute.AttrLocalPref:
		switch lp := attr.(type) {
		case *attribute.LocalPref:
			fmt.Fprintf(sb, textparse.ShortPref+" %d", uint32(*lp))
		case attribute.LocalPref:
			fmt.Fprintf(sb, textparse.ShortPref+" %d", uint32(lp))
		}
		return
	case attribute.AttrCommunity:
		if c, ok := attr.(*attribute.Communities); ok {
			sb.WriteString(textparse.ShortSCom + " ")
			for i, comm := range *c {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(comm.String())
			}
		}
		return
	case attribute.AttrLargeCommunity:
		if lc, ok := attr.(*attribute.LargeCommunities); ok {
			sb.WriteString(textparse.ShortLCom + " ")
			for i, comm := range *lc {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(comm.String())
			}
		}
		return
	case attribute.AttrExtCommunity:
		if ec, ok := attr.(*attribute.ExtendedCommunities); ok {
			sb.WriteString(textparse.ShortXCom + " ")
			for i, comm := range *ec {
				if i > 0 {
					sb.WriteString(",")
				}
				fmt.Fprintf(sb, "%x", comm[:])
			}
		}
		return
	}
	// Unknown attribute code — format as "attr-N hex"
	attrBuf := make([]byte, attr.Len())
	attr.WriteTo(attrBuf, 0)
	fmt.Fprintf(sb, "attr-%d %x", code, attrBuf)
}

// FormatOpen formats an OPEN message as text output.
// Format: peer <ip> <direction> open <msg-id> asn <asn> router-id <id> hold-time <t> [cap <code> <name> <value>]...
// ASN is the speaker's ASN (from the OPEN message).
// Capabilities use "cap <code> <name> <value>" format for easy parsing.
func FormatOpen(peer plugin.PeerInfo, open DecodedOpen, direction string, msgID uint64) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "peer %s asn %d %s open %d router-id %s hold-time %d",
		peer.Address, open.ASN, direction, msgID, open.RouterID, open.HoldTime)

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
func FormatNotification(peer plugin.PeerInfo, notify DecodedNotification, direction string, msgID uint64) string {
	dataHex := ""
	if len(notify.Data) > 0 {
		dataHex = fmt.Sprintf("%x", notify.Data)
	}

	// Replace spaces with hyphens in names for easier parsing
	codeName := strings.ReplaceAll(notify.ErrorCodeName, " ", "-")
	subcodeName := strings.ReplaceAll(notify.ErrorSubcodeName, " ", "-")

	return fmt.Sprintf("peer %s asn %d %s notification %d code %d subcode %d code-name %s subcode-name %s data %s\n",
		peer.Address, peer.PeerAS, direction, msgID, notify.ErrorCode, notify.ErrorSubcode,
		codeName, subcodeName, dataHex)
}

// FormatKeepalive formats a KEEPALIVE message as text output.
// Format: peer <ip> <direction> keepalive <msg-id>.
func FormatKeepalive(peer plugin.PeerInfo, direction string, msgID uint64) string {
	return fmt.Sprintf("peer %s asn %d %s keepalive %d\n", peer.Address, peer.PeerAS, direction, msgID)
}

// FormatRouteRefresh formats a ROUTE-REFRESH message as text output.
// RFC 7313: Type is "refresh" (subtype 0), "borr" (subtype 1), or "eorr" (subtype 2).
// Format: peer <ip> <direction> <type> <msg-id> family <family>.
func FormatRouteRefresh(peer plugin.PeerInfo, decoded DecodedRouteRefresh, direction string, msgID uint64) string {
	return fmt.Sprintf("peer %s asn %d %s %s %d family %s\n",
		peer.Address, peer.PeerAS, direction, decoded.SubtypeName, msgID, decoded.Family)
}

// FormatStateChange formats a peer state change event.
// State events are separate from BGP protocol messages.
// Common states: "up", "down", "connected", "established".
func FormatStateChange(peer plugin.PeerInfo, state, encoding string) string {
	if encoding == plugin.EncodingJSON {
		return formatStateChangeJSON(peer, state)
	}
	return formatStateChangeText(peer, state)
}

func formatStateChangeJSON(peer plugin.PeerInfo, state string) string {
	// ze-bgp JSON format: {"type":"bgp","bgp":{"message":{"type":"state"},"peer":{...},"state":"up"}}
	// State is a simple string value at bgp level
	return fmt.Sprintf(`{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"%s","asn":%d},"state":"%s"}}`+"\n",
		peer.Address, peer.PeerAS, state)
}

func formatStateChangeText(peer plugin.PeerInfo, state string) string {
	return fmt.Sprintf("peer %s asn %d state %s\n", peer.Address, peer.PeerAS, state)
}

// FormatNegotiated formats negotiated capabilities event.
// Sent after OPEN exchange to inform plugins of negotiated capabilities.
func FormatNegotiated(peer plugin.PeerInfo, neg DecodedNegotiated, encoder *JSONEncoder) string {
	// Always use JSON for negotiated - too complex for text format
	return encoder.Negotiated(peer, neg)
}

// FormatSentMessage formats a sent UPDATE message.
// Uses "type":"sent" instead of "type":"update" to distinguish from received messages.
// For text format, uses "sent update" instead of "received update".
func FormatSentMessage(peer plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig) string {
	// Format with direction override (no mutation of msg)
	output := FormatMessage(peer, msg, content, "sent")

	// Replace type indicator for JSON (text format uses direction field)
	// ze-bgp JSON format: {"type":"bgp","bgp":{"message":{"type":"update",...},...}}
	// We want to change "type":"update" to "type":"sent"
	if content.Encoding == plugin.EncodingJSON {
		// Change message type from update to sent
		output = strings.Replace(output, `"message":{"type":"update"`, `"message":{"type":"sent"`, 1)
	}

	return output
}
