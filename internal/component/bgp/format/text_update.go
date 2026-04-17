// Design: docs/architecture/api/json-format.md — UPDATE message formatting
// Related: text.go — non-UPDATE formatters + peer/JSON helpers reused here
// Related: text_human.go — appendFilterResultText / appendAttributesText
// Related: text_json.go — appendFilterResultJSON / appendAttributesJSON
// Related: summary.go — appendSummary / appendSummaryJSON

package format

import (
	"encoding/hex"
	"encoding/json"
	"strings"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	bgpfilter "codeberg.org/thomas-mangin/ze/internal/component/bgp/filter"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// messageTypeUpdate and messageTypeSent are the two values used as the
// `message.type` field in ze-bgp JSON for UPDATE messages. Threaded through
// appendFilterResultJSON / appendSummaryJSON so the sent-vs-received distinction
// is written at source rather than patched in via strings.Replace.
const (
	messageTypeUpdate = "update"
	messageTypeSent   = "sent"
)

// AppendMessage appends a RawMessage to buf based on ContentConfig.
// Uses lazy parsing via AttrsWire when available for optimal performance.
// Handles encoding (json/text), format (parsed/raw/full), and attribute filtering.
// If overrideDir is non-empty, it overrides msg.Direction for formatting.
func AppendMessage(buf []byte, peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, overrideDir string) []byte {
	return appendMessageTyped(buf, peer, msg, content, overrideDir, messageTypeUpdate)
}

// AppendSentMessage appends a sent UPDATE to buf.
// Emits "type":"sent" in JSON (instead of "update") and "sent" as the text
// direction. No strings.Replace surgery -- the message type is written at
// source by threading `messageType` into the JSON writers.
func AppendSentMessage(buf []byte, peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig) []byte {
	return appendMessageTyped(buf, peer, msg, content, messageTypeSent, messageTypeSent)
}

// appendMessageTyped is the shared implementation for AppendMessage and
// AppendSentMessage. messageType is the literal JSON value for `message.type`
// ("update" or "sent"); the text-form direction is chosen by overrideDir.
func appendMessageTyped(buf []byte, peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, overrideDir, messageType string) []byte {
	content = content.WithDefaults()

	// Compute effective direction
	direction := msg.Direction
	if overrideDir != "" {
		direction = overrideDir
	}

	// Summary format: lightweight NLRI metadata only (skip full attribute parsing).
	// Must short-circuit before filter setup for performance.
	if content.Format == plugin.FormatSummary && msg.Type == message.TypeUPDATE {
		return appendSummary(buf, peer, msg.RawBytes, msg.MessageID, direction, messageType)
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
			return appendEmptyUpdate(buf, peer, content, messageType)
		}

		// Get encoding context for ADD-PATH state
		var encCtx *bgpctx.EncodingContext
		if msg.AttrsWire != nil {
			encCtx = bgpctx.Registry.Get(msg.AttrsWire.SourceContext())
		}

		return appendFromFilterResult(buf, peer, msg, content, result, encCtx, direction, messageType)
	}

	// Non-UPDATE messages: format as raw
	return appendNonUpdate(buf, peer, msg, content, direction)
}

// appendEmptyUpdate appends an empty UPDATE message to buf.
// ze-bgp JSON format: {"type":"bgp","bgp":{"message":{"type":"update"},...}}.
func appendEmptyUpdate(buf []byte, peer *plugin.PeerInfo, content bgptypes.ContentConfig, messageType string) []byte {
	if content.Encoding == plugin.EncodingJSON {
		buf = append(buf, `{"type":"bgp","bgp":{"message":{"type":"`...)
		buf = append(buf, messageType...)
		buf = append(buf, `"},`...)
		buf = appendPeerJSON(buf, peer)
		buf = append(buf, `,"nlri":{}}}`...)
		buf = append(buf, '\n')
		return buf
	}
	buf = append(buf, "peer "...)
	buf = peer.Address.AppendTo(buf)
	buf = append(buf, " update\n"...)
	return buf
}

// appendNonUpdate appends non-UPDATE messages (OPEN, NOTIFICATION, KEEPALIVE) to buf.
// Routes to dedicated formatters for parsed output, falls back to raw for unknown types.
//
// NOTE: For PARSED format, this function ignores content.Encoding and always returns TEXT.
// For RAW format, it respects Encoding (JSON or text with raw hex).
// For structured JSON output of non-UPDATE messages, use Server.formatMessage()
// which has access to the shared JSONEncoder with proper counter semantics.
func appendNonUpdate(buf []byte, peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, direction string) []byte {
	// For parsed format, use dedicated text formatters.
	if content.Format != plugin.FormatRaw {
		switch msg.Type { //nolint:exhaustive // only specific types have dedicated formatters
		case message.TypeOPEN:
			decoded := DecodeOpen(msg.RawBytes)
			return AppendOpen(buf, peer, decoded, direction, msg.MessageID)
		case message.TypeNOTIFICATION:
			decoded := DecodeNotification(msg.RawBytes)
			return AppendNotification(buf, peer, decoded, direction, msg.MessageID)
		case message.TypeKEEPALIVE:
			return AppendKeepalive(buf, peer, direction, msg.MessageID)
		}
	}

	// Raw format or unknown type
	if content.Encoding == plugin.EncodingJSON {
		// ze-bgp JSON format: {"type":"bgp","bgp":{"message":{"type":"..."},...}}
		msgType := strings.ToLower(msg.Type.String())
		buf = append(buf, `{"type":"bgp","bgp":{"message":{"type":"`...)
		buf = append(buf, msgType...)
		buf = append(buf, `"},`...)
		buf = appendPeerJSON(buf, peer)
		buf = append(buf, `,"raw":{"message":"`...)
		buf = hex.AppendEncode(buf, msg.RawBytes)
		buf = append(buf, `"}}}`...)
		buf = append(buf, '\n')
		return buf
	}
	buf = append(buf, "peer "...)
	buf = peer.Address.AppendTo(buf)
	buf = append(buf, ' ')
	buf = append(buf, strings.ToLower(msg.Type.String())...)
	buf = append(buf, " raw "...)
	buf = hex.AppendEncode(buf, msg.RawBytes)
	buf = append(buf, '\n')
	return buf
}

// appendFromFilterResult appends UPDATE using lazy-parsed FilterResult to buf.
// This is the optimized path that only parses requested attributes.
// ctx provides ADD-PATH state per family (nil means no ADD-PATH).
func appendFromFilterResult(buf []byte, peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, result bgpfilter.FilterResult, ctx *bgpctx.EncodingContext, direction, messageType string) []byte {
	switch content.Format {
	case plugin.FormatRaw, plugin.FormatHex:
		return appendRawFromResult(buf, peer, msg, content, direction, messageType)
	case plugin.FormatFull:
		return appendFullFromResult(buf, peer, msg, content, result, ctx, direction, messageType)
	}
	// FormatParsed (the common case)
	return appendParsedFromResult(buf, peer, msg, content, result, ctx, direction, messageType)
}

// appendRawFromResult appends raw hex (does not need FilterResult attributes) to buf.
// ze-bgp JSON format: {"type":"bgp","bgp":{"message":{"type":"update",...},...}}.
func appendRawFromResult(buf []byte, peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, direction, messageType string) []byte {
	if content.Encoding == plugin.EncodingJSON {
		buf = append(buf, `{"type":"bgp","bgp":{"message":{"type":"`...)
		buf = append(buf, messageType...)
		buf = append(buf, '"')
		if direction != "" {
			// Defensive escape: direction comes from the BGP stack today
			// ("received"/"sent") but appendJSONString costs nothing on
			// those short ASCII strings and closes the gap vs. the legacy
			// fmt.Sprintf(%q) shape if the value ever widens.
			buf = append(buf, `,"direction":"`...)
			buf = appendJSONString(buf, direction)
			buf = append(buf, '"')
		}
		buf = append(buf, `},`...)
		buf = appendPeerJSON(buf, peer)
		buf = append(buf, `,"raw":{"update":"`...)
		buf = hex.AppendEncode(buf, msg.RawBytes)
		buf = append(buf, `"}}}`...)
		buf = append(buf, '\n')
		return buf
	}
	buf = append(buf, "peer "...)
	buf = peer.Address.AppendTo(buf)
	buf = append(buf, ' ')
	buf = append(buf, direction...)
	buf = append(buf, " update raw "...)
	buf = hex.AppendEncode(buf, msg.RawBytes)
	buf = append(buf, '\n')
	return buf
}

// appendParsedFromResult appends parsed UPDATE using FilterResult to buf.
// ctx provides ADD-PATH state per family.
func appendParsedFromResult(buf []byte, peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, result bgpfilter.FilterResult, ctx *bgpctx.EncodingContext, direction, messageType string) []byte {
	if content.Encoding == plugin.EncodingJSON {
		return appendFilterResultJSON(buf, peer, result, msg.MessageID, direction, ctx, messageType, true)
	}
	return appendFilterResultText(buf, peer, result, msg.MessageID, direction, ctx)
}

// appendFullFromResult appends both parsed content AND raw hex (ze-bgp JSON) to buf.
// ctx provides ADD-PATH state per family.
// Includes raw bytes nested under "raw" object: attributes, nlri, withdrawn.
// Instead of the legacy strings.HasSuffix + slice surgery, this writes the
// parsed body WITHOUT its final close, then appends `,"raw":{...}`,
// `,"route-meta":{...}` (if present), and the closing `}}\n` directly.
func appendFullFromResult(buf []byte, peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, result bgpfilter.FilterResult, ctx *bgpctx.EncodingContext, direction, messageType string) []byte {
	if content.Encoding != plugin.EncodingJSON {
		// Text path: parsed body + "peer <ip> <dir> update raw <hex>\n"
		buf = appendFilterResultText(buf, peer, result, msg.MessageID, direction, ctx)
		buf = append(buf, "peer "...)
		buf = peer.Address.AppendTo(buf)
		buf = append(buf, ' ')
		buf = append(buf, direction...)
		buf = append(buf, " update raw "...)
		buf = hex.AppendEncode(buf, msg.RawBytes)
		buf = append(buf, '\n')
		return buf
	}

	// JSON path: write the parsed body WITHOUT its final `}}}\n` close.
	//
	// INVARIANT (contract with appendFilterResultJSON): when
	// closeEnvelope=false, the writer leaves BOTH the outer `"bgp":{`
	// object and the inner `"update":{` object open. It ends with
	// `...,"nlri":{...}}` -- the last `}` closes the `nlri` sub-object.
	// This function completes the envelope by:
	//   1. writing `}` to close the `update` object
	//   2. writing `,"raw":{...}` as a sibling of `update` in `bgp`
	//   3. optionally writing `,"route-meta":{...}` as another sibling
	//   4. writing `}}\n` to close `bgp` and the outer `{"type":...` object
	//
	// If appendFilterResultJSON ever changes what it leaves open, this
	// function must update in lockstep. A shape mismatch here would produce
	// malformed JSON silently (the legacy strings.HasSuffix guard is gone).
	buf = appendFilterResultJSON(buf, peer, result, msg.MessageID, direction, ctx, messageType, false)

	// Close the update object first, then inject raw / route-meta into the
	// bgp object alongside update and peer.
	buf = append(buf, '}') // close "update"
	buf = append(buf, `,"raw":{`...)

	hasContent := false
	// Extract raw components if WireUpdate available
	if msg.WireUpdate != nil {
		rawComps, err := wireu.ExtractRawComponents(msg.WireUpdate)
		if err == nil && rawComps != nil {
			// attributes: raw bytes without MP_REACH/MP_UNREACH
			if len(rawComps.Attributes) > 0 {
				buf = append(buf, `"attributes":"`...)
				buf = hex.AppendEncode(buf, rawComps.Attributes)
				buf = append(buf, '"')
				hasContent = true
			}

			// nlri: per-fam raw bytes
			if len(rawComps.NLRI) > 0 {
				if hasContent {
					buf = append(buf, ',')
				}
				buf = append(buf, `"nlri":{`...)
				first := true
				for fam, nlriBytes := range rawComps.NLRI {
					if !first {
						buf = append(buf, ',')
					}
					first = false
					buf = append(buf, '"')
					buf = append(buf, fam.String()...)
					buf = append(buf, `":"`...)
					buf = hex.AppendEncode(buf, nlriBytes)
					buf = append(buf, '"')
				}
				buf = append(buf, '}')
				hasContent = true
			}

			// withdrawn: per-fam raw bytes
			if len(rawComps.Withdrawn) > 0 {
				if hasContent {
					buf = append(buf, ',')
				}
				buf = append(buf, `"withdrawn":{`...)
				first := true
				for fam, wdBytes := range rawComps.Withdrawn {
					if !first {
						buf = append(buf, ',')
					}
					first = false
					buf = append(buf, '"')
					buf = append(buf, fam.String()...)
					buf = append(buf, `":"`...)
					buf = hex.AppendEncode(buf, wdBytes)
					buf = append(buf, '"')
				}
				buf = append(buf, '}')
				hasContent = true
			}

			// RFC 7911 Section 3: ADD-PATH per-fam flags from negotiated capabilities.
			// Consumers (e.g., bgp-rib) need this to parse NLRI wire bytes correctly --
			// ADD-PATH prepends a 4-byte path-ID before each NLRI.
			if ctx != nil {
				buf, hasContent = appendAddPathFlags(buf, rawComps, ctx, hasContent)
			}
		}
	}

	// Add full update bytes
	if hasContent {
		buf = append(buf, ',')
	}
	buf = append(buf, `"update":"`...)
	buf = hex.AppendEncode(buf, msg.RawBytes)
	buf = append(buf, '"', '}') // close "update" hex then close "raw"

	// Inject route metadata if present (sideband, not in wire bytes).
	// Marshal error silently drops metadata (meta contains only string/bool values
	// from ingress filters; marshal failure requires a code bug, not external input).
	if len(msg.Meta) > 0 {
		if metaBytes, err := json.Marshal(msg.Meta); err == nil {
			buf = append(buf, `,"route-meta":`...)
			buf = append(buf, metaBytes...)
		}
	}

	// Close bgp and outer wrapper
	buf = append(buf, "}}\n"...)
	return buf
}

// appendAddPathFlags writes the RFC 7911 ADD-PATH per-family flags block to buf,
// returning the updated buf and whether the bgp.raw object already has content
// (so the caller knows whether to emit a separator before further keys).
// The flags block only appears when at least one family has ADD-PATH negotiated.
func appendAddPathFlags(buf []byte, rawComps *wireu.RawUpdateComponents, ctx *bgpctx.EncodingContext, hasContent bool) ([]byte, bool) {
	// Collect families with ADD-PATH set, preserving the legacy dedup behavior:
	// each family is emitted at most once, NLRI families first (iteration order),
	// then withdrawn-only families.
	first := true
	emit := func(famStr string) {
		if first {
			if hasContent {
				buf = append(buf, ',')
			}
			buf = append(buf, `"add-path":{`...)
			first = false
		} else {
			buf = append(buf, ',')
		}
		buf = append(buf, '"')
		buf = append(buf, famStr...)
		buf = append(buf, `":true`...)
	}

	for fam := range rawComps.NLRI {
		if ctx.AddPathFor(fam) {
			emit(fam.String())
		}
	}
	for fam := range rawComps.Withdrawn {
		if ctx.AddPathFor(fam) {
			if _, inNLRI := rawComps.NLRI[fam]; inNLRI {
				continue // already emitted from NLRI loop
			}
			emit(fam.String())
		}
	}
	if !first {
		buf = append(buf, '}')
		hasContent = true
	}
	return buf, hasContent
}
