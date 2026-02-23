// Design: docs/architecture/wire/nlri.md — MUP NLRI wire encoding from route commands
// RFC: rfc/short/draft-ietf-bess-mup-safi.md — MUP SAFI wire format

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

// mupParsed holds parsed route-type fields and computed data size.
// Populated by parse*Fields, consumed by write*Data. No allocation.
type mupParsed struct {
	prefix   netip.Prefix
	addr     netip.Addr
	ep       netip.Addr
	src      netip.Addr
	teid     uint32
	teidBits int
	qfi      uint8
	dataSize int
}

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

	// MUP NLRIs are small (max ~68 bytes for T1ST with IPv6).
	var buf [128]byte
	n, _, err := writeMUPNLRI(buf[:], 0, spec)
	if err != nil {
		return "", err
	}

	return strings.ToUpper(hex.EncodeToString(buf[:n])), nil
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

	// Build MUP NLRI — small (max ~68 bytes), stack array is sufficient.
	var buf [128]byte
	n, routeType, err := writeMUPNLRI(buf[:], 0, parsed)
	if err != nil {
		return nil, nil, fmt.Errorf("build NLRI: %w", err)
	}

	// Own copy for the return value.
	nlriBytes := make([]byte, n)
	copy(nlriBytes, buf[:n])

	// Parse next-hop
	var nextHop netip.Addr
	if parsed.NextHop != "" {
		nextHop, err = netip.ParseAddr(parsed.NextHop)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid next-hop: %w", err)
		}
	}

	// Build UPDATE using stack buf data.
	params := message.MUPParams{
		RouteType: routeType,
		IsIPv6:    isIPv6,
		NLRI:      buf[:n],
		NextHop:   nextHop,
	}
	update := ub.BuildMUP(params)
	updateBody := message.PackTo(update, nil)

	return updateBody, nlriBytes, nil
}

// writeMUPNLRI writes the complete MUP NLRI into buf at off.
// Returns (bytes written, route type code, error).
// No allocation — writes into caller-provided buffer.
func writeMUPNLRI(buf []byte, off int, spec bgptypes.MUPRouteSpec) (int, uint8, error) {
	routeType, err := parseMUPRouteType(spec.RouteType)
	if err != nil {
		return 0, 0, err
	}

	// Parse RD.
	var rd RouteDistinguisher
	rdSize := 0
	if spec.RD != "" {
		parsed, rdErr := ParseRDString(spec.RD)
		if rdErr != nil {
			return 0, 0, fmt.Errorf("invalid RD %q: %w", spec.RD, rdErr)
		}
		rd = parsed
		rdSize = 8
	}

	// Parse route-type fields and compute data size.
	var fields mupParsed
	switch routeType {
	case MUPISD:
		fields, err = parseISDFields(spec)
	case MUPDSD:
		fields, err = parseDSDFields(spec)
	case MUPT1ST:
		fields, err = parseT1STFields(spec)
	case MUPT2ST:
		fields, err = parseT2STFields(spec)
	}
	if err != nil {
		return 0, 0, err
	}

	// Write MUP header: arch(1) + routeType(2) + dataLength(1).
	pos := off
	buf[pos] = byte(MUPArch3GPP5G)
	binary.BigEndian.PutUint16(buf[pos+1:], uint16(routeType))
	buf[pos+3] = byte(rdSize + fields.dataSize) //nolint:gosec // max ~64, fits byte
	pos += 4

	// Write RD.
	if rdSize > 0 {
		pos += rd.WriteTo(buf, pos)
	}

	// Write route-type data.
	switch routeType {
	case MUPISD:
		pos += writeISDData(buf, pos, fields)
	case MUPDSD:
		pos += writeDSDData(buf, pos, fields)
	case MUPT1ST:
		pos += writeT1STData(buf, pos, fields)
	case MUPT2ST:
		pos += writeT2STData(buf, pos, fields)
	}

	return pos - off, uint8(routeType), nil //nolint:gosec // MUP route type is always 0-4
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

// --- Parse functions: validate inputs, compute sizes, no allocation ---

// parseISDFields validates and computes size for ISD route type.
func parseISDFields(spec bgptypes.MUPRouteSpec) (mupParsed, error) {
	if spec.Prefix == "" {
		return mupParsed{}, fmt.Errorf("MUP ISD requires prefix")
	}
	prefix, err := netip.ParsePrefix(spec.Prefix)
	if err != nil {
		return mupParsed{}, fmt.Errorf("invalid ISD prefix %q: %w", spec.Prefix, err)
	}
	return mupParsed{prefix: prefix, dataSize: MUPPrefixLen(prefix)}, nil
}

// parseDSDFields validates and computes size for DSD route type.
func parseDSDFields(spec bgptypes.MUPRouteSpec) (mupParsed, error) {
	if spec.Address == "" {
		return mupParsed{}, fmt.Errorf("MUP DSD requires address")
	}
	addr, err := netip.ParseAddr(spec.Address)
	if err != nil {
		return mupParsed{}, fmt.Errorf("invalid DSD address %q: %w", spec.Address, err)
	}
	return mupParsed{addr: addr, dataSize: addrByteLen(addr)}, nil
}

// parseT1STFields validates and computes size for T1ST route type.
func parseT1STFields(spec bgptypes.MUPRouteSpec) (mupParsed, error) {
	if spec.Prefix == "" {
		return mupParsed{}, fmt.Errorf("MUP T1ST requires prefix")
	}
	prefix, err := netip.ParsePrefix(spec.Prefix)
	if err != nil {
		return mupParsed{}, fmt.Errorf("invalid T1ST prefix %q: %w", spec.Prefix, err)
	}

	var f mupParsed
	f.prefix = prefix
	f.qfi = spec.QFI

	if spec.Endpoint != "" {
		f.ep, err = netip.ParseAddr(spec.Endpoint)
		if err != nil {
			return mupParsed{}, fmt.Errorf("invalid T1ST endpoint %q: %w", spec.Endpoint, err)
		}
	}
	if spec.Source != "" {
		f.src, err = netip.ParseAddr(spec.Source)
		if err != nil {
			return mupParsed{}, fmt.Errorf("invalid T1ST source %q: %w", spec.Source, err)
		}
	}

	f.teid, f.teidBits = parseTEIDWithBits(spec.TEID)

	// Compute data size: prefix + TEID(4 if set) + QFI(1) + endpoint + source.
	size := MUPPrefixLen(prefix)
	if f.teidBits > 0 {
		size += 4
	}
	size++ // QFI
	if f.ep.IsValid() {
		size += 1 + addrByteLen(f.ep) // length byte + address
	}
	if f.src.IsValid() {
		size += 1 + addrByteLen(f.src) // length byte + address
	}
	f.dataSize = size

	return f, nil
}

// parseT2STFields validates and computes size for T2ST route type.
func parseT2STFields(spec bgptypes.MUPRouteSpec) (mupParsed, error) {
	if spec.Address == "" {
		return mupParsed{}, fmt.Errorf("MUP T2ST requires address")
	}
	ep, err := netip.ParseAddr(spec.Address)
	if err != nil {
		return mupParsed{}, fmt.Errorf("invalid T2ST endpoint %q: %w", spec.Address, err)
	}
	teid, bits := parseTEIDWithBits(spec.TEID)
	return mupParsed{
		ep:       ep,
		teid:     teid,
		teidBits: bits,
		dataSize: 1 + addrByteLen(ep) + TEIDFieldLen(bits),
	}, nil
}

// --- Write functions: write into caller-provided buffer, no allocation ---

// writeISDData writes ISD route-type data into buf at off. Returns bytes written.
func writeISDData(buf []byte, off int, f mupParsed) int {
	WriteMUPPrefix(buf, off, f.prefix)
	return MUPPrefixLen(f.prefix)
}

// writeDSDData writes DSD route-type data into buf at off. Returns bytes written.
func writeDSDData(buf []byte, off int, f mupParsed) int {
	return writeAddr(buf, off, f.addr)
}

// writeT1STData writes T1ST route-type data into buf at off. Returns bytes written.
func writeT1STData(buf []byte, off int, f mupParsed) int {
	pos := off

	WriteMUPPrefix(buf, pos, f.prefix)
	pos += MUPPrefixLen(f.prefix)

	if f.teidBits > 0 {
		binary.BigEndian.PutUint32(buf[pos:], f.teid)
		pos += 4
	}

	buf[pos] = f.qfi
	pos++

	if f.ep.IsValid() {
		epLen := addrByteLen(f.ep)
		buf[pos] = byte(epLen * 8) //nolint:gosec // epLen is 4 or 16
		pos++
		pos += writeAddr(buf, pos, f.ep)
	}

	if f.src.IsValid() {
		srcLen := addrByteLen(f.src)
		buf[pos] = byte(srcLen * 8) //nolint:gosec // srcLen is 4 or 16
		pos++
		pos += writeAddr(buf, pos, f.src)
	}

	return pos - off
}

// writeT2STData writes T2ST route-type data into buf at off. Returns bytes written.
func writeT2STData(buf []byte, off int, f mupParsed) int {
	pos := off
	epLen := addrByteLen(f.ep)
	buf[pos] = byte(epLen*8 + f.teidBits) //nolint:gosec // epLen is 4 or 16, bits <= 32
	pos++
	pos += writeAddr(buf, pos, f.ep)
	writeTEIDWithBits(buf, pos, f.teid, f.teidBits)
	pos += TEIDFieldLen(f.teidBits)
	return pos - off
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
