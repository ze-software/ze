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

	// Binding SID sub-TLV (type 13) value lengths, RFC 9830 Section 2.4.2.
	bindingSIDLenNone uint8 = 2
	bindingSIDLenMPLS uint8 = 6
	bindingSIDLenSRv6 uint8 = 18

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
		bsidSRv6Compat   netip.Addr
		hasBSIDSRv6      bool
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
				if err := checkBindingSIDLabel(v); err != nil {
					return registry.PluginRoute{}, err
				}
				bsidMPLS = uint32(v)
				hasBSID = true
				i += 3
			case "null":
				bsidNull = true
				i += 2
			case "srv6":
				// The backward-compatible type-13 form of an SRv6 BSID. The operator
				// names it explicitly: srv6-binding-sid (type 20) stays the default.
				if i+2 >= len(content) {
					return registry.PluginRoute{}, fmt.Errorf("expected 'binding-sid srv6 <ipv6-address>'")
				}
				sid, err := parseBindingSIDSRv6(content[i+2])
				if err != nil {
					return registry.PluginRoute{}, err
				}
				bsidSRv6Compat = sid
				hasBSIDSRv6 = true
				i += 3
			default:
				return registry.PluginRoute{}, fmt.Errorf("expected 'binding-sid mpls <label>', 'binding-sid srv6 <ipv6-address>' or 'binding-sid null'")
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
		bsidSRv6Compat, hasBSIDSRv6,
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
		MapV4NextHop: false, // RFC 9830 Section 2.1: native next-hop width is independent of NLRI AFI.
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

// MPLS label bounds for SR Policy origination.
const (
	// mplsLabelMax is the largest 20-bit label value. RFC 3032 Section 2.1: "This
	// 20-bit field carries the actual value of the Label.".
	mplsLabelMax = 1<<20 - 1
	// bindingSIDLabelMin is the smallest Binding SID label outside the reserved range.
	bindingSIDLabelMin = 16
)

// checkBindingSIDLabel refuses a configured Binding SID MPLS label that cannot be sent
// as configured. It runs on origination only: RFC 9830 Section 5 forbids a receiver
// from judging these fields ("A BGP implementation MUST NOT perform semantic
// verification of such fields").
func checkBindingSIDLabel(label uint64) error {
	// RFC 9830 Section 2.4.2: "The Label field is validated by the SRPM but MUST NOT
	// contain the reserved MPLS label values (0-15)."
	if label < bindingSIDLabelMin {
		return fmt.Errorf("binding-sid mpls %d: labels 0 to 15 are reserved (RFC 9830 Section 2.4.2); use %d to %d",
			label, bindingSIDLabelMin, mplsLabelMax)
	}
	// RFC 9830 Section 2.4.2: "If the length is 6, then the BSID is encoded in 4 octets
	// using the format below." Figure 6 gives the Label its first 20 bits, so a wider
	// value would be truncated by buildBindingSIDSubTLV.
	if label > mplsLabelMax {
		return fmt.Errorf("binding-sid mpls %d: an MPLS label is 20 bits; use %d to %d",
			label, bindingSIDLabelMin, mplsLabelMax)
	}
	return nil
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
			// RFC 9830 Section 2.4.4.2.1: "Label:  20 bits of label value." A wider value
			// has no encoding and would be truncated by buildSegmentTypeA. The section
			// reserves no value for a Type A segment, so 0 to 15 are accepted.
			if label > mplsLabelMax {
				return sl, 0, fmt.Errorf("type-a mpls %d: an MPLS label is 20 bits; use 0 to %d", label, mplsLabelMax)
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
	bsidSRv6Compat netip.Addr, hasBSIDSRv6 bool,
	bsidSRv6 netip.Addr, hasSRv6BSID bool,
	segLists []srpSegmentList,
	policyName, candPathName string,
) []byte {
	var subTLVs []byte

	if hasPref {
		subTLVs = append(subTLVs, buildPreferenceSubTLV(preference)...)
	}
	// RFC 9830 Section 2.4.2: "it MUST NOT appear more than once in the SR
	// Policy encoding", so one form is written when the config names several.
	if hasBSID {
		subTLVs = append(subTLVs, buildBindingSIDSubTLV(bsidMPLS)...)
	} else if hasBSIDSRv6 {
		subTLVs = append(subTLVs, buildBindingSIDSRv6SubTLV(bsidSRv6Compat)...)
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

// RFC 9830 Section 2.4.2: "The unassigned bits in the Flags field MUST be set to zero upon transmission and MUST be ignored upon receipt."
// The zeroed buffer leaves Flags and RESERVED clear; no S/I behavior is requested.
//
// Byte offsets: 0       1        2       3         4..7
//
//	Type=13 Length=6 Flags=0 RESERVED=0 MPLS label entry
//
// Bits in bytes 4..7: | Label (20) | TC=0 (3) | S=0 (1) | TTL=0 (8) |.
func buildBindingSIDSubTLV(label uint32) []byte {
	buf := make([]byte, 8)
	buf[0] = subTLVBindingSID
	buf[1] = 6
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

// RFC 9830 Section 2.4.2: "The value MUST be 18 when a SRv6 BSID is present,
// 6 when an SR-MPLS BSID is present, or 2 when no BSID is present."
// RFC 9830 Section 2.4.2: "If the length is 18, then the BSID contains a
// 16-octet SRv6 SID."
// The type-13 SRv6 form is the backward-compatible one; srv6-binding-sid
// (type 20, Section 2.4.3) is the RECOMMENDED form and stays the default.
// The zeroed buffer leaves Flags and RESERVED clear; no S/I behavior is requested.
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|   Type=13     |  Length=18    |   Flags=0     |  RESERVED=0   |  octets 0..3
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|              SRv6 Binding SID (16 octets)                     |  octets 4..19
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
func buildBindingSIDSRv6SubTLV(sid netip.Addr) []byte {
	buf := make([]byte, 20)
	buf[0] = subTLVBindingSID
	buf[1] = bindingSIDLenSRv6
	a := sid.As16()
	copy(buf[4:20], a[:])
	return buf
}

// parseBindingSIDSRv6 reads the address of 'binding-sid srv6 <address>'. An
// SRv6 SID is a 128-bit IPv6 address, so an IPv4 or IPv4-mapped address is
// refused rather than written as a 16-octet value nobody configured.
func parseBindingSIDSRv6(token string) (netip.Addr, error) {
	sid, err := netip.ParseAddr(token)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("invalid binding-sid srv6 %q: %w; give an IPv6 address such as 2001:db8::1", token, err)
	}
	if !sid.Is6() {
		return netip.Addr{}, fmt.Errorf("invalid binding-sid srv6 %q: an SRv6 SID is an IPv6 address; give one such as 2001:db8::1", token)
	}
	if sid.Is4In6() {
		return netip.Addr{}, fmt.Errorf("invalid binding-sid srv6 %q: an IPv4-mapped address is not an SRv6 SID; give an IPv6 address such as 2001:db8::1", token)
	}
	if sid.Zone() != "" {
		return netip.Addr{}, fmt.Errorf("invalid binding-sid srv6 %q: an SRv6 SID carries no zone; remove the %%%s suffix", token, sid.Zone())
	}
	return sid, nil
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
