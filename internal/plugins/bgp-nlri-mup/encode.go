// Design: docs/architecture/wire/nlri.md — MUP NLRI wire encoding from route commands

package bgp_nlri_mup

import (
	"encoding/hex"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/route"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"
)

// EncodeNLRIHex encodes MUP NLRI from CLI-style args and returns uppercase hex.
// Args format: ["route-type", "mup-isd", "rd", "100:100", "prefix", "10.0.0.0/24"]
// For T1ST: ["route-type", "mup-t1st", "rd", "...", "prefix", "...", "teid", "12345", "qfi", "1", "endpoint", "1.2.3.4"]
// For T2ST: ["route-type", "mup-t2st", "rd", "...", "address", "1.2.3.4", "teid", "1234/32"]
// The "ipv6" arg controls AFI selection: ["ipv6", "true"] means AFI=IPv6.
// This implements the InProcessNLRIEncoder signature for the plugin registry.
func EncodeNLRIHex(family string, args []string) (string, error) {
	isIPv6 := strings.HasPrefix(family, "ipv6/")

	// Parse CLI-style args into MUPRouteSpec fields
	spec := bgptypes.MUPRouteSpec{IsIPv6: isIPv6}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "route-type":
			i++
			if i >= len(args) {
				return "", fmt.Errorf("route-type requires value")
			}
			spec.RouteType = args[i]
		case "rd":
			i++
			if i >= len(args) {
				return "", fmt.Errorf("rd requires value")
			}
			spec.RD = args[i]
		case "prefix":
			i++
			if i >= len(args) {
				return "", fmt.Errorf("prefix requires value")
			}
			spec.Prefix = args[i]
		case "address":
			i++
			if i >= len(args) {
				return "", fmt.Errorf("address requires value")
			}
			spec.Address = args[i]
		case "teid":
			i++
			if i >= len(args) {
				return "", fmt.Errorf("teid requires value")
			}
			spec.TEID = args[i]
		case "qfi":
			i++
			if i >= len(args) {
				return "", fmt.Errorf("qfi requires value")
			}
			v, err := strconv.ParseUint(args[i], 10, 8)
			if err != nil {
				return "", fmt.Errorf("invalid qfi: %w", err)
			}
			spec.QFI = uint8(v) //nolint:gosec // validated by ParseUint with bitSize 8
		case "endpoint":
			i++
			if i >= len(args) {
				return "", fmt.Errorf("endpoint requires value")
			}
			spec.Endpoint = args[i]
		case "source":
			i++
			if i >= len(args) {
				return "", fmt.Errorf("source requires value")
			}
			spec.Source = args[i]
		case "ipv6":
			i++
			if i >= len(args) {
				return "", fmt.Errorf("ipv6 requires value")
			}
			spec.IsIPv6 = args[i] == "true"
		}
	}

	if spec.RouteType == "" {
		return "", fmt.Errorf("route-type required for MUP")
	}

	nlriBytes, _, err := buildMUPNLRI(spec)
	if err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(nlriBytes)), nil
}

// EncodeRoute encodes a MUP route command into UPDATE body bytes and NLRI bytes.
// This implements the InProcessRouteEncoder signature for the plugin registry.
func EncodeRoute(routeCmd, family string, localAS uint32, isIBGP, asn4, addPath bool) ([]byte, []byte, error) {
	isIPv6 := strings.HasPrefix(family, "ipv6/")
	ub := message.NewUpdateBuilder(localAS, isIBGP, asn4, addPath)

	// Parse route command
	args := strings.Fields(routeCmd)
	if len(args) < 1 {
		return nil, nil, fmt.Errorf("missing MUP command")
	}

	// Parse using API parser
	parsed, err := route.ParseMUPArgs(args, isIPv6)
	if err != nil {
		return nil, nil, fmt.Errorf("parse error: %w", err)
	}

	// Build MUP NLRI
	nlriBytes, routeType, err := buildMUPNLRI(parsed)
	if err != nil {
		return nil, nil, fmt.Errorf("build NLRI: %w", err)
	}

	// Parse next-hop
	var nextHop netip.Addr
	if parsed.NextHop != "" {
		nextHop, err = netip.ParseAddr(parsed.NextHop)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid next-hop: %w", err)
		}
	}

	// Convert to MUPParams
	params := message.MUPParams{
		RouteType: routeType,
		IsIPv6:    isIPv6,
		NLRI:      nlriBytes,
		NextHop:   nextHop,
	}

	// Build UPDATE
	update := ub.BuildMUP(params)

	// Pack UPDATE body using PackTo
	updateBody := message.PackTo(update, nil)

	return updateBody, nlriBytes, nil
}

// buildMUPNLRI builds MUP NLRI bytes from MUPRouteSpec.
// Returns (nlri bytes, route type code, error).
func buildMUPNLRI(spec bgptypes.MUPRouteSpec) ([]byte, uint8, error) {
	// Determine route type code
	var routeType MUPRouteType
	switch spec.RouteType {
	case route.MUPRouteTypeISD:
		routeType = MUPISD
	case route.MUPRouteTypeDSD:
		routeType = MUPDSD
	case route.MUPRouteTypeT1ST:
		routeType = MUPT1ST
	case route.MUPRouteTypeT2ST:
		routeType = MUPT2ST
	default:
		return nil, 0, fmt.Errorf("unknown MUP route type: %s", spec.RouteType)
	}

	// Parse RD
	var rd RouteDistinguisher
	if spec.RD != "" {
		parsed, err := ParseRDString(spec.RD)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid RD %q: %w", spec.RD, err)
		}
		rd = parsed
	}

	// Build route-type-specific data
	var data []byte
	switch routeType {
	case MUPISD:
		if spec.Prefix == "" {
			return nil, 0, fmt.Errorf("MUP ISD requires prefix")
		}
		prefix, err := netip.ParsePrefix(spec.Prefix)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid ISD prefix %q: %w", spec.Prefix, err)
		}
		data = buildMUPPrefixBytes(prefix)

	case MUPDSD:
		if spec.Address == "" {
			return nil, 0, fmt.Errorf("MUP DSD requires address")
		}
		addr, err := netip.ParseAddr(spec.Address)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid DSD address %q: %w", spec.Address, err)
		}
		data = addr.AsSlice()

	case MUPT1ST:
		if spec.Prefix == "" {
			return nil, 0, fmt.Errorf("MUP T1ST requires prefix")
		}
		prefix, err := netip.ParsePrefix(spec.Prefix)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid T1ST prefix %q: %w", spec.Prefix, err)
		}
		data = buildMUPPrefixBytes(prefix)
		// Add TEID (4 bytes) if specified
		if spec.TEID != "" {
			teid, _ := parseTEIDWithBits(spec.TEID)
			data = append(data, byte(teid>>24), byte(teid>>16), byte(teid>>8), byte(teid)) //nolint:gosec // deliberate uint32→byte truncation for big-endian encoding
		}
		// Add QFI (1 byte)
		data = append(data, spec.QFI)
		// Add endpoint if specified
		if spec.Endpoint != "" {
			ep, epErr := netip.ParseAddr(spec.Endpoint)
			if epErr != nil {
				return nil, 0, fmt.Errorf("invalid T1ST endpoint %q: %w", spec.Endpoint, epErr)
			}
			epBytes := ep.AsSlice()
			data = append(data, byte(len(epBytes)*8)) //nolint:gosec // epBytes is 4 or 16 bytes
			data = append(data, epBytes...)
		}
		// Add source if specified
		if spec.Source != "" {
			src, srcErr := netip.ParseAddr(spec.Source)
			if srcErr != nil {
				return nil, 0, fmt.Errorf("invalid T1ST source %q: %w", spec.Source, srcErr)
			}
			srcBytes := src.AsSlice()
			data = append(data, byte(len(srcBytes)*8)) //nolint:gosec // srcBytes is 4 or 16 bytes
			data = append(data, srcBytes...)
		}

	case MUPT2ST:
		if spec.Address == "" {
			return nil, 0, fmt.Errorf("MUP T2ST requires address")
		}
		ep, err := netip.ParseAddr(spec.Address)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid T2ST endpoint %q: %w", spec.Address, err)
		}
		epBytes := ep.AsSlice()
		teid, bits := parseTEIDWithBits(spec.TEID)
		teidLen := TEIDFieldLen(bits)
		data = make([]byte, 1+len(epBytes)+teidLen)
		data[0] = byte(len(epBytes)*8 + bits) //nolint:gosec // epBytes is 4 or 16 bytes, bits <= 32
		copy(data[1:], epBytes)
		writeTEIDWithBits(data, 1+len(epBytes), teid, bits)
	}

	// Determine AFI
	afi := AFIIPv4
	if spec.IsIPv6 {
		afi = AFIIPv6
	}

	m := NewMUPFull(afi, MUPArch3GPP5G, routeType, rd, data)
	nlriBytes := m.Bytes()

	return nlriBytes, uint8(routeType), nil //nolint:gosec // MUP route type is always 0-4
}

// buildMUPPrefixBytes encodes a prefix for MUP NLRI.
func buildMUPPrefixBytes(prefix netip.Prefix) []byte {
	result := make([]byte, MUPPrefixLen(prefix))
	WriteMUPPrefix(result, 0, prefix)
	return result
}

// parseTEIDWithBits parses a TEID string "value/bits" into numeric TEID and bit length.
// If no bit specifier, defaults to 32 bits. Empty string returns (0, 0).
func parseTEIDWithBits(s string) (uint32, int) {
	if s == "" {
		return 0, 0
	}
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		v, _ := strconv.ParseUint(s, 10, 32)
		return uint32(v), 32 //nolint:gosec // validated by ParseUint with bitSize 32
	}
	v, _ := strconv.ParseUint(parts[0], 10, 32)
	bits, err := strconv.Atoi(parts[1])
	if err != nil {
		bits = 32
	}
	return uint32(v), bits //nolint:gosec // validated by ParseUint with bitSize 32
}

// writeTEIDWithBits writes TEID with the specified bit length into buf at off.
func writeTEIDWithBits(buf []byte, off int, teid uint32, bits int) {
	if bits <= 0 {
		return
	}
	byteLen := (bits + 7) / 8
	for i := range byteLen {
		shift := (byteLen - 1 - i) * 8
		buf[off+i] = byte(teid >> shift) //nolint:gosec // shift is bounded by byteLen
	}
}
