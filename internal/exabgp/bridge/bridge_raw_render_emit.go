// Design: docs/architecture/exabgp-bridge.md -- ExaBGP v4/v6 JSON and text buffer emitters for direct base64 unicast UPDATEs.
// Related: bridge_raw_render.go -- direct unicast plan builder and attribute/NLRI validation.
// Related: bridge_raw_b64.go -- ReadRawB64Update, RenderRawUpdate, and wire-backed Event fallback.
// Related: bridge_event_json.go -- marshalExabgpJSON spacing contract.
// Related: bridge_event_text.go -- AppendText and appendOneline sanitization.

//go:build ze_bgp

package bridge

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// appendRawPacketLine writes Response.packets for an UPDATE in text or JSON.
func appendRawPacketLine(buf []byte, raw RawUpdate, direction string, outFormat Format, host string, pid, ppid int, nowUnix int64, negotiatedJSON []byte) []byte {
	if outFormat == FormatV4Text {
		buf = append(buf, "neighbor "...)
		buf = appendOneline(buf, raw.Peer)
		buf = append(buf, ' ')
		buf = append(buf, direction...)
		buf = append(buf, " 2"...)
		buf = appendRawHeaderBodyText(buf, raw.Body)
		return append(buf, '\n')
	}

	buf = appendRawJSONEnvelopeOpen(buf, raw, direction, outFormat, host, false)
	buf = append(buf, `"message": { "body": "`...)
	buf = appendRawBodyHex(buf, raw.Body)
	buf = append(buf, `", "category": 2, "header": "`...)
	buf = appendRawHeaderHex(buf, len(raw.Body))
	buf = append(buf, `" }`...)
	buf = appendRawJSONEnvelopeClose(buf, raw, pid, ppid, nowUnix, negotiatedJSON)
	return append(buf, '\n')
}

// appendRawRenderedLine writes either a parsed or consolidated UPDATE line
// (or EOR marker) in ExaBGP v4/v6 JSON or ExaBGP text.
func appendRawRenderedLine(buf []byte, raw RawUpdate, plan *rawUnicastPlan, direction string, outFormat Format, host string, pid, ppid int, nowUnix int64, negotiatedJSON []byte, consolidated bool) []byte {
	if outFormat == FormatV4Text {
		return appendRawUpdateText(buf, raw, plan, direction, consolidated)
	}

	buf = appendRawJSONEnvelopeOpen(buf, raw, direction, outFormat, host, consolidated)
	if plan.isEOR {
		buf = append(buf, `"message": { "eor": { "afi": "`...)
		if plan.eorFam == family.IPv6Unicast {
			buf = append(buf, bridgeAFIv6...)
		} else {
			buf = append(buf, bridgeAFIv4...)
		}
		buf = append(buf, `", "safi": "unicast" } }`...)
	} else {
		buf = append(buf, `"message": { "update": `...)
		buf = appendRawUpdateJSON(buf, plan)
		buf = append(buf, rawJSONObjClose...)
	}
	buf = appendRawJSONEnvelopeClose(buf, raw, pid, ppid, nowUnix, negotiatedJSON)
	return append(buf, '\n')
}

// appendRawJSONEnvelopeOpen writes everything up to the start of the
// `"message"` member inside `"neighbor"`, preserving exact alphabetical order
// and ExaBGP spacing.
func appendRawJSONEnvelopeOpen(buf []byte, raw RawUpdate, direction string, outFormat Format, host string, consolidated bool) []byte {
	buf = append(buf, rawJSONObjOpen...)
	if consolidated {
		if len(raw.Body) > 0 {
			buf = append(buf, `"body": "`...)
			buf = appendRawBodyHex(buf, raw.Body)
			buf = append(buf, rawJSONQuoteSep...)
		}
	}
	buf = append(buf, `"exabgp": "`...)
	if outFormat == FormatV4JSON {
		buf = append(buf, JSONVersionV4...)
	} else {
		buf = append(buf, Version...)
	}
	buf = append(buf, rawJSONQuoteSep...)
	if consolidated {
		buf = append(buf, `"header": "`...)
		buf = appendRawHeaderHex(buf, len(raw.Body))
		buf = append(buf, rawJSONQuoteSep...)
	}
	buf = append(buf, `"host": `...)
	buf = appendJSONQuotedASCII(buf, host)
	buf = append(buf, `, "neighbor": { "address": { "local": `...)
	buf = appendJSONQuotedASCII(buf, raw.Local)
	buf = append(buf, `, "peer": `...)
	buf = appendJSONQuotedASCII(buf, raw.Peer)
	buf = append(buf, ` }, "asn": { "local": `...)
	buf = strconv.AppendUint(buf, uint64(raw.LocalAS), 10)
	buf = append(buf, `, "peer": `...)
	if raw.hasRemoteAS() {
		buf = strconv.AppendUint(buf, uint64(raw.RemoteAS), 10)
	} else {
		buf = append(buf, '0')
	}
	buf = append(buf, ` }, "direction": "`...)
	buf = append(buf, direction...)
	buf = append(buf, rawJSONQuoteSep...)
	return buf
}

// appendRawJSONEnvelopeClose closes `"neighbor"` (injecting cached
// `"negotiated"` bytes and optional `"router-id"` in alphabetical order) and
// writes the trailing top-level envelope fields `"pid"`, `"ppid"`, `"time"`,
// and `"type"`.
func appendRawJSONEnvelopeClose(buf []byte, raw RawUpdate, pid, ppid int, nowUnix int64, negotiatedJSON []byte) []byte {
	if len(negotiatedJSON) > 0 {
		buf = append(buf, `, "negotiated": `...)
		buf = append(buf, negotiatedJSON...)
	}
	if raw.RouterID != "" {
		buf = append(buf, `, "router-id": `...)
		buf = appendJSONQuotedASCII(buf, raw.RouterID)
	}
	buf = append(buf, ` }, "pid": `...)
	buf = strconv.AppendInt(buf, int64(pid), 10)
	buf = append(buf, `, "ppid": `...)
	buf = strconv.AppendInt(buf, int64(ppid), 10)
	buf = append(buf, `, "time": `...)
	buf = strconv.AppendInt(buf, nowUnix, 10)
	buf = append(buf, `, "type": "update" }`...)
	return buf
}

// appendRawUpdateJSON writes the `"update"` JSON object (`announce`,
// `attribute`, `withdraw` in alphabetical order, or `{}` when empty).
func appendRawUpdateJSON(buf []byte, plan *rawUnicastPlan) []byte {
	hasAnnounce := len(plan.ipv4Announce) > 0 || len(plan.ipv6Announce) > 0
	hasWithdraw := len(plan.ipv4Withdraw) > 0 || len(plan.ipv6Withdraw) > 0
	hasAttr := hasRawJSONAttrs(&plan.attrs, hasWithdraw)
	hasAny := hasAnnounce || hasAttr || hasWithdraw
	if !hasAny {
		return append(buf, "{}"...)
	}

	buf = append(buf, rawJSONObjOpen...)
	wrote := false

	if hasAnnounce {
		buf = append(buf, `"announce": { `...)
		wroteFam := false
		if len(plan.ipv4Announce) > 0 {
			buf = append(buf, `"`+rawFamilyIPv4Unicast+`": { "`...)
			buf = appendRawNextHopJSONKey(buf, plan.ipv4AnnounceNH)
			buf = append(buf, `": [ `...)
			buf = appendUnicastNLRIJSONList(buf, plan.ipv4Announce, false, plan.ipv4AddPath)
			buf = append(buf, ` ] }`...)
			wroteFam = true
		}
		if len(plan.ipv6Announce) > 0 {
			if wroteFam {
				buf = append(buf, rawJSONItemSep...)
			}
			buf = append(buf, `"`+rawFamilyIPv6Unicast+`": { "`...)
			buf = appendRawNextHopJSONKey(buf, plan.ipv6AnnounceNH)
			buf = append(buf, `": [ `...)
			buf = appendUnicastNLRIJSONList(buf, plan.ipv6Announce, true, plan.ipv6AddPath)
			buf = append(buf, ` ] }`...)
		}
		buf = append(buf, rawJSONObjClose...)
		wrote = true
	}

	if hasAttr {
		if wrote {
			buf = append(buf, rawJSONItemSep...)
		}
		buf = append(buf, `"attribute": { `...)
		buf = appendRawAttrsJSON(buf, &plan.attrs, hasWithdraw)
		buf = append(buf, rawJSONObjClose...)
		wrote = true
	}

	if hasWithdraw {
		if wrote {
			buf = append(buf, rawJSONItemSep...)
		}
		buf = append(buf, `"withdraw": { `...)
		wroteFam := false
		if len(plan.ipv4Withdraw) > 0 {
			buf = append(buf, `"`+rawFamilyIPv4Unicast+`": [ `...)
			buf = appendUnicastNLRIJSONList(buf, plan.ipv4Withdraw, false, plan.ipv4AddPath)
			buf = append(buf, rawJSONArrClose...)
			wroteFam = true
		}
		if len(plan.ipv6Withdraw) > 0 {
			if wroteFam {
				buf = append(buf, rawJSONItemSep...)
			}
			buf = append(buf, `"`+rawFamilyIPv6Unicast+`": [ `...)
			buf = appendUnicastNLRIJSONList(buf, plan.ipv6Withdraw, true, plan.ipv6AddPath)
			buf = append(buf, rawJSONArrClose...)
		}
		buf = append(buf, rawJSONObjClose...)
	}

	return append(buf, rawJSONObjClose...)
}

func appendRawNextHopJSONKey(buf []byte, nh netip.Addr) []byte {
	if nh.IsValid() {
		return nh.AppendTo(buf)
	}
	return append(buf, rawNoNextHop...)
}

func appendUnicastNLRIJSONList(buf, data []byte, isV6, addPath bool) []byte {
	offset := 0
	first := true
	for offset < len(data) {
		var pathID uint32
		if addPath {
			pathID = binary.BigEndian.Uint32(data[offset : offset+4])
			offset += 4
		}
		bits := int(data[offset])
		offset++
		prefixBytes := (bits + 7) / 8
		pfx := decodeUnicastPrefix(data[offset:offset+prefixBytes], bits, isV6)
		offset += prefixBytes

		if !first {
			buf = append(buf, rawJSONItemSep...)
		}
		first = false

		buf = append(buf, `{ "nlri": "`...)
		buf = pfx.AppendTo(buf)
		if addPath {
			buf = append(buf, `", "path-information": "`...)
			buf = appendPathIDDotted(buf, pathID)
			buf = append(buf, `" }`...)
		} else {
			buf = append(buf, `" }`...)
		}
	}
	return buf
}

// decodeUnicastPrefix turns validated prefix wire bytes into a canonical masked
// netip.Prefix matching nlri.ParseINET.
func decodeUnicastPrefix(prefixSlice []byte, bits int, isV6 bool) netip.Prefix {
	var addr netip.Addr
	if isV6 {
		var raw [16]byte
		copy(raw[:], prefixSlice)
		addr = netip.AddrFrom16(raw)
	} else {
		var raw [4]byte
		copy(raw[:], prefixSlice)
		addr = netip.AddrFrom4(raw)
	}
	pfx, err := addr.Prefix(bits)
	if err != nil {
		return netip.PrefixFrom(addr, bits)
	}
	return pfx
}

func appendPathIDDotted(buf []byte, pathID uint32) []byte {
	buf = strconv.AppendUint(buf, uint64(pathID>>24), 10)
	buf = append(buf, '.')
	buf = strconv.AppendUint(buf, uint64((pathID>>16)&0xFF), 10)
	buf = append(buf, '.')
	buf = strconv.AppendUint(buf, uint64((pathID>>8)&0xFF), 10)
	buf = append(buf, '.')
	return strconv.AppendUint(buf, uint64(pathID&0xFF), 10)
}

func hasRawJSONAttrs(a *rawParsedAttrs, hasWithdraw bool) bool {
	return a.hasOrigin ||
		a.hasASPath ||
		a.hasMED ||
		a.hasLocalPref ||
		a.hasAtomicAggregate ||
		a.hasAggregator ||
		a.hasCommunity ||
		a.hasOriginatorID ||
		a.hasClusterList ||
		a.hasLargeCommunity ||
		(a.hasNextHop && hasWithdraw) ||
		a.unknownCount > 0
}

// appendRawAttrsJSON writes every whitelisted attribute inside `"attribute"` in
// exact alphabetical key order with ExaBGP spacing.
func appendRawAttrsJSON(buf []byte, a *rawParsedAttrs, hasWithdraw bool) []byte {
	wrote := false
	sep := func(b []byte) []byte {
		if wrote {
			return append(b, rawJSONItemSep...)
		}
		wrote = true
		return b
	}

	if a.hasAggregator {
		buf = sep(buf)
		buf = append(buf, `"aggregator": "`...)
		buf = strconv.AppendUint(buf, uint64(a.aggregatorASN), 10)
		buf = append(buf, ':')
		buf = a.aggregatorAddr.AppendTo(buf)
		buf = append(buf, '"')
	}

	if a.hasASPath {
		buf = sep(buf)
		buf = append(buf, `"as-path": { `...)
		buf = appendRawASPathJSON(buf, a.asPathRaw, a.asn4)
		buf = append(buf, rawJSONObjClose...)
	}

	if a.hasAtomicAggregate {
		buf = sep(buf)
		buf = append(buf, `"atomic-aggregate": true`...)
	}

	for i := range a.unknownCount {
		u := a.unknowns[i]
		buf = sep(buf)
		buf = append(buf, `"attribute-0x`...)
		buf = appendHexByte(buf, uint8(u.code))
		buf = append(buf, `-0x`...)
		buf = appendHexByte(buf, uint8(u.flags))
		buf = append(buf, `": "`+rawHexPrefix...)
		buf = appendLowerHexBytes(buf, u.val)
		buf = append(buf, '"')
	}

	if a.hasClusterList {
		buf = sep(buf)
		buf = append(buf, `"cluster-list": `...)
		if len(a.clusterListRaw) == 0 {
			buf = append(buf, "[]"...)
		} else {
			buf = append(buf, rawJSONArrOpen...)
			for off := 0; off < len(a.clusterListRaw); off += 4 {
				if off > 0 {
					buf = append(buf, rawJSONItemSep...)
				}
				buf = append(buf, '"')
				buf = netip.AddrFrom4([4]byte(a.clusterListRaw[off : off+4])).AppendTo(buf)
				buf = append(buf, '"')
			}
			buf = append(buf, rawJSONArrClose...)
		}
	}

	if a.hasCommunity {
		buf = sep(buf)
		buf = append(buf, `"community": `...)
		if len(a.communityRaw) == 0 {
			buf = append(buf, "[]"...)
		} else {
			buf = append(buf, rawJSONArrOpen...)
			for off := 0; off < len(a.communityRaw); off += 4 {
				if off > 0 {
					buf = append(buf, rawJSONItemSep...)
				}
				hi := binary.BigEndian.Uint16(a.communityRaw[off : off+2])
				lo := binary.BigEndian.Uint16(a.communityRaw[off+2 : off+4])
				buf = append(buf, rawJSONArrOpen...)
				buf = strconv.AppendUint(buf, uint64(hi), 10)
				buf = append(buf, rawJSONItemSep...)
				buf = strconv.AppendUint(buf, uint64(lo), 10)
				buf = append(buf, rawJSONArrClose...)
			}
			buf = append(buf, rawJSONArrClose...)
		}
	}

	if a.hasLargeCommunity {
		buf = sep(buf)
		buf = append(buf, `"large-community": `...)
		if len(a.largeCommunityRaw) == 0 {
			buf = append(buf, "[]"...)
		} else {
			buf = append(buf, rawJSONArrOpen...)
			for off := 0; off < len(a.largeCommunityRaw); off += 12 {
				if off > 0 {
					buf = append(buf, rawJSONItemSep...)
				}
				ga := binary.BigEndian.Uint32(a.largeCommunityRaw[off : off+4])
				ld1 := binary.BigEndian.Uint32(a.largeCommunityRaw[off+4 : off+8])
				ld2 := binary.BigEndian.Uint32(a.largeCommunityRaw[off+8 : off+12])
				buf = append(buf, rawJSONArrOpen...)
				buf = strconv.AppendUint(buf, uint64(ga), 10)
				buf = append(buf, rawJSONItemSep...)
				buf = strconv.AppendUint(buf, uint64(ld1), 10)
				buf = append(buf, rawJSONItemSep...)
				buf = strconv.AppendUint(buf, uint64(ld2), 10)
				buf = append(buf, rawJSONArrClose...)
			}
			buf = append(buf, rawJSONArrClose...)
		}
	}

	if a.hasLocalPref {
		buf = sep(buf)
		buf = append(buf, `"local-preference": `...)
		buf = strconv.AppendUint(buf, uint64(a.localPref), 10)
	}

	if a.hasMED {
		buf = sep(buf)
		buf = append(buf, `"med": `...)
		buf = strconv.AppendUint(buf, uint64(a.med), 10)
	}

	if a.hasNextHop && hasWithdraw {
		buf = sep(buf)
		buf = append(buf, `"next-hop": "`...)
		buf = a.nextHop.AppendTo(buf)
		buf = append(buf, '"')
	}

	if a.hasOrigin {
		buf = sep(buf)
		buf = append(buf, `"origin": "`...)
		buf = append(buf, rawOriginName(a.origin)...)
		buf = append(buf, '"')
	}

	if a.hasOriginatorID {
		buf = sep(buf)
		buf = append(buf, `"originator-id": "`...)
		buf = a.originatorID.AppendTo(buf)
		buf = append(buf, '"')
	}

	return buf
}

func appendRawASPathJSON(buf, val []byte, asn4 bool) []byte {
	asnSize := 2
	if asn4 {
		asnSize = 4
	}
	offset := 0
	segIdx := 0
	for offset < len(val) {
		if segIdx > 0 {
			buf = append(buf, rawJSONItemSep...)
		}
		segType := attribute.ASPathSegmentType(val[offset])
		count := int(val[offset+1])
		offset += 2

		buf = append(buf, '"')
		buf = strconv.AppendInt(buf, int64(segIdx), 10)
		buf = append(buf, `": { "element": "`...)
		buf = append(buf, rawASPathElementName(segType)...)
		buf = append(buf, `", "value": [ `...)
		for i := range count {
			if i > 0 {
				buf = append(buf, rawJSONItemSep...)
			}
			var asn uint32
			if asn4 {
				asn = binary.BigEndian.Uint32(val[offset : offset+4])
			} else {
				asn = uint32(binary.BigEndian.Uint16(val[offset : offset+2]))
			}
			offset += asnSize
			buf = strconv.AppendUint(buf, uint64(asn), 10)
		}
		buf = append(buf, ` ] }`...)
		segIdx++
	}
	return buf
}

func rawOriginName(code uint8) string {
	switch attribute.Origin(code) {
	case attribute.OriginIGP:
		return "igp"
	case attribute.OriginEGP:
		return "egp"
	case attribute.OriginIncomplete:
		return "incomplete"
	default:
		panic("BUG: unvalidated BGP ORIGIN code in rawUnicastPlan")
	}
}

func rawASPathElementName(segType attribute.ASPathSegmentType) string {
	switch segType {
	case attribute.ASSet:
		return "as-set"
	case attribute.ASSequence:
		return "as-sequence"
	case attribute.ASConfedSequence:
		return "as-confed-sequence"
	case attribute.ASConfedSet:
		return "as-confed-set"
	default:
		panic("BUG: unvalidated AS_PATH segment type in rawUnicastPlan")
	}
}

// appendRawUpdateText renders an EOR marker or unicast UPDATE in ExaBGP text
// format (start, per-route announced/withdrawn lines, optional consolidated
// header/body line, and end), preserving appendOneline peer sanitization.
func appendRawUpdateText(buf []byte, raw RawUpdate, plan *rawUnicastPlan, direction string, consolidated bool) []byte {
	var prefixScratch [128]byte
	prefix := append(prefixScratch[:0], "neighbor "...)
	prefix = appendOneline(prefix, raw.Peer)
	prefix = append(prefix, ' ')
	prefix = append(prefix, direction...)
	prefix = append(prefix, " update"...)

	buf = append(buf, prefix...)
	buf = append(buf, " start\n"...)

	if plan.isEOR {
		buf = append(buf, prefix...)
		if plan.eorFam == family.IPv6Unicast {
			buf = append(buf, " route eor 2/1 ("+rawFamilyIPv6Unicast+")\n"...)
		} else {
			buf = append(buf, " route eor 1/1 ("+rawFamilyIPv4Unicast+")\n"...)
		}
	} else {
		var attrScratch [512]byte
		var attrBytes []byte
		if len(plan.ipv4Announce) > 0 || len(plan.ipv6Announce) > 0 {
			attrBytes = appendRawTextAttributes(attrScratch[:0], &plan.attrs)
		}

		// Legacy appendUpdateText walks families alphabetically ("ipv4/unicast"
		// before "ipv6/unicast") and within each family emits announces before withdraws.
		if len(plan.ipv4Announce) > 0 {
			buf = appendUnicastTextRoutes(buf, prefix, plan.ipv4Announce, false, plan.ipv4AddPath, plan.ipv4AnnounceNH, attrBytes, true)
		}
		if len(plan.ipv4Withdraw) > 0 {
			buf = appendUnicastTextRoutes(buf, prefix, plan.ipv4Withdraw, false, plan.ipv4AddPath, netip.Addr{}, nil, false)
		}
		if len(plan.ipv6Announce) > 0 {
			buf = appendUnicastTextRoutes(buf, prefix, plan.ipv6Announce, true, plan.ipv6AddPath, plan.ipv6AnnounceNH, attrBytes, true)
		}
		if len(plan.ipv6Withdraw) > 0 {
			buf = appendUnicastTextRoutes(buf, prefix, plan.ipv6Withdraw, true, plan.ipv6AddPath, netip.Addr{}, nil, false)
		}
	}

	if consolidated {
		buf = appendRawHeaderBodyText(buf, raw.Body)
		buf = append(buf, '\n')
	}

	buf = append(buf, prefix...)
	return append(buf, " end\n"...)
}

func appendUnicastTextRoutes(buf, prefix, data []byte, isV6, addPath bool, nh netip.Addr, attrBytes []byte, announced bool) []byte {
	offset := 0
	for offset < len(data) {
		var pathID uint32
		if addPath {
			pathID = binary.BigEndian.Uint32(data[offset : offset+4])
			offset += 4
		}
		bits := int(data[offset])
		offset++
		prefixBytes := (bits + 7) / 8
		pfx := decodeUnicastPrefix(data[offset:offset+prefixBytes], bits, isV6)
		offset += prefixBytes

		buf = append(buf, prefix...)
		if announced {
			buf = append(buf, " announced "...)
		} else {
			buf = append(buf, " withdrawn "...)
		}
		buf = pfx.AppendTo(buf)
		if addPath {
			buf = append(buf, " path-information "...)
			buf = appendPathIDDotted(buf, pathID)
		}
		if announced {
			if nh.IsValid() {
				buf = append(buf, " next-hop "...)
				buf = nh.AppendTo(buf)
			}
			buf = append(buf, attrBytes...)
		}
		buf = append(buf, '\n')
	}
	return buf
}

// appendRawTextAttributes formats whitelisted path attributes in ascending IANA
// attribute-code order matching appendTextAttributes and bridge_event_text_community.go.
func appendRawTextAttributes(buf []byte, a *rawParsedAttrs) []byte {
	uIdx := 0
	emitUnknownsBefore := func(b []byte, limit attribute.AttributeCode) []byte {
		for uIdx < a.unknownCount && a.unknowns[uIdx].code < limit {
			b = appendRawTextUnknownAttr(b, a.unknowns[uIdx])
			uIdx++
		}
		return b
	}

	buf = emitUnknownsBefore(buf, attribute.AttrOrigin)
	if a.hasOrigin {
		buf = append(buf, " origin "...)
		buf = append(buf, rawOriginName(a.origin)...)
	}

	buf = emitUnknownsBefore(buf, attribute.AttrASPath)
	if a.hasASPath {
		buf = append(buf, " as-path [ "...)
		buf = appendRawASPathTextFlat(buf, a.asPathRaw, a.asn4)
		buf = append(buf, ']')
	}

	buf = emitUnknownsBefore(buf, attribute.AttrMED)
	if a.hasMED {
		buf = append(buf, " med "...)
		buf = strconv.AppendUint(buf, uint64(a.med), 10)
	}

	buf = emitUnknownsBefore(buf, attribute.AttrLocalPref)
	if a.hasLocalPref {
		buf = append(buf, " local-preference "...)
		buf = strconv.AppendUint(buf, uint64(a.localPref), 10)
	}

	buf = emitUnknownsBefore(buf, attribute.AttrAtomicAggregate)
	if a.hasAtomicAggregate {
		buf = append(buf, " atomic-aggregate"...)
	}

	buf = emitUnknownsBefore(buf, attribute.AttrAggregator)
	if a.hasAggregator && a.aggregatorCode == attribute.AttrAggregator {
		buf = appendRawTextAggregator(buf, a.aggregatorASN, a.aggregatorAddr)
	}

	buf = emitUnknownsBefore(buf, attribute.AttrCommunity)
	if a.hasCommunity && len(a.communityRaw) > 0 {
		buf = appendRawTextCommunities(buf, a.communityRaw)
	}

	buf = emitUnknownsBefore(buf, attribute.AttrOriginatorID)
	if a.hasOriginatorID {
		buf = append(buf, " originator-id "...)
		buf = a.originatorID.AppendTo(buf)
	}

	buf = emitUnknownsBefore(buf, attribute.AttrClusterList)
	if a.hasClusterList && len(a.clusterListRaw) > 0 {
		buf = append(buf, " cluster-list [ "...)
		for off := 0; off < len(a.clusterListRaw); off += 4 {
			buf = netip.AddrFrom4([4]byte(a.clusterListRaw[off : off+4])).AppendTo(buf)
			buf = append(buf, ' ')
		}
		buf = append(buf, ']')
	}

	buf = emitUnknownsBefore(buf, attribute.AttrAS4Aggregator)
	if a.hasAggregator && a.aggregatorCode == attribute.AttrAS4Aggregator {
		buf = appendRawTextAggregator(buf, a.aggregatorASN, a.aggregatorAddr)
	}

	buf = emitUnknownsBefore(buf, attribute.AttrLargeCommunity)
	if a.hasLargeCommunity && len(a.largeCommunityRaw) > 0 {
		buf = appendRawTextLargeCommunities(buf, a.largeCommunityRaw)
	}

	for uIdx < a.unknownCount {
		buf = appendRawTextUnknownAttr(buf, a.unknowns[uIdx])
		uIdx++
	}
	return buf
}

func appendRawTextUnknownAttr(buf []byte, u rawUnknownAttr) []byte {
	buf = append(buf, " attribute [ "+rawHexPrefix...)
	buf = appendHexByte(buf, uint8(u.code))
	buf = append(buf, ' ')
	buf = appendLowerHexBytes(buf, u.val)
	return append(buf, rawJSONArrClose...)
}

func appendRawTextAggregator(buf []byte, asn uint32, addr netip.Addr) []byte {
	buf = append(buf, " aggregator ( "...)
	buf = strconv.AppendUint(buf, uint64(asn), 10)
	buf = append(buf, ':')
	buf = addr.AppendTo(buf)
	return append(buf, " )"...)
}

func appendRawASPathTextFlat(buf, val []byte, asn4 bool) []byte {
	asnSize := 2
	if asn4 {
		asnSize = 4
	}
	for offset := 0; offset < len(val); {
		count := int(val[offset+1])
		offset += 2
		for range count {
			var asn uint32
			if asn4 {
				asn = binary.BigEndian.Uint32(val[offset : offset+4])
			} else {
				asn = uint32(binary.BigEndian.Uint16(val[offset : offset+2]))
			}
			offset += asnSize
			buf = strconv.AppendUint(buf, uint64(asn), 10)
			buf = append(buf, ' ')
		}
	}
	return buf
}

func appendRawTextCommunities(buf, raw []byte) []byte {
	count := len(raw) / 4
	var vals [rawCommunitiesSortMax]uint32
	for i := range count {
		vals[i] = binary.BigEndian.Uint32(raw[i*4 : i*4+4])
	}
	sorted := vals[:count]
	slices.Sort(sorted)

	buf = append(buf, " community "...)
	if count > 1 {
		buf = append(buf, rawJSONArrOpen...)
	}
	for i, v := range sorted {
		if i > 0 {
			buf = append(buf, ' ')
		}
		if name, known := exabgpWellKnownCommunities[v]; known {
			buf = append(buf, name...)
		} else {
			buf = strconv.AppendUint(buf, uint64(v>>16), 10)
			buf = append(buf, ':')
			buf = strconv.AppendUint(buf, uint64(v&0xFFFF), 10)
		}
	}
	if count > 1 {
		buf = append(buf, rawJSONArrClose...)
	}
	return buf
}

func appendRawTextLargeCommunities(buf, raw []byte) []byte {
	count := len(raw) / 12
	var items [rawCommunitiesSortMax][12]byte
	for i := range count {
		copy(items[i][:], raw[i*12:i*12+12])
	}
	sorted := items[:count]
	slices.SortFunc(sorted, func(a, b [12]byte) int {
		return bytes.Compare(a[:], b[:])
	})

	buf = append(buf, " large-community "...)
	if count > 1 {
		buf = append(buf, rawJSONArrOpen...)
	}
	for i := range count {
		if i > 0 {
			buf = append(buf, ' ')
		}
		it := sorted[i]
		buf = strconv.AppendUint(buf, uint64(binary.BigEndian.Uint32(it[0:4])), 10)
		buf = append(buf, ':')
		buf = strconv.AppendUint(buf, uint64(binary.BigEndian.Uint32(it[4:8])), 10)
		buf = append(buf, ':')
		buf = strconv.AppendUint(buf, uint64(binary.BigEndian.Uint32(it[8:12])), 10)
	}
	if count > 1 {
		buf = append(buf, rawJSONArrClose...)
	}
	return buf
}

func appendRawHeaderHex(buf []byte, bodyLen int) []byte {
	buf = append(buf, rawHeaderMarkerHex...)
	length := uint16(19 + bodyLen) //nolint:gosec // G115: bodyLen <= rawUpdateBodyMax (65516) checked by caller
	buf = appendHexByte(buf, uint8(length>>8))
	buf = appendHexByte(buf, uint8(length&0xFF))
	return appendHexByte(buf, rawUpdateTypeCode)
}

func appendRawBodyHex(buf, body []byte) []byte {
	buf = append(buf, rawHexPrefix...)
	for _, b := range body {
		buf = appendHexByte(buf, b)
	}
	return buf
}

func appendRawHeaderBodyText(buf, body []byte) []byte {
	buf = append(buf, " header "...)
	buf = appendRawHeaderHex(buf, len(body))
	if len(body) == 0 {
		return buf
	}
	buf = append(buf, " body "...)
	return appendRawBodyHex(buf, body)
}

func appendLowerHexBytes(buf, data []byte) []byte {
	for _, b := range data {
		buf = appendLowerHexByte(buf, b)
	}
	return buf
}

func isASCIIString(s string) bool {
	for i := range len(s) {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func isSafeJSONASCIIByte(c byte) bool {
	return c >= 0x20 && c != '"' && c != '\\' && c != '<' && c != '>' && c != '&'
}

// appendJSONQuotedASCII writes s as a JSON string literal byte-identical to
// Go's encoding/json.Marshal (including HTML-escaping of '<', '>', '&' and
// \u00XX escapes for control characters). Caller MUST verify isASCIIString(s).
func appendJSONQuotedASCII(buf []byte, s string) []byte {
	buf = append(buf, '"')
	start := 0
	for i := range len(s) {
		c := s[i]
		if isSafeJSONASCIIByte(c) {
			continue
		}
		if i > start {
			buf = append(buf, s[start:i]...)
		}
		switch c {
		case '"':
			buf = append(buf, `\"`...)
		case '\\':
			buf = append(buf, `\\`...)
		case '\n':
			buf = append(buf, `\n`...)
		case '\r':
			buf = append(buf, `\r`...)
		case '\t':
			buf = append(buf, `\t`...)
		case '<':
			buf = append(buf, `\u003c`...)
		case '>':
			buf = append(buf, `\u003e`...)
		case '&':
			buf = append(buf, `\u0026`...)
		default:
			buf = append(buf, `\u00`...)
			buf = appendLowerHexByte(buf, c)
		}
		start = i + 1
	}
	if start < len(s) {
		buf = append(buf, s[start:]...)
	}
	return append(buf, '"')
}
