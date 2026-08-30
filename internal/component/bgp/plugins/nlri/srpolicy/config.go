// Design: docs/architecture/config/syntax.md -- SR-Policy config route parsing
// RFC: rfc/short/rfc9830.md -- SR-Policy NLRI wire format (SAFI 73)
// RFC: rfc/short/rfc9012.md -- Tunnel Encapsulation attribute
// Related: register.go -- plugin registration
// Related: types.go -- SRPolicy NLRI type

package srpolicy

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/family"
)

const (
	tunnelTypeSRPolicyCP uint16 = 15 // IANA Tunnel Type for SR Policy CP.

	subTLVPreference       uint8 = 12
	subTLVBindingSID       uint8 = 13
	subTLVPriority         uint8 = 15
	subTLVSRv6BindingSID   uint8 = 20
	subTLVSegmentList      uint8 = 128
	subTLVCandidatePathNam uint8 = 129
	subTLVPolicyName       uint8 = 130

	segSubTLVWeight   uint8 = 9
	segSubTLVTypeA    uint8 = 1
	segSubTLVTypeBSID uint8 = 13

	attrCodeTunnelEncap  uint8 = 23
	attrFlagOptTransFlag uint8 = 0xC0 // Optional + Transitive.
)

var errSRPolicyMissingFields = errors.New("sr-policy: requires distinguisher, color, and endpoint")

// parseConfigRoute implements InProcessConfigRouteParser for SR-Policy.
// req.Content is the tokens after the operation keyword (e.g., ["distinguisher","0","color","100",...]).
// SR-Policy carries all of its attributes inside the NLRI content (Tunnel
// Encapsulation), so it ignores the pre-parsed attribute{} fields.
func parseConfigRoute(req registry.ConfigRouteRequest) (registry.PluginRoute, error) {
	content := req.Content
	nextHop := req.NextHop
	isIPv6 := req.IsIPv6
	var (
		distinguisher    uint32
		color            uint32
		hasDistinguisher bool
		hasColor         bool
		endpoint         string
		preference       uint32
		hasPref          bool
		priority         uint8
		hasPriority      bool
		bsidMPLS         uint32
		hasBSID          bool
		bsidNull         bool
		bsidSRv6         netip.Addr
		hasSRv6BSID      bool
		segLists         []srpSegmentList
		policyName       string
		candPathName     string
	)

	i := 0
	for i < len(content) {
		key := content[i]
		switch key {
		case fieldDistinguisher:
			if i+1 >= len(content) {
				return registry.PluginRoute{}, fmt.Errorf("missing value for %s", key)
			}
			v, err := strconv.ParseUint(content[i+1], 10, 32)
			if err != nil {
				return registry.PluginRoute{}, fmt.Errorf("invalid distinguisher: %w", err)
			}
			distinguisher = uint32(v)
			hasDistinguisher = true
			i += 2

		case fieldColor:
			if i+1 >= len(content) {
				return registry.PluginRoute{}, fmt.Errorf("missing value for %s", key)
			}
			v, err := strconv.ParseUint(content[i+1], 10, 32)
			if err != nil {
				return registry.PluginRoute{}, fmt.Errorf("invalid color: %w", err)
			}
			color = uint32(v)
			hasColor = true
			i += 2

		case fieldEndpoint:
			if i+1 >= len(content) {
				return registry.PluginRoute{}, fmt.Errorf("missing value for %s", key)
			}
			endpoint = content[i+1]
			i += 2

		case "preference":
			if i+1 >= len(content) {
				return registry.PluginRoute{}, fmt.Errorf("missing value for %s", key)
			}
			v, err := strconv.ParseUint(content[i+1], 10, 32)
			if err != nil {
				return registry.PluginRoute{}, fmt.Errorf("invalid preference: %w", err)
			}
			preference = uint32(v)
			hasPref = true
			i += 2

		case "priority":
			if i+1 >= len(content) {
				return registry.PluginRoute{}, fmt.Errorf("missing value for %s", key)
			}
			v, err := strconv.ParseUint(content[i+1], 10, 8)
			if err != nil {
				return registry.PluginRoute{}, fmt.Errorf("invalid priority: %w", err)
			}
			priority = uint8(v)
			hasPriority = true
			i += 2

		case "binding-sid":
			if i+1 >= len(content) {
				return registry.PluginRoute{}, fmt.Errorf("missing value for binding-sid")
			}
			switch content[i+1] {
			case "mpls":
				if i+2 >= len(content) {
					return registry.PluginRoute{}, fmt.Errorf("expected 'binding-sid mpls <label>'")
				}
				v, err := strconv.ParseUint(content[i+2], 10, 32)
				if err != nil {
					return registry.PluginRoute{}, fmt.Errorf("invalid binding-sid label: %w", err)
				}
				bsidMPLS = uint32(v)
				hasBSID = true
				i += 3
			case "null":
				bsidNull = true
				i += 2
			default:
				return registry.PluginRoute{}, fmt.Errorf("expected 'binding-sid mpls <label>' or 'binding-sid null'")
			}

		case "srv6-binding-sid":
			if i+1 >= len(content) {
				return registry.PluginRoute{}, fmt.Errorf("missing value for %s", key)
			}
			addr, err := netip.ParseAddr(content[i+1])
			if err != nil {
				return registry.PluginRoute{}, fmt.Errorf("invalid srv6-binding-sid: %w", err)
			}
			bsidSRv6 = addr
			hasSRv6BSID = true
			i += 2

		case "segment-list":
			sl, consumed, err := parseSegmentList(content[i+1:])
			if err != nil {
				return registry.PluginRoute{}, fmt.Errorf("segment-list: %w", err)
			}
			segLists = append(segLists, sl)
			i += 1 + consumed

		case "policy-name":
			if i+1 >= len(content) {
				return registry.PluginRoute{}, fmt.Errorf("missing value for %s", key)
			}
			policyName = content[i+1]
			i += 2

		case "candidate-path-name":
			if i+1 >= len(content) {
				return registry.PluginRoute{}, fmt.Errorf("missing value for %s", key)
			}
			candPathName = content[i+1]
			i += 2

		default:
			return registry.PluginRoute{}, fmt.Errorf("unknown sr-policy keyword: %s", key)
		}
	}

	if !hasDistinguisher || !hasColor || endpoint == "" {
		return registry.PluginRoute{}, errSRPolicyMissingFields
	}

	ep, err := netip.ParseAddr(endpoint)
	if err != nil {
		return registry.PluginRoute{}, fmt.Errorf("invalid endpoint: %w", err)
	}

	afi := family.AFIIPv4
	if isIPv6 {
		afi = family.AFIIPv6
	}
	if err := validateSRPolicyEndpoint(afi, ep); err != nil {
		return registry.PluginRoute{}, err
	}

	nlri := New(afi, distinguisher, color, ep).Bytes()

	tunnelEncapValue := buildTunnelEncap(
		preference, hasPref,
		priority, hasPriority,
		bsidMPLS, hasBSID, bsidNull,
		bsidSRv6, hasSRv6BSID,
		segLists,
		policyName, candPathName,
	)

	var attrs []registry.PluginRouteAttr
	if len(tunnelEncapValue) > 0 {
		attrs = []registry.PluginRouteAttr{{
			Code:  attrCodeTunnelEncap,
			Flags: attrFlagOptTransFlag,
			Value: tunnelEncapValue,
		}}
	}

	return registry.PluginRoute{
		IsIPv6:       isIPv6,
		NLRI:         nlri,
		NextHop:      nextHop,
		Attrs:        attrs,
		MapV4NextHop: true, // multiprotocol next-hop: IPv4-mapped IPv6 for IPv6 family.
	}, nil
}

type srpSegment struct {
	typeA               bool
	mplsLabel           uint32
	typeB               bool
	srv6SID             netip.Addr
	hasEndpointBehavior bool
	endpointBehavior    uint16
	sidStructure        [4]uint8
}

type srpSegmentList struct {
	weight   uint32
	segments []srpSegment
}

func parseSegmentList(tokens []string) (srpSegmentList, int, error) {
	sl := srpSegmentList{}
	i := 0

	if i < len(tokens) && tokens[i] == "weight" {
		if i+1 >= len(tokens) {
			return sl, 0, fmt.Errorf("missing weight value")
		}
		v, err := strconv.ParseUint(tokens[i+1], 10, 32)
		if err != nil {
			return sl, 0, fmt.Errorf("invalid weight: %w", err)
		}
		sl.weight = uint32(v)
		i += 2
	}

	for i < len(tokens) && tokens[i] == "segment" {
		i++
		if i >= len(tokens) {
			return sl, 0, fmt.Errorf("missing segment type")
		}
		segType := tokens[i]
		i++

		var seg srpSegment
		switch segType {
		case "type-a":
			if i+1 >= len(tokens) || tokens[i] != "mpls" {
				return sl, 0, fmt.Errorf("expected 'type-a mpls <label>'")
			}
			label, err := strconv.ParseUint(tokens[i+1], 10, 32)
			if err != nil {
				return sl, 0, fmt.Errorf("invalid MPLS label: %w", err)
			}
			seg.typeA = true
			seg.mplsLabel = uint32(label)
			i += 2

		case "type-b":
			if i+1 >= len(tokens) || tokens[i] != "srv6" {
				return sl, 0, fmt.Errorf("expected 'type-b srv6 <sid>'")
			}
			addr, err := netip.ParseAddr(tokens[i+1])
			if err != nil {
				return sl, 0, fmt.Errorf("invalid SRv6 SID: %w", err)
			}
			seg.typeB = true
			seg.srv6SID = addr
			i += 2

			if i < len(tokens) && tokens[i] == "endpoint-behavior" {
				i++
				if i+4 >= len(tokens) {
					return sl, 0, fmt.Errorf("endpoint-behavior requires 5 values")
				}
				eb, err := strconv.ParseUint(tokens[i], 10, 16)
				if err != nil {
					return sl, 0, fmt.Errorf("invalid endpoint-behavior: %w", err)
				}
				seg.hasEndpointBehavior = true
				seg.endpointBehavior = uint16(eb)
				for j := range 4 {
					v, err := strconv.ParseUint(tokens[i+1+j], 10, 8)
					if err != nil {
						return sl, 0, fmt.Errorf("invalid SID structure field: %w", err)
					}
					seg.sidStructure[j] = uint8(v)
				}
				i += 5
			}

		default:
			return sl, 0, fmt.Errorf("unknown segment type: %s", segType)
		}
		sl.segments = append(sl.segments, seg)
	}

	return sl, i, nil
}

func buildTunnelEncap(
	preference uint32, hasPref bool,
	priority uint8, hasPriority bool,
	bsidMPLS uint32, hasBSID bool, bsidNull bool,
	bsidSRv6 netip.Addr, hasSRv6BSID bool,
	segLists []srpSegmentList,
	policyName, candPathName string,
) []byte {
	var subTLVs []byte

	if hasPref {
		subTLVs = append(subTLVs, buildPreferenceSubTLV(preference)...)
	}
	if hasBSID {
		subTLVs = append(subTLVs, buildBindingSIDSubTLV(bsidMPLS)...)
	} else if bsidNull {
		subTLVs = append(subTLVs, buildBindingSIDNullSubTLV()...)
	}
	if hasSRv6BSID {
		subTLVs = append(subTLVs, buildSRv6BindingSIDSubTLV(bsidSRv6)...)
	}
	if hasPriority {
		subTLVs = append(subTLVs, buildPrioritySubTLV(priority)...)
	}
	for i := range segLists {
		subTLVs = append(subTLVs, buildSegmentListSubTLV(segLists[i])...)
	}
	if policyName != "" {
		subTLVs = append(subTLVs, buildNameSubTLV(subTLVPolicyName, policyName)...)
	}
	if candPathName != "" {
		subTLVs = append(subTLVs, buildNameSubTLV(subTLVCandidatePathNam, candPathName)...)
	}

	tlv := make([]byte, 4+len(subTLVs))
	binary.BigEndian.PutUint16(tlv[0:2], tunnelTypeSRPolicyCP)
	binary.BigEndian.PutUint16(tlv[2:4], uint16(len(subTLVs)))
	copy(tlv[4:], subTLVs)

	return tlv
}

func buildPreferenceSubTLV(pref uint32) []byte {
	buf := make([]byte, 8)
	buf[0] = subTLVPreference
	buf[1] = 6
	binary.BigEndian.PutUint32(buf[4:8], pref)
	return buf
}

func buildBindingSIDSubTLV(label uint32) []byte {
	buf := make([]byte, 8)
	buf[0] = subTLVBindingSID
	buf[1] = 6
	buf[2] = 0x10
	// RFC 9830 §2.4.2: MPLS label stack entry, S bit MUST be zero.
	mplsEntry := label << 4
	buf[4] = byte(mplsEntry >> 16)
	buf[5] = byte(mplsEntry >> 8)
	buf[6] = byte(mplsEntry)
	return buf
}

func buildBindingSIDNullSubTLV() []byte {
	buf := make([]byte, 4)
	buf[0] = subTLVBindingSID
	buf[1] = 2
	return buf
}

func buildPrioritySubTLV(p uint8) []byte {
	buf := make([]byte, 4)
	buf[0] = subTLVPriority
	buf[1] = 2
	buf[2] = p
	return buf
}

func buildSRv6BindingSIDSubTLV(sid netip.Addr) []byte {
	buf := make([]byte, 20)
	buf[0] = subTLVSRv6BindingSID
	buf[1] = 18
	a := sid.As16()
	copy(buf[4:20], a[:])
	return buf
}

func buildSegmentListSubTLV(sl srpSegmentList) []byte {
	payload := make([]byte, 0, 9+8*len(sl.segments))
	payload = append(payload, 0x00)

	wbuf := make([]byte, 8)
	wbuf[0] = segSubTLVWeight
	wbuf[1] = 6
	binary.BigEndian.PutUint32(wbuf[4:8], sl.weight)
	payload = append(payload, wbuf...)

	for i := range sl.segments {
		payload = append(payload, buildSegmentSubSubTLV(sl.segments[i])...)
	}

	header := make([]byte, 3, 3+len(payload))
	header[0] = subTLVSegmentList
	binary.BigEndian.PutUint16(header[1:3], uint16(len(payload)))

	return append(header, payload...)
}

func buildSegmentSubSubTLV(seg srpSegment) []byte {
	if seg.typeA {
		return buildSegmentTypeA(seg.mplsLabel)
	}
	return buildSegmentTypeB(seg)
}

func buildSegmentTypeA(label uint32) []byte {
	buf := make([]byte, 8)
	buf[0] = segSubTLVTypeA
	buf[1] = 6
	// RFC 9830 §2.4.4.2.1: S bit MUST be zero on transmission.
	mplsEntry := label << 4
	buf[4] = byte(mplsEntry >> 16)
	buf[5] = byte(mplsEntry >> 8)
	buf[6] = byte(mplsEntry)
	return buf
}

func buildSegmentTypeB(seg srpSegment) []byte {
	valueLen := 18
	if seg.hasEndpointBehavior {
		valueLen = 26
	}

	buf := make([]byte, 2+valueLen)
	buf[0] = segSubTLVTypeBSID
	buf[1] = byte(valueLen)

	if seg.hasEndpointBehavior {
		buf[2] = 0x10
	}
	a := seg.srv6SID.As16()
	copy(buf[4:20], a[:])

	if seg.hasEndpointBehavior {
		binary.BigEndian.PutUint16(buf[20:22], seg.endpointBehavior)
		// EB flags (2 bytes, reserved)
		// SID structure: LBL, LNL, FL, AL
		buf[24] = seg.sidStructure[0]
		buf[25] = seg.sidStructure[1]
		buf[26] = seg.sidStructure[2]
		buf[27] = seg.sidStructure[3]
	}

	return buf
}

func buildNameSubTLV(stype uint8, name string) []byte {
	nameBytes := []byte(name)
	valueLen := 1 + len(nameBytes)
	buf := make([]byte, 3+valueLen)
	buf[0] = stype
	binary.BigEndian.PutUint16(buf[1:3], uint16(valueLen))
	copy(buf[4:], nameBytes)
	return buf
}
