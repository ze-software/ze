// Design: docs/architecture/exabgp-bridge.md -- direct wire-to-buffer ExaBGP v4/v6 JSON and text renderer for hot unicast UPDATEs.
// Related: bridge_raw_render_emit.go -- JSON and text buffer emitters for validated rawUnicastPlan.
// Related: bridge_raw_b64.go -- ReadRawB64Update, RenderRawUpdate, and wire-backed Event fallback.
// Related: bridge_event.go -- exabgpJSON, normalizeOutgoingAttributes, cachedProcessIdentity.
// Related: bridge_event_json.go -- marshalExabgpJSON spacing contract.
// Related: bridge_event_text.go -- AppendText and appendOneline sanitization.
// Related: bridge_negotiated.go -- Sessions and pre-serialized negotiated JSON bytes.

//go:build ze_bgp

package bridge

import (
	"bytes"
	"encoding/binary"
	"log/slog"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

// Formatting tokens and stack-buffer bounds for the direct base64 unicast renderer.
const (
	rawFamilyIPv4Unicast  = "ipv4 unicast"
	rawFamilyIPv6Unicast  = "ipv6 unicast"
	rawHexPrefix          = "0x"
	rawJSONItemSep        = ", "
	rawJSONQuoteSep       = `", `
	rawJSONArrOpen        = "[ "
	rawJSONArrClose       = " ]"
	rawJSONObjOpen        = "{ "
	rawJSONObjClose       = " }"
	rawNoNextHop          = "no-nexthop"
	rawUnknownAttrsMax    = 8
	rawCommunitiesSortMax = 32
	rawASPathSegmentsMax  = 10
	rawHeaderMarkerHex    = "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"
	rawUpdateTypeCode     = 2
)

// rawUnknownAttr holds one unrecognized attribute span parsed from wire bytes.
type rawUnknownAttr struct {
	code  attribute.AttributeCode
	flags attribute.AttributeFlags
	val   []byte
}

// rawParsedAttrs holds the whitelisted path attributes of a hot unicast UPDATE
// without allocating Go maps or slices.
type rawParsedAttrs struct {
	hasOrigin          bool
	origin             uint8
	hasASPath          bool
	asPathRaw          []byte
	asn4               bool
	hasNextHop         bool
	nextHop            netip.Addr
	hasMED             bool
	med                uint32
	hasLocalPref       bool
	localPref          uint32
	hasAtomicAggregate bool
	hasAggregator      bool
	aggregatorCode     attribute.AttributeCode
	aggregatorASN      uint32
	aggregatorAddr     netip.Addr
	hasCommunity       bool
	communityRaw       []byte
	hasOriginatorID    bool
	originatorID       netip.Addr
	hasClusterList     bool
	clusterListRaw     []byte
	hasLargeCommunity  bool
	largeCommunityRaw  []byte
	unknownCount       int
	unknowns           [rawUnknownAttrsMax]rawUnknownAttr
}

// rawUnicastPlan holds the validated sections and attributes of one hot IPv4 or
// IPv6 unicast UPDATE (or unicast EOR marker) ready for direct serialization.
type rawUnicastPlan struct {
	isEOR          bool
	eorFam         family.Family
	ipv4Announce   []byte
	ipv4AnnounceNH netip.Addr
	ipv4Withdraw   []byte
	ipv6Announce   []byte
	ipv6AnnounceNH netip.Addr
	ipv6Withdraw   []byte
	ipv4AddPath    bool
	ipv6AddPath    bool
	attrs          rawParsedAttrs
}

// noteRawJSON applies Q-E4 reconnect invalidation for a base64 UPDATE and
// returns the peer's pre-serialized ExaBGP-spaced negotiated JSON bytes and
// effective ASN4 width without allocating a fallback map on cache misses.
// Safe for concurrent use.
func (s *Sessions) noteRawJSON(peer string, connID uint64, hasASN4, asn4 bool) (negotiatedJSON []byte, sessionASN4 bool) {
	s.mu.Lock()
	entry, ok := s.peers[peer]
	if ok {
		if reconnectedRawSession(entry, connID) {
			delete(s.peers, peer)
			ok = false
		}
	}
	s.mu.Unlock()

	sessionASN4 = true
	if hasASN4 {
		sessionASN4 = asn4
	} else if ok && entry.hasASN4 {
		sessionASN4 = entry.asn4
	}

	if !ok {
		return unexchangedNegotiatedJSONBytes(), sessionASN4
	}
	return entry.blockJSON, sessionASN4
}

// renderRawUpdateDirect renders one base64 UPDATE directly from wire bytes
// into a []byte buffer for packet-only forms, unicast EOR markers, and hot
// IPv4/IPv6 unicast UPDATEs carrying whitelisted attributes. It returns
// (nil, false) for any non-unicast family, unsplit NLRI, or attribute outside
// the proven direct set so the caller falls back to renderRawUpdateFallback.
func (s *Sessions) renderRawUpdateDirect(raw RawUpdate, outFormat Format, forms Forms, grantNegotiated bool, _ *slog.Logger) ([]byte, bool) {
	switch outFormat {
	case FormatV4Text, FormatV4JSON, FormatV6JSON:
	case FormatUnspecified:
		panic("BUG: bridge format unspecified")
	default:
		panic("BUG: invalid bridge format")
	}

	negotiatedJSON, asn4 := s.noteRawJSON(raw.Peer, raw.ConnID, raw.hasASN4(), raw.asn4())
	if forms == 0 {
		return nil, true
	}
	if len(raw.Body) > rawUpdateBodyMax {
		return nil, false
	}
	if grantNegotiated {
		if len(negotiatedJSON) == 0 {
			return nil, false
		}
	}
	if !grantNegotiated {
		negotiatedJSON = nil
	}

	direction := raw.Direction()

	host, pid, ppid := cachedProcessIdentity()
	if outFormat != FormatV4Text {
		if !isASCIIString(host) {
			return nil, false
		}
		if !isASCIIString(raw.Peer) {
			return nil, false
		}
		if !isASCIIString(raw.Local) {
			return nil, false
		}
		if !isASCIIString(raw.RouterID) {
			return nil, false
		}
	}

	// Packet-only form needs only the wire body and envelope metadata, never
	// attribute or NLRI decoding, so it renders directly for any UPDATE.
	if forms == FormPackets {
		buf := make([]byte, 0, estimateRawDirectCap(len(raw.Body), len(negotiatedJSON), true))
		buf = appendRawPacketLine(buf, raw, direction, outFormat, host, pid, ppid, time.Now().Unix(), negotiatedJSON)
		return buf, true
	}

	var plan rawUnicastPlan
	if !buildRawUnicastPlan(&plan, raw, asn4) {
		return nil, false
	}

	needsHex := forms&(FormPackets|FormConsolidated) != 0
	buf := make([]byte, 0, estimateRawDirectCap(len(raw.Body), len(negotiatedJSON), needsHex))
	nowUnix := time.Now().Unix()

	if forms&FormPackets != 0 {
		buf = appendRawPacketLine(buf, raw, direction, outFormat, host, pid, ppid, nowUnix, negotiatedJSON)
	}
	if forms&FormConsolidated != 0 {
		buf = appendRawRenderedLine(buf, raw, &plan, direction, outFormat, host, pid, ppid, nowUnix, negotiatedJSON, true)
	}
	if forms&FormParsed != 0 {
		buf = appendRawRenderedLine(buf, raw, &plan, direction, outFormat, host, pid, ppid, nowUnix, negotiatedJSON, false)
	}
	return buf, true
}

func estimateRawDirectCap(bodyLen, negLen int, withHex bool) int {
	base := 384 + negLen + bodyLen*2
	if withHex {
		base += 128 + bodyLen*2
	}
	return base
}

// buildRawUnicastPlan validates and extracts all sections and whitelisted
// attributes of one raw UPDATE into plan without allocating. Returns false to
// trigger the full wire-backed Event fallback on any unsupported family,
// attribute, or wire framing error.
func buildRawUnicastPlan(plan *rawUnicastPlan, raw RawUpdate, asn4 bool) bool {
	sections, err := wire.ParseUpdateSections(raw.Body)
	// refuse-by-default: guards renderRawUpdateDirect -- false hands malformed wire bytes to renderRawUpdateFallback.
	if err != nil {
		return false
	}
	withdrawnData := sections.Withdrawn(raw.Body)
	attrSection := sections.Attrs(raw.Body)
	nlriData := sections.NLRI(raw.Body)

	var spanIdx attribute.SpanIndex
	if len(attrSection) > 0 {
		// refuse-by-default: guards renderRawUpdateDirect -- false hands malformed attribute bytes to renderRawUpdateFallback.
		if err := spanIdx.Rebuild(attrSection); err != nil {
			return false
		}
	}

	var mpReach wireu.MPReachWire
	if sp, ok := spanIdx.Find(attribute.AttrMPReachNLRI); ok {
		parsedMP, mpErr := wireu.NewMPReachWire(attrSection[sp.Offset : sp.Offset+sp.Length])
		// refuse-by-default: guards renderRawUpdateDirect -- false hands malformed MP_REACH_NLRI to renderRawUpdateFallback.
		if mpErr != nil {
			return false
		}
		mpReach = parsedMP
	}

	var mpUnreach wireu.MPUnreachWire
	if sp, ok := spanIdx.Find(attribute.AttrMPUnreachNLRI); ok {
		rawUnreach := attrSection[sp.Offset : sp.Offset+sp.Length]
		if len(rawUnreach) < 3 {
			return false
		}
		mpUnreach = wireu.MPUnreachWire(rawUnreach)
	}

	// End-of-RIB detection matching wireu.WireUpdate.IsEOR (RFC 4724 Section 2).
	if len(withdrawnData) == 0 && len(nlriData) == 0 && mpReach == nil {
		if mpUnreach != nil {
			if len(mpUnreach.WithdrawnBytes()) == 0 {
				fam := mpUnreach.Family()
				switch fam {
				case family.IPv4Unicast, family.IPv6Unicast:
					plan.isEOR = true
					plan.eorFam = fam
					return true
				default:
					return false
				}
			}
		} else if len(attrSection) == 0 {
			plan.isEOR = true
			plan.eorFam = family.IPv4Unicast
			return true
		}
	}

	plan.ipv4AddPath = raw.addPathIPv4Unicast()
	plan.ipv6AddPath = raw.addPathIPv6Unicast()

	if !parseRawWhitelistedAttrs(&plan.attrs, attrSection, &spanIdx, asn4) {
		return false
	}

	if len(nlriData) > 0 {
		if !validateUnicastNLRI(nlriData, false, plan.ipv4AddPath) {
			return false
		}
		plan.ipv4Announce = nlriData
		if plan.attrs.hasNextHop {
			plan.ipv4AnnounceNH = plan.attrs.nextHop
		}
	}
	if len(withdrawnData) > 0 {
		if !validateUnicastNLRI(withdrawnData, false, plan.ipv4AddPath) {
			return false
		}
		plan.ipv4Withdraw = withdrawnData
	}

	if mpReach != nil {
		fam := mpReach.Family()
		nh := mpReach.NextHop()
		if !nh.IsValid() {
			return false
		}
		mpNLRI := mpReach.NLRIBytes()
		if len(mpNLRI) == 0 {
			return false
		}
		switch fam {
		case family.IPv4Unicast:
			if len(plan.ipv4Announce) > 0 {
				return false
			}
			if !validateUnicastNLRI(mpNLRI, false, plan.ipv4AddPath) {
				return false
			}
			plan.ipv4Announce = mpNLRI
			plan.ipv4AnnounceNH = nh
		case family.IPv6Unicast:
			if !validateUnicastNLRI(mpNLRI, true, plan.ipv6AddPath) {
				return false
			}
			plan.ipv6Announce = mpNLRI
			plan.ipv6AnnounceNH = nh
		default:
			return false
		}
	}

	if mpUnreach != nil {
		fam := mpUnreach.Family()
		mpWD := mpUnreach.WithdrawnBytes()
		if len(mpWD) == 0 {
			return false
		}
		switch fam {
		case family.IPv4Unicast:
			if len(plan.ipv4Withdraw) > 0 {
				return false
			}
			if !validateUnicastNLRI(mpWD, false, plan.ipv4AddPath) {
				return false
			}
			plan.ipv4Withdraw = mpWD
		case family.IPv6Unicast:
			if !validateUnicastNLRI(mpWD, true, plan.ipv6AddPath) {
				return false
			}
			plan.ipv6Withdraw = mpWD
		default:
			return false
		}
	}

	return true
}

// parseRawWhitelistedAttrs validates and records every attribute in spanIdx
// that the direct renderer supports. Any unsupported, malformed, or tricky
// attribute returns false so the caller falls back cleanly.
func parseRawWhitelistedAttrs(dst *rawParsedAttrs, attrSection []byte, spanIdx *attribute.SpanIndex, asn4 bool) bool {
	dst.asn4 = asn4
	for i := range spanIdx.Len() {
		sp := spanIdx.At(i)
		val := attrSection[sp.Offset : sp.Offset+sp.Length]
		flags := attribute.AttributeFlags(attrSection[sp.Offset-uint16(sp.HdrLen)])

		// Subset of hot unicast attributes handled directly; all others fall back.
		//exhaustive:ignore
		switch sp.Code {
		case attribute.AttrMPReachNLRI, attribute.AttrMPUnreachNLRI:
			continue
		case attribute.AttrOrigin:
			if len(val) != 1 {
				return false
			}
			if val[0] > uint8(attribute.OriginIncomplete) {
				return false
			}
			dst.hasOrigin = true
			dst.origin = val[0]
		case attribute.AttrASPath:
			hasSegs, ok := validateRawASPath(val, asn4)
			if !ok {
				return false
			}
			if hasSegs {
				dst.hasASPath = true
				dst.asPathRaw = val
			}
		case attribute.AttrNextHop:
			switch len(val) {
			case 4:
				dst.nextHop = netip.AddrFrom4([4]byte(val))
			case 16:
				dst.nextHop = netip.AddrFrom16([16]byte(val))
			default:
				return false
			}
			dst.hasNextHop = true
		case attribute.AttrMED:
			if len(val) != 4 {
				return false
			}
			dst.hasMED = true
			dst.med = binary.BigEndian.Uint32(val)
		case attribute.AttrLocalPref:
			if len(val) != 4 {
				return false
			}
			dst.hasLocalPref = true
			dst.localPref = binary.BigEndian.Uint32(val)
		case attribute.AttrAtomicAggregate:
			if len(val) != 0 {
				return false
			}
			dst.hasAtomicAggregate = true
		case attribute.AttrAggregator:
			if dst.hasAggregator {
				return false
			}
			if asn4 {
				if len(val) != 8 {
					return false
				}
				dst.aggregatorASN = binary.BigEndian.Uint32(val[0:4])
				dst.aggregatorAddr = netip.AddrFrom4([4]byte(val[4:8]))
			} else {
				if len(val) != 6 {
					return false
				}
				dst.aggregatorASN = uint32(binary.BigEndian.Uint16(val[0:2]))
				dst.aggregatorAddr = netip.AddrFrom4([4]byte(val[2:6]))
			}
			dst.hasAggregator = true
			dst.aggregatorCode = attribute.AttrAggregator
		case attribute.AttrAS4Aggregator:
			if dst.hasAggregator {
				return false
			}
			if len(val) != 8 {
				return false
			}
			dst.hasAggregator = true
			dst.aggregatorCode = attribute.AttrAS4Aggregator
			dst.aggregatorASN = binary.BigEndian.Uint32(val[0:4])
			dst.aggregatorAddr = netip.AddrFrom4([4]byte(val[4:8]))
		case attribute.AttrCommunity:
			if attribute.GetJSONFormatter(attribute.AttrCommunity) == nil {
				return false
			}
			if len(val)%4 != 0 {
				return false
			}
			if len(val)/4 > rawCommunitiesSortMax {
				return false
			}
			dst.hasCommunity = true
			dst.communityRaw = val
		case attribute.AttrOriginatorID:
			if attribute.GetJSONFormatter(attribute.AttrOriginatorID) == nil {
				return false
			}
			if len(val) != 4 {
				return false
			}
			dst.hasOriginatorID = true
			dst.originatorID = netip.AddrFrom4([4]byte(val))
		case attribute.AttrClusterList:
			if attribute.GetJSONFormatter(attribute.AttrClusterList) == nil {
				return false
			}
			if len(val)%4 != 0 {
				return false
			}
			dst.hasClusterList = true
			dst.clusterListRaw = val
		case attribute.AttrLargeCommunity:
			if attribute.GetJSONFormatter(attribute.AttrLargeCommunity) == nil {
				return false
			}
			if len(val)%12 != 0 {
				return false
			}
			if !validateUniqueLargeCommunities(val) {
				return false
			}
			dst.hasLargeCommunity = true
			dst.largeCommunityRaw = val
		default:
			if sp.Code.Recognized() {
				return false
			}
			if attribute.GetJSONFormatter(sp.Code) != nil {
				return false
			}
			if dst.unknownCount >= rawUnknownAttrsMax {
				return false
			}
			insertRawUnknownAttr(dst, rawUnknownAttr{code: sp.Code, flags: flags, val: val})
		}
	}
	return true
}

// insertRawUnknownAttr keeps dst.unknowns sorted by ascending attribute code so
// both JSON ("attribute-0xCC-0xFF") and text emit in exact deterministic order.
func insertRawUnknownAttr(dst *rawParsedAttrs, item rawUnknownAttr) {
	pos := dst.unknownCount
	for pos > 0 && dst.unknowns[pos-1].code > item.code {
		dst.unknowns[pos] = dst.unknowns[pos-1]
		pos--
	}
	dst.unknowns[pos] = item
	dst.unknownCount++
}

// validateRawASPath verifies AS_PATH framing, segment types, total ASN bound,
// and segment count bound (<= 10 so string keys "0".."9" sort numerically).
func validateRawASPath(val []byte, asn4 bool) (hasSegments, ok bool) {
	if len(val) == 0 {
		return false, true
	}
	asnSize := 2
	if asn4 {
		asnSize = 4
	}
	offset := 0
	segCount := 0
	totalASNs := 0
	for offset < len(val) {
		if offset+2 > len(val) {
			return false, false
		}
		segType := attribute.ASPathSegmentType(val[offset])
		count := int(val[offset+1])
		offset += 2
		if segType < attribute.ASSet {
			return false, false
		}
		if segType > attribute.ASConfedSet {
			return false, false
		}
		if count == 0 {
			return false, false
		}
		totalASNs += count
		if totalASNs > attribute.MaxASPathTotalLength {
			return false, false
		}
		needed := count * asnSize
		if offset+needed > len(val) {
			return false, false
		}
		offset += needed
		segCount++
		if segCount > rawASPathSegmentsMax {
			return false, false
		}
	}
	return segCount > 0, true
}

// validateUniqueLargeCommunities checks count <= rawCommunitiesSortMax and no
// duplicate 12-octet values (RFC 8092 deduplication differs between JSON and
// raw text, so duplicates defer to the fallback renderer).
func validateUniqueLargeCommunities(val []byte) bool {
	count := len(val) / 12
	if count > rawCommunitiesSortMax {
		return false
	}
	for i := 1; i < count; i++ {
		cur := val[i*12 : i*12+12]
		for j := range i {
			if bytes.Equal(cur, val[j*12:j*12+12]) {
				return false
			}
		}
	}
	return true
}

// validateUnicastNLRI verifies that every IPv4 or IPv6 unicast prefix (with
// optional 4-octet RFC 7911 Path Identifier) splits cleanly to the end of data.
func validateUnicastNLRI(data []byte, isV6, addPath bool) bool {
	maxBits := 32
	if isV6 {
		maxBits = 128
	}
	offset := 0
	for offset < len(data) {
		if addPath {
			if offset+4 > len(data) {
				return false
			}
			offset += 4
		}
		if offset >= len(data) {
			return false
		}
		bits := int(data[offset])
		offset++
		if bits > maxBits {
			return false
		}
		prefixBytes := (bits + 7) / 8
		if offset+prefixBytes > len(data) {
			return false
		}
		offset += prefixBytes
	}
	return true
}
