package main

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

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/attribute"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/message"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
	"codeberg.org/thomas-mangin/zebgp/pkg/plugin"
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
		_, _ = fmt.Fprintf(encodeStderr, `Usage: zebgp encode [options] [route-command]

Encode API route command to BGP message hex.
Route command can be provided as argument or via stdin.

Options:
`)
		fs.PrintDefaults()
		_, _ = fmt.Fprintf(encodeStderr, `
Examples:
  # IPv4/IPv6 Unicast
  zebgp encode "route 10.0.0.0/24 next-hop 192.168.1.1"
  zebgp encode -f "ipv6/unicast" "route 2001:db8::/32 next-hop 2001:db8::1"

  # L3VPN (mpls-vpn)
  zebgp encode -f "ipv4/mpls-vpn" "10.0.0.0/24 rd 100:1 next-hop 1.2.3.4 label 100"
  zebgp encode -f "ipv4/mpls-vpn" "10.0.0.0/24 rd 1.2.3.4:100 next-hop 1.2.3.4 label 100"

  # Labeled Unicast (nlri-mpls)
  zebgp encode -f "ipv4/nlri-mpls" "10.0.0.0/24 next-hop 1.2.3.4 label 100"

  # EVPN
  zebgp encode -f "l2vpn/evpn" "mac-ip rd 100:1 esi 0 etag 0 mac 00:11:22:33:44:55 label 100 next-hop 1.2.3.4"
  zebgp encode -f "l2vpn/evpn" "ip-prefix rd 100:1 esi 0 etag 0 prefix 10.0.0.0/24 gateway 0.0.0.0 label 100 next-hop 1.2.3.4"
  zebgp encode -f "l2vpn/evpn" "multicast rd 100:1 etag 0 next-hop 1.2.3.4"

  # Output options
  zebgp encode -n "route 10.0.0.0/24 next-hop 1.2.3.4"       # NLRI only
  zebgp encode --no-header "route 10.0.0.0/24 next-hop 1.2.3.4"  # No BGP header
  echo "route 10.0.0.0/24 next-hop 1.2.3.4" | zebgp encode   # stdin
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

	// Build encoding context
	ctx := &nlri.PackContext{
		ASN4:    *asn4,
		AddPath: *pathInfo,
	}

	// Create UpdateBuilder
	// #nosec G115 - localAS is from uint flag, bounded by flag validation
	ub := message.NewUpdateBuilder(uint32(*localAS), isIBGP, ctx)

	// Encode based on family
	var updateBytes []byte
	var nlriBytes []byte

	switch {
	case afi == nlri.AFIIPv4 && safi == nlri.SAFIUnicast:
		updateBytes, nlriBytes, err = encodeUnicastRoute(ub, routeCmd, false, ctx)
	case afi == nlri.AFIIPv6 && safi == nlri.SAFIUnicast:
		updateBytes, nlriBytes, err = encodeUnicastRoute(ub, routeCmd, true, ctx)
	case afi == nlri.AFIIPv4 && safi == nlri.SAFIVPN:
		updateBytes, nlriBytes, err = encodeL3VPNRoute(ub, routeCmd, false, ctx)
	case afi == nlri.AFIIPv6 && safi == nlri.SAFIVPN:
		updateBytes, nlriBytes, err = encodeL3VPNRoute(ub, routeCmd, true, ctx)
	case afi == nlri.AFIIPv4 && safi == nlri.SAFIMPLSLabel:
		updateBytes, nlriBytes, err = encodeLabeledUnicastRoute(ub, routeCmd, false, ctx)
	case afi == nlri.AFIIPv6 && safi == nlri.SAFIMPLSLabel:
		updateBytes, nlriBytes, err = encodeLabeledUnicastRoute(ub, routeCmd, true, ctx)
	case afi == nlri.AFIL2VPN && safi == nlri.SAFIEVPN:
		updateBytes, nlriBytes, err = encodeEVPNRoute(ub, routeCmd, ctx)
	case safi == nlri.SAFIFlowSpec:
		updateBytes, nlriBytes, err = encodeFlowSpecRoute(ub, routeCmd, afi == nlri.AFIIPv6, ctx)
	case afi == nlri.AFIL2VPN && safi == nlri.SAFIVPLS:
		updateBytes, nlriBytes, err = encodeVPLSRoute(ub, routeCmd, ctx)
	case safi == nlri.SAFIMUP:
		updateBytes, nlriBytes, err = encodeMUPRoute(ub, routeCmd, afi == nlri.AFIIPv6, ctx)
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
func encodeUnicastRoute(ub *message.UpdateBuilder, routeCmd string, isIPv6 bool, ctx *nlri.PackContext) ([]byte, []byte, error) {
	// Parse route command - expects "route <prefix> next-hop <addr> [attributes...]"
	args := strings.Fields(routeCmd)
	if len(args) < 1 || args[0] != "route" {
		return nil, nil, fmt.Errorf("expected 'route' keyword, got: %s", routeCmd)
	}

	// Parse using API parser
	parsed, err := plugin.ParseRouteAttributes(args[1:], plugin.UnicastKeywords)
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
		nlriLen := nlri.LenWithContext(inet, ctx)
		nlriBytes = make([]byte, nlriLen)
		nlri.WriteNLRI(inet, nlriBytes, 0, ctx)
	} else {
		// For IPv4, NLRI is inline
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, parsed.Route.Prefix, 0)
		nlriLen := nlri.LenWithContext(inet, ctx)
		nlriBytes = make([]byte, nlriLen)
		nlri.WriteNLRI(inet, nlriBytes, 0, ctx)
	}

	// Pack UPDATE body - create minimal Negotiated for packing
	neg := &message.Negotiated{
		ASN4:            ctx.ASN4,
		ExtendedMessage: false,
	}
	updateBody, err := update.Pack(neg)
	if err != nil {
		return nil, nil, fmt.Errorf("pack update: %w", err)
	}

	return updateBody, nlriBytes, nil
}

// routeSpecToUnicastParams converts a RouteSpec to UnicastParams.
// Extracts address from RouteNextHop (must be explicit, not self).
func routeSpecToUnicastParams(r plugin.RouteSpec) message.UnicastParams {
	attrs := extractCommonAttrs(r.Origin, r.LocalPreference, r.MED, r.ASPath,
		r.Communities, r.LargeCommunities, r.ExtendedCommunities)

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

// packExtendedCommunities packs extended communities to wire format.
// ExtendedCommunity is [8]byte, so we just copy the bytes directly.
func packExtendedCommunities(comms []attribute.ExtendedCommunity) []byte {
	if len(comms) == 0 {
		return nil
	}
	buf := make([]byte, len(comms)*8)
	for i, c := range comms {
		copy(buf[i*8:], c[:])
	}
	return buf
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

// extractCommonAttrs extracts common attributes from API route types.
// Handles: Origin, LocalPreference, MED, ASPath, Communities, LargeCommunities, ExtendedCommunities.
func extractCommonAttrs(
	origin *uint8,
	localPref *uint32,
	med *uint32,
	asPath []uint32,
	communities []uint32,
	largeCommunities []plugin.LargeCommunity,
	extCommunities []attribute.ExtendedCommunity,
) commonAttrs {
	attrs := commonAttrs{
		Origin:      attribute.OriginIGP,
		ASPath:      asPath,
		Communities: communities,
	}

	if origin != nil {
		attrs.Origin = attribute.Origin(*origin)
	}
	if localPref != nil {
		attrs.LocalPreference = *localPref
	}
	if med != nil {
		attrs.MED = *med
	}

	if len(largeCommunities) > 0 {
		attrs.LargeCommunities = make([][3]uint32, len(largeCommunities))
		for i, lc := range largeCommunities {
			attrs.LargeCommunities[i] = [3]uint32{lc.GlobalAdmin, lc.LocalData1, lc.LocalData2}
		}
	}

	if len(extCommunities) > 0 {
		attrs.ExtCommunityBytes = packExtendedCommunities(extCommunities)
	}

	return attrs
}

// encodeEVPNRoute parses and encodes an EVPN route command.
// Returns (update body bytes, NLRI bytes, error).
func encodeEVPNRoute(ub *message.UpdateBuilder, routeCmd string, ctx *nlri.PackContext) ([]byte, []byte, error) {
	// Parse route command - expects "mac-ip|ip-prefix|... <args>"
	args := strings.Fields(routeCmd)
	if len(args) < 1 {
		return nil, nil, fmt.Errorf("missing EVPN route type")
	}

	// Parse using API parser
	parsed, err := plugin.ParseL2VPNArgs(args)
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

	// Pack UPDATE body
	neg := &message.Negotiated{
		ASN4:            ctx.ASN4,
		ExtendedMessage: false,
	}
	updateBody, err := update.Pack(neg)
	if err != nil {
		return nil, nil, fmt.Errorf("pack update: %w", err)
	}

	// For NLRI bytes, we need to build and pack the NLRI
	var evpnNLRI nlri.EVPN
	switch params.RouteType {
	case 1:
		evpnNLRI = nlri.NewEVPNType1(params.RD, params.ESI, params.EthernetTag, params.Labels)
	case 2:
		evpnNLRI = nlri.NewEVPNType2(params.RD, params.ESI, params.EthernetTag, params.MAC, params.IP, params.Labels)
	case 3:
		evpnNLRI = nlri.NewEVPNType3(params.RD, params.EthernetTag, params.OriginatorIP)
	case 4:
		evpnNLRI = nlri.NewEVPNType4(params.RD, params.ESI, params.OriginatorIP)
	case 5:
		evpnNLRI = nlri.NewEVPNType5(params.RD, params.ESI, params.EthernetTag, params.Prefix, params.Gateway, params.Labels)
	}
	nlriLen := nlri.LenWithContext(evpnNLRI, ctx)
	nlriBytes := make([]byte, nlriLen)
	nlri.WriteNLRI(evpnNLRI, nlriBytes, 0, ctx)

	return updateBody, nlriBytes, nil
}

// l2vpnRouteToEVPNParams converts L2VPNRoute to EVPNParams.
func l2vpnRouteToEVPNParams(r plugin.L2VPNRoute) (message.EVPNParams, error) {
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
	esi, err := nlri.ParseESIString(r.ESI)
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
func encodeL3VPNRoute(ub *message.UpdateBuilder, routeCmd string, isIPv6 bool, ctx *nlri.PackContext) ([]byte, []byte, error) {
	// Parse route command - expects "<prefix> rd <rd> next-hop <addr> label <label> [attributes...]"
	args := strings.Fields(routeCmd)
	if len(args) < 1 {
		return nil, nil, fmt.Errorf("missing route command")
	}

	// Parse using API parser
	parsed, err := plugin.ParseL3VPNAttributes(args)
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

	// Pack UPDATE body
	neg := &message.Negotiated{
		ASN4:            ctx.ASN4,
		ExtendedMessage: false,
	}
	updateBody, err := update.Pack(neg)
	if err != nil {
		return nil, nil, fmt.Errorf("pack update: %w", err)
	}

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
	vpnNLRI := nlri.NewIPVPN(family, rd, []uint32{label}, parsed.Prefix, 0)
	nlriBytes := vpnNLRI.Bytes()

	return updateBody, nlriBytes, nil
}

// l3vpnRouteToVPNParams converts L3VPNRoute to VPNParams.
// Takes pre-parsed RD to avoid double parsing.
func l3vpnRouteToVPNParams(r plugin.L3VPNRoute, rd nlri.RouteDistinguisher) message.VPNParams {
	attrs := extractCommonAttrs(r.Origin, r.LocalPreference, r.MED, r.ASPath,
		r.Communities, r.LargeCommunities, r.ExtendedCommunities)

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
func encodeLabeledUnicastRoute(ub *message.UpdateBuilder, routeCmd string, isIPv6 bool, ctx *nlri.PackContext) ([]byte, []byte, error) {
	// Parse route command - expects "<prefix> next-hop <addr> label <label> [attributes...]"
	args := strings.Fields(routeCmd)
	if len(args) < 1 {
		return nil, nil, fmt.Errorf("missing route command")
	}

	// Parse using API parser
	parsed, err := plugin.ParseLabeledUnicastAttributes(args)
	if err != nil {
		return nil, nil, fmt.Errorf("parse error: %w", err)
	}

	// Convert to LabeledUnicastParams
	params := labeledUnicastRouteToParams(parsed)

	// Build UPDATE
	update := ub.BuildLabeledUnicast(params)

	// Pack UPDATE body
	neg := &message.Negotiated{
		ASN4:            ctx.ASN4,
		ExtendedMessage: false,
	}
	updateBody, err := update.Pack(neg)
	if err != nil {
		return nil, nil, fmt.Errorf("pack update: %w", err)
	}

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
func labeledUnicastRouteToParams(r plugin.LabeledUnicastRoute) message.LabeledUnicastParams {
	attrs := extractCommonAttrs(r.Origin, r.LocalPreference, r.MED, r.ASPath,
		r.Communities, r.LargeCommunities, r.ExtendedCommunities)

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
func encodeFlowSpecRoute(ub *message.UpdateBuilder, routeCmd string, isIPv6 bool, ctx *nlri.PackContext) ([]byte, []byte, error) {
	// Parse route command - expects "match <spec> then <action>"
	args := strings.Fields(routeCmd)
	if len(args) < 1 {
		return nil, nil, fmt.Errorf("missing FlowSpec command")
	}

	// Parse using API parser
	parsed, err := plugin.ParseFlowSpecArgs(args)
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

	fs := nlri.NewFlowSpec(family)

	// Add components based on parsed route
	if parsed.DestPrefix != nil {
		fs.AddComponent(nlri.NewFlowDestPrefixComponent(*parsed.DestPrefix))
	}
	if parsed.SourcePrefix != nil {
		fs.AddComponent(nlri.NewFlowSourcePrefixComponent(*parsed.SourcePrefix))
	}
	if len(parsed.Protocols) > 0 {
		fs.AddComponent(nlri.NewFlowIPProtocolComponent(parsed.Protocols...))
	}
	if len(parsed.Ports) > 0 {
		fs.AddComponent(nlri.NewFlowPortComponent(parsed.Ports...))
	}
	if len(parsed.DestPorts) > 0 {
		fs.AddComponent(nlri.NewFlowDestPortComponent(parsed.DestPorts...))
	}
	if len(parsed.SourcePorts) > 0 {
		fs.AddComponent(nlri.NewFlowSourcePortComponent(parsed.SourcePorts...))
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

	// Pack UPDATE body
	neg := &message.Negotiated{
		ASN4:            ctx.ASN4,
		ExtendedMessage: false,
	}
	updateBody, err := update.Pack(neg)
	if err != nil {
		return nil, nil, fmt.Errorf("pack update: %w", err)
	}

	return updateBody, nlriBytes, nil
}

// flowSpecRouteToParams converts FlowSpecRoute to FlowSpecParams.
func flowSpecRouteToParams(r plugin.FlowSpecRoute, nlriBytes []byte) (message.FlowSpecParams, error) {
	p := message.FlowSpecParams{
		IsIPv6: r.Family == plugin.AFINameIPv6,
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
func encodeVPLSRoute(ub *message.UpdateBuilder, routeCmd string, ctx *nlri.PackContext) ([]byte, []byte, error) {
	// Parse route command
	args := strings.Fields(routeCmd)
	if len(args) < 1 {
		return nil, nil, fmt.Errorf("missing VPLS command")
	}

	// Parse using API parser
	parsed, err := plugin.ParseVPLSArgs(args)
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

	// Pack UPDATE body
	neg := &message.Negotiated{
		ASN4:            ctx.ASN4,
		ExtendedMessage: false,
	}
	updateBody, err := update.Pack(neg)
	if err != nil {
		return nil, nil, fmt.Errorf("pack update: %w", err)
	}

	// For -n flag, build VPLS NLRI
	vplsNLRI := nlri.NewVPLSFull(rd, parsed.VEBlockOffset, parsed.VEBlockOffset, parsed.VEBlockSize, parsed.LabelBase)
	nlriBytes := vplsNLRI.Bytes()

	return updateBody, nlriBytes, nil
}

// encodeMUPRoute parses and encodes a MUP route command.
// Returns (update body bytes, NLRI bytes, error).
func encodeMUPRoute(ub *message.UpdateBuilder, routeCmd string, isIPv6 bool, ctx *nlri.PackContext) ([]byte, []byte, error) {
	// Parse route command
	args := strings.Fields(routeCmd)
	if len(args) < 1 {
		return nil, nil, fmt.Errorf("missing MUP command")
	}

	// Parse using API parser
	parsed, err := plugin.ParseMUPArgs(args, isIPv6)
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

	// Pack UPDATE body
	neg := &message.Negotiated{
		ASN4:            ctx.ASN4,
		ExtendedMessage: false,
	}
	updateBody, err := update.Pack(neg)
	if err != nil {
		return nil, nil, fmt.Errorf("pack update: %w", err)
	}

	return updateBody, nlriBytes, nil
}

// buildMUPNLRI builds MUP NLRI bytes from MUPRouteSpec.
// Returns (nlri bytes, route type code, error).
func buildMUPNLRI(spec plugin.MUPRouteSpec) ([]byte, uint8, error) {
	// Determine route type code
	var routeType nlri.MUPRouteType
	switch spec.RouteType {
	case plugin.MUPRouteTypeISD:
		routeType = nlri.MUPISD
	case plugin.MUPRouteTypeDSD:
		routeType = nlri.MUPDSD
	case plugin.MUPRouteTypeT1ST:
		routeType = nlri.MUPT1ST
	case plugin.MUPRouteTypeT2ST:
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
	nlriBytes := mup.Pack(nil)

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

// vplsRouteToParams converts VPLSRoute to VPLSParams.
func vplsRouteToParams(r plugin.VPLSRoute, rd nlri.RouteDistinguisher) message.VPLSParams {
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
