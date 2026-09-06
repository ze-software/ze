// Design: docs/architecture/config/exabgp-syntax.md -- ExaBGP flow route scope
// Overview: migrate_routes.go -- flow route conversion to Ze update blocks

package migration

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The interface-set extended community, octet by octet
// (draft-ietf-idr-flowspec-interfaceset-03 Section 5):
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  Type (0x07)  |      0x02     |    Autonomous System Number   :
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	:     AS Number (cont.)         |O|I|      Group Identifier     |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// Octet 0 carries the type, octet 1 the sub-type, octets 2 to 5 the AS number,
// and octets 6 and 7 the two direction flags above a 14-bit group identifier.
const (
	// interfaceSetType is the transitive type octet. The draft asks IANA for a
	// value and ExaBGP encodes 0x07, which is what an ExaBGP peer reads back
	// (community/extended/flowspec_scope.py, InterfaceSet.COMMUNITY_TYPE).
	interfaceSetType = 0x07
	// interfaceSetNonTransitive is the bit that makes the type non-transitive,
	// the second form the draft defines: "this document proposes to have a
	// transitive as well as a non transitive version of this extended
	// community" (Section 5).
	interfaceSetNonTransitive = 0x40
	// interfaceSetSubType is the sub-type octet, 0x02 in the diagram above.
	interfaceSetSubType = 0x02
	// interfaceSetGroupIDMax is the largest group identifier: "The Group
	// Identifier is coded as a 14-bit number (values goes from 0 to 16383)"
	// (Section 5).
	interfaceSetGroupIDMax = 16383
	// interfaceSetGroupIDBits is the width of that identifier, so the two
	// direction flags sit above it in the same 16-bit field.
	interfaceSetGroupIDBits = 14
)

// interfaceSetKeyword is the one keyword an ExaBGP flow route's scope block
// takes (src/exabgp/configuration/flow/scope.py, ParseFlowScope.schema).
const interfaceSetKeyword = "interface-set"

// flowScopeExtCommunities converts a flow route's scope block into the ze
// extended communities that carry the same interface sets.
//
// The scope block is the third section of an ExaBGP flow route, beside match
// and then, and it holds interface-set values alone. A keyword ExaBGP does not
// declare there is refused rather than dropped: the migrated config would
// otherwise announce a filter with a wider reach than the one it was written
// from.
func flowScopeExtCommunities(routeName string, scope *config.Tree) ([]string, error) {
	var extComms []string

	for _, key := range scope.Values() {
		val, _ := scope.Get(key)
		keyword, value := parseFlowMatchEntry(key, val)
		if keyword != interfaceSetKeyword {
			return nil, fmt.Errorf("flow route %s: scope keyword %q is not %s, the one keyword exabgp declares there",
				routeName, keyword, interfaceSetKeyword)
		}

		// Both `interface-set [ a b ];` and `interface-set a;` reach here, and
		// the freeform parser writes the first with its brackets kept.
		for item := range strings.FieldsSeq(strings.Trim(value, "[] ")) {
			extComm, err := interfaceSetExtCommunity(item)
			if err != nil {
				return nil, fmt.Errorf("flow route %s: %w", routeName, err)
			}
			extComms = append(extComms, extComm)
		}
	}

	return extComms, nil
}

// interfaceSetExtCommunity converts one ExaBGP interface-set value into the
// hexadecimal extended community that carries the same eight octets.
//
// ze has no name for this community. parseOneExtCommunity
// (internal/component/bgp/config/routeattr_community.go) reads target, origin,
// redirect, mup, l2info, the FlowSpec actions, and a raw hexadecimal form, and
// nothing there decodes `interface-set:`. The hexadecimal form is that parser's
// escape hatch for a community it has no keyword for, and it reaches the wire
// as the same eight octets, so the migrated config announces what the ExaBGP
// config announced. Writing `interface-set:...` instead would produce a config
// ze refuses to load.
//
// ExaBGP takes two forms, and so does this function
// (src/exabgp/configuration/flow/parser.py, _interface_set):
//
//	<transitive|non-transitive>:<input|output|input-output>:<asn>:<group-id>
//	<input|output|input-output>:<asn>:<group-id>          (transitive)
func interfaceSetExtCommunity(value string) (string, error) {
	fields := strings.Split(value, ":")

	typeOctet := byte(interfaceSetType)
	if len(fields) == 4 {
		switch fields[0] {
		case "transitive":
		case "non-transitive":
			typeOctet |= interfaceSetNonTransitive
		default:
			return "", fmt.Errorf("interface-set %q: %q is neither transitive nor non-transitive", value, fields[0])
		}
		fields = fields[1:]
	}
	if len(fields) != 3 {
		return "", fmt.Errorf("interface-set %q: expected <transitive|non-transitive>:<direction>:<asn>:<group-id>", value)
	}

	direction, known := interfaceSetDirection(fields[0])
	if !known {
		return "", fmt.Errorf("interface-set %q: %q is not input, output or input-output", value, fields[0])
	}

	// ExaBGP refuses a dotted AS number here, and so does ParseUint, which
	// reports it as the malformed number it is.
	asn, err := strconv.ParseUint(fields[1], 10, 32)
	if err != nil {
		return "", fmt.Errorf("interface-set %q: %q is not a 32-bit AS number", value, fields[1])
	}

	groupID, err := strconv.ParseUint(fields[2], 10, 16)
	if err != nil || groupID > interfaceSetGroupIDMax {
		return "", fmt.Errorf("interface-set %q: group-id %q is not 0 to %d", value, fields[2], interfaceSetGroupIDMax)
	}

	flagsAndGroup := direction<<interfaceSetGroupIDBits | uint16(groupID)

	var octets [8]byte
	octets[0] = typeOctet
	octets[1] = interfaceSetSubType
	octets[2] = byte(asn >> 24)
	octets[3] = byte(asn >> 16)
	octets[4] = byte(asn >> 8)
	octets[5] = byte(asn)
	octets[6] = byte(flagsAndGroup >> 8)
	octets[7] = byte(flagsAndGroup)

	var tb textbuf.Buffer
	return tb.Str("0x").Str(hex.EncodeToString(octets[:])).String(), nil
}

// interfaceSetDirection answers the two direction flags a direction name sets.
//
// The draft gives the field two bits, O then I: "if set, the flow specification
// rule MUST be applied in outbound direction", and "if set, the flow
// specification rule MUST be applied in input direction" (Section 5). The names
// are ExaBGP's (flowspec_scope.py, InterfaceSet.names), and neither ExaBGP nor
// this function can write both flags clear, which the draft calls an error:
// "An interface-set extended community with both flags set to zero MUST be
// treated as an error".
func interfaceSetDirection(name string) (uint16, bool) {
	switch name {
	case "input":
		return 1, true // I
	case "output":
		return 2, true // O
	case "input-output":
		return 3, true // O and I
	}
	return 0, false
}
