// Design: docs/architecture/api/json-format.md — UPDATE message formatting
// Related: text.go — non-UPDATE formatters + peer/JSON helpers reused here
// Related: text_human.go — appendFilterResultText / appendAttributesText
// Related: text_json.go — appendFilterResultJSON / appendAttributesJSON
// Related: summary.go — appendSummary / appendSummaryJSON

package format

import (
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	bgpfilter "github.com/ze-software/ze/internal/component/bgp/filter"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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
func AppendMessage(buf []byte, peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig) []byte {
	return appendMessageTyped(buf, peer, msg, content, rpc.DirectionUnspecified, messageTypeUpdate)
}

// AppendSentMessage appends a sent UPDATE to buf.
// Emits "type":"sent" in JSON (instead of "update") and "sent" as the text
// direction. No strings.Replace surgery -- the message type is written at
// source by threading `messageType` into the JSON writers.
func AppendSentMessage(buf []byte, peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig) []byte {
	return appendMessageTyped(buf, peer, msg, content, rpc.DirectionSent, messageTypeSent)
}

// appendMessageTyped is the shared implementation for AppendMessage and
// AppendSentMessage. messageType is the literal JSON value for `message.type`
// ("update" or "sent"); dirOverride replaces msg.Direction when non-zero.
func appendMessageTyped(buf []byte, peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, dirOverride rpc.MessageDirection, messageType string) []byte {
	content = content.WithDefaults()

	direction := msg.Direction
	if dirOverride != rpc.DirectionUnspecified {
		direction = dirOverride
	}

	if content.Format == plugin.FormatSummary && msg.Type == msgtype.TypeUPDATE {
		return appendSummary(buf, peer, msg.RawBytes, msg.MessageID, direction, messageType)
	}

	// Fast path: parsed JSON with no attribute or NLRI filter. Bypasses the
	// filter machinery (map alloc, []Attribute slice, NLRI parsing) and writes
	// directly from AttrsWire + body NLRI bytes. Falls through to the generic
	// path for selective filters, text encoding, or non-UPDATE messages.
	if msg.Type == msgtype.TypeUPDATE &&
		content.Encoding == plugin.EncodingJSON &&
		content.Format == plugin.FormatParsed &&
		content.Attributes == nil && content.NLRI == nil &&
		msg.AttrsWire != nil {
		var encCtx *bgpctx.EncodingContext
		if msg.AttrsWire != nil {
			encCtx = bgpctx.Registry.Get(msg.AttrsWire.SourceContext())
		}
		return appendParsedUpdateJSONDirect(buf, peer, msg, encCtx, direction, messageType)
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
	if msg.Type == msgtype.TypeUPDATE {
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

	// Non-UPDATE messages: pass typed direction (no .String() on hot path).
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
func appendNonUpdate(buf []byte, peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, direction rpc.MessageDirection) []byte {
	// For parsed format, use dedicated text formatters.
	if content.Format != plugin.FormatRaw {
		switch msg.Type { //nolint:exhaustive // only specific types have dedicated formatters
		case msgtype.TypeOPEN:
			decoded := DecodeOpen(msg.RawBytes)
			return AppendOpen(buf, peer, decoded, direction, msg.MessageID)
		case msgtype.TypeNOTIFICATION:
			decoded := DecodeNotification(msg.RawBytes)
			return AppendNotification(buf, peer, decoded, direction, msg.MessageID)
		case msgtype.TypeKEEPALIVE:
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
func appendFromFilterResult(buf []byte, peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, result bgpfilter.FilterResult, ctx *bgpctx.EncodingContext, direction rpc.MessageDirection, messageType string) []byte {
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
func appendRawFromResult(buf []byte, peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, direction rpc.MessageDirection, messageType string) []byte {
	if content.Encoding == plugin.EncodingJSON {
		buf = append(buf, `{"type":"bgp","bgp":{"message":{"type":"`...)
		buf = append(buf, messageType...)
		buf = append(buf, '"')
		if direction != rpc.DirectionUnspecified {
			buf = append(buf, `,"direction":"`...)
			buf = direction.AppendTo(buf)
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
	buf = direction.AppendTo(buf)
	buf = append(buf, " update raw "...)
	buf = hex.AppendEncode(buf, msg.RawBytes)
	buf = append(buf, '\n')
	return buf
}

// appendParsedUpdateJSONDirect writes parsed JSON directly from AttrsWire and
// body NLRI bytes, bypassing the filter machinery. This is the zero-alloc fast
// path for the common case: all attributes, all families, parsed JSON encoding.
// On a warm AttrsWire (attributes already parsed and cached), no allocations
// occur; the entire output is appended to the caller-owned buffer.
func appendParsedUpdateJSONDirect(buf []byte, peer *plugin.PeerInfo, msg bgptypes.RawMessage, ctx *bgpctx.EncodingContext, direction rpc.MessageDirection, messageType string) []byte {
	buf = append(buf, `{"type":"bgp","bgp":{`...)

	buf = append(buf, `"message":{"type":"`...)
	buf = append(buf, messageType...)
	buf = append(buf, '"')
	if msg.MessageID > 0 {
		buf = append(buf, `,"id":`...)
		buf = strconv.AppendUint(buf, msg.MessageID, 10)
	}
	if direction != rpc.DirectionUnspecified {
		buf = append(buf, `,"direction":"`...)
		buf = direction.AppendTo(buf)
		buf = append(buf, '"')
	}
	buf = append(buf, '}', ',')
	buf = appendPeerJSON(buf, peer)

	buf = append(buf, `,"update":{`...)

	// Attributes: iterate AttrsWire directly (no []Attribute slice allocation).
	// Skip MP_REACH/MP_UNREACH (rendered in the NLRI section).
	hasAttrs := false
	if msg.AttrsWire != nil {
		buf = append(buf, `"attr":{`...)
		first := true
		_ = msg.AttrsWire.ForEach(func(code attribute.AttributeCode, attr attribute.Attribute) bool {
			if code == attribute.AttrMPReachNLRI || code == attribute.AttrMPUnreachNLRI {
				return true
			}
			if !first {
				buf = append(buf, ',')
			}
			first = false
			buf = appendAttributeJSON(buf, code, attr, false)
			return true
		})
		buf = append(buf, `},`...)
		hasAttrs = true
	}
	if !hasAttrs {
		buf = append(buf, `"attr":{},`...)
	}

	// NLRI: extract from body + MP attributes without allocating intermediate
	// slices. Body sections give IPv4 unicast; MP attributes give other families.
	// RFC 4760: a single UPDATE may carry both body NLRI (legacy IPv4) and
	// MP_REACH_NLRI for the same family with different next-hops.
	buf = append(buf, `"nlri":{`...)
	famFirst := true

	ipv4AddPath := false
	if ctx != nil {
		ipv4AddPath = ctx.AddPathFor(family.IPv4Unicast)
	}

	nlriData, _ := msg.WireUpdate.NLRI()
	withdrawnData, _ := msg.WireUpdate.Withdrawn()
	mpReach, _ := msg.WireUpdate.MPReach()
	mpUnreach, _ := msg.WireUpdate.MPUnreach()

	// Check if MP_REACH also targets IPv4/unicast (dual next-hop case).
	mpReachIsIPv4 := mpReach != nil && mpReach.Family() == family.IPv4Unicast

	// IPv4 unicast: merge body NLRI and any MP_REACH for ipv4/unicast into
	// a single "ipv4/unicast" array so dual next-hop UPDATEs produce two
	// operation groups under one family key.
	hasIPv4 := len(nlriData) > 0 || len(withdrawnData) > 0 || mpReachIsIPv4
	if hasIPv4 {
		buf = append(buf, `"ipv4/unicast":[`...)
		opFirst := true

		if len(nlriData) > 0 {
			var nextHop netip.Addr
			if nh, err := msg.AttrsWire.Get(attribute.AttrNextHop); err == nil && nh != nil {
				if nhAttr, ok := nh.(*attribute.NextHop); ok {
					nextHop = nhAttr.Addr
				}
			}
			buf = append(buf, '{')
			if nextHop.IsValid() {
				buf = append(buf, `"next-hop":"`...)
				buf = nextHop.AppendTo(buf)
				buf = append(buf, `",`...)
			}
			buf = append(buf, `"action":"add","nlri":[`...)
			buf = appendIPv4PrefixesFromWire(buf, nlriData, ipv4AddPath)
			buf = append(buf, `]}`...)
			opFirst = false
		}

		if mpReachIsIPv4 {
			if !opFirst {
				buf = append(buf, ',')
			}
			nh := mpReach.NextHop()
			buf = append(buf, `{"next-hop":"`...)
			buf = nh.AppendTo(buf)
			buf = append(buf, `","action":"add","nlri":[`...)
			mpNLRIs, err := mpReach.NLRIs(ipv4AddPath)
			if err == nil {
				for j, n := range mpNLRIs {
					if j > 0 {
						buf = append(buf, ',')
					}
					buf = appendNLRIJSONValue(buf, n, family.IPv4Unicast)
				}
			}
			buf = append(buf, `]}`...)
			opFirst = false
		}

		if len(withdrawnData) > 0 {
			if !opFirst {
				buf = append(buf, ',')
			}
			buf = append(buf, `{"action":"del","nlri":[`...)
			buf = appendIPv4PrefixesFromWire(buf, withdrawnData, ipv4AddPath)
			buf = append(buf, `]}`...)
		}

		buf = append(buf, ']')
		famFirst = false
	}

	// Non-IPv4 MP_REACH families.
	if mpReach != nil && !mpReachIsIPv4 {
		if !famFirst {
			buf = append(buf, ',')
		}
		fam := mpReach.Family()
		mpAddPath := false
		if ctx != nil {
			mpAddPath = ctx.AddPathFor(fam)
		}
		buf = append(buf, '"')
		buf = fam.AppendTo(buf)
		buf = append(buf, `":[{"next-hop":"`...)
		nh := mpReach.NextHop()
		buf = nh.AppendTo(buf)
		buf = append(buf, `","action":"add","nlri":[`...)
		mpNLRIs, err := mpReach.NLRIs(mpAddPath)
		if err == nil {
			for j, n := range mpNLRIs {
				if j > 0 {
					buf = append(buf, ',')
				}
				buf = appendNLRIJSONValue(buf, n, fam)
			}
		}
		buf = append(buf, `]}]`...)
		famFirst = false
	}

	// MP_UNREACH families.
	if mpUnreach != nil {
		fam := mpUnreach.Family()
		mpAddPath := false
		if ctx != nil {
			mpAddPath = ctx.AddPathFor(fam)
		}
		mpWdNLRIs, err := mpUnreach.NLRIs(mpAddPath)
		if err == nil && len(mpWdNLRIs) > 0 {
			if !famFirst {
				buf = append(buf, ',')
			}
			buf = append(buf, '"')
			buf = fam.AppendTo(buf)
			buf = append(buf, `":[{"action":"del","nlri":[`...)
			for j, n := range mpWdNLRIs {
				if j > 0 {
					buf = append(buf, ',')
				}
				buf = appendNLRIJSONValue(buf, n, fam)
			}
			buf = append(buf, `]}]`...)
		}
	}

	buf = append(buf, '}') // close nlri
	buf = append(buf, "}}}\n"...)
	return buf
}

// appendIPv4PrefixesFromWire iterates raw IPv4 NLRI wire bytes and appends
// each prefix as a JSON string (or ADD-PATH object) directly, without
// allocating parsed NLRI structs or slices.
func appendIPv4PrefixesFromWire(buf, data []byte, addPath bool) []byte {
	offset := 0
	first := true
	for offset < len(data) {
		var pathID uint32
		if addPath {
			if offset+4 > len(data) {
				break
			}
			pathID = uint32(data[offset])<<24 | uint32(data[offset+1])<<16 |
				uint32(data[offset+2])<<8 | uint32(data[offset+3])
			offset += 4
		}
		if offset >= len(data) {
			break
		}
		prefixBits := int(data[offset])
		prefixBytes := (prefixBits + 7) / 8
		offset++
		if offset+prefixBytes > len(data) {
			break
		}
		var addr [4]byte
		copy(addr[:], data[offset:offset+prefixBytes])
		offset += prefixBytes

		pfx := netip.PrefixFrom(netip.AddrFrom4(addr), prefixBits)
		if !first {
			buf = append(buf, ',')
		}
		first = false
		if addPath && pathID != 0 {
			buf = append(buf, `{"prefix":"`...)
			buf = pfx.AppendTo(buf)
			buf = append(buf, `","path-id":`...)
			buf = strconv.AppendUint(buf, uint64(pathID), 10)
			buf = append(buf, '}')
		} else {
			buf = append(buf, '"')
			buf = pfx.AppendTo(buf)
			buf = append(buf, '"')
		}
	}
	return buf
}

// appendParsedFromResult appends parsed UPDATE using FilterResult to buf.
// ctx provides ADD-PATH state per family.
func appendParsedFromResult(buf []byte, peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, result bgpfilter.FilterResult, ctx *bgpctx.EncodingContext, direction rpc.MessageDirection, messageType string) []byte {
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
func appendFullFromResult(buf []byte, peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, result bgpfilter.FilterResult, ctx *bgpctx.EncodingContext, direction rpc.MessageDirection, messageType string) []byte {
	if content.Encoding != plugin.EncodingJSON {
		// Text path: parsed body + "peer <ip> <dir> update raw <hex>\n"
		buf = appendFilterResultText(buf, peer, result, msg.MessageID, direction, ctx)
		buf = append(buf, "peer "...)
		buf = peer.Address.AppendTo(buf)
		buf = append(buf, ' ')
		buf = direction.AppendTo(buf)
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
	// Write raw components directly from WireUpdate sections, without
	// allocating the RawUpdateComponents struct or its per-family maps.
	if msg.WireUpdate != nil {
		buf, hasContent = appendRawSectionsJSON(buf, msg.WireUpdate, ctx)
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
	hasMeta := len(msg.Meta) > 0 || msg.SourcePeerStr != ""
	if hasMeta {
		buf = append(buf, `,"route-meta":{`...)
		first := true
		if msg.SourcePeerStr != "" {
			buf = append(buf, `"source-peer":`...)
			if sb, err := json.Marshal(msg.SourcePeerStr); err == nil {
				buf = append(buf, sb...)
				first = false
			}
		}
		for k, v := range msg.Meta {
			kb, kerr := json.Marshal(k)
			vb, verr := json.Marshal(v)
			if kerr != nil || verr != nil {
				continue
			}
			if !first {
				buf = append(buf, ',')
			}
			buf = append(buf, kb...)
			buf = append(buf, ':')
			buf = append(buf, vb...)
			first = false
		}
		buf = append(buf, '}')
	}

	// Close bgp and outer wrapper
	buf = append(buf, "}}\n"...)
	return buf
}

// appendRawSectionsJSON writes the "raw" object contents directly from
// WireUpdate sections, without allocating RawUpdateComponents or its maps.
// Returns the updated buf and whether any content was written.
func appendRawSectionsJSON(buf []byte, wu *wireu.WireUpdate, ctx *bgpctx.EncodingContext) ([]byte, bool) {
	hasContent := false

	// Attributes hex: packed bytes with MP_REACH (14) and MP_UNREACH (15) excluded.
	// Skip the key entirely when filtering removes all attributes (all-MP UPDATE).
	attrs, _ := wu.Attrs()
	if attrs != nil {
		packed := attrs.Packed()
		if len(packed) > 0 {
			mark := len(buf)
			buf = append(buf, `"attributes":"`...)
			beforeHex := len(buf)
			buf = appendAttrHexFilterMP(buf, packed)
			if len(buf) > beforeHex {
				buf = append(buf, '"')
				hasContent = true
			} else {
				buf = buf[:mark]
			}
		}
	}

	// NLRI per family: body IPv4 unicast + MP_REACH.
	// When MP_REACH is also IPv4/unicast (dual next-hop), the old code's map
	// overwrote body with MP. Replicate that: prefer MP_REACH bytes for the
	// overlapping family to match ExtractAllRawNLRI semantics.
	nlriData, _ := wu.NLRI()
	mpReach, _ := wu.MPReach()
	mpReachIsIPv4 := mpReach != nil && mpReach.Family() == family.IPv4Unicast
	hasNLRI := len(nlriData) > 0 || mpReach != nil
	if hasNLRI {
		if hasContent {
			buf = append(buf, ',')
		}
		buf = append(buf, `"nlri":{`...)
		nlriFirst := true
		if len(nlriData) > 0 && !mpReachIsIPv4 {
			buf = append(buf, `"ipv4/unicast":"`...)
			buf = hex.AppendEncode(buf, nlriData)
			buf = append(buf, '"')
			nlriFirst = false
		}
		if mpReach != nil {
			if !nlriFirst {
				buf = append(buf, ',')
			}
			fam := mpReach.Family()
			buf = append(buf, '"')
			buf = fam.AppendTo(buf)
			buf = append(buf, `":"`...)
			buf = hex.AppendEncode(buf, mpReach.NLRIBytes())
			buf = append(buf, '"')
		}
		buf = append(buf, '}')
		hasContent = true
	}

	// Withdrawn per family: body IPv4 unicast + MP_UNREACH.
	wdData, _ := wu.Withdrawn()
	mpUnreach, _ := wu.MPUnreach()
	hasWd := len(wdData) > 0 || mpUnreach != nil
	if hasWd {
		if hasContent {
			buf = append(buf, ',')
		}
		buf = append(buf, `"withdrawn":{`...)
		wdFirst := true
		if len(wdData) > 0 {
			buf = append(buf, `"ipv4/unicast":"`...)
			buf = hex.AppendEncode(buf, wdData)
			buf = append(buf, '"')
			wdFirst = false
		}
		if mpUnreach != nil {
			if !wdFirst {
				buf = append(buf, ',')
			}
			fam := mpUnreach.Family()
			buf = append(buf, '"')
			buf = fam.AppendTo(buf)
			buf = append(buf, `":"`...)
			buf = hex.AppendEncode(buf, mpUnreach.WithdrawnBytes())
			buf = append(buf, '"')
		}
		buf = append(buf, '}')
		hasContent = true
	}

	// RFC 7911: ADD-PATH per-family flags.
	if ctx != nil {
		addPathFirst := true
		emitAddPath := func(fam family.Family) {
			if !ctx.AddPathFor(fam) {
				return
			}
			if addPathFirst {
				if hasContent {
					buf = append(buf, ',')
				}
				buf = append(buf, `"add-path":{`...)
				addPathFirst = false
			} else {
				buf = append(buf, ',')
			}
			buf = append(buf, '"')
			buf = fam.AppendTo(buf)
			buf = append(buf, `":true`...)
		}
		if len(nlriData) > 0 {
			emitAddPath(family.IPv4Unicast)
		}
		if mpReach != nil {
			emitAddPath(mpReach.Family())
		}
		if len(wdData) > 0 && len(nlriData) == 0 {
			emitAddPath(family.IPv4Unicast)
		}
		if mpUnreach != nil && (mpReach == nil || mpReach.Family() != mpUnreach.Family()) {
			emitAddPath(mpUnreach.Family())
		}
		if !addPathFirst {
			buf = append(buf, '}')
			hasContent = true
		}
	}

	return buf, hasContent
}

// appendAttrHexFilterMP hex-encodes packed attribute bytes, skipping
// MP_REACH_NLRI (14) and MP_UNREACH_NLRI (15) attributes inline.
func appendAttrHexFilterMP(buf, packed []byte) []byte {
	offset := 0
	for offset < len(packed) {
		if offset+2 > len(packed) {
			break
		}
		flags := packed[offset]
		code := packed[offset+1]

		var attrLen int
		headerLen := 3
		if flags&0x10 != 0 { // Extended length
			if offset+4 > len(packed) {
				break
			}
			attrLen = int(packed[offset+2])<<8 | int(packed[offset+3])
			headerLen = 4
		} else {
			if offset+3 > len(packed) {
				break
			}
			attrLen = int(packed[offset+2])
		}

		totalLen := headerLen + attrLen
		if offset+totalLen > len(packed) {
			break
		}

		if code != byte(attribute.AttrMPReachNLRI) && code != byte(attribute.AttrMPUnreachNLRI) {
			buf = hex.AppendEncode(buf, packed[offset:offset+totalLen])
		}
		offset += totalLen
	}
	return buf
}
