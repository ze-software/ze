// Design: docs/architecture/api/json-format.md — UPDATE message formatting
// Related: text.go — non-UPDATE formatters + peer/JSON helpers reused here

package format

import (
	"encoding/json"
	"fmt"
	"strings"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	bgpfilter "codeberg.org/thomas-mangin/ze/internal/component/bgp/filter"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

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
	// For parsed format, use dedicated text formatters. Each Append* call
	// uses a stack-local scratch; the single string conversion is the
	// boundary allocation (spec AC-9 edge).
	if content.Format != plugin.FormatRaw {
		var scratch [512]byte
		switch msg.Type { //nolint:exhaustive // only specific types have dedicated formatters
		case message.TypeOPEN:
			decoded := DecodeOpen(msg.RawBytes)
			return string(AppendOpen(scratch[:0], peer, decoded, direction, msg.MessageID))
		case message.TypeNOTIFICATION:
			decoded := DecodeNotification(msg.RawBytes)
			return string(AppendNotification(scratch[:0], peer, decoded, direction, msg.MessageID))
		case message.TypeKEEPALIVE:
			return string(AppendKeepalive(scratch[:0], peer, direction, msg.MessageID))
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
