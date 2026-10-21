// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec text-to-wire encoding
// RFC: rfc/short/rfc5575.md
// Overview: plugin.go — plugin entry points, CLI, families
// Related: plugin_decode.go — wire-to-JSON decoding and formatting
// Related: plugin_protocol.go — stdin/stdout protocol dispatch

package flowspec

import (
	"errors"
	"fmt"
	"net/netip"
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
	errProtocolRequiresValue               = errors.New("protocol requires value")
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
	// not a second component: "port >80 <100 port >443 <500" is one Type 4
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

	switch keyword {
	case flowspec.KwDestinationIPv4, flowspec.KwDestinationIPv6:
		return parsePrefixComponentText(args, flowspec.FlowDestPrefix, fam)
	case flowspec.KwSourceIPv4, flowspec.KwSourceIPv6:
		return parsePrefixComponentText(args, flowspec.FlowSourcePrefix, fam)
	case flowspec.KwProtocol, flowspec.KwNextHeader:
		return parseProtocolComponentText(args[1:])
	case flowspec.KwPort:
		return parseNumericComponentText(args[1:], flowspec.FlowPort)
	case flowspec.KwDestPort:
		return parseNumericComponentText(args[1:], flowspec.FlowDestPort)
	case flowspec.KwSourcePort:
		return parseNumericComponentText(args[1:], flowspec.FlowSourcePort)
	case flowspec.KwICMPType:
		return parseNumericComponentText(args[1:], flowspec.FlowICMPType)
	case flowspec.KwICMPCode:
		return parseNumericComponentText(args[1:], flowspec.FlowICMPCode)
	case flowspec.KwTCPFlags:
		return parseTCPFlagsComponentText(args[1:])
	case flowspec.KwPacketLength:
		return parseNumericComponentText(args[1:], flowspec.FlowPacketLength)
	case flowspec.KwDSCP, flowspec.KwTrafficClass:
		return parseNumericComponentText(args[1:], flowspec.FlowDSCP)
	case flowspec.KwFragment:
		return parseFragmentComponentText(args[1:])
	case flowspec.KwFlowLabel:
		return parseNumericComponentText(args[1:], flowspec.FlowFlowLabel)
	case flowspec.KwRD:
		// EncodeFlowSpecComponents removes the rd pair from a VPN family's arguments
		// before this walk starts, so an rd reaching here names a family that carries
		// no Route Distinguisher.
		return nil, 0, errRdOnlyForVpnFamily
	}

	// Unknown keyword - return error (not silent ignore)
	return nil, 0, fmt.Errorf("unknown component: %s", keyword)
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

// parseProtocolComponentText parses protocol values (names or numbers).
func parseProtocolComponentText(args []string) (flowspec.FlowComponent, int, error) {
	var protocols []uint8
	consumed := 0

	for i := range args {
		token := strings.ToLower(args[i])
		if isComponentKeyword(token) {
			break
		}

		// Accept an optional '=' equality operator so `protocol =6` and
		// `protocol 6` (likewise `=tcp`/`tcp`) parse identically, consistent with
		// the port/numeric components which accept the '=' operator. Protocol is
		// an equality-only match, so '=' is the natural and only operator.
		token = strings.TrimPrefix(token, "=")

		// Try name first
		if num, ok := flowspec.ProtocolNameToNumber[token]; ok {
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
		return nil, 0, errProtocolRequiresValue
	}

	return flowspec.NewFlowIPProtocolComponent(protocols...), consumed + 1, nil
}

// parseNumericComponentText parses numeric component with operators.
func parseNumericComponentText(args []string, compType flowspec.FlowComponentType) (flowspec.FlowComponent, int, error) {
	var matches []flowspec.FlowMatch
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

		matches = append(matches, flowspec.FlowMatch{
			Op:    op,
			Value: value,
			And:   consumed > 0,
		})
		consumed++
	}

	if len(matches) == 0 {
		return nil, 0, fmt.Errorf("%s requires value", compType)
	}

	return flowspec.NewFlowNumericComponent(compType, matches), consumed + 1, nil
}

// parseTCPFlagsComponentText parses TCP flags with bitmask operators.
func parseTCPFlagsComponentText(args []string) (flowspec.FlowComponent, int, error) {
	var matches []flowspec.FlowMatch
	consumed := 0

	for i := range args {
		token := strings.ToLower(args[i])
		if isComponentKeyword(token) {
			break
		}

		// Parse modifiers and flags
		op, flags, hasAndPrefix, err := parseBitmaskValue(token, flowspec.TCPFlagNameToValue)
		if err != nil {
			if consumed == 0 {
				return nil, 0, fmt.Errorf("invalid tcp-flags: %w", err)
			}
			break
		}

		matches = append(matches, flowspec.FlowMatch{
			Op:    op,
			Value: uint64(flags),
			And:   hasAndPrefix, // AND only if explicit & prefix
		})
		consumed++
	}

	if len(matches) == 0 {
		return nil, 0, errTcpFlagsRequiresValue
	}

	return flowspec.NewFlowTCPFlagsMatchComponent(matches), consumed + 1, nil
}

// parseFragmentComponentText parses fragment flags.
func parseFragmentComponentText(args []string) (flowspec.FlowComponent, int, error) {
	var matches []flowspec.FlowMatch
	consumed := 0

	for i := range args {
		token := strings.ToLower(args[i])
		if isComponentKeyword(token) {
			break
		}

		op, flags, hasAndPrefix, err := parseBitmaskValue(token, flowspec.FragmentFlagNameToValue)
		if err != nil {
			if consumed == 0 {
				return nil, 0, fmt.Errorf("invalid fragment: %w", err)
			}
			break
		}

		matches = append(matches, flowspec.FlowMatch{
			Op:    op,
			Value: uint64(flags),
			And:   hasAndPrefix, // AND only if explicit & prefix
		})
		consumed++
	}

	if len(matches) == 0 {
		return nil, 0, errFragmentRequiresValue
	}

	return flowspec.NewFlowFragmentMatchComponent(matches), consumed + 1, nil
}

// isComponentKeyword checks if token is a component keyword.
func isComponentKeyword(token string) bool {
	switch token {
	case flowspec.KwDestinationIPv4, flowspec.KwDestinationIPv6, flowspec.KwSourceIPv4, flowspec.KwSourceIPv6,
		flowspec.KwProtocol, flowspec.KwNextHeader, flowspec.KwPort, flowspec.KwDestPort, flowspec.KwSourcePort,
		flowspec.KwICMPType, flowspec.KwICMPCode, flowspec.KwTCPFlags, flowspec.KwPacketLength, flowspec.KwDSCP, flowspec.KwTrafficClass,
		flowspec.KwFragment, flowspec.KwFlowLabel, flowspec.KwRD:
		return true
	}
	return false
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
		return 63
	default: // 32-bit max for flow-label and other types
		return 0xFFFFFFFF
	}
}

// parseOperatorValue parses "<op><value>" like "=80", ">100", "80".
func parseOperatorValue(token string) (flowspec.FlowOperator, uint64, error) {
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

	value, err := parseUint64(s)
	if err != nil {
		return 0, 0, err
	}

	return op, value, nil
}

// parseBitmaskValue parses bitmask with modifiers like "!syn", "=ack", "syn&ack", "&!is-fragment".
// Returns operator, flags value, whether token had AND prefix, and error.
func parseBitmaskValue(token string, nameToValue map[string]uint8) (flowspec.FlowOperator, uint8, bool, error) {
	var op flowspec.FlowOperator
	s := token

	// Handle leading & (AND connector with previous) - strip it first
	hasAndPrefix := strings.HasPrefix(s, "&")
	s = strings.TrimPrefix(s, "&")

	// Parse modifiers (!, =) that may come after & prefix
	if strings.HasPrefix(s, "!") {
		op |= flowspec.FlowOpNot
		s = s[1:]
	}
	if strings.HasPrefix(s, "=") {
		op |= flowspec.FlowOpMatch
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
