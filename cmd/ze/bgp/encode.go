package bgp

import (
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"math"
	"net/netip"
	"os"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/route"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"

	evpn "codeberg.org/thomas-mangin/ze/internal/plugins/bgp-evpn"
	flowspec "codeberg.org/thomas-mangin/ze/internal/plugins/bgp-flowspec"
	vpn "codeberg.org/thomas-mangin/ze/internal/plugins/bgp-vpn"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
)

// encodeStdout, encodeStderr, and encodeStdin allow tests to capture I/O.
// encodeStdinIsTTY is mockable for testing.
var (
	encodeStdout     io.Writer = os.Stdout
	encodeStderr     io.Writer = os.Stderr
	encodeStdin      io.Reader = os.Stdin
	encodeStdinIsTTY           = func() bool {
		fi, err := os.Stdin.Stat()
		if err != nil {
			return false
		}
		return fi.Mode()&os.ModeCharDevice != 0
	}
)

// cmdEncode handles the 'encode' subcommand.
// Encodes API route commands to BGP message hex.
func cmdEncode(args []string) int {
	fs := flag.NewFlagSet("encode", flag.ContinueOnError)
	fs.SetOutput(encodeStderr)

	family := fs.String("f", "ipv4/unicast", "address family (e.g., 'ipv4/unicast', 'ipv6/unicast', 'l2vpn/evpn')")
	localAS := fs.Uint("a", 65533, "local AS number")
	peerAS := fs.Uint("z", 65533, "peer AS number")
	pathInfo := fs.Bool("i", false, "enable ADD-PATH (include path-id)")
	nlriOnly := fs.Bool("n", false, "output only NLRI bytes")
	noHeader := fs.Bool("no-header", false, "exclude 19-byte BGP header")
	asn4 := fs.Bool("asn4", true, "use 4-byte ASN encoding")

	fs.Usage = func() {
		_, _ = fmt.Fprintf(encodeStderr, `Usage: ze bgp encode [options] [route-command]

Encode API route command to BGP message hex.
Route command can be provided as argument or via stdin.

Options:
`)
		fs.PrintDefaults()
		_, _ = fmt.Fprintf(encodeStderr, `
Examples:
  # IPv4/IPv6 Unicast
  ze bgp encode "route 10.0.0.0/24 next-hop 192.168.1.1"
  ze bgp encode -f "ipv6/unicast" "route 2001:db8::/32 next-hop 2001:db8::1"

  # L3VPN (mpls-vpn)
  ze bgp encode -f "ipv4/mpls-vpn" "10.0.0.0/24 rd 100:1 next-hop 1.2.3.4 label 100"
  ze bgp encode -f "ipv4/mpls-vpn" "10.0.0.0/24 rd 1.2.3.4:100 next-hop 1.2.3.4 label 100"

  # Labeled Unicast (nlri-mpls)
  ze bgp encode -f "ipv4/nlri-mpls" "10.0.0.0/24 next-hop 1.2.3.4 label 100"

  # EVPN
  ze bgp encode -f "l2vpn/evpn" "mac-ip rd 100:1 esi 0 etag 0 mac 00:11:22:33:44:55 label 100 next-hop 1.2.3.4"
  ze bgp encode -f "l2vpn/evpn" "ip-prefix rd 100:1 esi 0 etag 0 prefix 10.0.0.0/24 gateway 0.0.0.0 label 100 next-hop 1.2.3.4"
  ze bgp encode -f "l2vpn/evpn" "multicast rd 100:1 etag 0 next-hop 1.2.3.4"

  # Output options
  ze bgp encode -n "route 10.0.0.0/24 next-hop 1.2.3.4"       # NLRI only
  ze bgp encode --no-header "route 10.0.0.0/24 next-hop 1.2.3.4"  # No BGP header
  echo "route 10.0.0.0/24 next-hop 1.2.3.4" | ze bgp encode   # stdin
`)
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Get route command from args or stdin
	var routeCmd string
	if fs.NArg() < 1 {
		// Check if stdin is a terminal (would block forever waiting for input)
		if encodeStdinIsTTY() {
			_, _ = fmt.Fprintf(encodeStderr, "error: missing route command\n")
			fs.Usage()
			return 1
		}
		// Read from stdin (piped input)
		input, err := io.ReadAll(encodeStdin)
		if err != nil {
			_, _ = fmt.Fprintf(encodeStderr, "error reading stdin: %v\n", err)
			return 1
		}
		routeCmd = strings.TrimSpace(string(input))
		if routeCmd == "" {
			_, _ = fmt.Fprintf(encodeStderr, "error: missing route command\n")
			fs.Usage()
			return 1
		}
	} else {
		// Join remaining args as the route command
		routeCmd = strings.Join(fs.Args(), " ")
	}

	// Parse family
	afi, safi, err := parseEncodingFamily(*family)
	if err != nil {
		_, _ = fmt.Fprintf(encodeStderr, "error: %v\n", err)
		return 1
	}

	// Determine iBGP vs eBGP
	isIBGP := *localAS == *peerAS

	// Create UpdateBuilder
	// #nosec G115 - localAS is from uint flag, bounded by flag validation
	ub := message.NewUpdateBuilder(uint32(*localAS), isIBGP, *asn4, *pathInfo)

	// Encode based on family
	var updateBytes []byte
	var nlriBytes []byte

	switch {
	case afi == nlri.AFIIPv4 && safi == nlri.SAFIUnicast:
		updateBytes, nlriBytes, err = encodeUnicastRoute(ub, routeCmd, false, *asn4, *pathInfo)
	case afi == nlri.AFIIPv6 && safi == nlri.SAFIUnicast:
		updateBytes, nlriBytes, err = encodeUnicastRoute(ub, routeCmd, true, *asn4, *pathInfo)
	case afi == nlri.AFIIPv4 && safi == nlri.SAFIVPN:
		updateBytes, nlriBytes, err = encodeL3VPNRoute(ub, routeCmd, false, *asn4, *pathInfo)
	case afi == nlri.AFIIPv6 && safi == nlri.SAFIVPN:
		updateBytes, nlriBytes, err = encodeL3VPNRoute(ub, routeCmd, true, *asn4, *pathInfo)
	case afi == nlri.AFIIPv4 && safi == nlri.SAFIMPLSLabel:
		updateBytes, nlriBytes, err = encodeLabeledUnicastRoute(ub, routeCmd, false, *asn4, *pathInfo)
	case afi == nlri.AFIIPv6 && safi == nlri.SAFIMPLSLabel:
		updateBytes, nlriBytes, err = encodeLabeledUnicastRoute(ub, routeCmd, true, *asn4, *pathInfo)
	case afi == nlri.AFIL2VPN && safi == nlri.SAFIEVPN:
		updateBytes, nlriBytes, err = encodeEVPNRoute(ub, routeCmd, *asn4, *pathInfo)
	case safi == nlri.SAFIFlowSpec:
		updateBytes, nlriBytes, err = encodeFlowSpecRoute(ub, routeCmd, afi == nlri.AFIIPv6, *asn4, *pathInfo)
	case afi == nlri.AFIL2VPN && safi == nlri.SAFIVPLS:
		updateBytes, nlriBytes, err = encodeVPLSRoute(ub, routeCmd, *asn4, *pathInfo)
	case safi == nlri.SAFIMUP:
		updateBytes, nlriBytes, err = encodeMUPRoute(ub, routeCmd, afi == nlri.AFIIPv6, *asn4, *pathInfo)
	default:
		err = fmt.Errorf("unsupported family: %s", *family)
	}

	if err != nil {
		_, _ = fmt.Fprintf(encodeStderr, "error: %v\n", err)
		return 1
	}

	// Determine what to output
	// Note: updateBytes already includes the BGP header (from Update.Pack)
	var output []byte
	switch {
	case *nlriOnly:
		output = nlriBytes
	case *noHeader:
		// Strip the 19-byte BGP header
		if len(updateBytes) > message.HeaderLen {
			output = updateBytes[message.HeaderLen:]
		} else {
			output = updateBytes
		}
	default:
		// Full message with header (already included)
		output = updateBytes
	}

	// Output as uppercase hex
	_, _ = fmt.Fprintln(encodeStdout, strings.ToUpper(hex.EncodeToString(output)))
	return 0
}

// parseEncodingFamily parses family string to AFI/SAFI.
// Requires "afi/safi" format (e.g., "ipv4/unicast").
func parseEncodingFamily(family string) (nlri.AFI, nlri.SAFI, error) {
	f, ok := nlri.ParseFamily(strings.ToLower(family))
	if !ok {
		return 0, 0, fmt.Errorf("unknown family: %s (expected afi/safi format)", family)
	}
	return f.AFI, f.SAFI, nil
}

// encodeUnicastRoute parses and encodes a unicast route command.
// Returns (update body bytes, NLRI bytes, error).
func encodeUnicastRoute(ub *message.UpdateBuilder, routeCmd string, isIPv6 bool, _, addPath bool) ([]byte, []byte, error) {
	// Parse route command - expects "route <prefix> next-hop <addr> [attributes...]"
	args := strings.Fields(routeCmd)
	if len(args) < 1 || args[0] != "route" {
		return nil, nil, fmt.Errorf("expected 'route' keyword, got: %s", routeCmd)
	}

	// Parse using API parser
	parsed, err := route.ParseRouteAttributes(args[1:], route.UnicastKeywords)
	if err != nil {
		return nil, nil, fmt.Errorf("parse error: %w", err)
	}

	// Convert RouteSpec to UnicastParams
	params := routeSpecToUnicastParams(parsed.Route)

	// Build UPDATE
	update := ub.BuildUnicast(params)

	// Extract NLRI bytes
	var nlriBytes []byte
	if isIPv6 {
		// For IPv6, NLRI is in MP_REACH_NLRI - extract from path attributes
		// For now, just pack the prefix directly
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}, parsed.Route.Prefix, 0)
		nlriLen := nlri.LenWithContext(inet, addPath)
		nlriBytes = make([]byte, nlriLen)
		nlri.WriteNLRI(inet, nlriBytes, 0, addPath)
	} else {
		// For IPv4, NLRI is inline
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, parsed.Route.Prefix, 0)
		nlriLen := nlri.LenWithContext(inet, addPath)
		nlriBytes = make([]byte, nlriLen)
		nlri.WriteNLRI(inet, nlriBytes, 0, addPath)
	}

	// Pack UPDATE body using PackTo
	updateBody := message.PackTo(update, nil)

	return updateBody, nlriBytes, nil
}

// routeSpecToUnicastParams converts a RouteSpec to UnicastParams.
// Extracts address from RouteNextHop (must be explicit, not self).
// Uses wire-first approach: prefers Wire, then Attrs (Builder).
func routeSpecToUnicastParams(r bgptypes.RouteSpec) message.UnicastParams {
	var attrs commonAttrs

	if r.Wire != nil {
		// Extract attributes from wire format
		attrs = extractAttrsFromWire(r.Wire)
	} else {
		// Use defaults
		attrs = commonAttrs{
			Origin: attribute.OriginIGP,
		}
	}

	return message.UnicastParams{
		Prefix:            r.Prefix,
		NextHop:           r.NextHop.Addr, // Extract address from RouteNextHop
		Origin:            attrs.Origin,
		LocalPreference:   attrs.LocalPreference,
		MED:               attrs.MED,
		ASPath:            attrs.ASPath,
		Communities:       attrs.Communities,
		LargeCommunities:  attrs.LargeCommunities,
		ExtCommunityBytes: attrs.ExtCommunityBytes,
	}
}

// extractAttrsFromWire extracts commonAttrs from AttributesWire.
func extractAttrsFromWire(wire *attribute.AttributesWire) commonAttrs {
	var attrs commonAttrs
	attrs.Origin = attribute.OriginIGP // default

	if wire == nil {
		return attrs
	}

	// Extract ORIGIN
	if originAttr, err := wire.Get(attribute.AttrOrigin); err == nil && originAttr != nil {
		if o, ok := originAttr.(attribute.Origin); ok {
			attrs.Origin = o
		}
	}
	// Extract LOCAL_PREF
	if lpAttr, err := wire.Get(attribute.AttrLocalPref); err == nil && lpAttr != nil {
		if lp, ok := lpAttr.(attribute.LocalPref); ok {
			attrs.LocalPreference = uint32(lp)
		}
	}
	// Extract MED
	if medAttr, err := wire.Get(attribute.AttrMED); err == nil && medAttr != nil {
		if med, ok := medAttr.(attribute.MED); ok {
			attrs.MED = uint32(med)
		}
	}
	// Extract AS_PATH
	if asPathAttr, err := wire.Get(attribute.AttrASPath); err == nil {
		if asp, ok := asPathAttr.(*attribute.ASPath); ok && len(asp.Segments) > 0 {
			attrs.ASPath = asp.Segments[0].ASNs
		}
	}
	// Extract COMMUNITY
	if commAttr, err := wire.Get(attribute.AttrCommunity); err == nil {
		if comms, ok := commAttr.(attribute.Communities); ok {
			attrs.Communities = make([]uint32, len(comms))
			for i, c := range comms {
				attrs.Communities[i] = uint32(c)
			}
		}
	}
	// Extract LARGE_COMMUNITY
	if lcAttr, err := wire.Get(attribute.AttrLargeCommunity); err == nil {
		if lcs, ok := lcAttr.(attribute.LargeCommunities); ok {
			attrs.LargeCommunities = make([][3]uint32, len(lcs))
			for i, c := range lcs {
				attrs.LargeCommunities[i] = [3]uint32{c.GlobalAdmin, c.LocalData1, c.LocalData2}
			}
		}
	}
	// Extract EXTENDED_COMMUNITIES
	if ecAttr, err := wire.Get(attribute.AttrExtCommunity); err == nil {
		if ecs, ok := ecAttr.(attribute.ExtendedCommunities); ok {
			buf := make([]byte, ecs.Len())
			ecs.WriteTo(buf, 0)
			attrs.ExtCommunityBytes = buf
		}
	}

	return attrs
}

// commonAttrs holds extracted common BGP path attributes.
// Used to avoid duplicate code in route-to-params conversion functions.
type commonAttrs struct {
	Origin            attribute.Origin
	LocalPreference   uint32
	MED               uint32
	ASPath            []uint32
	Communities       []uint32
	LargeCommunities  [][3]uint32
	ExtCommunityBytes []byte
}

// encodeEVPNRoute parses and encodes an EVPN route command.
// Returns (update body bytes, NLRI bytes, error).
func encodeEVPNRoute(ub *message.UpdateBuilder, routeCmd string, _, addPath bool) ([]byte, []byte, error) {
	// Parse route command - expects "mac-ip|ip-prefix|... <args>"
	args := strings.Fields(routeCmd)
	if len(args) < 1 {
		return nil, nil, fmt.Errorf("missing EVPN route type")
	}

	// Parse using API parser
	parsed, err := parseL2VPNArgs(args)
	if err != nil {
		return nil, nil, fmt.Errorf("parse error: %w", err)
	}

	// Convert L2VPNRoute to EVPNParams
	params, err := l2vpnRouteToEVPNParams(parsed)
	if err != nil {
		return nil, nil, fmt.Errorf("conversion error: %w", err)
	}

	// Build UPDATE
	update := ub.BuildEVPN(params)

	// Pack UPDATE body using PackTo
	updateBody := message.PackTo(update, nil)

	// For NLRI bytes, we need to build and pack the NLRI
	var evpnNLRI evpn.EVPN
	switch params.RouteType {
	case 1:
		evpnNLRI = evpn.NewEVPNType1(params.RD, params.ESI, params.EthernetTag, params.Labels)
	case 2:
		evpnNLRI = evpn.NewEVPNType2(params.RD, params.ESI, params.EthernetTag, params.MAC, params.IP, params.Labels)
	case 3:
		evpnNLRI = evpn.NewEVPNType3(params.RD, params.EthernetTag, params.OriginatorIP)
	case 4:
		evpnNLRI = evpn.NewEVPNType4(params.RD, params.ESI, params.OriginatorIP)
	case 5:
		evpnNLRI = evpn.NewEVPNType5(params.RD, params.ESI, params.EthernetTag, params.Prefix, params.Gateway, params.Labels)
	}
	nlriLen := nlri.LenWithContext(evpnNLRI, addPath)
	nlriBytes := make([]byte, nlriLen)
	nlri.WriteNLRI(evpnNLRI, nlriBytes, 0, addPath)

	return updateBody, nlriBytes, nil
}

// l2vpnRouteToEVPNParams converts L2VPNRoute to EVPNParams.
//
//nolint:goconst // String literals are clearer for route type matching
func l2vpnRouteToEVPNParams(r bgptypes.L2VPNRoute) (message.EVPNParams, error) {
	p := message.EVPNParams{
		NextHop:     r.NextHop,
		EthernetTag: r.EthernetTag,
		Origin:      attribute.OriginIGP,
	}

	// Parse RD
	if r.RD != "" {
		rd, err := nlri.ParseRDString(r.RD)
		if err != nil {
			return p, fmt.Errorf("invalid RD: %w", err)
		}
		p.RD = rd
	}

	// Parse ESI
	esi, err := evpn.ParseESIString(r.ESI)
	if err != nil {
		return p, fmt.Errorf("invalid ESI: %w", err)
	}
	p.ESI = [10]byte(esi)

	// Route type mapping
	switch r.RouteType {
	case "ethernet-ad":
		p.RouteType = 1
		if r.Label1 != 0 {
			p.Labels = []uint32{r.Label1}
		}
	case "mac-ip":
		p.RouteType = 2
		mac, err := parseMAC(r.MAC)
		if err != nil {
			return p, fmt.Errorf("invalid MAC: %w", err)
		}
		p.MAC = mac
		p.IP = r.IP
		if r.Label1 != 0 {
			p.Labels = []uint32{r.Label1}
			if r.Label2 != 0 {
				p.Labels = append(p.Labels, r.Label2)
			}
		}
	case "multicast":
		p.RouteType = 3
		p.OriginatorIP = r.NextHop
	case "ethernet-segment":
		p.RouteType = 4
		p.OriginatorIP = r.NextHop
	case "ip-prefix":
		p.RouteType = 5
		p.Prefix = r.Prefix
		p.Gateway = r.Gateway
		if r.Label1 != 0 {
			p.Labels = []uint32{r.Label1}
		}
	default:
		return p, fmt.Errorf("unknown EVPN route type: %s", r.RouteType)
	}

	return p, nil
}

// encodeL3VPNRoute parses and encodes an L3VPN (mpls-vpn) route command.
// Returns (update body bytes, NLRI bytes, error).
func encodeL3VPNRoute(ub *message.UpdateBuilder, routeCmd string, isIPv6 bool, _, _ bool) ([]byte, []byte, error) {
	// Parse route command - expects "<prefix> rd <rd> next-hop <addr> label <label> [attributes...]"
	args := strings.Fields(routeCmd)
	if len(args) < 1 {
		return nil, nil, fmt.Errorf("missing route command")
	}

	// Parse using API parser
	parsed, err := route.ParseL3VPNAttributes(args)
	if err != nil {
		return nil, nil, fmt.Errorf("parse error: %w", err)
	}

	// Parse RD once for both NLRI and params
	var rd nlri.RouteDistinguisher
	if parsed.RD != "" {
		rd, err = nlri.ParseRDString(parsed.RD)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid RD: %w", err)
		}
	}

	// Convert L3VPNRoute to VPNParams (pass pre-parsed RD)
	params := l3vpnRouteToVPNParams(parsed, rd)

	// Build UPDATE
	update := ub.BuildVPN(params)

	// Pack UPDATE body using PackTo
	updateBody := message.PackTo(update, nil)

	// Build NLRI for -n flag
	var family nlri.Family
	if isIPv6 {
		family = nlri.IPv6VPN
	} else {
		family = nlri.IPv4VPN
	}
	var label uint32
	if len(parsed.Labels) > 0 {
		label = parsed.Labels[0]
	}
	vpnNLRI := vpn.NewVPN(family, rd, []uint32{label}, parsed.Prefix, 0)
	nlriBytes := vpnNLRI.Bytes()

	return updateBody, nlriBytes, nil
}

// l3vpnRouteToVPNParams converts L3VPNRoute to VPNParams.
// Takes pre-parsed RD to avoid double parsing.
func l3vpnRouteToVPNParams(r bgptypes.L3VPNRoute, rd nlri.RouteDistinguisher) message.VPNParams {
	attrs := extractAttrsFromWire(r.Wire)

	p := message.VPNParams{
		Prefix:            r.Prefix,
		NextHop:           r.NextHop,
		Origin:            attrs.Origin,
		LocalPreference:   attrs.LocalPreference,
		MED:               attrs.MED,
		ASPath:            attrs.ASPath,
		Communities:       attrs.Communities,
		LargeCommunities:  attrs.LargeCommunities,
		ExtCommunityBytes: attrs.ExtCommunityBytes,
	}

	// Use pre-parsed RD
	rdBytes := rd.Bytes()
	copy(p.RDBytes[:], rdBytes)

	// Labels (copy from route)
	p.Labels = r.Labels

	return p
}

// encodeLabeledUnicastRoute parses and encodes a labeled unicast (nlri-mpls) route command.
// Returns (update body bytes, NLRI bytes, error).
func encodeLabeledUnicastRoute(ub *message.UpdateBuilder, routeCmd string, isIPv6 bool, _, _ bool) ([]byte, []byte, error) {
	// Parse route command - expects "<prefix> next-hop <addr> label <label> [attributes...]"
	args := strings.Fields(routeCmd)
	if len(args) < 1 {
		return nil, nil, fmt.Errorf("missing route command")
	}

	// Parse using API parser
	parsed, err := route.ParseLabeledUnicastAttributes(args)
	if err != nil {
		return nil, nil, fmt.Errorf("parse error: %w", err)
	}

	// Convert to LabeledUnicastParams
	params := labeledUnicastRouteToParams(parsed)

	// Build UPDATE
	update := ub.BuildLabeledUnicast(params)

	// Pack UPDATE body using PackTo
	updateBody := message.PackTo(update, nil)

	// Build NLRI for -n flag
	var family nlri.Family
	if isIPv6 {
		family = nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIMPLSLabel}
	} else {
		family = nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIMPLSLabel}
	}
	var label uint32
	if len(parsed.Labels) > 0 {
		label = parsed.Labels[0]
	}
	labeledNLRI := nlri.NewLabeledUnicast(family, parsed.Prefix, []uint32{label}, parsed.PathID)
	nlriBytes := labeledNLRI.Bytes()

	return updateBody, nlriBytes, nil
}

// labeledUnicastRouteToParams converts LabeledUnicastRoute to LabeledUnicastParams.
func labeledUnicastRouteToParams(r bgptypes.LabeledUnicastRoute) message.LabeledUnicastParams {
	attrs := extractAttrsFromWire(r.Wire)

	p := message.LabeledUnicastParams{
		Prefix:            r.Prefix,
		NextHop:           r.NextHop,
		PathID:            r.PathID,
		Origin:            attrs.Origin,
		LocalPreference:   attrs.LocalPreference,
		MED:               attrs.MED,
		ASPath:            attrs.ASPath,
		Communities:       attrs.Communities,
		LargeCommunities:  attrs.LargeCommunities,
		ExtCommunityBytes: attrs.ExtCommunityBytes,
	}

	// Labels (copy from route)
	p.Labels = r.Labels

	return p
}

// parseMAC parses a MAC address string like "00:11:22:33:44:55".
func parseMAC(s string) ([6]byte, error) {
	var mac [6]byte
	if s == "" {
		return mac, nil
	}

	// Handle different separators
	s = strings.ReplaceAll(s, "-", ":")
	parts := strings.Split(s, ":")
	if len(parts) != 6 {
		return mac, fmt.Errorf("invalid MAC format: %s", s)
	}

	for i, p := range parts {
		b, err := strconv.ParseUint(p, 16, 8)
		if err != nil {
			return mac, fmt.Errorf("invalid MAC byte: %s", p)
		}
		mac[i] = byte(b)
	}

	return mac, nil
}

// encodeFlowSpecRoute parses and encodes a FlowSpec route command.
// Returns (update body bytes, NLRI bytes, error).
func encodeFlowSpecRoute(ub *message.UpdateBuilder, routeCmd string, isIPv6 bool, _, _ bool) ([]byte, []byte, error) {
	// Parse route command - expects "match <spec> then <action>"
	args := strings.Fields(routeCmd)
	if len(args) < 1 {
		return nil, nil, fmt.Errorf("missing FlowSpec command")
	}

	// Parse using API parser
	parsed, err := route.ParseFlowSpecArgs(args)
	if err != nil {
		return nil, nil, fmt.Errorf("parse error: %w", err)
	}

	// Build FlowSpec NLRI
	var family nlri.Family
	if isIPv6 {
		family = nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIFlowSpec}
	} else {
		family = nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIFlowSpec}
	}

	fs := flowspec.NewFlowSpec(family)

	// Add components based on parsed route
	if parsed.DestPrefix != nil {
		fs.AddComponent(flowspec.NewFlowDestPrefixComponent(*parsed.DestPrefix))
	}
	if parsed.SourcePrefix != nil {
		fs.AddComponent(flowspec.NewFlowSourcePrefixComponent(*parsed.SourcePrefix))
	}
	if len(parsed.Protocols) > 0 {
		fs.AddComponent(flowspec.NewFlowIPProtocolComponent(parsed.Protocols...))
	}
	if len(parsed.Ports) > 0 {
		fs.AddComponent(flowspec.NewFlowPortComponent(parsed.Ports...))
	}
	if len(parsed.DestPorts) > 0 {
		fs.AddComponent(flowspec.NewFlowDestPortComponent(parsed.DestPorts...))
	}
	if len(parsed.SourcePorts) > 0 {
		fs.AddComponent(flowspec.NewFlowSourcePortComponent(parsed.SourcePorts...))
	}

	// Get NLRI bytes
	nlriBytes := fs.Bytes()

	// Convert to FlowSpecParams
	params, err := flowSpecRouteToParams(parsed, nlriBytes)
	if err != nil {
		return nil, nil, err
	}

	// Build UPDATE
	update := ub.BuildFlowSpec(params)

	// Pack UPDATE body using PackTo
	updateBody := message.PackTo(update, nil)

	return updateBody, nlriBytes, nil
}

// flowSpecRouteToParams converts FlowSpecRoute to FlowSpecParams.
func flowSpecRouteToParams(r bgptypes.FlowSpecRoute, nlriBytes []byte) (message.FlowSpecParams, error) {
	p := message.FlowSpecParams{
		IsIPv6: r.Family == bgptypes.AFINameIPv6,
		NLRI:   nlriBytes,
	}

	// Convert actions to extended communities
	var extComms []byte

	// Discard action = rate-limit to 0 (RFC 5575)
	if r.Actions.Discard {
		// Traffic-rate with rate=0 means discard
		// Type 0x80, Subtype 0x06, 2 reserved bytes, 4-byte IEEE 754 float (0.0)
		extComms = append(extComms, 0x80, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00)
	}

	// Rate-limit action (RFC 5575)
	if r.Actions.RateLimit > 0 {
		// Traffic-rate extended community
		// Type 0x80, Subtype 0x06, 2 reserved bytes, 4-byte IEEE 754 float
		rate := float32(r.Actions.RateLimit)
		bits := floatToIEEE754(rate)
		extComms = append(extComms, 0x80, 0x06, 0x00, 0x00)
		extComms = append(extComms, byte(bits>>24), byte(bits>>16), byte(bits>>8), byte(bits))
	}

	// DSCP marking (RFC 5575)
	if r.Actions.MarkDSCP > 0 {
		// Traffic-marking extended community
		// Type 0x80, Subtype 0x09, 6 bytes with DSCP in last byte
		extComms = append(extComms, 0x80, 0x09, 0x00, 0x00, 0x00, 0x00, 0x00, r.Actions.MarkDSCP)
	}

	// Redirect action (RFC 5575/7674)
	if r.Actions.Redirect != "" {
		ec, err := parseRedirectTarget(r.Actions.Redirect)
		if err != nil {
			return p, fmt.Errorf("invalid redirect: %w", err)
		}
		extComms = append(extComms, ec[:]...)
	}

	p.ExtCommunityBytes = extComms

	return p, nil
}

// floatToIEEE754 converts a float32 to IEEE 754 bits.
func floatToIEEE754(f float32) uint32 {
	// Use math.Float32bits for proper IEEE 754 conversion
	return math.Float32bits(f)
}

// encodeVPLSRoute parses and encodes a VPLS route command.
// Returns (update body bytes, NLRI bytes, error).
func encodeVPLSRoute(ub *message.UpdateBuilder, routeCmd string, _, _ bool) ([]byte, []byte, error) {
	// Parse route command
	args := strings.Fields(routeCmd)
	if len(args) < 1 {
		return nil, nil, fmt.Errorf("missing VPLS command")
	}

	// Parse using API parser
	parsed, err := parseVPLSArgs(args)
	if err != nil {
		return nil, nil, fmt.Errorf("parse error: %w", err)
	}

	// Parse RD
	rd, err := nlri.ParseRDString(parsed.RD)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid RD: %w", err)
	}

	// Convert to VPLSParams
	params := vplsRouteToParams(parsed, rd)

	// Build UPDATE
	update := ub.BuildVPLS(params)

	// Pack UPDATE body using PackTo
	updateBody := message.PackTo(update, nil)

	// For -n flag, build VPLS NLRI
	vplsNLRI := nlri.NewVPLSFull(rd, parsed.VEBlockOffset, parsed.VEBlockOffset, parsed.VEBlockSize, parsed.LabelBase)
	nlriBytes := vplsNLRI.Bytes()

	return updateBody, nlriBytes, nil
}

// encodeMUPRoute parses and encodes a MUP route command.
// Returns (update body bytes, NLRI bytes, error).
func encodeMUPRoute(ub *message.UpdateBuilder, routeCmd string, isIPv6 bool, _, _ bool) ([]byte, []byte, error) {
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
	var routeType nlri.MUPRouteType
	switch spec.RouteType {
	case route.MUPRouteTypeISD:
		routeType = nlri.MUPISD
	case route.MUPRouteTypeDSD:
		routeType = nlri.MUPDSD
	case route.MUPRouteTypeT1ST:
		routeType = nlri.MUPT1ST
	case route.MUPRouteTypeT2ST:
		routeType = nlri.MUPT2ST
	default:
		return nil, 0, fmt.Errorf("unknown MUP route type: %s", spec.RouteType)
	}

	// Parse RD
	var rd nlri.RouteDistinguisher
	if spec.RD != "" {
		parsed, err := nlri.ParseRDString(spec.RD)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid RD %q: %w", spec.RD, err)
		}
		rd = parsed
	}

	// Build route-type-specific data
	var data []byte
	switch routeType {
	case nlri.MUPISD:
		if spec.Prefix == "" {
			return nil, 0, fmt.Errorf("MUP ISD requires prefix")
		}
		prefix, err := netip.ParsePrefix(spec.Prefix)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid ISD prefix %q: %w", spec.Prefix, err)
		}
		data = buildMUPPrefixBytes(prefix)

	case nlri.MUPDSD:
		if spec.Address == "" {
			return nil, 0, fmt.Errorf("MUP DSD requires address")
		}
		addr, err := netip.ParseAddr(spec.Address)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid DSD address %q: %w", spec.Address, err)
		}
		data = addr.AsSlice()

	case nlri.MUPT1ST:
		if spec.Prefix == "" {
			return nil, 0, fmt.Errorf("MUP T1ST requires prefix")
		}
		prefix, err := netip.ParsePrefix(spec.Prefix)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid T1ST prefix %q: %w", spec.Prefix, err)
		}
		data = buildMUPPrefixBytes(prefix)

	case nlri.MUPT2ST:
		if spec.Address == "" {
			return nil, 0, fmt.Errorf("MUP T2ST requires address")
		}
		ep, err := netip.ParseAddr(spec.Address)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid T2ST endpoint %q: %w", spec.Address, err)
		}
		epBytes := ep.AsSlice()
		data = append(data, byte(len(epBytes)*8))
		data = append(data, epBytes...)
	}

	// Determine AFI
	afi := nlri.AFIIPv4
	if spec.IsIPv6 {
		afi = nlri.AFIIPv6
	}

	mup := nlri.NewMUPFull(afi, nlri.MUPArch3GPP5G, routeType, rd, data)
	nlriBytes := mup.Bytes()

	return nlriBytes, uint8(routeType), nil //nolint:gosec // MUP route type is always 0-3
}

// buildMUPPrefixBytes encodes a prefix for MUP NLRI.
func buildMUPPrefixBytes(prefix netip.Prefix) []byte {
	bits := prefix.Bits()
	addr := prefix.Addr()
	addrBytes := addr.AsSlice()
	prefixBytes := (bits + 7) / 8
	result := make([]byte, 1+prefixBytes)
	result[0] = byte(bits)
	copy(result[1:], addrBytes[:prefixBytes])
	return result
}

// parseVPLSArgs parses VPLS command arguments for encode command.
// Format: rd <rd> ve-block-offset <n> ve-block-size <n> label <n> next-hop <addr>.
func parseVPLSArgs(args []string) (bgptypes.VPLSRoute, error) {
	var route bgptypes.VPLSRoute

	for i := 0; i < len(args)-1; i += 2 {
		key := strings.ToLower(args[i])
		value := args[i+1]

		switch key {
		case "rd":
			route.RD = value
		case "ve-block-offset":
			n, err := strconv.ParseUint(value, 10, 16)
			if err != nil {
				return route, fmt.Errorf("invalid ve-block-offset: %s", value)
			}
			route.VEBlockOffset = uint16(n)
		case "ve-block-size":
			n, err := strconv.ParseUint(value, 10, 16)
			if err != nil {
				return route, fmt.Errorf("invalid ve-block-size: %s", value)
			}
			route.VEBlockSize = uint16(n)
		case "label-base", "label":
			n, err := strconv.ParseUint(value, 10, 32)
			if err != nil {
				return route, fmt.Errorf("invalid label: %s", value)
			}
			route.LabelBase = uint32(n)
		case "next-hop":
			nh, err := netip.ParseAddr(value)
			if err != nil {
				return route, fmt.Errorf("invalid next-hop: %s", value)
			}
			route.NextHop = nh

		default:
			return route, fmt.Errorf("unknown vpls keyword: %s", key)
		}
	}

	if route.RD == "" {
		return route, fmt.Errorf("missing route-distinguisher")
	}

	return route, nil
}

// parseL2VPNArgs parses L2VPN/EVPN command arguments for encode command.
//
//nolint:goconst // String literals are clearer for route type parsing
func parseL2VPNArgs(args []string) (bgptypes.L2VPNRoute, error) {
	var route bgptypes.L2VPNRoute

	if len(args) < 1 {
		return route, fmt.Errorf("missing route type")
	}

	// First argument is route type
	routeType := strings.ToLower(args[0])
	switch routeType {
	case "mac-ip", "macip", "type2":
		route.RouteType = "mac-ip"
	case "ip-prefix", "ipprefix", "type5":
		route.RouteType = "ip-prefix"
	case "multicast", "inclusive-multicast", "type3":
		route.RouteType = "multicast"
	case "ethernet-segment", "es", "type4":
		route.RouteType = "ethernet-segment"
	case "ethernet-ad", "ead", "type1":
		route.RouteType = "ethernet-ad"
	default:
		return route, fmt.Errorf("invalid route type: %s", routeType)
	}

	// Parse remaining key-value pairs
	for i := 1; i < len(args)-1; i += 2 {
		key := strings.ToLower(args[i])
		value := args[i+1]

		switch key {
		case "rd":
			route.RD = value
		case "esi":
			route.ESI = value
		case "ethernet-tag", "etag":
			n, err := strconv.ParseUint(value, 10, 32)
			if err != nil {
				return route, fmt.Errorf("invalid ethernet-tag: %s", value)
			}
			route.EthernetTag = uint32(n)
		case "mac":
			route.MAC = value
		case "ip":
			ip, err := netip.ParseAddr(value)
			if err != nil {
				return route, fmt.Errorf("invalid ip: %s", value)
			}
			route.IP = ip
		case "prefix":
			prefix, err := netip.ParsePrefix(value)
			if err != nil {
				return route, fmt.Errorf("invalid prefix: %s", value)
			}
			route.Prefix = prefix
		case "gateway", "gw":
			gw, err := netip.ParseAddr(value)
			if err != nil {
				return route, fmt.Errorf("invalid gateway: %s", value)
			}
			route.Gateway = gw
		case "label", "label1":
			n, err := strconv.ParseUint(value, 10, 32)
			if err != nil {
				return route, fmt.Errorf("invalid label: %s", value)
			}
			route.Label1 = uint32(n)
		case "label2":
			n, err := strconv.ParseUint(value, 10, 32)
			if err != nil {
				return route, fmt.Errorf("invalid label2: %s", value)
			}
			route.Label2 = uint32(n)
		case "next-hop":
			nh, err := netip.ParseAddr(value)
			if err != nil {
				return route, fmt.Errorf("invalid next-hop: %s", value)
			}
			route.NextHop = nh

		default:
			return route, fmt.Errorf("unknown l2vpn keyword: %s", key)
		}
	}

	// Validate required fields based on route type
	if route.RD == "" {
		return route, fmt.Errorf("missing route-distinguisher")
	}

	if route.RouteType == "mac-ip" && route.MAC == "" {
		return route, fmt.Errorf("missing mac address")
	}

	if route.RouteType == "ip-prefix" && !route.Prefix.IsValid() {
		return route, fmt.Errorf("missing prefix")
	}

	return route, nil
}

// vplsRouteToParams converts VPLSRoute to VPLSParams.
func vplsRouteToParams(r bgptypes.VPLSRoute, rd nlri.RouteDistinguisher) message.VPLSParams {
	p := message.VPLSParams{
		NextHop:  r.NextHop,
		Offset:   r.VEBlockOffset,
		Size:     r.VEBlockSize,
		Base:     r.LabelBase,
		Endpoint: r.VEBlockOffset, // VE ID typically matches offset
		Origin:   attribute.OriginIGP,
	}

	// Copy RD bytes
	rdBytes := rd.Bytes()
	copy(p.RD[:], rdBytes)

	return p
}

// parseRedirectTarget parses a redirect target in ASN:value format.
// Supports both 2-byte ASN (RFC 5575) and 4-byte ASN (RFC 7674).
func parseRedirectTarget(s string) ([8]byte, error) {
	var ec [8]byte
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return ec, fmt.Errorf("invalid redirect format: %s", s)
	}

	asn, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return ec, fmt.Errorf("invalid ASN in redirect: %s", parts[0])
	}

	if asn <= 65535 {
		// 2-byte ASN format (RFC 5575)
		// Type 0x80, Subtype 0x08, 2-byte ASN, 4-byte local value
		target, err := strconv.ParseUint(parts[1], 10, 32)
		if err != nil {
			return ec, fmt.Errorf("invalid target in redirect: %s", parts[1])
		}
		ec[0] = 0x80
		ec[1] = 0x08
		ec[2] = byte(asn >> 8)
		ec[3] = byte(asn)
		ec[4] = byte(target >> 24)
		ec[5] = byte(target >> 16)
		ec[6] = byte(target >> 8)
		ec[7] = byte(target)
	} else {
		// 4-byte ASN format (RFC 7674)
		// Type 0x82, Subtype 0x08, 4-byte ASN, 2-byte local value
		target, err := strconv.ParseUint(parts[1], 10, 16)
		if err != nil {
			return ec, fmt.Errorf("invalid target in redirect (4-byte ASN max 16-bit local): %s", parts[1])
		}
		ec[0] = 0x82
		ec[1] = 0x08
		ec[2] = byte(asn >> 24)
		ec[3] = byte(asn >> 16)
		ec[4] = byte(asn >> 8)
		ec[5] = byte(asn)
		ec[6] = byte(target >> 8)
		ec[7] = byte(target)
	}

	return ec, nil
}
