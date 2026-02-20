package bgp

import (
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/route"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"
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

	// Encode based on family
	var updateBytes []byte
	var nlriBytes []byte

	switch { //nolint:staticcheck // QF1002: untagged switch avoids exhaustive check; default handles all non-unicast via registry
	case safi == nlri.SAFIUnicast:
		// Unicast is handled locally (not a plugin family)
		// #nosec G115 - localAS is from uint flag, bounded by flag validation
		ub := message.NewUpdateBuilder(uint32(*localAS), isIBGP, *asn4, *pathInfo)
		updateBytes, nlriBytes, err = encodeUnicastRoute(ub, routeCmd, afi == nlri.AFIIPv6, *asn4, *pathInfo)
	default:
		// All other families dispatch via plugin registry
		canonicalFamily := (nlri.Family{AFI: afi, SAFI: safi}).String()
		encoder := registry.RouteEncoderByFamily(canonicalFamily)
		if encoder == nil {
			err = fmt.Errorf("unsupported family: %s", *family)
		} else {
			// #nosec G115 - localAS is from uint flag, bounded by flag validation
			updateBytes, nlriBytes, err = encoder(routeCmd, canonicalFamily, uint32(*localAS), isIBGP, *asn4, *pathInfo)
		}
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
func encodeUnicastRoute(ub *message.UpdateBuilder, routeCmd string, isIPv6, _, addPath bool) ([]byte, []byte, error) {
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
	update := ub.BuildUnicast(&params)

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
	var attrs message.CommonAttrs

	if r.Wire != nil {
		// Extract attributes from wire format
		attrs = message.ExtractAttrsFromWire(r.Wire)
	} else {
		// Use defaults
		attrs = message.CommonAttrs{
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
