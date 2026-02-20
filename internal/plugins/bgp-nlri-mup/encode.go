// Design: docs/architecture/wire/nlri.md — MUP NLRI wire encoding from route commands

package bgp_nlri_mup

import (
	"encoding/binary"
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
//
// All route-type data is pre-computed for size, allocated once, then written
// at offsets — no append(). Final NLRI uses WriteTo into a single buffer.
func buildMUPNLRI(spec bgptypes.MUPRouteSpec) ([]byte, uint8, error) {
	routeType, err := parseMUPRouteType(spec.RouteType)
	if err != nil {
		return nil, 0, err
	}

	// Parse RD
	var rd RouteDistinguisher
	if spec.RD != "" {
		parsed, rdErr := ParseRDString(spec.RD)
		if rdErr != nil {
			return nil, 0, fmt.Errorf("invalid RD %q: %w", spec.RD, rdErr)
		}
		rd = parsed
	}

	// Build route-type-specific data into pre-allocated buffer.
	var data []byte
	switch routeType {
	case MUPISD:
		data, err = buildISDData(spec)
	case MUPDSD:
		data, err = buildDSDData(spec)
	case MUPT1ST:
		data, err = buildT1STData(spec)
	case MUPT2ST:
		data, err = buildT2STData(spec)
	}
	if err != nil {
		return nil, 0, err
	}

	return finishMUPNLRI(spec.IsIPv6, routeType, rd, data)
}

// parseMUPRouteType converts route type string to MUPRouteType.
func parseMUPRouteType(s string) (MUPRouteType, error) {
	switch s {
	case route.MUPRouteTypeISD:
		return MUPISD, nil
	case route.MUPRouteTypeDSD:
		return MUPDSD, nil
	case route.MUPRouteTypeT1ST:
		return MUPT1ST, nil
	case route.MUPRouteTypeT2ST:
		return MUPT2ST, nil
	}
	return 0, fmt.Errorf("unknown MUP route type: %s", s)
}

// buildISDData builds ISD route-type data: prefix bytes only.
func buildISDData(spec bgptypes.MUPRouteSpec) ([]byte, error) {
	if spec.Prefix == "" {
		return nil, fmt.Errorf("MUP ISD requires prefix")
	}
	prefix, err := netip.ParsePrefix(spec.Prefix)
	if err != nil {
		return nil, fmt.Errorf("invalid ISD prefix %q: %w", spec.Prefix, err)
	}
	data := make([]byte, MUPPrefixLen(prefix))
	WriteMUPPrefix(data, 0, prefix)
	return data, nil
}

// buildDSDData builds DSD route-type data: address bytes only.
func buildDSDData(spec bgptypes.MUPRouteSpec) ([]byte, error) {
	if spec.Address == "" {
		return nil, fmt.Errorf("MUP DSD requires address")
	}
	addr, err := netip.ParseAddr(spec.Address)
	if err != nil {
		return nil, fmt.Errorf("invalid DSD address %q: %w", spec.Address, err)
	}
	data := make([]byte, addrByteLen(addr))
	writeAddr(data, 0, addr)
	return data, nil
}

// buildT1STData builds T1ST route-type data with pre-computed size, no append().
func buildT1STData(spec bgptypes.MUPRouteSpec) ([]byte, error) {
	if spec.Prefix == "" {
		return nil, fmt.Errorf("MUP T1ST requires prefix")
	}
	prefix, err := netip.ParsePrefix(spec.Prefix)
	if err != nil {
		return nil, fmt.Errorf("invalid T1ST prefix %q: %w", spec.Prefix, err)
	}

	// Parse optional addresses upfront for size computation.
	var ep, src netip.Addr
	if spec.Endpoint != "" {
		ep, err = netip.ParseAddr(spec.Endpoint)
		if err != nil {
			return nil, fmt.Errorf("invalid T1ST endpoint %q: %w", spec.Endpoint, err)
		}
	}
	if spec.Source != "" {
		src, err = netip.ParseAddr(spec.Source)
		if err != nil {
			return nil, fmt.Errorf("invalid T1ST source %q: %w", spec.Source, err)
		}
	}

	// Pre-compute total size: prefix + TEID(4 if set) + QFI(1) + endpoint + source.
	size := MUPPrefixLen(prefix)
	teid, teidBits := parseTEIDWithBits(spec.TEID)
	if spec.TEID != "" {
		size += 4
	}
	size++ // QFI
	if ep.IsValid() {
		size += 1 + addrByteLen(ep) // length byte + address
	}
	if src.IsValid() {
		size += 1 + addrByteLen(src) // length byte + address
	}

	// Allocate once, write at offsets.
	data := make([]byte, size)
	off := 0

	WriteMUPPrefix(data, off, prefix)
	off += MUPPrefixLen(prefix)

	if spec.TEID != "" {
		binary.BigEndian.PutUint32(data[off:], teid)
		off += 4
	}

	data[off] = spec.QFI
	off++

	if ep.IsValid() {
		epLen := addrByteLen(ep)
		data[off] = byte(epLen * 8) //nolint:gosec // epLen is 4 or 16
		off++
		off += writeAddr(data, off, ep)
	}

	if src.IsValid() {
		srcLen := addrByteLen(src)
		data[off] = byte(srcLen * 8) //nolint:gosec // srcLen is 4 or 16
		off++
		writeAddr(data, off, src)
	}

	_ = teidBits // bits not used for T1ST (always full 4-byte TEID)

	return data, nil
}

// buildT2STData builds T2ST route-type data: endpoint + TEID with bit length.
func buildT2STData(spec bgptypes.MUPRouteSpec) ([]byte, error) {
	if spec.Address == "" {
		return nil, fmt.Errorf("MUP T2ST requires address")
	}
	ep, err := netip.ParseAddr(spec.Address)
	if err != nil {
		return nil, fmt.Errorf("invalid T2ST endpoint %q: %w", spec.Address, err)
	}
	epLen := addrByteLen(ep)
	teid, bits := parseTEIDWithBits(spec.TEID)
	teidLen := TEIDFieldLen(bits)
	data := make([]byte, 1+epLen+teidLen)
	data[0] = byte(epLen*8 + bits) //nolint:gosec // epLen is 4 or 16, bits <= 32
	writeAddr(data, 1, ep)
	writeTEIDWithBits(data, 1+epLen, teid, bits)
	return data, nil
}

// finishMUPNLRI wraps route-type data into a full MUP NLRI using WriteTo.
func finishMUPNLRI(isIPv6 bool, routeType MUPRouteType, rd RouteDistinguisher, data []byte) ([]byte, uint8, error) {
	afi := AFIIPv4
	if isIPv6 {
		afi = AFIIPv6
	}
	m := NewMUPFull(afi, MUPArch3GPP5G, routeType, rd, data)
	buf := make([]byte, m.Len())
	m.WriteTo(buf, 0)
	return buf, uint8(routeType), nil //nolint:gosec // MUP route type is always 0-4
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
