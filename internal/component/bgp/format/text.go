// Design: docs/architecture/api/json-format.md — message formatting
// Related: ../textparse/keywords.go — shared keyword constants and alias resolution

package format

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	bgpfilter "codeberg.org/thomas-mangin/ze/internal/component/bgp/filter"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// jsonSafeReplacer escapes characters that would break JSON string values.
// Config validation restricts peer names to [a-zA-Z0-9_-], so this is
// defense-in-depth -- it should never trigger in practice.
var jsonSafeReplacer = strings.NewReplacer(`\`, `\\`, `"`, `\"`)

// writePeerJSON writes the "peer":{...} JSON fragment to a strings.Builder.
// Structure matches YANG peer-info grouping: address, name, remote.as, group.
// Always includes address, name, and remote.as. Includes group when non-empty.
// Key order is alphabetical (matching json.Marshal output from peerMap).
func writePeerJSON(sb *strings.Builder, peer *plugin.PeerInfo) {
	sb.WriteString(`,"peer":{"address":"`)
	sb.WriteString(peer.Address.String())
	sb.WriteByte('"')
	if peer.GroupName != "" {
		sb.WriteString(`,"group":"`)
		sb.WriteString(jsonSafeReplacer.Replace(peer.GroupName))
		sb.WriteByte('"')
	}
	if peer.LocalAS > 0 || peer.LocalAddress.IsValid() {
		sb.WriteString(`,"local":{`)
		first := true
		if peer.LocalAddress.IsValid() {
			sb.WriteString(`"address":"`)
			sb.WriteString(peer.LocalAddress.String())
			sb.WriteByte('"')
			first = false
		}
		if peer.LocalAS > 0 {
			if !first {
				sb.WriteByte(',')
			}
			sb.WriteString(`"as":`)
			sb.WriteString(strconv.FormatUint(uint64(peer.LocalAS), 10))
		}
		sb.WriteByte('}')
	}
	sb.WriteString(`,"name":"`)
	sb.WriteString(jsonSafeReplacer.Replace(peer.Name))
	sb.WriteString(`","remote":{"as":`)
	sb.WriteString(strconv.FormatUint(uint64(peer.PeerAS), 10))
	sb.WriteString(`}}`)
}

// peerJSONInline returns the peer JSON object as a string (without leading comma).
// Used by fmt.Sprintf sites where a Builder is not available.
// Structure matches YANG peer-info grouping: address, name, remote.as, group.
// Key order is alphabetical (matching json.Marshal output from peerMap).
func peerJSONInline(peer *plugin.PeerInfo) string {
	var sb strings.Builder
	sb.WriteString(`"peer":{"address":"`)
	sb.WriteString(peer.Address.String())
	sb.WriteByte('"')
	if peer.GroupName != "" {
		sb.WriteString(`,"group":"`)
		sb.WriteString(jsonSafeReplacer.Replace(peer.GroupName))
		sb.WriteByte('"')
	}
	if peer.LocalAS > 0 || peer.LocalAddress.IsValid() {
		sb.WriteString(`,"local":{`)
		first := true
		if peer.LocalAddress.IsValid() {
			sb.WriteString(`"address":"`)
			sb.WriteString(peer.LocalAddress.String())
			sb.WriteByte('"')
			first = false
		}
		if peer.LocalAS > 0 {
			if !first {
				sb.WriteByte(',')
			}
			sb.WriteString(`"as":`)
			sb.WriteString(strconv.FormatUint(uint64(peer.LocalAS), 10))
		}
		sb.WriteByte('}')
	}
	sb.WriteString(`,"name":"`)
	sb.WriteString(jsonSafeReplacer.Replace(peer.Name))
	sb.WriteString(`","remote":{"as":`)
	sb.WriteString(strconv.FormatUint(uint64(peer.PeerAS), 10))
	sb.WriteString(`}}`)
	return sb.String()
}

// FormatMessage formats a RawMessage based on ContentConfig.
// Uses lazy parsing via AttrsWire when available for optimal performance.
// Handles encoding (json/text), format (parsed/raw/full), and attribute filtering.
// If overrideDir is non-empty, it overrides msg.Direction for formatting.
func FormatMessage(peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, overrideDir string) string {
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
func formatEmptyUpdate(peer *plugin.PeerInfo, content bgptypes.ContentConfig) string {
	if content.Encoding == plugin.EncodingJSON {
		return `{"type":"bgp","bgp":{"message":{"type":"update"},` + peerJSONInline(peer) + `,"nlri":{}}}` + "\n"
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
func formatNonUpdate(peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, direction string) string {
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
		return `{"type":"bgp","bgp":{"message":{"type":"` + msgType + `"},` + peerJSONInline(peer) + `,"raw":{"message":"` + rawHex + `"}}}` + "\n"
	}
	return fmt.Sprintf("peer %s %s raw %s\n",
		peer.Address, strings.ToLower(msg.Type.String()), rawHex)
}

// formatFromFilterResult formats UPDATE using lazy-parsed FilterResult.
// This is the optimized path that only parses requested attributes.
// ctx provides ADD-PATH state per family (nil means no ADD-PATH).
func formatFromFilterResult(peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, result bgpfilter.FilterResult, ctx *bgpctx.EncodingContext, direction string) string {
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
func formatRawFromResult(peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, direction string) string {
	rawHex := fmt.Sprintf("%x", msg.RawBytes)
	if content.Encoding == plugin.EncodingJSON {
		var msgFields string
		if direction != "" {
			msgFields = fmt.Sprintf(`,"direction":%q`, direction)
		}
		return `{"type":"bgp","bgp":{"message":{"type":"update"` + msgFields + `},` + peerJSONInline(peer) + `,"raw":{"update":"` + rawHex + `"}}}` + "\n"
	}
	return fmt.Sprintf("peer %s %s update raw %s\n", peer.Address, direction, rawHex)
}

// formatParsedFromResult formats parsed UPDATE using FilterResult.
// ctx provides ADD-PATH state per family.
func formatParsedFromResult(peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, result bgpfilter.FilterResult, ctx *bgpctx.EncodingContext, direction string) string {
	if content.Encoding == plugin.EncodingJSON {
		return formatFilterResultJSON(peer, result, msg.MessageID, direction, ctx)
	}
	return formatFilterResultText(peer, result, msg.MessageID, direction, ctx)
}

// formatFullFromResult formats both parsed content AND raw hex (ze-bgp JSON).
// ctx provides ADD-PATH state per family.
// Includes raw bytes nested under "raw" object: attributes, nlri, withdrawn.
func formatFullFromResult(peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, result bgpfilter.FilterResult, ctx *bgpctx.EncodingContext, direction string) string {
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

				// nlri: per-fam raw bytes
				if len(rawComps.NLRI) > 0 {
					if hasContent {
						rawObj.WriteString(",")
					}
					rawObj.WriteString(`"nlri":{`)
					first := true
					for fam, nlriBytes := range rawComps.NLRI {
						if !first {
							rawObj.WriteString(",")
						}
						first = false
						fmt.Fprintf(&rawObj, `"%s":"%x"`, fam.String(), nlriBytes)
					}
					rawObj.WriteString(`}`)
					hasContent = true
				}

				// withdrawn: per-fam raw bytes
				if len(rawComps.Withdrawn) > 0 {
					if hasContent {
						rawObj.WriteString(",")
					}
					rawObj.WriteString(`"withdrawn":{`)
					first := true
					for fam, wdBytes := range rawComps.Withdrawn {
						if !first {
							rawObj.WriteString(",")
						}
						first = false
						fmt.Fprintf(&rawObj, `"%s":"%x"`, fam.String(), wdBytes)
					}
					rawObj.WriteString(`}`)
					hasContent = true
				}

				// RFC 7911 Section 3: ADD-PATH per-fam flags from negotiated capabilities.
				// Consumers (e.g., bgp-rib) need this to parse NLRI wire bytes correctly —
				// ADD-PATH prepends a 4-byte path-ID before each NLRI.
				if ctx != nil {
					var addPathBuf strings.Builder
					addPathFirst := true
					for fam := range rawComps.NLRI {
						if ctx.AddPathFor(fam) {
							if addPathFirst {
								addPathBuf.WriteString(`"add-path":{`)
							} else {
								addPathBuf.WriteString(",")
							}
							addPathFirst = false
							fmt.Fprintf(&addPathBuf, `"%s":true`, fam.String())
						}
					}
					for fam := range rawComps.Withdrawn {
						if ctx.AddPathFor(fam) {
							if _, inNLRI := rawComps.NLRI[fam]; inNLRI {
								continue // already emitted from NLRI loop
							}
							if addPathFirst {
								addPathBuf.WriteString(`"add-path":{`)
							} else {
								addPathBuf.WriteString(",")
							}
							addPathFirst = false
							fmt.Fprintf(&addPathBuf, `"%s":true`, fam.String())
						}
					}
					if !addPathFirst {
						addPathBuf.WriteString(`}`)
						if hasContent {
							rawObj.WriteString(",")
						}
						rawObj.WriteString(addPathBuf.String())
						hasContent = true
					}
				}
			}
		}

		// Add full update bytes
		if hasContent {
			rawObj.WriteString(",")
		}
		fmt.Fprintf(&rawObj, `"update":"%s"`, rawHex)

		rawObj.WriteString(`}`)

		// Inject route metadata if present (sideband, not in wire bytes).
		// Marshal error silently drops metadata (meta contains only string/bool values
		// from ingress filters; marshal failure requires a code bug, not external input).
		var metaJSON string
		if len(msg.Meta) > 0 {
			if metaBytes, err := json.Marshal(msg.Meta); err == nil {
				metaJSON = `,"route-meta":` + string(metaBytes)
			}
		}

		// ze-bgp JSON: inject raw + meta into bgp object (ends with "}}\n")
		// Replace trailing "}}\n" with ","raw":{...},"route-meta":{...}}}\n"
		if strings.HasSuffix(parsed, "}}\n") {
			return parsed[:len(parsed)-3] + "," + rawObj.String() + metaJSON + "}}\n"
		}
		return parsed
	}

	// For text, append raw line
	return parsed + fmt.Sprintf("peer %s %s update raw %s\n", peer.Address, direction, rawHex)
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

// FormatOpen formats an OPEN message as text output.
// Format: peer <ip> remote as <asn> <direction> open <msg-id> router-id <id> hold-time <t> [cap <code> <name> <value>]...
// ASN is the speaker's ASN (from the OPEN message).
// Capabilities use "cap <code> <name> <value>" format for easy parsing.
func FormatOpen(peer *plugin.PeerInfo, open DecodedOpen, direction string, msgID uint64) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "peer %s remote as %d %s open %d router-id %s hold-time %d",
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
// Format: peer <ip> remote as <asn> <direction> notification <msg-id> code <n> subcode <n> code-name <name> subcode-name <name> data <hex>.
// Names are hyphenated for single-word parsing (e.g., "Administrative-Shutdown").
func FormatNotification(peer *plugin.PeerInfo, notify DecodedNotification, direction string, msgID uint64) string {
	dataHex := ""
	if len(notify.Data) > 0 {
		dataHex = fmt.Sprintf("%x", notify.Data)
	}

	// Replace spaces with hyphens in names for easier parsing
	codeName := strings.ReplaceAll(notify.ErrorCodeName, " ", "-")
	subcodeName := strings.ReplaceAll(notify.ErrorSubcodeName, " ", "-")

	return fmt.Sprintf("peer %s remote as %d %s notification %d code %d subcode %d code-name %s subcode-name %s data %s\n",
		peer.Address, peer.PeerAS, direction, msgID, notify.ErrorCode, notify.ErrorSubcode,
		codeName, subcodeName, dataHex)
}

// FormatKeepalive formats a KEEPALIVE message as text output.
// Format: peer <ip> remote as <asn> <direction> keepalive <msg-id>.
func FormatKeepalive(peer *plugin.PeerInfo, direction string, msgID uint64) string {
	return fmt.Sprintf("peer %s remote as %d %s keepalive %d\n", peer.Address, peer.PeerAS, direction, msgID)
}

// FormatRouteRefresh formats a ROUTE-REFRESH message as text output.
// RFC 7313: Type is "refresh" (subtype 0), "borr" (subtype 1), or "eorr" (subtype 2).
// Format: peer <ip> remote as <asn> <direction> <type> <msg-id> family <family>.
func FormatRouteRefresh(peer *plugin.PeerInfo, decoded DecodedRouteRefresh, direction string, msgID uint64) string {
	return fmt.Sprintf("peer %s remote as %d %s %s %d family %s\n",
		peer.Address, peer.PeerAS, direction, decoded.SubtypeName, msgID, decoded.Family)
}

// FormatStateChange formats a peer state change event.
// State events are separate from BGP protocol messages.
// Common states: "up", "down", "connected", "established".
// FormatStateChange formats a peer state change event.
// reason is the close reason (empty for "up"): "tcp-failure", "notification", etc.
func FormatStateChange(peer *plugin.PeerInfo, state, reason, encoding string) string {
	if encoding == plugin.EncodingJSON {
		return formatStateChangeJSON(peer, state, reason)
	}
	return formatStateChangeText(peer, state, reason)
}

// FormatEOR formats an End-of-RIB marker event.
// RFC 4724 Section 2: EOR signals that initial routing information exchange is complete.
// family is the address family (e.g., "ipv4/unicast", "ipv6/unicast").
func FormatEOR(peer *plugin.PeerInfo, family, encoding string) string {
	if encoding == plugin.EncodingJSON {
		return `{"type":"bgp","bgp":{"message":{"type":"eor"},` + peerJSONInline(peer) + `,"eor":{"family":"` + family + `"}}}` + "\n"
	}
	return fmt.Sprintf("peer %s remote as %d eor %s\n", peer.Address, peer.PeerAS, family)
}

// FormatCongestion formats a forward-path congestion event.
// eventType is "congested" or "resumed". peerAddr is the destination peer address.
func FormatCongestion(peer *plugin.PeerInfo, eventType, encoding string) string {
	if encoding == plugin.EncodingJSON {
		return `{"type":"bgp","bgp":{"message":{"type":"` + eventType + `"},` + peerJSONInline(peer) + `}}` + "\n"
	}
	return fmt.Sprintf("peer %s remote as %d %s\n", peer.Address, peer.PeerAS, eventType)
}

// FormatNegotiated formats negotiated capabilities event.
// Sent after OPEN exchange to inform plugins of negotiated capabilities.
func FormatNegotiated(peer *plugin.PeerInfo, neg DecodedNegotiated, encoder *JSONEncoder) string {
	// Always use JSON for negotiated - too complex for text format
	return encoder.Negotiated(peer, neg)
}

// FormatSentMessage formats a sent UPDATE message.
// Uses "type":"sent" instead of "type":"update" to distinguish from received messages.
// For text format, uses "sent update" instead of "received update".
func FormatSentMessage(peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig) string {
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
