// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec text-to-wire encoding
// RFC: rfc/short/rfc5575.md
// Overview: plugin.go — plugin entry points, CLI, families
// Related: plugin_decode.go — wire-to-JSON decoding and formatting
// Related: plugin_protocol.go — stdin/stdout protocol dispatch

package flowspec

import (
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/flowspec"
)

var (
	errFlowspecRequiresAtLeastOneComponent = errors.New("flowspec requires at least one component")
	errRdRequiredForVpnFamily              = errors.New("rd required for VPN family")
	errRdOnlyForVpnFamily                  = errors.New("rd valid only for a VPN family")
	errExpectedComponent                   = errors.New("expected component")
	errIpv4PrefixForIpv6Flowspec           = errors.New("IPv4 prefix for IPv6 flowspec")
	errIpv6PrefixForIpv4Flowspec           = errors.New("IPv6 prefix for IPv4 flowspec")
	errTcpFlagsRequiresValue               = errors.New("tcp-flags requires value")
	errFragmentRequiresValue               = errors.New("fragment requires value")
)

// familyModeBoth declares a family this plugin both sends and receives.
//
// The family names themselves are not declared here. family.MustRegister in
// register.go joins each one from its AFI and SAFI parts, so flowSpecFamilySet in
// plugin.go reads them back from the registry rather than spelling them again.
const familyModeBoth = "both"

// EncodeFlowSpecComponents parses text components and returns wire bytes.
// Format: <component>+ where component is one of:
//   - destination-ipv4 <prefix>, destination-ipv6 <prefix>
//   - source-ipv4 <prefix>, source-ipv6 <prefix>
//   - protocol <num|name>+
//   - port <op><num>+
//   - rd <type:admin:value> (for VPN families)
//   - etc.
//
// This function is used by the engine to delegate FlowSpec text parsing to the plugin.
func EncodeFlowSpecComponents(fam flowspec.Family, args []string) ([]byte, error) {
	isVPN := fam.SAFI == flowspec.SAFIFlowSpecVPN

	var fs *flowspec.FlowSpec
	var fsv *flowspec.FlowSpecVPN
	var rd flowspec.RouteDistinguisher

	// Parse RD first if VPN family
	if isVPN {
		var err error
		rd, args, err = parseRDFromArgs(args)
		if err != nil {
			return nil, err
		}
		fsv = flowspec.NewFlowSpecVPN(fam, rd)
	} else {
		fs = flowspec.NewFlowSpec(fam)
	}

	addComponent := func(c flowspec.FlowComponent) error {
		if fsv != nil {
			return fsv.AddComponent(c)
		}
		return fs.AddComponent(c)
	}

	// Parse components. A repeated keyword is another OR group of one component,
	// not a second component: "port >80&<100 port >443&<500" is one Type 4
	// component whose operator list holds both groups, because RFC 8955 Section 4.2
	// allows each type exactly once. AddComponent performs that join.
	i := 0
	for i < len(args) {
		comp, consumed, err := parseComponentText(args[i:], fam)
		if err != nil {
			return nil, err
		}
		if err := addComponent(comp); err != nil {
			return nil, err
		}
		i += consumed
	}

	// Return wire bytes. A route no packet can match is refused, RFC 8955 Section 4.2.
	if fsv != nil {
		if len(fsv.Components()) == 0 {
			return nil, errFlowspecRequiresAtLeastOneComponent
		}
		if err := fsv.CheckMatchable(); err != nil {
			return nil, err
		}
		return fsv.Bytes(), nil
	}
	if len(fs.Components()) == 0 {
		return nil, errFlowspecRequiresAtLeastOneComponent
	}
	if err := fs.CheckMatchable(); err != nil {
		return nil, err
	}
	return fs.Bytes(), nil
}

// parseRDFromArgs parses "rd <value>" from args and returns the arguments with
// that keyword-value pair removed, so the component walk that follows never meets
// the rd keyword again.
func parseRDFromArgs(args []string) (flowspec.RouteDistinguisher, []string, error) {
	for i := range len(args) - 1 {
		if args[i] != flowspec.KwRD {
			continue
		}
		rd, err := nlri.ParseRDString(args[i+1])
		if err != nil {
			return flowspec.RouteDistinguisher{}, nil, fmt.Errorf("invalid rd: %w", err)
		}
		remaining := make([]string, 0, len(args)-2)
		remaining = append(remaining, args[:i]...)
		remaining = append(remaining, args[i+2:]...)
		return rd, remaining, nil
	}
	return flowspec.RouteDistinguisher{}, nil, errRdRequiredForVpnFamily
}

// parseComponentText parses a single FlowSpec component from args.
// Named differently from parseFlowComponent in the core codec
// (internal/core/bgp/nlri/flowspec/types.go), which parses wire format.
func parseComponentText(args []string, fam flowspec.Family) (flowspec.FlowComponent, int, error) {
	if len(args) == 0 {
		return nil, 0, errExpectedComponent
	}

	keyword := strings.ToLower(args[0])
	if keyword == flowspec.KwRD {
		// EncodeFlowSpecComponents removes the rd pair from a VPN family's arguments
		// before this walk starts, so an rd reaching here names a family that carries
		// no Route Distinguisher.
		return nil, 0, errRdOnlyForVpnFamily
	}

	// The core vocabulary states which keyword encodes which type; this walk
	// only chooses the value parser for the type (flowspec.KeywordComponentType).
	componentType, ok := flowspec.KeywordComponentType(keyword)
	if !ok {
		// Unknown keyword - return error (not silent ignore)
		return nil, 0, fmt.Errorf("unknown component: %s", keyword)
	}

	// Class C: KeywordComponentType answers only the named types below.
	switch componentType {
	case flowspec.FlowDestPrefix, flowspec.FlowSourcePrefix:
		return parsePrefixComponentText(args, componentType, fam)
	case flowspec.FlowTCPFlags:
		return parseTCPFlagsComponentText(args[1:])
	case flowspec.FlowFragment:
		return parseFragmentComponentText(args[1:])
	case flowspec.FlowIPProtocol, flowspec.FlowPort, flowspec.FlowDestPort, flowspec.FlowSourcePort,
		flowspec.FlowICMPType, flowspec.FlowICMPCode, flowspec.FlowPacketLength,
		flowspec.FlowDSCP, flowspec.FlowFlowLabel:
		return parseNumericComponentText(args[1:], componentType)
	default:
		panic("BUG: flowspec.KeywordComponentType answered a type no text parser handles")
	}
}

// parsePrefixComponentText parses destination or source prefix from text.
func parsePrefixComponentText(args []string, compType flowspec.FlowComponentType, fam flowspec.Family) (flowspec.FlowComponent, int, error) {
	if len(args) < 2 {
		return nil, 0, fmt.Errorf("%s requires prefix", args[0])
	}

	prefix, err := netip.ParsePrefix(args[1])
	if err != nil {
		return nil, 0, fmt.Errorf("invalid prefix: %w", err)
	}

	// Validate AFI match
	if prefix.Addr().Is4() && fam.AFI != flowspec.AFIIPv4 {
		return nil, 0, errIpv4PrefixForIpv6Flowspec
	}
	if prefix.Addr().Is6() && fam.AFI != flowspec.AFIIPv6 {
		return nil, 0, errIpv6PrefixForIpv4Flowspec
	}

	if compType == flowspec.FlowDestPrefix {
		return flowspec.NewFlowDestPrefixComponent(prefix), 2, nil
	}
	return flowspec.NewFlowSourcePrefixComponent(prefix), 2, nil
}

// parseNumericComponentText parses numeric component with operators.
//
// Successive values are alternatives, and "&" joins a term to the one before
// it: `destination-port >8080&<8088 =3128` (or `>8080 &<8088 =3128`) is
// (>8080 AND <8088) OR =3128. RFC 8955 Section 4.2.1.1: "a - and bit. If unset,
// the result of the previous {operator, value} pair is ORed with the current
// one. If set, it is ANDed." Plain successive terms therefore leave the bit
// clear, the spelling of Ze's config, of the bitmask components and of ExaBGP
// 6.0, whose list text the bridge passes through unchanged.
//
// Protocol (type 3) is a numeric component like the others, RFC 8955 Section
// 4.2.2.3: "Encoding: <type (1 octet), [numeric_op, value]+>". Its value may
// also be a protocol name (`>=tcp`), resolved through
// flowspec.ProtocolNameToNumber.
func parseNumericComponentText(args []string, compType flowspec.FlowComponentType) (flowspec.FlowComponent, int, error) {
	var matches []flowspec.FlowMatch
	consumed := 0

	maxValue := componentMaxValue(compType)
	names := componentValueNames(compType)

	for i := range args {
		token := args[i]
		if isComponentKeyword(strings.ToLower(token)) {
			break
		}

		// A token that is not a keyword is a value of this component, so a
		// malformed one is refused here, by this component's name.
		terms, err := parseNumericTermsText(token, len(matches) > 0, maxValue, names)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid %s value: %w", compType, err)
		}
		matches = append(matches, terms...)
		consumed++
	}

	if len(matches) == 0 {
		return nil, 0, fmt.Errorf("%s requires value", compType)
	}

	return flowspec.NewFlowNumericComponent(compType, matches), consumed + 1, nil
}

// parseNumericTermsText parses one token of a numeric component: one or more
// `<op><value>` terms joined by "&", optionally led by "&" to join the first
// term to the previous token's last one. hasPrevious says whether a term comes
// before this token; a leading "&" with nothing before it is refused, as is an
// "&" with no term after it. names, when not nil, maps a value name to its
// number.
func parseNumericTermsText(token string, hasPrevious bool, maxValue uint64, names map[string]uint8) ([]flowspec.FlowMatch, error) {
	joined := strings.HasPrefix(token, "&")
	if joined && !hasPrevious {
		return nil, fmt.Errorf("%q starts with &, which joins a term to the one before it, and no term comes before it: drop the &", token)
	}
	body := strings.TrimPrefix(token, "&")

	var terms []flowspec.FlowMatch
	for part := range strings.SplitSeq(body, "&") {
		if part == "" {
			return nil, fmt.Errorf("%q holds an & with no term after it: write <op><value> after each &, as in >8080&<8088", token)
		}
		op, value, err := parseOperatorValue(part, names)
		if err != nil {
			return nil, err
		}
		if value > maxValue {
			return nil, fmt.Errorf("value %d exceeds max %d: write a value from 0 to %d", value, maxValue, maxValue)
		}
		// RFC 8955 Section 4.2.1.1: the a bit ANDs this term with the previous one.
		terms = append(terms, flowspec.FlowMatch{Op: op, Value: value, And: joined})
		joined = true
	}
	return terms, nil
}

// parseTCPFlagsComponentText parses the values of a tcp-flags component with
// the bitmask grammar (parseBitmaskTermsText).
func parseTCPFlagsComponentText(args []string) (flowspec.FlowComponent, int, error) {
	matches, consumed, err := parseBitmaskComponentText(args, flowspec.KwTCPFlags, flowspec.TCPFlagNameToValue)
	if err != nil {
		return nil, 0, err
	}
	if len(matches) == 0 {
		return nil, 0, errTcpFlagsRequiresValue
	}
	return flowspec.NewFlowTCPFlagsMatchComponent(matches), consumed + 1, nil
}

// parseFragmentComponentText parses the values of a fragment component with
// the bitmask grammar (parseBitmaskTermsText).
func parseFragmentComponentText(args []string) (flowspec.FlowComponent, int, error) {
	matches, consumed, err := parseBitmaskComponentText(args, flowspec.KwFragment, flowspec.FragmentFlagNameToValue)
	if err != nil {
		return nil, 0, err
	}
	if len(matches) == 0 {
		return nil, 0, errFragmentRequiresValue
	}
	return flowspec.NewFlowFragmentMatchComponent(matches), consumed + 1, nil
}

// parseBitmaskComponentText reads the tokens of one bitmask component up to the
// next component keyword. A token that is not a keyword is a value of this
// component, so a malformed one is refused here, by this component's name.
func parseBitmaskComponentText(args []string, kw string, names map[string]uint8) ([]flowspec.FlowMatch, int, error) {
	var matches []flowspec.FlowMatch
	consumed := 0
	for _, arg := range args {
		token := strings.ToLower(arg)
		if isComponentKeyword(token) {
			break
		}
		terms, err := parseBitmaskTermsText(token, len(matches) > 0, names)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid %s value: %w", kw, err)
		}
		matches = append(matches, terms...)
		consumed++
	}
	return matches, consumed, nil
}

// isComponentKeyword checks if token is a component keyword.
func isComponentKeyword(token string) bool {
	if token == flowspec.KwRD {
		return true
	}
	_, ok := flowspec.KeywordComponentType(token)
	return ok
}

// componentMaxValue returns max valid value for component type.
func componentMaxValue(compType flowspec.FlowComponentType) uint64 {
	//exhaustive:ignore // This numeric-width projection specializes bounded criteria; others keep the 32-bit limit.
	switch compType {
	case flowspec.FlowIPProtocol, flowspec.FlowICMPType, flowspec.FlowICMPCode:
		return 255
	case flowspec.FlowPort, flowspec.FlowDestPort, flowspec.FlowSourcePort, flowspec.FlowPacketLength:
		return 65535
	case flowspec.FlowDSCP:
		// RFC 8955 Section 4.2.2.11: "The six least significant bits contain
		// the DSCP value."
		return 63
	case flowspec.FlowFlowLabel:
		// RFC 8956 Section 3.7: "match the 20-bit Flow Label IPv6 header field".
		return 0xFFFFF
	default: // 32-bit max for any other numeric type
		return 0xFFFFFFFF
	}
}

// componentValueNames answers the value names a numeric component accepts in
// place of a number, or nil when it accepts numbers only.
func componentValueNames(compType flowspec.FlowComponentType) map[string]uint8 {
	//exhaustive:ignore // Only protocol and the ICMP components have named values; every other numeric component takes numbers.
	switch compType {
	case flowspec.FlowIPProtocol:
		return flowspec.ProtocolNameToNumber
	case flowspec.FlowICMPType:
		return icmpTypeNames
	case flowspec.FlowICMPCode:
		return icmpCodeNames
	default:
		return nil
	}
}

// parseOperatorValue parses "<op><value>" like "=80", ">100", "80", or with
// names, ">=tcp".
func parseOperatorValue(token string, names map[string]uint8) (flowspec.FlowOperator, uint64, error) {
	op := flowspec.FlowOpEqual
	s := token

	// Parse operator prefix
	switch {
	case strings.HasPrefix(s, ">="):
		op = flowspec.FlowOpGreater | flowspec.FlowOpEqual
		s = s[2:]
	case strings.HasPrefix(s, "<="):
		op = flowspec.FlowOpLess | flowspec.FlowOpEqual
		s = s[2:]
	case strings.HasPrefix(s, "!="):
		op = flowspec.FlowOpNotEq
		s = s[2:]
	case strings.HasPrefix(s, ">"):
		op = flowspec.FlowOpGreater
		s = s[1:]
	case strings.HasPrefix(s, "<"):
		op = flowspec.FlowOpLess
		s = s[1:]
	case strings.HasPrefix(s, "="):
		op = flowspec.FlowOpEqual
		s = s[1:]
	}

	if num, ok := names[strings.ToLower(s)]; ok {
		return op, uint64(num), nil
	}
	value, err := parseUint64(s)
	if err != nil {
		return 0, 0, operatorValueError(token, names)
	}

	return op, value, nil
}

// operatorValueError refuses a term whose value is not a number and, where the
// component has value names, not one of them. The fix names what may follow the
// operator, the names read from the component's own table.
func operatorValueError(token string, names map[string]uint8) error {
	if names == nil {
		return fmt.Errorf("%q is not <op><value>: write a number after the operator, as in >1024", token)
	}
	known := slices.Sorted(maps.Keys(names))
	return fmt.Errorf("%q is not <op><value>: write a number or a name (%s) after the operator, as in =%s", token, strings.Join(known, ", "), known[0])
}

// parseBitmaskTermsText parses one token of a bitmask component (tcp-flags,
// fragment), the grammar config and update text share and ExaBGP 6.0 reads:
//
//   - "&" joins a term to the one before it with the AND bit, and a leading
//     "&" joins the token's first term to the previous token's last one:
//     `syn&ack` is SYN set AND ACK set.
//   - successive tokens are alternatives, ORed: `syn ack` is SYN set OR ACK set.
//   - "+" names several bits of ONE operand: `syn+ack` is one term whose mask
//     holds both bits, true when either is set, or both with `=`.
//   - "=", "!" and "!=" lead a term and set its match bit, its NOT bit, or both.
//   - an operand is a flag name from names or a number from 0 to 255.
//
// hasPrevious says whether a term comes before this token. A leading "&" with
// nothing before it is refused, as is an "&" or a "+" with nothing after it.
//
// RFC 8955 Section 4.2.1.2: "not (NOT bit): If set, logical negation of
// operation." and "m (Match bit): If set, this is a bitwise match operation
// defined as "(data AND value) == value"; if unset, (data AND value) evaluates
// to TRUE if any of the bits in the value mask are set in the data.". The AND
// bit is the numeric operator's, RFC 8955 Section 4.2.1.1: "a - and bit. If
// unset, the result of the previous {operator, value} pair is ORed with the
// current one. If set, it is ANDed.".
func parseBitmaskTermsText(token string, hasPrevious bool, names map[string]uint8) ([]flowspec.FlowMatch, error) {
	joined := strings.HasPrefix(token, "&")
	if joined && !hasPrevious {
		return nil, fmt.Errorf("%q starts with &, which joins a term to the one before it, and no term comes before it: drop the &", token)
	}
	body := strings.TrimPrefix(token, "&")

	var terms []flowspec.FlowMatch
	for part := range strings.SplitSeq(body, "&") {
		if part == "" {
			return nil, fmt.Errorf("%q holds an & with no term after it: write a flag after each &, as in syn&ack", token)
		}
		op, value, err := parseBitmaskOperand(part, names)
		if err != nil {
			return nil, err
		}
		// RFC 8955 Section 4.2.1.1: the a bit ANDs this term with the previous one.
		terms = append(terms, flowspec.FlowMatch{Op: op, Value: uint64(value), And: joined})
		joined = true
	}
	return terms, nil
}

// parseBitmaskOperand parses one bitmask term, `[!|=|!=]<flag>[+<flag>...]`,
// into its operator bits and its mask.
func parseBitmaskOperand(part string, names map[string]uint8) (flowspec.FlowOperator, uint8, error) {
	var op flowspec.FlowOperator
	s := part
	if strings.HasPrefix(s, "!") {
		op |= flowspec.FlowOpNot
		s = s[1:]
	}
	if strings.HasPrefix(s, "=") {
		op |= flowspec.FlowOpMatch
		s = s[1:]
	}
	known := strings.Join(slices.Sorted(maps.Keys(names)), ", ")
	if s == "" {
		return 0, 0, fmt.Errorf("%q has no flag after its operator: write one of %s, or a number from 0 to 255", part, known)
	}

	var flags uint8
	for name := range strings.SplitSeq(s, "+") {
		if val, ok := names[name]; ok {
			flags |= val
			continue
		}
		if name == "" {
			return 0, 0, fmt.Errorf("%q holds a + with no flag after it: write a flag after each +, as in syn+ack", part)
		}
		num, err := parseUint8(name)
		if err != nil {
			return 0, 0, fmt.Errorf("%q is not a flag: write one of %s, or a number from 0 to 255", name, known)
		}
		flags |= num
	}
	return op, flags, nil
}

func parseUint8(s string) (uint8, error) {
	v, err := parseUint64(s)
	if err != nil {
		return 0, err
	}
	if v > 255 {
		return 0, fmt.Errorf("value %d exceeds uint8", v)
	}
	return uint8(v), nil //nolint:gosec // bounds checked
}

func parseUint64(s string) (uint64, error) {
	// Handle hex
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return strconv.ParseUint(s[2:], 16, 64)
	}
	return strconv.ParseUint(s, 10, 64)
}
