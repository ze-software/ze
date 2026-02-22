// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec text-to-wire encoding
// Design: rfc/short/rfc5575.md
// Related: plugin.go — plugin entry points, CLI, families
// Related: plugin_decode.go — wire-to-JSON decoding and formatting
// Related: plugin_protocol.go — stdin/stdout protocol dispatch

package bgp_nlri_flowspec

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
)

// FlowSpec component keywords.
const (
	kwDestination     = "destination"      // Type 1 (IPv4)
	kwDestinationIPv6 = "destination-ipv6" // Type 1 (IPv6)
	kwSource          = "source"           // Type 2 (IPv4)
	kwSourceIPv6      = "source-ipv6"      // Type 2 (IPv6)
	kwProtocol        = "protocol"         // Type 3 (IPv4)
	kwNextHeader      = "next-header"      // Type 3 (IPv6)
	kwPort            = "port"             // Type 4
	kwDestPort        = "destination-port" // Type 5
	kwSourcePort      = "source-port"      // Type 6
	kwICMPType        = "icmp-type"        // Type 7
	kwICMPCode        = "icmp-code"        // Type 8
	kwTCPFlags        = "tcp-flags"        // Type 9
	kwPacketLength    = "packet-length"    // Type 10
	kwDSCP            = "dscp"             // Type 11
	kwFragment        = "fragment"         // Type 12
	kwFlowLabel       = "flow-label"       // Type 13 (IPv6 only)
	kwRD              = "rd"               // Route Distinguisher (VPN)
)

// protocolNameToNumber maps protocol names to numbers.
// IANA Protocol Numbers: https://www.iana.org/assignments/protocol-numbers
var protocolNameToNumber = map[string]uint8{
	"icmp":   1,
	"igmp":   2,
	"tcp":    6,
	"udp":    17,
	"gre":    47,
	"icmpv6": 58,
	"ospf":   89,
	"sctp":   132,
}

// tcpFlagNameToValue maps TCP flag names to values.
// RFC 8955 Section 4.2.2.9.
var tcpFlagNameToValue = map[string]uint8{
	"fin":  0x01,
	"syn":  0x02,
	"rst":  0x04,
	"psh":  0x08,
	"push": 0x08, // alias for psh
	"ack":  0x10,
	"urg":  0x20,
	"ece":  0x40,
	"cwr":  0x80,
}

// fragmentFlagNameToValue maps fragment flag names to values.
// RFC 8955 Section 4.2.2.12.
var fragmentFlagNameToValue = map[string]uint8{
	"dont-fragment":  0x01,
	"is-fragment":    0x02,
	"first-fragment": 0x04,
	"last-fragment":  0x08,
	"df":             0x01, // alias
	"isf":            0x02, // alias
	"ff":             0x04, // alias
	"lf":             0x08, // alias
}

// EncodeFlowSpecComponents parses text components and returns wire bytes.
// Format: <component>+ where component is one of:
//   - destination <prefix>
//   - source <prefix>
//   - protocol <num|name>+
//   - port <op><num>+
//   - rd <type:admin:value> (for VPN families)
//   - etc.
//
// This function is used by the engine to delegate FlowSpec text parsing to the plugin.
func EncodeFlowSpecComponents(family Family, args []string) ([]byte, error) {
	isVPN := family.SAFI == SAFIFlowSpecVPN

	var fs *FlowSpec
	var fsv *FlowSpecVPN
	var rd RouteDistinguisher

	// Parse RD first if VPN family
	if isVPN {
		var consumed int
		var err error
		rd, consumed, err = parseRDFromArgs(args)
		if err != nil {
			return nil, err
		}
		args = args[consumed:]
		fsv = NewFlowSpecVPN(family, rd)
	} else {
		fs = NewFlowSpec(family)
	}

	addComponent := func(c FlowComponent) {
		if fsv != nil {
			fsv.AddComponent(c)
		} else {
			fs.AddComponent(c)
		}
	}

	// Parse components
	i := 0
	for i < len(args) {
		comp, consumed, err := parseComponentText(args[i:], family)
		if err != nil {
			return nil, err
		}
		addComponent(comp)
		i += consumed
	}

	// Return wire bytes
	if fsv != nil {
		if len(fsv.Components()) == 0 {
			return nil, fmt.Errorf("flowspec requires at least one component")
		}
		return fsv.Bytes(), nil
	}
	if len(fs.Components()) == 0 {
		return nil, fmt.Errorf("flowspec requires at least one component")
	}
	return fs.Bytes(), nil
}

// parseRDFromArgs parses "rd <value>" from args.
func parseRDFromArgs(args []string) (RouteDistinguisher, int, error) {
	for i := range len(args) - 1 {
		if args[i] == kwRD {
			rd, err := nlri.ParseRDString(args[i+1])
			if err != nil {
				return RouteDistinguisher{}, 0, fmt.Errorf("invalid rd: %w", err)
			}
			return rd, 2, nil
		}
	}
	return RouteDistinguisher{}, 0, fmt.Errorf("rd required for VPN family")
}

// parseComponentText parses a single FlowSpec component from args.
// Named differently from parseFlowComponent in types.go (which parses wire format).
func parseComponentText(args []string, family Family) (FlowComponent, int, error) {
	if len(args) == 0 {
		return nil, 0, fmt.Errorf("expected component")
	}

	keyword := strings.ToLower(args[0])

	switch keyword {
	case kwDestination, kwDestinationIPv6:
		return parsePrefixComponentText(args, FlowDestPrefix, family)
	case kwSource, kwSourceIPv6:
		return parsePrefixComponentText(args, FlowSourcePrefix, family)
	case kwProtocol, kwNextHeader:
		return parseProtocolComponentText(args[1:])
	case kwPort:
		return parseNumericComponentText(args[1:], FlowPort)
	case kwDestPort:
		return parseNumericComponentText(args[1:], FlowDestPort)
	case kwSourcePort:
		return parseNumericComponentText(args[1:], FlowSourcePort)
	case kwICMPType:
		return parseNumericComponentText(args[1:], FlowICMPType)
	case kwICMPCode:
		return parseNumericComponentText(args[1:], FlowICMPCode)
	case kwTCPFlags:
		return parseTCPFlagsComponentText(args[1:])
	case kwPacketLength:
		return parseNumericComponentText(args[1:], FlowPacketLength)
	case kwDSCP:
		return parseNumericComponentText(args[1:], FlowDSCP)
	case kwFragment:
		return parseFragmentComponentText(args[1:])
	case kwFlowLabel:
		return parseNumericComponentText(args[1:], FlowFlowLabel)
	case kwRD:
		// Skip rd - already parsed
		return nil, 2, nil
	}

	// Unknown keyword - return error (not silent ignore)
	return nil, 0, fmt.Errorf("unknown component: %s", keyword)
}

// parsePrefixComponentText parses destination or source prefix from text.
func parsePrefixComponentText(args []string, compType FlowComponentType, family Family) (FlowComponent, int, error) {
	if len(args) < 2 {
		return nil, 0, fmt.Errorf("%s requires prefix", args[0])
	}

	prefix, err := netip.ParsePrefix(args[1])
	if err != nil {
		return nil, 0, fmt.Errorf("invalid prefix: %w", err)
	}

	// Validate AFI match
	if prefix.Addr().Is4() && family.AFI != AFIIPv4 {
		return nil, 0, fmt.Errorf("IPv4 prefix for IPv6 flowspec")
	}
	if prefix.Addr().Is6() && family.AFI != AFIIPv6 {
		return nil, 0, fmt.Errorf("IPv6 prefix for IPv4 flowspec")
	}

	if compType == FlowDestPrefix {
		return NewFlowDestPrefixComponent(prefix), 2, nil
	}
	return NewFlowSourcePrefixComponent(prefix), 2, nil
}

// parseProtocolComponentText parses protocol values (names or numbers).
func parseProtocolComponentText(args []string) (FlowComponent, int, error) {
	var protocols []uint8
	consumed := 0

	for i := range args {
		token := strings.ToLower(args[i])
		if isComponentKeyword(token) {
			break
		}

		// Try name first
		if num, ok := protocolNameToNumber[token]; ok {
			protocols = append(protocols, num)
			consumed++
			continue
		}

		// Try number
		num, err := parseUint8(token)
		if err != nil {
			if consumed == 0 {
				return nil, 0, fmt.Errorf("invalid protocol: %s", token)
			}
			break
		}
		protocols = append(protocols, num)
		consumed++
	}

	if len(protocols) == 0 {
		return nil, 0, fmt.Errorf("protocol requires value")
	}

	return NewFlowIPProtocolComponent(protocols...), consumed + 1, nil
}

// parseNumericComponentText parses numeric component with operators.
func parseNumericComponentText(args []string, compType FlowComponentType) (FlowComponent, int, error) {
	var matches []FlowMatch
	consumed := 0

	maxValue := componentMaxValue(compType)

	for i := range args {
		token := args[i]
		if isComponentKeyword(strings.ToLower(token)) {
			break
		}

		op, value, err := parseOperatorValue(token)
		if err != nil {
			if consumed == 0 {
				return nil, 0, fmt.Errorf("invalid %s value: %w", compType, err)
			}
			break
		}

		if value > maxValue {
			return nil, 0, fmt.Errorf("%s value %d exceeds max %d", compType, value, maxValue)
		}

		matches = append(matches, FlowMatch{
			Op:    op,
			Value: value,
			And:   consumed > 0,
		})
		consumed++
	}

	if len(matches) == 0 {
		return nil, 0, fmt.Errorf("%s requires value", compType)
	}

	return NewFlowNumericComponent(compType, matches), consumed + 1, nil
}

// parseTCPFlagsComponentText parses TCP flags with bitmask operators.
func parseTCPFlagsComponentText(args []string) (FlowComponent, int, error) {
	var matches []FlowMatch
	consumed := 0

	for i := range args {
		token := strings.ToLower(args[i])
		if isComponentKeyword(token) {
			break
		}

		// Parse modifiers and flags
		op, flags, hasAndPrefix, err := parseBitmaskValue(token, tcpFlagNameToValue)
		if err != nil {
			if consumed == 0 {
				return nil, 0, fmt.Errorf("invalid tcp-flags: %w", err)
			}
			break
		}

		matches = append(matches, FlowMatch{
			Op:    op,
			Value: uint64(flags),
			And:   hasAndPrefix, // AND only if explicit & prefix
		})
		consumed++
	}

	if len(matches) == 0 {
		return nil, 0, fmt.Errorf("tcp-flags requires value")
	}

	return NewFlowTCPFlagsMatchComponent(matches), consumed + 1, nil
}

// parseFragmentComponentText parses fragment flags.
func parseFragmentComponentText(args []string) (FlowComponent, int, error) {
	var matches []FlowMatch
	consumed := 0

	for i := range args {
		token := strings.ToLower(args[i])
		if isComponentKeyword(token) {
			break
		}

		op, flags, hasAndPrefix, err := parseBitmaskValue(token, fragmentFlagNameToValue)
		if err != nil {
			if consumed == 0 {
				return nil, 0, fmt.Errorf("invalid fragment: %w", err)
			}
			break
		}

		matches = append(matches, FlowMatch{
			Op:    op,
			Value: uint64(flags),
			And:   hasAndPrefix, // AND only if explicit & prefix
		})
		consumed++
	}

	if len(matches) == 0 {
		return nil, 0, fmt.Errorf("fragment requires value")
	}

	return NewFlowFragmentMatchComponent(matches), consumed + 1, nil
}

// isComponentKeyword checks if token is a component keyword.
func isComponentKeyword(token string) bool {
	switch token {
	case kwDestination, kwDestinationIPv6, kwSource, kwSourceIPv6,
		kwProtocol, kwNextHeader, kwPort, kwDestPort, kwSourcePort,
		kwICMPType, kwICMPCode, kwTCPFlags, kwPacketLength, kwDSCP,
		kwFragment, kwFlowLabel, kwRD:
		return true
	}
	return false
}

// componentMaxValue returns max valid value for component type.
func componentMaxValue(compType FlowComponentType) uint64 {
	switch compType { //nolint:exhaustive // Only numeric types
	case FlowIPProtocol, FlowICMPType, FlowICMPCode:
		return 255
	case FlowPort, FlowDestPort, FlowSourcePort, FlowPacketLength:
		return 65535
	case FlowDSCP:
		return 63
	default: // 32-bit max for flow-label and other types
		return 0xFFFFFFFF
	}
}

// parseOperatorValue parses "<op><value>" like "=80", ">100", "80".
func parseOperatorValue(token string) (FlowOperator, uint64, error) {
	op := FlowOpEqual
	s := token

	// Parse operator prefix
	switch {
	case strings.HasPrefix(s, ">="):
		op = FlowOpGreater | FlowOpEqual
		s = s[2:]
	case strings.HasPrefix(s, "<="):
		op = FlowOpLess | FlowOpEqual
		s = s[2:]
	case strings.HasPrefix(s, "!="):
		op = FlowOpNotEq
		s = s[2:]
	case strings.HasPrefix(s, ">"):
		op = FlowOpGreater
		s = s[1:]
	case strings.HasPrefix(s, "<"):
		op = FlowOpLess
		s = s[1:]
	case strings.HasPrefix(s, "="):
		op = FlowOpEqual
		s = s[1:]
	}

	value, err := parseUint64(s)
	if err != nil {
		return 0, 0, err
	}

	return op, value, nil
}

// parseBitmaskValue parses bitmask with modifiers like "!syn", "=ack", "syn&ack", "&!is-fragment".
// Returns operator, flags value, whether token had AND prefix, and error.
func parseBitmaskValue(token string, nameToValue map[string]uint8) (FlowOperator, uint8, bool, error) {
	var op FlowOperator
	s := token

	// Handle leading & (AND connector with previous) - strip it first
	hasAndPrefix := strings.HasPrefix(s, "&")
	s = strings.TrimPrefix(s, "&")

	// Parse modifiers (!, =) that may come after & prefix
	if strings.HasPrefix(s, "!") {
		op |= FlowOpNot
		s = s[1:]
	}
	if strings.HasPrefix(s, "=") {
		op |= FlowOpMatch
		s = s[1:]
	}

	// Parse flag names (may be combined with &)
	var flags uint8
	for part := range strings.SplitSeq(s, "&") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if val, ok := nameToValue[part]; ok {
			flags |= val
		} else {
			// Try numeric
			num, err := parseUint8(part)
			if err != nil {
				return 0, 0, false, fmt.Errorf("unknown flag: %s", part)
			}
			flags |= num
		}
	}

	return op, flags, hasAndPrefix, nil
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
