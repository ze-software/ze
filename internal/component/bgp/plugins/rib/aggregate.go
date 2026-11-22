// Design: docs/architecture/plugin/rib-storage-design.md -- RFC 4271 Section 9.2.2.2 and RFC 9774 Section 5.2 route aggregation
// RFC: rfc/short/rfc4271.md -- Section 5.1.7 AGGREGATOR and Section 9.2.2.2 route aggregation
// RFC: rfc/short/rfc9774.md -- Section 5.2 consistent brief BGP aggregation
// Related: config_tx.go -- parses bgp.aggregate configuration and speaker identity
// Related: rib_bestchange.go -- triggers aggregate recomputation when contributing CIDR prefixes change

package rib

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/textparse"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errAggregateDifferentMED     = errors.New("bgp rib: cannot aggregate routes with different MULTI_EXIT_DISC attributes")
	errAggregateNonLocalNextHop  = errors.New("bgp rib: aggregate next-hop must identify a local interface on this BGP speaker")
	errAggregateNoLocalNextHop   = errors.New("bgp rib: no local interface address available for aggregate NEXT_HOP")
	errAggregateMissingSpeakerID = errors.New("bgp rib: AGGREGATOR requires valid local AS number and IPv4 router-id")
	errAggregateOriginASMissing  = errors.New("bgp rib: brief-origin-as not present in common leading AS_PATH")
)

// aggregateSpeaker identifies the local BGP speaker performing route
// aggregation (bgp.session.asn.local and bgp.router-id).
type aggregateSpeaker struct {
	LocalAS  uint32
	RouterID netip.Addr
}

// aggregateRule describes one configured `bgp aggregate <prefix>` entry.
type aggregateRule struct {
	Prefix            netip.Prefix
	Family            family.Family
	NextHop           netip.Addr
	IncludeAggregator bool
	BriefOriginAS     uint32
	HasBriefOriginAS  bool
}

// aggregateConfig holds all configured aggregate rules and the local speaker's
// own AS number and BGP Identifier.
type aggregateConfig struct {
	Speaker aggregateSpeaker
	Rules   []aggregateRule
}

// aggregateContributor captures the path attributes of one selected
// more-specific route contributing to a configured aggregate prefix.
type aggregateContributor struct {
	Prefix                netip.Prefix
	Origin                attribute.Origin
	ASPath                []uint32
	HadNonSequenceAS      bool
	NextHop               netip.Addr
	HasMED                bool
	MED                   uint32
	HasAtomicAggregate    bool
	ContributorAggregator [][]byte
}

// synthesizedAggregate is the resulting aggregate route produced by
// synthesizeAggregate from the current contributing Loc-RIB routes.
type synthesizedAggregate struct {
	Prefix          netip.Prefix
	Family          family.Family
	Origin          attribute.Origin
	ASPath          []uint32
	NextHop         netip.Addr
	HasMED          bool
	MED             uint32
	AtomicAggregate bool
	HasAggregator   bool
	AggregatorAS    uint32
	AggregatorIP    netip.Addr
	Attributes      []byte
}

// equal reports whether two synthesized aggregates produce identical forwarding
// and wire attribute output.
func (a *synthesizedAggregate) equal(b *synthesizedAggregate) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil {
		return false
	}
	if b == nil {
		return false
	}
	if a.Prefix != b.Prefix {
		return false
	}
	if a.Family != b.Family {
		return false
	}
	if a.NextHop != b.NextHop {
		return false
	}
	return bytes.Equal(a.Attributes, b.Attributes)
}

// parseAggregateConfig reads `bgp.router-id`, `bgp.session.asn.local`, and
// `bgp.aggregate` from the delivered BGP configuration subtree. Returns an
// empty aggregateConfig (len(Rules) == 0) when no aggregate entries are configured.
func parseAggregateConfig(bgpCfg map[string]any) (aggregateConfig, error) {
	var cfg aggregateConfig
	aggRaw, present := bgpCfg["aggregate"]
	if present && aggRaw != nil {
		aggMap, ok := aggRaw.(map[string]any)
		if !ok {
			return aggregateConfig{}, fmt.Errorf("bgp rib: aggregate must be a map, got %T", aggRaw)
		}
		if len(aggMap) > 0 {
			speaker, err := parseAggregateSpeaker(bgpCfg)
			if err != nil {
				return aggregateConfig{}, err
			}
			keys := make([]string, 0, len(aggMap))
			for key := range aggMap {
				keys = append(keys, key)
			}
			slices.Sort(keys)

			rules := make([]aggregateRule, 0, len(keys))
			for _, key := range keys {
				entryMap, ok := aggMap[key].(map[string]any)
				if !ok {
					return aggregateConfig{}, fmt.Errorf("bgp rib: aggregate %q must be an object, got %T", key, aggMap[key])
				}
				rule, err := parseAggregateRule(key, entryMap)
				if err != nil {
					return aggregateConfig{}, err
				}
				rules = append(rules, rule)
			}
			cfg.Speaker = speaker
			cfg.Rules = rules
		}
	}
	return cfg, nil
}

func parseAggregateSpeaker(bgpCfg map[string]any) (aggregateSpeaker, error) {
	var speaker aggregateSpeaker
	if ridStr, ok := bgpCfg["router-id"].(string); ok && ridStr != "" {
		addr, err := netip.ParseAddr(ridStr)
		if err != nil {
			return aggregateSpeaker{}, fmt.Errorf("bgp rib: invalid bgp router-id %q: %w", ridStr, err)
		}
		if !addr.Is4() {
			return aggregateSpeaker{}, fmt.Errorf("bgp rib: bgp router-id %q must be IPv4", ridStr)
		}
		speaker.RouterID = addr
	}
	if sessMap, ok := bgpCfg["session"].(map[string]any); ok {
		if asnMap, ok := sessMap["asn"].(map[string]any); ok {
			if rawLocal, present := asnMap["local"]; present {
				localAS, err := parseAggregateASN(rawLocal)
				if err != nil {
					return aggregateSpeaker{}, fmt.Errorf("bgp rib: invalid session.asn.local: %w", err)
				}
				speaker.LocalAS = localAS
			}
		}
	}
	return speaker, nil
}

func parseAggregateRule(key string, entry map[string]any) (aggregateRule, error) {
	pfxStr := key
	if rawPfx, ok := entry["prefix"].(string); ok && rawPfx != "" {
		pfxStr = rawPfx
	}
	pfx, err := netip.ParsePrefix(pfxStr)
	if err != nil {
		return aggregateRule{}, fmt.Errorf("bgp rib: invalid aggregate prefix %q: %w", pfxStr, err)
	}
	pfx = pfx.Masked()
	fam := family.IPv4Unicast
	if pfx.Addr().Is6() {
		fam = family.IPv6Unicast
	}

	rule := aggregateRule{
		Prefix:            pfx,
		Family:            fam,
		IncludeAggregator: true,
	}

	if rawNH, present := entry["next-hop"]; present && rawNH != nil {
		nhStr, ok := rawNH.(string)
		if !ok {
			return aggregateRule{}, fmt.Errorf("bgp rib: aggregate %s next-hop must be a string", pfx)
		}
		if nhStr != "" {
			nh, err := netip.ParseAddr(nhStr)
			if err != nil {
				return aggregateRule{}, fmt.Errorf("bgp rib: aggregate %s invalid next-hop %q: %w", pfx, nhStr, err)
			}
			if nh.Is4() != pfx.Addr().Is4() {
				return aggregateRule{}, fmt.Errorf("bgp rib: aggregate %s next-hop %s address family mismatch", pfx, nh)
			}
			rule.NextHop = nh
		}
	}

	if rawAgg, present := entry["aggregator"]; present && rawAgg != nil {
		include, err := parseAggregateBool(rawAgg)
		if err != nil {
			return aggregateRule{}, fmt.Errorf("bgp rib: aggregate %s aggregator: %w", pfx, err)
		}
		rule.IncludeAggregator = include
	}

	if rawOrigin, present := entry["brief-origin-as"]; present && rawOrigin != nil {
		originAS, err := parseAggregateASN(rawOrigin)
		if err != nil {
			return aggregateRule{}, fmt.Errorf("bgp rib: aggregate %s brief-origin-as: %w", pfx, err)
		}
		if originAS == 0 {
			return aggregateRule{}, fmt.Errorf("bgp rib: aggregate %s brief-origin-as must be non-zero", pfx)
		}
		rule.BriefOriginAS = originAS
		rule.HasBriefOriginAS = true
	}

	return rule, nil
}

func parseAggregateBool(raw any) (bool, error) {
	switch v := raw.(type) {
	case bool:
		return v, nil
	case string:
		b, err := strconv.ParseBool(v)
		if err != nil {
			return false, fmt.Errorf("invalid boolean %q: %w", v, err)
		}
		return b, nil
	default:
		return false, fmt.Errorf("unsupported boolean type %T", raw)
	}
}

func parseAggregateASN(raw any) (uint32, error) {
	switch v := raw.(type) {
	case float64:
		if v < 0 {
			return 0, fmt.Errorf("ASN %v out of range", v)
		}
		if v > float64(^uint32(0)) {
			return 0, fmt.Errorf("ASN %v out of range", v)
		}
		if float64(uint32(v)) != v {
			return 0, fmt.Errorf("ASN %v out of range", v)
		}
		return uint32(v), nil
	case string:
		if highStr, lowStr, hasDot := strings.Cut(v, "."); hasDot {
			high, err := strconv.ParseUint(highStr, 10, 16)
			if err != nil {
				return 0, fmt.Errorf("invalid asdot ASN %q: %w", v, err)
			}
			low, err := strconv.ParseUint(lowStr, 10, 16)
			if err != nil {
				return 0, fmt.Errorf("invalid asdot ASN %q: %w", v, err)
			}
			return uint32(high)<<16 | uint32(low), nil
		}
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid ASN %q: %w", v, err)
		}
		return uint32(n), nil
	default:
		return 0, fmt.Errorf("unsupported ASN type %T", raw)
	}
}

// synthesizeAggregate builds an aggregated BGP route from contributing selected
// routes according to RFC 4271 Sections 5.1.7 and 9.2.2.2 and RFC 9774 Section 5.2.
//
// RFC 4271 Section 9.2.2.2: "Routes that have different MULTI_EXIT_DISC
// attributes SHALL NOT be aggregated."
//
// RFC 4271 Section 9.2.2.2: "ORIGIN attribute: If at least one route among
// routes that are aggregated has ORIGIN with the value INCOMPLETE, then the
// aggregated route MUST have the ORIGIN attribute with the value INCOMPLETE.
// Otherwise, if at least one route among routes that are aggregated has ORIGIN
// with the value EGP, then the aggregated route MUST have the ORIGIN attribute
// with the value EGP."
//
// RFC 4271 Section 9.2.2.2: "NEXT_HOP: When aggregating routes that have
// different NEXT_HOP attributes, the NEXT_HOP attribute of the aggregated route
// SHALL identify an interface on the BGP speaker that performs the aggregation."
//
// RFC 4271 Section 9.2.2.2: "ATOMIC_AGGREGATE: If at least one of the routes to
// be aggregated has ATOMIC_AGGREGATE path attribute, then the aggregated route
// SHALL have this attribute as well."
//
// RFC 4271 Section 9.2.2.2: "AGGREGATOR: Any AGGREGATOR attributes from the
// routes to be aggregated MUST NOT be included in the aggregated route."
//
// RFC 4271 Section 5.1.7: "A BGP speaker that performs route aggregation MAY
// add the AGGREGATOR attribute, which SHALL contain its own AS number and IP
// address."
//
// RFC 9774 Section 5.2: "To ensure a consistent BGP origin AS is announced for
// aggregate BGP routes for implementations of \"brief\" BGP aggregation, the
// implementation MUST be configured to truncate the AS_PATH after the
// right-most instance of the desired origin AS for the aggregate."
func synthesizeAggregate(rule aggregateRule, speaker aggregateSpeaker, localAddrs []netip.Addr, contributors []aggregateContributor) (*synthesizedAggregate, error) {
	if len(contributors) == 0 {
		return nil, nil
	}

	// RFC 4271 Section 9.2.2.2: refuse aggregation if MED differs.
	hasMED, commonMED, err := aggregateCheckMED(contributors)
	if err != nil {
		return nil, err
	}

	// RFC 4271 Section 9.2.2.2: compute ORIGIN (INCOMPLETE > EGP > IGP).
	origin := aggregateOrigin(contributors)

	// RFC 4271 Section 9.2.2.2: resolve NEXT_HOP.
	nextHop, err := aggregateNextHop(rule, localAddrs, contributors)
	if err != nil {
		return nil, err
	}

	// RFC 9774 Section 5.2 and RFC 4271 Section 9.2.2.2: compute brief or
	// consistent-brief AS_PATH without AS_SET.
	asPath, asExcluded, err := aggregateBriefASPath(rule, speaker, contributors)
	if err != nil {
		return nil, err
	}

	// RFC 4271 Section 9.2.2.2 and Section 5.1.6: inherit ATOMIC_AGGREGATE or
	// attach when AS numbers were excluded by brief aggregation.
	atomicAgg := aggregateAtomic(contributors, asExcluded)

	// RFC 4271 Section 5.1.7: attach only this speaker's own AS and IP when enabled.
	hasAggregator, aggAS, aggIP, err := aggregateOwnAggregator(rule, speaker)
	if err != nil {
		return nil, err
	}

	agg := &synthesizedAggregate{
		Prefix:          rule.Prefix,
		Family:          rule.Family,
		Origin:          origin,
		ASPath:          asPath,
		NextHop:         nextHop,
		HasMED:          hasMED,
		MED:             commonMED,
		AtomicAggregate: atomicAgg,
		HasAggregator:   hasAggregator,
		AggregatorAS:    aggAS,
		AggregatorIP:    aggIP,
	}
	agg.Attributes = encodeAggregateAttributes(agg, contributors)
	return agg, nil
}

// aggregateCheckMED enforces RFC 4271 Section 9.2.2.2: "Routes that have
// different MULTI_EXIT_DISC attributes SHALL NOT be aggregated."
func aggregateCheckMED(contributors []aggregateContributor) (bool, uint32, error) {
	firstHas := contributors[0].HasMED
	firstMED := contributors[0].MED
	for i := 1; i < len(contributors); i++ {
		if contributors[i].HasMED != firstHas {
			return false, 0, errAggregateDifferentMED
		}
		if firstHas && contributors[i].MED != firstMED {
			return false, 0, errAggregateDifferentMED
		}
	}
	return firstHas, firstMED, nil
}

// aggregateOrigin enforces RFC 4271 Section 9.2.2.2: "ORIGIN attribute: If at
// least one route among routes that are aggregated has ORIGIN with the value
// INCOMPLETE, then the aggregated route MUST have the ORIGIN attribute with the
// value INCOMPLETE. Otherwise, if at least one route among routes that are
// aggregated has ORIGIN with the value EGP, then the aggregated route MUST have
// the ORIGIN attribute with the value EGP."
func aggregateOrigin(contributors []aggregateContributor) attribute.Origin {
	hasEGP := false
	for i := range contributors {
		if contributors[i].Origin == attribute.OriginIncomplete {
			return attribute.OriginIncomplete
		}
		if contributors[i].Origin == attribute.OriginEGP {
			hasEGP = true
		}
	}
	if hasEGP {
		return attribute.OriginEGP
	}
	return attribute.OriginIGP
}

// aggregateNextHop enforces RFC 4271 Section 9.2.2.2: "NEXT_HOP: When
// aggregating routes that have different NEXT_HOP attributes, the NEXT_HOP
// attribute of the aggregated route SHALL identify an interface on the BGP
// speaker that performs the aggregation."
//
// Only addresses belonging to localAddrs (the speaker's configured interface
// addresses and session local addresses) qualify. A BGP Identifier (router-id)
// is not assumed to be an interface IP unless it also appears in localAddrs.
func aggregateNextHop(rule aggregateRule, localAddrs []netip.Addr, contributors []aggregateContributor) (netip.Addr, error) {
	firstNH := contributors[0].NextHop
	allSame := firstNH.IsValid()
	for i := 1; i < len(contributors); i++ {
		if contributors[i].NextHop != firstNH {
			allSame = false
			break
		}
	}
	if allSame {
		return firstNH, nil
	}

	wantV4 := rule.Family == family.IPv4Unicast
	if rule.NextHop.IsValid() {
		if !isLocalSpeakerAddress(rule.NextHop, localAddrs) {
			return netip.Addr{}, fmt.Errorf("%w: %s", errAggregateNonLocalNextHop, rule.NextHop)
		}
		return rule.NextHop, nil
	}

	for _, addr := range localAddrs {
		if !addr.IsValid() {
			continue
		}
		if addr.Is4() != wantV4 {
			continue
		}
		if addr.IsLoopback() {
			continue
		}
		if addr.IsUnspecified() {
			continue
		}
		if addr.IsLinkLocalUnicast() {
			continue
		}
		return addr, nil
	}
	return netip.Addr{}, errAggregateNoLocalNextHop
}

func isLocalSpeakerAddress(candidate netip.Addr, localAddrs []netip.Addr) bool {
	if !candidate.IsValid() {
		return false
	}
	return slices.Contains(localAddrs, candidate.Unmap())
}

// aggregateBriefASPath computes the longest common leading AS_SEQUENCE across
// contributors without emitting any AS_SET (RFC 9774 Section 3), and when
// rule.HasBriefOriginAS is set applies consistent brief truncation.
//
// RFC 9774 Section 5.2: "To ensure a consistent BGP origin AS is announced for
// aggregate BGP routes for implementations of \"brief\" BGP aggregation, the
// implementation MUST be configured to truncate the AS_PATH after the
// right-most instance of the desired origin AS for the aggregate."
func aggregateBriefASPath(rule aggregateRule, speaker aggregateSpeaker, contributors []aggregateContributor) ([]uint32, bool, error) {
	common := slices.Clone(contributors[0].ASPath)
	excluded := contributors[0].HadNonSequenceAS
	for i := 1; i < len(contributors); i++ {
		if contributors[i].HadNonSequenceAS {
			excluded = true
		}
		other := contributors[i].ASPath
		limit := min(len(common), len(other))
		match := 0
		for match < limit && common[match] == other[match] {
			match++
		}
		if match < len(common) || match < len(other) {
			excluded = true
		}
		common = common[:match]
	}

	if !rule.HasBriefOriginAS {
		return common, excluded, nil
	}

	rightMost := -1
	for i, asnVal := range common {
		if asnVal == rule.BriefOriginAS {
			rightMost = i
		}
	}
	if rightMost >= 0 {
		if rightMost+1 < len(common) {
			excluded = true
		}
		return slices.Clone(common[:rightMost+1]), excluded, nil
	}
	// RFC 9774 Section 5.2: "The desired origin AS could be the aggregating AS itself."
	if speaker.LocalAS != 0 && rule.BriefOriginAS == speaker.LocalAS {
		if len(common) > 0 {
			excluded = true
		}
		return nil, excluded, nil
	}
	return nil, false, fmt.Errorf("%w: %d", errAggregateOriginASMissing, rule.BriefOriginAS)
}

// aggregateAtomic enforces RFC 4271 Section 9.2.2.2: "ATOMIC_AGGREGATE: If at
// least one of the routes to be aggregated has ATOMIC_AGGREGATE path attribute,
// then the aggregated route SHALL have this attribute as well."
func aggregateAtomic(contributors []aggregateContributor, asExcluded bool) bool {
	if asExcluded {
		return true
	}
	for i := range contributors {
		if contributors[i].HasAtomicAggregate {
			return true
		}
	}
	return false
}

// aggregateOwnAggregator enforces RFC 4271 Section 5.1.7: "A BGP speaker that
// performs route aggregation MAY add the AGGREGATOR attribute, which SHALL
// contain its own AS number and IP address."
func aggregateOwnAggregator(rule aggregateRule, speaker aggregateSpeaker) (bool, uint32, netip.Addr, error) {
	if !rule.IncludeAggregator {
		return false, 0, netip.Addr{}, nil
	}
	if speaker.LocalAS == 0 {
		return false, 0, netip.Addr{}, errAggregateMissingSpeakerID
	}
	if !speaker.RouterID.IsValid() {
		return false, 0, netip.Addr{}, errAggregateMissingSpeakerID
	}
	if !speaker.RouterID.Is4() {
		return false, 0, netip.Addr{}, errAggregateMissingSpeakerID
	}
	return true, speaker.LocalAS, speaker.RouterID, nil
}

// aggregateAggregatorAttributes enforces RFC 4271 Section 9.2.2.2: "AGGREGATOR:
// Any AGGREGATOR attributes from the routes to be aggregated MUST NOT be
// included in the aggregated route." It explicitly discards all contributor
// AGGREGATOR and AS4_AGGREGATOR attributes and appends only this speaker's own
// AGGREGATOR when agg.HasAggregator is set.
func aggregateAggregatorAttributes(dst []byte, agg *synthesizedAggregate, contributors []aggregateContributor) []byte {
	// Discard every contributor AGGREGATOR / AS4_AGGREGATOR: none may be copied
	// into the aggregated route regardless of whether the speaker adds its own.
	_ = contributors
	if !agg.HasAggregator {
		return dst
	}
	var buf [8]byte
	binary.BigEndian.PutUint32(buf[0:4], agg.AggregatorAS)
	ip4 := agg.AggregatorIP.As4()
	copy(buf[4:8], ip4[:])
	return appendAggregateAttr(dst, byte(attribute.FlagOptional|attribute.FlagTransitive), byte(attribute.AttrAggregator), buf[:])
}

// encodeAggregateAttributes encodes the synthesized aggregate's BGP path
// attributes in canonical ascending type-code order (RFC 4271 Sections 4.3, 5,
// 5.1.7, and 9.2.2.2), using 4-byte ASNs to match bgp-rib's internal wire format.
func encodeAggregateAttributes(agg *synthesizedAggregate, contributors []aggregateContributor) []byte {
	var out []byte

	// Type 1: ORIGIN (well-known mandatory, flags 0x40, length 1).
	out = append(out, byte(attribute.FlagTransitive), byte(attribute.AttrOrigin), 1, byte(agg.Origin))

	// Type 2: AS_PATH (well-known mandatory, flags 0x40/0x50, 4-byte AS_SEQUENCE).
	asVal := encodeASPath4(agg.ASPath)
	out = appendAggregateAttr(out, byte(attribute.FlagTransitive), byte(attribute.AttrASPath), asVal)

	// Type 3: NEXT_HOP (well-known mandatory for IPv4 unicast, flags 0x40, length 4).
	if agg.Family == family.IPv4Unicast && agg.NextHop.Is4() {
		nh4 := agg.NextHop.As4()
		out = appendAggregateAttr(out, byte(attribute.FlagTransitive), byte(attribute.AttrNextHop), nh4[:])
	}

	// Type 4: MULTI_EXIT_DISC (optional non-transitive, flags 0x80, length 4).
	if agg.HasMED {
		var med [4]byte
		binary.BigEndian.PutUint32(med[:], agg.MED)
		out = appendAggregateAttr(out, byte(attribute.FlagOptional), byte(attribute.AttrMED), med[:])
	}

	// Type 5: LOCAL_PREF (well-known discretionary, flags 0x40, length 4, default 100).
	var lp [4]byte
	binary.BigEndian.PutUint32(lp[:], 100)
	out = appendAggregateAttr(out, byte(attribute.FlagTransitive), byte(attribute.AttrLocalPref), lp[:])

	// Type 6: ATOMIC_AGGREGATE (well-known discretionary, flags 0x40, length 0).
	if agg.AtomicAggregate {
		out = appendAggregateAttr(out, byte(attribute.FlagTransitive), byte(attribute.AttrAtomicAggregate), nil)
	}

	// Type 7: AGGREGATOR (optional transitive, flags 0xC0, speaker's own only;
	// contributor AGGREGATOR / AS4_AGGREGATOR attributes are never included).
	out = aggregateAggregatorAttributes(out, agg, contributors)

	return out
}

// formatAggregateAnnounceCommand builds the engine `update text` announce
// command that advertises agg to BGP peers (Adj-RIB-Out -> wire UPDATE).
// Using `update text` passes the aggregate through the reactor's Builder path
// (planBatchAttrs), which handles per-peer eBGP AS_PATH prepending, LOCAL_PREF
// stripping on eBGP, RFC 6793 2-byte/4-byte AGGREGATOR transcoding, IPv4
// NEXT_HOP, and IPv6 MP_REACH_NLRI next-hop encoding.
func formatAggregateAnnounceCommand(agg *synthesizedAggregate) string {
	var sb textbuf.Buffer
	sb.Str("update text origin ").Str(agg.Origin.LowerString())
	if len(agg.ASPath) > 0 {
		p := &attribute.ASPath{Segments: []attribute.ASPathSegment{{Type: attribute.ASSequence, ASNs: agg.ASPath}}}
		var scratch [128]byte
		sb.Byte(' ')
		sb.Write(p.AppendText(scratch[:0]))
	}
	if agg.HasMED {
		sb.Str(" med ").Uint32(agg.MED)
	}
	sb.Str(" local-preference 100")
	if agg.AtomicAggregate {
		sb.Str(" atomic-aggregate")
	}
	if agg.HasAggregator {
		sb.Str(" aggregator ").Uint32(agg.AggregatorAS).Byte(':').Str(agg.AggregatorIP.String())
	}
	sb.Str(" nhop ").Str(agg.NextHop.String())
	if textparse.LegacyNLRISection(agg.Family.String(), textparse.IPv6NextHopText(agg.NextHop.String())) {
		sb.Str(" nlri add ")
	} else {
		sb.Str(" mp ").Str(agg.Family.String()).Str(" add ")
	}
	sb.Str(agg.Prefix.String())
	return sb.String()
}

// formatAggregateWithdrawCommand builds the engine `update text` withdrawal
// command that withdraws agg from BGP peers when its contributors disappear or
// become ineligible (e.g., differing MED).
func formatAggregateWithdrawCommand(agg *synthesizedAggregate) string {
	var sb textbuf.Buffer
	sb.Str("update text")
	if textparse.LegacyNLRISection(agg.Family.String(), false) {
		sb.Str(" nlri del ")
	} else {
		sb.Str(" mp ").Str(agg.Family.String()).Str(" del ")
	}
	sb.Str(agg.Prefix.String())
	return sb.String()
}

func encodeASPath4(asns []uint32) []byte {
	if len(asns) == 0 {
		return nil
	}
	var out []byte
	for start := 0; start < len(asns); start += 255 {
		end := min(start+255, len(asns))
		seg := asns[start:end]
		out = append(out, byte(attribute.ASSequence), byte(len(seg)))
		for _, a := range seg {
			var b [4]byte
			binary.BigEndian.PutUint32(b[:], a)
			out = append(out, b[:]...)
		}
	}
	return out
}

func appendAggregateAttr(dst []byte, flags, code byte, val []byte) []byte {
	if len(val) > 255 {
		flags |= 0x10 // Extended Length bit (RFC 4271 Section 4.3)
		dst = append(dst, flags, code, byte(len(val)>>8), byte(len(val)))
	} else {
		dst = append(dst, flags, code, byte(len(val)))
	}
	return append(dst, val...)
}

// parseContributorAttrs extracts RFC 4271 Section 9.2.2.2 aggregation inputs
// from a selected route's canonical 4-byte wire attribute block.
func parseContributorAttrs(pfx netip.Prefix, attrs []byte, fallbackNH netip.Addr) aggregateContributor {
	c := aggregateContributor{
		Prefix:  pfx,
		Origin:  attribute.OriginIGP,
		NextHop: fallbackNH,
	}
	iter := attribute.NewAttrIterator(attrs)
	for code, _, val, ok := iter.Next(); ok; code, _, val, ok = iter.Next() {
		// Only attributes participating in RFC 4271 Section 9.2.2.2 aggregation are read.
		//exhaustive:ignore
		switch code {
		case attribute.AttrOrigin:
			if len(val) == 1 {
				c.Origin = attribute.Origin(val[0])
			}
		case attribute.AttrASPath:
			c.ASPath, c.HadNonSequenceAS = parseASPath4ForAggregate(val)
		case attribute.AttrNextHop:
			if len(val) == 4 {
				c.NextHop = parseNextHopAddr(val)
			}
		case attribute.AttrMED:
			if len(val) == 4 {
				c.HasMED = true
				c.MED = binary.BigEndian.Uint32(val)
			}
		case attribute.AttrAtomicAggregate:
			c.HasAtomicAggregate = true
		case attribute.AttrAggregator, attribute.AttrAS4Aggregator:
			c.ContributorAggregator = append(c.ContributorAggregator, slices.Clone(val))
		}
	}
	return c
}

// parseASPath4ForAggregate reads the leading contiguous AS_SEQUENCE ASNs from a
// 4-byte wire AS_PATH. On encountering any non-AS_SEQUENCE segment (AS_SET,
// AS_CONFED_SEQUENCE, AS_CONFED_SET), it sets hadNonSeq and stops immediately
// so trailing AS_SEQUENCE segments after a non-sequence boundary are never
// concatenated onto the leading sequence.
func parseASPath4ForAggregate(data []byte) ([]uint32, bool) {
	var seq []uint32
	hadNonSeq := false
	off := 0
	for off+2 <= len(data) {
		segType := attribute.ASPathSegmentType(data[off])
		count := int(data[off+1])
		off += 2
		need := count * 4
		if off+need > len(data) {
			break
		}
		if segType != attribute.ASSequence {
			hadNonSeq = true
			break
		}
		for i := range count {
			seq = append(seq, binary.BigEndian.Uint32(data[off+i*4:off+i*4+4]))
		}
		off += need
	}
	return seq, hadNonSeq
}

// ActiveAggregate returns a snapshot copy of the currently synthesized
// aggregate route for prefix, and reports whether one is active.
func (r *RIBManager) ActiveAggregate(prefix netip.Prefix) (*synthesizedAggregate, bool) {
	r.aggregateMu.Lock()
	defer r.aggregateMu.Unlock()
	agg, found := r.activeAggregates[prefix.Masked()]
	if !found {
		return nil, false
	}
	cp := *agg
	cp.ASPath = slices.Clone(agg.ASPath)
	cp.Attributes = slices.Clone(agg.Attributes)
	return &cp, true
}

func (r *RIBManager) snapshotActiveAggregates() []*synthesizedAggregate {
	r.aggregateMu.Lock()
	defer r.aggregateMu.Unlock()
	if len(r.activeAggregates) == 0 {
		return nil
	}
	out := make([]*synthesizedAggregate, 0, len(r.activeAggregates))
	for _, agg := range r.activeAggregates {
		cp := *agg
		cp.ASPath = slices.Clone(agg.ASPath)
		cp.Attributes = slices.Clone(agg.Attributes)
		out = append(out, &cp)
	}
	slices.SortFunc(out, func(a, b *synthesizedAggregate) int {
		if a.Prefix.Bits() != b.Prefix.Bits() {
			if a.Prefix.Bits() < b.Prefix.Bits() {
				return -1
			}
			return 1
		}
		return a.Prefix.Addr().Compare(b.Prefix.Addr())
	})
	return out
}

// appendActiveAggregateReplayEntries appends BestChangeAdd entries for every
// currently active aggregate route into changesByFamily for replayBestPaths
// and replayRedistribute.
func (r *RIBManager) appendActiveAggregateReplayEntries(changesByFamily map[family.Family][]bestChangeEntry) {
	for _, agg := range r.snapshotActiveAggregates() {
		entry := bestChangeEntry{
			Action:       ribevents.BestChangeAdd,
			Prefix:       agg.Prefix,
			NextHop:      agg.NextHop,
			Priority:     200,
			Metric:       agg.MED,
			ProtocolType: routeaction.ProtocolIBGP,
		}
		setEntryPath(&entry, agg.ASPath, agg.Attributes)
		changesByFamily[agg.Family] = append(changesByFamily[agg.Family], entry)
	}
}

// replayAggregatesToPeer advertises all currently active aggregate routes to a
// peer that has just transitioned to Established.
func (r *RIBManager) replayAggregatesToPeer(peerAddr string) {
	if peerAddr == "" {
		return
	}
	aggregates := r.snapshotActiveAggregates()
	if len(aggregates) == 0 {
		return
	}
	meta := map[string]any{"rib-aggregate": true}
	for _, agg := range aggregates {
		cmd := formatAggregateAnnounceCommand(agg)
		_ = r.updateRouteWithMeta(peerAddr, cmd, meta)
	}
}

// recomputeAllAggregates recomputes every configured aggregate rule and
// withdraws any previously active aggregate whose rule was removed.
func (r *RIBManager) recomputeAllAggregates() {
	cfg := r.aggregateCfg.Load()
	configured := make(map[netip.Prefix]struct{})
	if cfg != nil {
		for _, rule := range cfg.Rules {
			configured[rule.Prefix] = struct{}{}
			r.recomputeAggregateRule(cfg.Speaker, rule)
		}
	}

	r.aggregatePublishMu.Lock()
	defer r.aggregatePublishMu.Unlock()

	r.aggregateMu.Lock()
	var removed []*synthesizedAggregate
	for pfx, prev := range r.activeAggregates {
		if _, exists := configured[pfx]; !exists {
			delete(r.activeAggregates, pfx)
			removed = append(removed, prev)
		}
	}
	r.aggregateMu.Unlock()

	for _, prev := range removed {
		r.withdrawAggregateRoute(prev)
	}
}

// recomputeAggregatesForPrefix recomputes every configured aggregate rule whose
// prefix strictly covers changedPfx (i.e. changedPfx is a more-specific route).
func (r *RIBManager) recomputeAggregatesForPrefix(fam family.Family, changedPfx netip.Prefix) {
	cfg := r.aggregateCfg.Load()
	if cfg == nil {
		return
	}
	for _, rule := range cfg.Rules {
		if rule.Family != fam {
			continue
		}
		if !rule.Prefix.Contains(changedPfx.Addr()) {
			continue
		}
		if changedPfx.Bits() <= rule.Prefix.Bits() {
			continue
		}
		r.recomputeAggregateRule(cfg.Speaker, rule)
	}
}

func (r *RIBManager) recomputeAggregateRule(speaker aggregateSpeaker, rule aggregateRule) {
	r.aggregatePublishMu.Lock()
	defer r.aggregatePublishMu.Unlock()

	contributors := r.gatherAggregateContributors(rule.Family, rule.Prefix)
	localAddrs := r.aggregateLocalAddresses()

	// RFC 4271 Section 9.2.2.2, Section 5.1.7; RFC 9774 Section 5.2
	nextAgg, err := synthesizeAggregate(rule, speaker, localAddrs, contributors)
	if err != nil {
		logger().Warn("bgp-rib: aggregate route suppressed",
			"code", codeDoctorBGPSelectedOutput,
			"prefix", rule.Prefix,
			"error", err)
		nextAgg = nil
	}

	r.aggregateMu.Lock()
	if r.activeAggregates == nil {
		r.activeAggregates = make(map[netip.Prefix]*synthesizedAggregate)
	}
	prev := r.activeAggregates[rule.Prefix]
	if prev.equal(nextAgg) {
		r.aggregateMu.Unlock()
		return
	}
	if nextAgg == nil {
		delete(r.activeAggregates, rule.Prefix)
	} else {
		r.activeAggregates[rule.Prefix] = nextAgg
	}
	r.aggregateMu.Unlock()

	if nextAgg == nil {
		r.withdrawAggregateRoute(prev)
		return
	}
	action := ribevents.BestChangeAdd
	if prev != nil {
		action = ribevents.BestChangeUpdate
	}
	r.publishAggregateRoute(nextAgg, action)
}

// gatherAggregateContributors walks selected CIDR routes strictly more specific
// than aggPrefix. It calls r.replayPath under each shard's read lock, obeying
// replayPath's shard-before-peerMu lock contract so peerIdx cannot race with
// concurrent peer-down removal.
func (r *RIBManager) gatherAggregateContributors(fam family.Family, aggPrefix netip.Prefix) []aggregateContributor {
	fs := r.bestPrev.familyShards(fam, false)
	if fs == nil {
		return nil
	}

	var contributors []aggregateContributor
	for i := range fs.shards {
		sh := &fs.shards[i]
		sh.mu.RLock()
		if sh.store.cidr {
			sh.store.direct.Iterate(func(pfx netip.Prefix, prev bestPrevRecord) bool {
				if !aggPrefix.Contains(pfx.Addr()) {
					return true
				}
				if pfx.Bits() <= aggPrefix.Bits() {
					return true
				}
				owner := r.bestPathInterner.ownerAt(prev.rec.peerIdx())
				peerAddr := r.bestPathInterner.peerAt(prev.rec.peerIdx())
				nextHop := r.bestPathInterner.nextHopAt(prev.rec.nextHopIdx())
				_, attrs, _, _, held := r.replayPath(fam, pfx, nil, owner, peerAddr, prev.pathID)
				if !held {
					return true
				}
				contributors = append(contributors, parseContributorAttrs(pfx, attrs, nextHop))
				return true
			})
		}
		sh.mu.RUnlock()
	}

	if len(contributors) == 0 {
		return nil
	}
	slices.SortFunc(contributors, func(a, b aggregateContributor) int {
		if a.Prefix.Bits() != b.Prefix.Bits() {
			if a.Prefix.Bits() < b.Prefix.Bits() {
				return -1
			}
			return 1
		}
		return a.Prefix.Addr().Compare(b.Prefix.Addr())
	})
	return contributors
}

func (r *RIBManager) aggregateLocalAddresses() []netip.Addr {
	var out []netip.Addr
	seen := make(map[netip.Addr]struct{})
	add := func(addr netip.Addr) {
		if !addr.IsValid() {
			return
		}
		addr = addr.Unmap()
		if _, dup := seen[addr]; dup {
			return
		}
		seen[addr] = struct{}{}
		out = append(out, addr)
	}

	r.peerMu.RLock()
	for _, meta := range r.peerMeta {
		if meta != nil && meta.LocalAddress.IsValid() {
			add(meta.LocalAddress)
		}
	}
	r.peerMu.RUnlock()

	if r.localAddrsFn != nil {
		for _, addr := range r.localAddrsFn() {
			add(addr)
		}
		return out
	}
	sysAddrs, err := systemLocalInterfaceAddrs()
	if err != nil {
		logger().Warn("bgp-rib: reading local interface addresses for aggregate next-hop failed",
			"code", codeDoctorBGPSelectedOutput,
			"error", err)
		return out
	}
	for _, addr := range sysAddrs {
		add(addr)
	}
	return out
}

func systemLocalInterfaceAddrs() ([]netip.Addr, error) {
	ifAddrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	out := make([]netip.Addr, 0, len(ifAddrs))
	for _, raw := range ifAddrs {
		ipNet, ok := raw.(*net.IPNet)
		if !ok {
			continue
		}
		if ipNet == nil {
			continue
		}
		if addr, ok := netip.AddrFromSlice(ipNet.IP); ok {
			out = append(out, addr.Unmap())
		}
	}
	return out, nil
}

func (r *RIBManager) dispatchAggregateUpdate(cmd string) {
	meta := map[string]any{"rib-aggregate": true}
	if r.updateHook != nil {
		_ = r.updateRouteWithMeta("*", cmd, meta)
		return
	}
	r.peerMu.RLock()
	var targets []string
	for owner, up := range r.peerUp {
		if !up {
			continue
		}
		if metaInfo := r.peerMeta[owner]; metaInfo != nil && metaInfo.Address.IsValid() {
			targets = append(targets, metaInfo.Address.String())
		}
	}
	r.peerMu.RUnlock()

	slices.Sort(targets)
	for _, peer := range targets {
		_ = r.updateRouteWithMeta(peer, cmd, meta)
	}
}

func (r *RIBManager) publishAggregateRoute(agg *synthesizedAggregate, action routeaction.Action) {
	if loc := r.locRIB.Load(); loc != nil {
		loc.Insert(agg.Family, agg.Prefix, locrib.Path{
			Source:   bgpProtocolID,
			Instance: bgpLocRIBInstance + 1,
			NextHop:  agg.NextHop,
			IsBGP:    true,
			Metric:   agg.MED,
		})
	}
	entry := bestChangeEntry{
		Action:       action,
		Prefix:       agg.Prefix,
		NextHop:      agg.NextHop,
		Priority:     200,
		Metric:       agg.MED,
		ProtocolType: routeaction.ProtocolIBGP,
	}
	setEntryPath(&entry, agg.ASPath, agg.Attributes)
	publishBestChanges([]bestChangeEntry{entry}, agg.Family)
	r.dispatchAggregateUpdate(formatAggregateAnnounceCommand(agg))
}

func (r *RIBManager) withdrawAggregateRoute(agg *synthesizedAggregate) {
	if agg == nil {
		return
	}
	if loc := r.locRIB.Load(); loc != nil {
		loc.Remove(agg.Family, agg.Prefix, bgpProtocolID, bgpLocRIBInstance+1)
	}
	entry := bestChangeEntry{
		Action:       ribevents.BestChangeWithdraw,
		Prefix:       agg.Prefix,
		NextHop:      agg.NextHop,
		ProtocolType: routeaction.ProtocolIBGP,
	}
	publishBestChanges([]bestChangeEntry{entry}, agg.Family)
	r.dispatchAggregateUpdate(formatAggregateWithdrawCommand(agg))
}
