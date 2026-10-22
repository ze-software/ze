// Design: docs/architecture/wire/attributes.md -- path attribute encoding
// RFC: rfc/drafts/draft-ietf-idr-flowspec-interfaceset.txt -- interface-set extended community (Section 5)
// RFC: rfc/short/rfc4761.md -- Layer2 Info extended community (Section 3.2.4)
// Related: extcomm_decoded.go -- the decode half, which renders the names this file reads
// Related: flowspec_action.go -- the colon-less FlowSpec keywords, the other shared table
// Related: builder_parse.go -- ParseSingleExtCommunity, one of the three parsers that consult this table

package attribute

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/asn"
)

// extCommKeywords holds the named extended communities whose text form is
// `<keyword>:<value>`, keyed by the keyword.
//
// Ze has three text parsers for one extended community: the `update text` API
// (route/route_community.go parseExtendedCommunity), the config
// (config/routeattr_community.go parseOneExtCommunity), and the read-back of
// AppendDecoded's output (ParseSingleExtCommunity). Each asks this table
// first, through ParseExtendedCommunityKeyword, so a community added here is
// accepted on every surface and none of the three is edited for it.
var extCommKeywords = map[string]func(value string) (ExtendedCommunity, error){
	interfaceSetKeyword:  parseInterfaceSet,
	layer2InfoKeyword:    parseLayer2Info,
	trafficMarkKeyword:   parseTrafficMark,
	trafficActionKeyword: FlowSpecTrafficAction,
}

// The names AppendDecoded writes and the parsers read.
const (
	interfaceSetKeyword  = "interface-set"
	layer2InfoKeyword    = "l2info"
	trafficMarkKeyword   = "mark"
	trafficActionKeyword = "traffic-action"
)

// parseTrafficMark reads the value of `mark:<dscp>`, the traffic-marking
// community AppendDecoded writes: a decimal DSCP, which FlowSpecTrafficMarking
// bounds.
//
// RFC 8955 Section 7.5: "The traffic marking Extended Community instructs a
// system to modify the DSCP bits in the IP header (Section 3 of [RFC2474]) of
// a transiting IP packet to the corresponding value encoded in the 6 least
// significant bits of the Extended Community value". A value above
// 63 is refused there, never truncated.
func parseTrafficMark(value string) (ExtendedCommunity, error) {
	dscp, err := strconv.ParseUint(value, 10, 8)
	if err != nil {
		return ExtendedCommunity{}, fmt.Errorf("mark %q: the DSCP is a decimal number from 0 to %d", value, FlowSpecDSCPMax)
	}
	return FlowSpecTrafficMarking(dscp)
}

// ParseExtendedCommunityKeyword reads one extended community written as
// `<keyword>:<value>` for a keyword this package registers. matched is false
// when the text names no registered keyword, and the caller then tries its own
// forms; err is set only when the keyword matched and the value is malformed.
func ParseExtendedCommunityKeyword(s string) (ec ExtendedCommunity, matched bool, err error) {
	keyword, value, hasValue := strings.Cut(s, ":")
	if !hasValue {
		return ExtendedCommunity{}, false, nil
	}
	parse, known := extCommKeywords[strings.ToLower(keyword)]
	if !known {
		return ExtendedCommunity{}, false, nil
	}
	ec, err = parse(value)
	return ec, true, err
}

// parseInterfaceSet reads the value of an interface-set extended community,
// in the two forms ExaBGP takes (configuration/grammar/types/flow.py,
// _one_interface_set) and the first of which AppendDecoded writes:
//
//	<transitive|non-transitive>:<input|output|input-output>:<asn>:<group-id>
//	<input|output|input-output>:<asn>:<group-id>          (transitive)
//
// draft-ietf-idr-flowspec-interfaceset Section 5:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  Type (TBD)   |      0x02     |    Autonomous System Number   :
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	:     AS Number (cont.)         |O|I|      Group Identifier     |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// Octet 0 is the type, octet 1 the sub-type, octets 2 to 5 the AS number, and
// octets 6 and 7 the O and I flags above the 14-bit group identifier. The draft
// leaves the type to IANA; Ze writes 0x07, and 0x47 for the non-transitive
// form, as ExaBGP does (InterfaceSet.COMMUNITY_TYPE in flowspec_scope.py), so
// an ExaBGP peer reads back what it was sent.
func parseInterfaceSet(value string) (ExtendedCommunity, error) {
	fields := strings.Split(value, ":")

	// draft-ietf-idr-flowspec-interfaceset Section 5: "this document proposes
	// to have a transitive as well as a non transitive version of this
	// extended community".
	typeSubtype := uint16(extCommInterfaceSet)
	if len(fields) == 4 {
		switch fields[0] {
		case "transitive":
		case "non-transitive":
			typeSubtype = extCommInterfaceSetNT
		default:
			return ExtendedCommunity{}, fmt.Errorf("interface-set %q: %q is neither transitive nor non-transitive", value, fields[0])
		}
		fields = fields[1:]
	}
	if len(fields) != 3 {
		return ExtendedCommunity{}, fmt.Errorf("interface-set %q: expected [transitive|non-transitive:]<input|output|input-output>:<asn>:<group-id>", value)
	}

	direction, err := interfaceSetDirection(fields[0])
	if err != nil {
		return ExtendedCommunity{}, fmt.Errorf("interface-set %q: %w", value, err)
	}

	// The AS number takes every RFC 5396 spelling, as it does everywhere else
	// in Ze; ExaBGP accepts only the plain form, which is a subset of these.
	asNumber, err := asn.Parse(fields[1])
	if err != nil {
		return ExtendedCommunity{}, fmt.Errorf("interface-set %q: %w", value, err)
	}

	// draft-ietf-idr-flowspec-interfaceset Section 5: "The Group Identifier is
	// coded as a 14-bit number (values goes from 0 to 16383)."
	groupID, err := strconv.ParseUint(fields[2], 10, interfaceSetGroupIDBits)
	if err != nil {
		return ExtendedCommunity{}, fmt.Errorf("interface-set %q: group-id %q is not 0 to %d", value, fields[2], 1<<interfaceSetGroupIDBits-1)
	}

	var ec ExtendedCommunity
	binary.BigEndian.PutUint16(ec[0:2], typeSubtype)
	binary.BigEndian.PutUint32(ec[2:6], asNumber)
	binary.BigEndian.PutUint16(ec[6:8], direction<<interfaceSetGroupIDBits|uint16(groupID)) //nolint:gosec // G115: bounded by ParseUint 14-bit
	return ec, nil
}

// interfaceSetDirection answers the two flag bits a direction name sets, the
// index of the name in interfaceSetDirections.
//
// draft-ietf-idr-flowspec-interfaceset Section 5: "An interface-set extended
// community with both flags set to zero MUST be treated as an error", so the
// name for index 0, which only the decoder writes, is refused here.
func interfaceSetDirection(name string) (uint16, error) {
	for flags := 1; flags < len(interfaceSetDirections); flags++ {
		if interfaceSetDirections[flags] == name {
			return uint16(flags), nil //nolint:gosec // G115: below len(interfaceSetDirections), 4
		}
	}
	return 0, fmt.Errorf("direction %q is not input, output or input-output", name)
}

// parseLayer2Info reads the value of a Layer2 Info extended community,
// `<encaps>:<control>:<mtu>:<reserved>`, the four fields AppendDecoded writes.
//
// RFC 4761 Section 3.2.4 Figure 3:
//
//	+------------------------------------+
//	| Extended community type (2 octets) |  0x800A
//	+------------------------------------+
//	|  Encaps Type (1 octet)             |  octet 2
//	+------------------------------------+
//	|  Control Flags (1 octet)           |  octet 3
//	+------------------------------------+
//	|  Layer-2 MTU (2 octet)             |  octets 4-5
//	+------------------------------------+
//	|  Reserved (2 octets)               |  octets 6-7
//	+------------------------------------+
//
// RFC 4761 Section 3.2.4: "The extended community value is to be allocated by
// IANA (currently used value is 0x800A)." The fourth field is written as given:
// draft-ietf-l2vpn-vpls-multihoming carries a preference there, and ExaBGP
// sends `l2info:19:0:1500:111` as 0x006F (qa/api/api-vpls.ci).
func parseLayer2Info(value string) (ExtendedCommunity, error) {
	fields := strings.Split(value, ":")
	if len(fields) != 4 {
		return ExtendedCommunity{}, fmt.Errorf("l2info %q: expected <encaps>:<control>:<mtu>:<reserved>", value)
	}
	encaps, err := strconv.ParseUint(fields[0], 10, 8)
	if err != nil {
		return ExtendedCommunity{}, fmt.Errorf("l2info %q: encaps %q is not 0 to 255", value, fields[0])
	}
	control, err := strconv.ParseUint(fields[1], 10, 8)
	if err != nil {
		return ExtendedCommunity{}, fmt.Errorf("l2info %q: control %q is not 0 to 255", value, fields[1])
	}
	mtu, err := strconv.ParseUint(fields[2], 10, 16)
	if err != nil {
		return ExtendedCommunity{}, fmt.Errorf("l2info %q: mtu %q is not 0 to 65535", value, fields[2])
	}
	reserved, err := strconv.ParseUint(fields[3], 10, 16)
	if err != nil {
		return ExtendedCommunity{}, fmt.Errorf("l2info %q: reserved %q is not 0 to 65535", value, fields[3])
	}

	var ec ExtendedCommunity
	binary.BigEndian.PutUint16(ec[0:2], extCommLayer2Info)
	ec[2] = byte(encaps)                                  //nolint:gosec // G115: bounded by ParseUint 8-bit
	ec[3] = byte(control)                                 //nolint:gosec // G115: bounded by ParseUint 8-bit
	binary.BigEndian.PutUint16(ec[4:6], uint16(mtu))      //nolint:gosec // G115: bounded by ParseUint 16-bit
	binary.BigEndian.PutUint16(ec[6:8], uint16(reserved)) //nolint:gosec // G115: bounded by ParseUint 16-bit
	return ec, nil
}
