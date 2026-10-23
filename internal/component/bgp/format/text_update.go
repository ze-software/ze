// Design: docs/architecture/api/json-format.md — UPDATE message formatting
// Related: text.go — non-UPDATE formatters + peer/JSON helpers reused here
// Related: text_human.go — appendFilterResultText / appendAttributesText
// Related: text_json.go — appendFilterResultJSON / appendAttributesJSON
// Related: summary.go — appendSummary / appendSummaryJSON
// RFC: rfc/short/rfc4271.md -- Section 4.3, the body NLRI and Withdrawn Routes fields
// RFC: rfc/short/rfc7606.md -- Section 5.1, NLRI fields in any combination

package format

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	bgpfilter "github.com/ze-software/ze/internal/component/bgp/filter"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
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

	// An End-of-RIB marker names the family whose initial advertisement ended,
	// and that family is the marker's whole content. The writers below render
	// it from its wire sections, which are empty by construction, so they write
	// an UPDATE with an empty `attr` and an empty `nlri` and the family is
	// gone. A consumer then cannot tell which RIB ended, and the ExaBGP bridge
	// cannot write the `eor` member ExaBGP's own encoder writes for the same
	// message (src/exabgp/reactor/api/response/json.py, _update).
	//
	// RFC 4724 Section 2: "An UPDATE message with no reachable Network Layer
	// Reachability Information (NLRI) and empty withdrawn NLRI is specified as
	// the End-of-RIB marker that can be used by a BGP speaker to indicate to
	// its peer the completion of the initial routing update after the session
	// is established."
	//
	// The parsed and the `full` JSON renderings are answered here. `full`
	// carries the marker's wire bytes beside its parsed form, and until
	// 2026-10-22 that parsed form was the empty UPDATE, so the ExaBGP bridge,
	// which subscribes `full`, wrote an End-of-RIB as an UPDATE of nothing
	// (TestOldResponses50). `raw` and `hex` carry the bytes alone, from which
	// the family is read back. The text encoding states the marker through
	// AppendEOR, which already names the family.
	if isEORJSONUpdate(msg, content) {
		if fam, isEOR := msg.WireUpdate.IsEOR(); isEOR {
			return appendEORUpdateJSON(buf, peer, msg, fam, direction, messageType, content.Format == plugin.FormatFull)
		}
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
		encCtx, known := msg.SourceEncodingContext()
		if !known {
			// The ADD-PATH layout is unknown, so no route can be read from it.
			return appendEmptyUpdate(buf, peer, msg, content, direction, messageType)
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
			return appendEmptyUpdate(buf, peer, msg, content, direction, messageType)
		}

		// RFC 7911 Section 3: the ADD-PATH state comes from the UPDATE itself,
		// so a withdraw-only UPDATE (no attribute section) keeps its Path
		// Identifiers. An UPDATE naming a context the registry does not hold
		// has no known layout, so no route is read from it.
		encCtx, known := msg.SourceEncodingContext()
		if !known {
			return appendEmptyUpdate(buf, peer, msg, content, direction, messageType)
		}
		return appendFromFilterResult(buf, peer, msg, content, result, encCtx, direction, messageType)
	}

	// Non-UPDATE messages: pass typed direction (no .String() on hot path).
	return appendNonUpdate(buf, peer, msg, content, direction)
}

// isEORJSONUpdate reports whether msg is an UPDATE this call renders as ze-bgp
// JSON with a parsed form off its wire sections, the `parsed` and the `full`
// formats, where an End-of-RIB marker is written as its `eor` member. A message
// whose WireUpdate is nil is not one: nothing can be read back from it.
func isEORJSONUpdate(msg bgptypes.RawMessage, content bgptypes.ContentConfig) bool {
	if msg.Type != msgtype.TypeUPDATE || msg.WireUpdate == nil {
		return false
	}
	if content.Encoding != plugin.EncodingJSON {
		return false
	}
	return content.Format == plugin.FormatParsed || content.Format == plugin.FormatFull
}

// appendEORUpdateJSON appends an End-of-RIB marker to buf, in the ze-bgp JSON
// an UPDATE event carries.
//
// The `eor` member sits where `attr` and `nlri` sit for every other UPDATE, and
// it spells the family the way the dedicated EOR event spells it (AppendEOR),
// so one concept has one name in both:
//
//	{"type":"bgp","bgp":{"message":{"type":"sent","id":6,"direction":"sent"},
//	 "peer":{...},"update":{"eor":{"family":"ipv4/unicast"}}}}
//
// A `full` subscriber's marker also carries its bytes, as every UPDATE does
// (appendFullFromResult): `,"raw":{"update":"<hex>"}` beside `update`.
func appendEORUpdateJSON(buf []byte, peer *plugin.PeerInfo, msg bgptypes.RawMessage, fam family.Family, direction rpc.MessageDirection, messageType string, full bool) []byte {
	buf = append(buf, `{"type":"bgp","bgp":{"message":{"type":"`...)
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
	buf = append(buf, `,"update":{"eor":{"family":"`...)
	buf = fam.AppendTo(buf)
	buf = append(buf, `"}}`...)
	if full {
		buf = append(buf, `,"raw":{"update":"`...)
		buf = hex.AppendEncode(buf, msg.RawBytes)
		buf = append(buf, `"}`...)
	}
	buf = append(buf, `}}`...)
	return append(buf, '\n')
}

// appendEmptyUpdate appends an UPDATE whose attributes did not parse, as an
// UPDATE that states no route.
//
// The message's direction is still written, and so are its bytes for a
// subscriber of a format that carries them (`full`, `raw`, `hex`): a reader of
// the packet is owed the packet whether or not ze could read a route from it.
// Until 2026-10-22 both were dropped, so the ExaBGP bridge wrote no `packets`
// line and a `consolidate` line with no header or body for such an UPDATE
// (TestOldResponses42), and a sent one read as received.
func appendEmptyUpdate(buf []byte, peer *plugin.PeerInfo, msg bgptypes.RawMessage, content bgptypes.ContentConfig, direction rpc.MessageDirection, messageType string) []byte {
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
		buf = append(buf, `,"nlri":{}`...)
		if content.Format == plugin.FormatFull || content.Format == plugin.FormatRaw || content.Format == plugin.FormatHex {
			buf = append(buf, `,"raw":{"update":"`...)
			buf = hex.AppendEncode(buf, msg.RawBytes)
			buf = append(buf, '"', '}')
		}
		buf = append(buf, `}}`...)
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
		switch msg.Type {
		case msgtype.TypeOPEN:
			decoded := DecodeOpen(msg.RawBytes)
			return AppendOpen(buf, peer, decoded, direction, msg.MessageID)
		case msgtype.TypeNOTIFICATION:
			decoded := DecodeNotification(msg.RawBytes)
			return AppendNotification(buf, peer, decoded, direction, msg.MessageID)
		case msgtype.TypeKEEPALIVE:
			return AppendKeepalive(buf, peer, direction, msg.MessageID)
		case msgtype.TypeUPDATE, msgtype.TypeROUTEREFRESH:
			// These named types use the raw representation below.
		default:
			// Message types are an open wire set; preserve unknown bytes below.
		}
	}

	// Raw format or a type without a dedicated formatter.
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

	buf = appendUpdateNLRIJSONDirect(buf, msg, ctx)
	buf = append(buf, "}}}\n"...)
	return buf
}

// updateFamilyKeysMax is the most family keys one UPDATE can state in the
// fast path: ipv4/unicast for the body fields, MP_REACH_NLRI's family and
// MP_UNREACH_NLRI's family.
const updateFamilyKeysMax = 3

// appendUpdateNLRIJSONDirect writes the "nlri" member of the fast path from the
// body fields and the MP attributes, without allocating intermediate slices for
// the body.
//
// RFC 7606 Section 5.1: "Since older BGP speakers may not implement these
// restrictions, an implementation MUST still be prepared to receive these
// fields in any position or combination."
//
// So each family is ONE key whose array holds every operation of that family,
// in the order appendFamiliesJSON's writeFamily uses for the filtered
// producer: announces (body NLRI, then MP_REACH_NLRI), then withdrawals (body
// Withdrawn Routes, then MP_UNREACH_NLRI). A family stated twice loses its
// first copy in every map-based reader. The filtered producer's writer is not
// shared because it reads parsed []nlri.NLRI, which this path never builds for
// the body.
func appendUpdateNLRIJSONDirect(buf []byte, msg bgptypes.RawMessage, ctx *bgpctx.EncodingContext) []byte {
	buf = append(buf, `"nlri":{`...)

	nlriData, _ := msg.WireUpdate.NLRI()
	withdrawnData, _ := msg.WireUpdate.Withdrawn()
	mpReach, _ := msg.WireUpdate.MPReach()
	mpUnreach, _ := msg.WireUpdate.MPUnreach()
	ipv4AddPath := addPathFor(ctx, family.IPv4Unicast)

	var keys [updateFamilyKeysMax]family.Family
	keyCount := 0
	if len(nlriData) > 0 || len(withdrawnData) > 0 {
		keyCount = appendFamilyKey(&keys, keyCount, family.IPv4Unicast)
	}

	var reachNLRIs []nlri.NLRI
	var reachErr error
	if mpReach != nil {
		reachFamily := mpReach.Family()
		reachNLRIs, reachErr = mpReach.NLRIs(addPathFor(ctx, reachFamily))
		keyCount = appendFamilyKey(&keys, keyCount, reachFamily)
	}

	// An MP_UNREACH_NLRI with no route and no error states no operation; the
	// End-of-RIB form of it never reaches this function (appendEORUpdateJSON).
	var unreachNLRIs []nlri.NLRI
	var unreachErr error
	unreachHasOperation := false
	if mpUnreach != nil {
		unreachFamily := mpUnreach.Family()
		unreachNLRIs, unreachErr = mpUnreach.NLRIs(addPathFor(ctx, unreachFamily))
		unreachHasOperation = unreachErr != nil || len(unreachNLRIs) > 0
		if unreachHasOperation {
			keyCount = appendFamilyKey(&keys, keyCount, unreachFamily)
		}
	}

	for k, fam := range keys[:keyCount] {
		if k > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '"')
		buf = fam.AppendTo(buf)
		buf = append(buf, `":[`...)
		opFirst := true

		if fam == family.IPv4Unicast && len(nlriData) > 0 {
			buf = appendBodyIPv4AddJSON(buf, msg, nlriData, ipv4AddPath)
			opFirst = false
		}
		if mpReach != nil && mpReach.Family() == fam {
			if !opFirst {
				buf = append(buf, ',')
			}
			buf = appendMPReachOperationJSON(buf, mpReach, reachNLRIs, reachErr)
			opFirst = false
		}
		if fam == family.IPv4Unicast && len(withdrawnData) > 0 {
			if !opFirst {
				buf = append(buf, ',')
			}
			buf = append(buf, `{"action":"del",`...)
			buf = appendIPv4FieldJSON(buf, withdrawnData, ipv4AddPath)
			buf = append(buf, '}')
			opFirst = false
		}
		if unreachHasOperation && mpUnreach.Family() == fam {
			if !opFirst {
				buf = append(buf, ',')
			}
			buf = appendMPUnreachOperationJSON(buf, mpUnreach, unreachNLRIs, unreachErr)
		}
		buf = append(buf, ']')
	}

	return append(buf, '}')
}

// addPathFor answers whether ADD-PATH is negotiated for fam in the receive
// direction ctx describes. A nil ctx negotiated nothing.
func addPathFor(ctx *bgpctx.EncodingContext, fam family.Family) bool {
	if ctx == nil {
		return false
	}
	return ctx.AddPathFor(fam)
}

// appendFamilyKey adds fam to keys[:count] unless it is already there and
// returns the new count. count never passes updateFamilyKeysMax because the
// fast path has three sources of a family.
func appendFamilyKey(keys *[updateFamilyKeysMax]family.Family, count int, fam family.Family) int {
	if slices.Contains(keys[:count], fam) {
		return count
	}
	keys[count] = fam
	return count + 1
}

// appendBodyIPv4AddJSON writes the add operation of the body NLRI field, with
// the NEXT_HOP attribute as its next hop.
func appendBodyIPv4AddJSON(buf []byte, msg bgptypes.RawMessage, data []byte, addPath bool) []byte {
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
	buf = append(buf, `"action":"add",`...)
	buf = appendIPv4FieldJSON(buf, data, addPath)
	return append(buf, '}')
}

// appendIPv4FieldJSON writes the "nlri" member of a body NLRI or Withdrawn
// Routes field, or, when the field does not split, the error member in its
// place: the routes before a malformed prefix are no answer, because the list
// would read as the whole field.
func appendIPv4FieldJSON(buf, data []byte, addPath bool) []byte {
	mark := len(buf)
	buf = append(buf, `"nlri":[`...)
	buf, err := appendIPv4PrefixesFromWire(buf, data, addPath)
	if err != nil {
		return appendNLRISplitErrorJSON(buf[:mark], err, data)
	}
	return append(buf, ']')
}

// appendMPReachOperationJSON writes the add operation of an MP_REACH_NLRI.
func appendMPReachOperationJSON(buf []byte, mpReach wireu.MPReachWire, nlris []nlri.NLRI, splitErr error) []byte {
	buf = append(buf, `{"next-hop":"`...)
	buf = mpReach.NextHop().AppendTo(buf)
	buf = append(buf, `","action":"add",`...)
	if splitErr != nil {
		// An NLRI field that does not split names no route. Rendering
		// "nlri":[] would read as an announce of nothing, so the operation
		// carries the error and the bytes instead.
		buf = appendNLRISplitErrorJSON(buf, splitErr, mpReach.NLRIBytes())
		return append(buf, '}')
	}
	fam := mpReach.Family()
	buf = append(buf, `"nlri":[`...)
	for j, n := range nlris {
		if j > 0 {
			buf = append(buf, ',')
		}
		buf = appendNLRIJSONValue(buf, n, fam, false)
	}
	return append(buf, `]}`...)
}

// appendMPUnreachOperationJSON writes the del operation of an MP_UNREACH_NLRI.
func appendMPUnreachOperationJSON(buf []byte, mpUnreach wireu.MPUnreachWire, nlris []nlri.NLRI, splitErr error) []byte {
	buf = append(buf, `{"action":"del",`...)
	if splitErr != nil {
		// A withdrawn field that does not split still names a family the
		// peer withdrew from. Dropping the operation would hide the
		// withdrawal, so it carries the error and the bytes.
		buf = appendNLRISplitErrorJSON(buf, splitErr, mpUnreach.WithdrawnBytes())
		return append(buf, '}')
	}
	fam := mpUnreach.Family()
	buf = append(buf, `"nlri":[`...)
	for j, n := range nlris {
		if j > 0 {
			buf = append(buf, ',')
		}
		buf = appendNLRIJSONValue(buf, n, fam, true)
	}
	return append(buf, `]}`...)
}

// appendNLRISplitErrorJSON writes the members of a family operation whose NLRI
// field did not split into routes: the reason and the field's bytes, in the
// event format's error convention ({"error":...,"parsed":false,"raw":...}),
// in place of the "nlri" list a consumer would read as holding no route.
func appendNLRISplitErrorJSON(buf []byte, err error, raw []byte) []byte {
	buf = append(buf, `"error":"`...)
	buf = appendJSONString(buf, err.Error())
	buf = append(buf, `","parsed":false,"raw":"`...)
	buf = textbuf.HexUpper(buf, raw)
	return append(buf, '"')
}

// appendIPv4PrefixesFromWire iterates raw IPv4 NLRI wire bytes and appends
// each prefix as a JSON string (or ADD-PATH object) directly, without
// allocating parsed NLRI structs or slices. A field that does not split
// returns an error naming the offset of the prefix that broke, in the words
// wireu.ParseNLRIs uses for the same bytes, so both producers agree; buf then
// holds a partial list the caller discards.
func appendIPv4PrefixesFromWire(buf, data []byte, addPath bool) ([]byte, error) {
	offset := 0
	first := true
	for offset < len(data) {
		start := offset
		var pathID uint32
		if addPath {
			if offset+4 > len(data) {
				return buf, ipv4FieldError(start, nlri.ErrShortRead)
			}
			pathID = uint32(data[offset])<<24 | uint32(data[offset+1])<<16 |
				uint32(data[offset+2])<<8 | uint32(data[offset+3])
			offset += 4
		}
		if offset >= len(data) {
			return buf, ipv4FieldError(start, nlri.ErrShortRead)
		}
		// RFC 4271 Section 4.3: "The Length field indicates the length in
		// bits of the IP address prefix." An IPv4 address has 32 bits, so a
		// longer length names no prefix, and RFC 4271 Section 6.3: "The NLRI
		// field in the UPDATE message is checked for syntactic validity."
		prefixBits := int(data[offset])
		if prefixBits > 32 {
			return buf, ipv4FieldError(start, nlri.ErrInvalidPrefix)
		}
		prefixBytes := (prefixBits + 7) / 8
		offset++
		if offset+prefixBytes > len(data) {
			return buf, ipv4FieldError(start, nlri.ErrShortRead)
		}
		var addr [4]byte
		copy(addr[:], data[offset:offset+prefixBytes])
		offset += prefixBytes

		pfx := netip.PrefixFrom(netip.AddrFrom4(addr), prefixBits)
		if !first {
			buf = append(buf, ',')
		}
		first = false
		// RFC 7911 Section 3: "In order to carry the Path Identifier in an
		// UPDATE message, the NLRI encoding MUST be extended by prepending the
		// Path Identifier field, which is of four octets."
		//
		// The negotiated layout decides the shape, never the value read out of
		// it. The field has no absent value, so a route whose identifier is zero
		// carried one and says so.
		if addPath {
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
	return buf, nil
}

// ipv4FieldError is the error of a body IPv4 field that broke at offset,
// worded as wireu.ParseNLRIs words it.
func ipv4FieldError(offset int, cause error) error {
	return fmt.Errorf("parsing NLRI at offset %d: %w", offset, cause)
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
	// allocating an intermediate struct or its per-family maps.
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

	// Inject route metadata if present (sideband, not in wire bytes). A sent
	// event carries the forward provenance and ReceivedUpdate.Meta; a received
	// event carries what the ingress steps decided (docs/architecture/meta/README.md,
	// Key Registry). A value of a type appendRouteMetaValue cannot write drops its
	// key: only a Ze defect puts one there, never external input.
	hasMeta := len(msg.Meta) > 0 || msg.SourcePeerStr != "" || msg.SourceOwner != 0 || msg.SourceLocal || len(msg.SentPathSources) != 0
	if hasMeta {
		buf = append(buf, `,"route-meta":{`...)
		first := true
		if msg.SourcePeerStr != "" {
			buf = append(buf, `"source-peer":"`...)
			buf = appendJSONString(buf, msg.SourcePeerStr)
			buf = append(buf, '"')
			first = false
		}
		if msg.SourceMessageID != 0 {
			if !first {
				buf = append(buf, ',')
			}
			buf = append(buf, `"source-message-id":`...)
			buf = strconv.AppendUint(buf, msg.SourceMessageID, 10)
			first = false
		}
		if msg.SourceID != 0 {
			if !first {
				buf = append(buf, ',')
			}
			buf = append(buf, `"source-id":`...)
			buf = strconv.AppendUint(buf, uint64(msg.SourceID), 10)
			first = false
		}
		if msg.SourceOwner != 0 {
			if !first {
				buf = append(buf, ',')
			}
			buf = append(buf, `"source-owner":"`...)
			buf = strconv.AppendUint(buf, msg.SourceOwner, 10)
			buf = append(buf, '"')
			first = false
		}
		if msg.SourceLocal {
			if !first {
				buf = append(buf, ',')
			}
			buf = append(buf, `"source-local":true`...)
			first = false
		}
		if len(msg.SentPathSources) != 0 {
			if !first {
				buf = append(buf, ',')
			}
			buf = append(buf, `"sent-path-sources":[`...)
			for index, path := range msg.SentPathSources {
				if index != 0 {
					buf = append(buf, ',')
				}
				buf = append(buf, `{"family":"`...)
				buf = path.Family.AppendTo(buf)
				buf = append(buf, `","ordinal":`...)
				buf = strconv.AppendUint(buf, uint64(path.Ordinal), 10)
				buf = append(buf, `,"path-id":`...)
				buf = strconv.AppendUint(buf, uint64(path.PathID), 10)
				buf = append(buf, '}')
			}
			buf = append(buf, ']')
			first = false
		}
		for k, v := range msg.Meta {
			if k == "source-peer" || k == "source-message-id" || k == "source-id" ||
				k == bgptypes.SourceOwnerMeta || k == bgptypes.SourceLocalMeta || k == bgptypes.SentPathSourcesMeta {
				continue // Transport provenance cannot be replaced by route policy.
			}
			mark := len(buf)
			if !first {
				buf = append(buf, ',')
			}
			buf = append(buf, '"')
			buf = appendJSONString(buf, k)
			buf = append(buf, '"', ':')
			var written bool
			buf, written = appendRouteMetaValue(buf, v)
			if !written {
				buf = buf[:mark]
				continue
			}
			first = false
		}
		buf = append(buf, '}')
	}

	// Close bgp and outer wrapper
	buf = append(buf, "}}\n"...)
	return buf
}

// appendRouteMetaValue appends one route-meta value as JSON and reports whether
// it wrote one. The registered value types (docs/architecture/meta/README.md) are
// written directly, because this runs for every received UPDATE an ingress step
// annotated: a string role and the uint8 stale level are the common cases, and
// json.Marshal allocates for each. Any other type goes through json.Marshal,
// which is the cold path only a key outside the registry reaches.
func appendRouteMetaValue(buf []byte, v any) ([]byte, bool) {
	switch value := v.(type) {
	case string:
		buf = append(buf, '"')
		buf = appendJSONString(buf, value)
		return append(buf, '"'), true
	case uint8:
		return strconv.AppendUint(buf, uint64(value), 10), true
	case bool:
		return strconv.AppendBool(buf, value), true
	}
	encoded, err := json.Marshal(v)
	if err != nil {
		return buf, false
	}
	return append(buf, encoded...), true
}

// appendRawSectionsJSON writes the "raw" object contents directly from
// WireUpdate sections, without allocating an intermediate struct or its maps.
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
		if flags&0x10 == 0 {
			if offset+3 > len(packed) {
				break
			}
			attrLen = int(packed[offset+2])
		} else { // Extended length
			if offset+4 > len(packed) {
				break
			}
			attrLen = int(packed[offset+2])<<8 | int(packed[offset+3])
			headerLen = 4
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
