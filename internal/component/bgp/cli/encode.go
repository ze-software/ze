// Design: docs/architecture/wire/messages.md — UPDATE encoding from CLI args
// Related: decode_answer.go — the input routes and the shared leaf helpers

package cli

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/route"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// encodeStdout and encodeStderr are where the root `ze bgp encode` route
// prints, and encodeStdin is where the local route reads a route given on no
// line; tests replace them. encodeStdinIsTTY is mockable for testing. The
// show bgp encode handler never writes to either writer.
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

// The YANG leaves of show bgp encode that select what is answered, and the
// names of the message parts the answer can carry.
const (
	leafEncodeFamily       = "family"
	leafEncodeLocalAS      = "local-as"
	leafEncodePeerAS       = "peer-as"
	leafEncodePathID       = "path-id"
	leafEncodePart         = "part"
	leafEncodeASNWidth     = "asn-width"
	leafEncodeAnnouncement = "announcement"

	encodePartMessage = "message"
	encodePartBody    = "body"
	encodePartNLRI    = "nlri"
)

// encodeAnswer is the answer of show bgp encode: one envelope key, so the text
// drawing registered under it renders the text route.
type encodeAnswer struct {
	Encode encodeResult `json:"bgp-encode"`
}

// encodeResult is one encoded route: the family it was encoded for, which
// part of the message is carried, and that part in uppercase hexadecimal.
type encodeResult struct {
	Family string `json:"family"`
	Part   string `json:"part"`
	Hex    string `json:"hex"`
}

// drawEncode is the text drawing of an encode: the hexadecimal on one line, as
// the operator and every script of this tool have always read it.
func drawEncode(result *encodeResult) string {
	return result.Hex + "\n"
}

// drawEncodeText is the text renderer registered under keyEncode.
func drawEncodeText(raw json.RawMessage) (string, error) {
	var result encodeResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	return drawEncode(&result), nil
}

// dataEncode is the show bgp encode handler. It writes nothing: every word it
// has for the operator is in the answer.
func dataEncode(validated command.ValidatedArgs) command.LocalAnswer {
	return answerEncode(validated, restWords(validated, leafEncodeAnnouncement), inputRouteLocal)
}

// cmdEncode is the root `ze bgp encode` route: it answers through the same
// handler body and prints the hexadecimal drawing on encodeStdout and the
// Diagnostic on encodeStderr.
func cmdEncode(args []string) int {
	answer := answerEncode(command.ValidatedArgs{}, args, inputRouteLocal)
	_, _ = io.WriteString(encodeStderr, answer.Diagnostic) //nolint:errcheck // CLI diagnostic
	if answer.Payload == nil {
		return answer.Code
	}
	encoded, ok := answer.Payload.(encodeAnswer)
	if !ok {
		panic("BUG: cmdEncode: the encode handler answered a foreign payload")
	}
	_, _ = io.WriteString(encodeStdout, drawEncode(&encoded.Encode)) //nolint:errcheck // CLI output
	return answer.Code
}

// encodeRequest is the encode the words and the bound leaves asked for.
type encodeRequest struct {
	family   string
	localAS  uint
	peerAS   uint
	pathInfo bool
	nlriOnly bool
	noHeader bool
	asn4     bool
	route    string
}

// answerEncode answers an encode. validated carries the leaves the model bound
// by keyword; words carries the rest of the line, offline flags included. The
// root route passes the zero ValidatedArgs, which binds none.
func answerEncode(validated command.ValidatedArgs, words []string, route inputRoute) command.LocalAnswer {
	request, refusal := parseEncode(validated, words, route)
	if refusal != nil {
		return *refusal
	}

	afi, safi, err := parseEncodingFamily(request.family)
	if err != nil {
		return command.LocalRefusal(1, command.LocalErrorLine(err))
	}

	// Determine iBGP vs eBGP
	isIBGP := request.localAS == request.peerAS

	var updateBytes []byte
	var nlriBytes []byte

	//exhaustive:ignore // Unicast is local; all other families dispatch through the plugin registry.
	switch safi {
	case family.SAFIUnicast:
		// Unicast is handled locally (not a plugin family)
		// #nosec G115 - localAS is from uint flag, bounded by flag validation
		ub := message.GetUpdateBuilder(uint32(request.localAS), isIBGP, request.asn4, request.pathInfo)
		defer message.PutUpdateBuilder(ub)
		updateBytes, nlriBytes, err = encodeUnicastRoute(ub, request.route, afi == family.AFIIPv6, request.asn4, request.pathInfo)
	default:
		// All other families dispatch via plugin registry
		canonicalFamily := (family.Family{AFI: afi, SAFI: safi}).String()
		encoder := registry.RouteEncoderByFamily(canonicalFamily)
		if encoder == nil {
			err = errors.New("unsupported family: " + request.family)
		} else {
			// #nosec G115 - localAS is from uint flag, bounded by flag validation
			updateBytes, nlriBytes, err = encoder(request.route, canonicalFamily, uint32(request.localAS), isIBGP, request.asn4, request.pathInfo)
		}
	}
	if err != nil {
		return command.LocalRefusal(1, command.LocalErrorLine(err))
	}

	// Note: updateBytes already includes the BGP header (from Update.Pack)
	result := encodeResult{Family: request.family, Part: encodePartMessage}
	output := updateBytes
	switch {
	case request.nlriOnly:
		result.Part = encodePartNLRI
		output = nlriBytes
	case request.noHeader:
		result.Part = encodePartBody
		// Strip the 19-byte BGP header
		if len(updateBytes) > message.HeaderLen {
			output = updateBytes[message.HeaderLen:]
		}
	}
	result.Hex = strings.ToUpper(hex.EncodeToString(output))
	return command.LocalAnswer{Payload: encodeAnswer{Encode: result}}
}

// parseEncode reads the encode the words and the bound leaves ask for. A word
// the flag parser refuses, a help request and a missing route come back as
// the refusal, with the text the parser wrote; nothing reaches stderr.
func parseEncode(validated command.ValidatedArgs, words []string, route inputRoute) (encodeRequest, *command.LocalAnswer) {
	var diag textbuf.Buffer
	fs := flag.NewFlagSet("encode", flag.ContinueOnError)
	fs.SetOutput(&diag)

	fam := fs.String("f", "ipv4/unicast", "address family (e.g., 'ipv4/unicast', 'ipv6/unicast', 'l2vpn/evpn')")
	localAS := fs.Uint("a", 65533, "local AS number")
	peerAS := fs.Uint("z", 65533, "peer AS number")
	pathInfo := fs.Bool("i", false, "enable ADD-PATH (include path-id)")
	nlriOnly := fs.Bool("n", false, "output only NLRI bytes")
	noHeader := fs.Bool("no-header", false, "exclude 19-byte BGP header")
	asn4 := fs.Bool("asn4", true, "use 4-byte ASN encoding")
	fs.Usage = func() { writeEncodeUsage(&diag) }

	// Every refusal of the parser exits 1, the help page included, as it
	// always did.
	if err := fs.Parse(words); err != nil {
		refusal := command.LocalRefusal(1, diag.String())
		return encodeRequest{}, &refusal
	}
	if refusal := applyEncodeLeaves(fs, validated); refusal != nil {
		return encodeRequest{}, refusal
	}

	request := encodeRequest{
		family: *fam, localAS: *localAS, peerAS: *peerAS,
		pathInfo: *pathInfo, nlriOnly: *nlriOnly, noHeader: *noHeader, asn4: *asn4,
	}
	if fs.NArg() > 0 {
		request.route = textbuf.Join(fs.Args(), " ")
		return request, nil
	}

	// No route on the line: the local process reads it from a piped standard
	// input, and a terminal would block forever waiting for one.
	if !route.readsStdin() {
		refusal := command.LocalRefusal(1, "error: missing route command: give the route after the command; "+
			"reading it from standard input needs 'ze "+pathEncode+"' on the host that holds it\n")
		return encodeRequest{}, &refusal
	}
	if encodeStdinIsTTY() {
		diag.Str("error: missing route command\n")
		writeEncodeUsage(&diag)
		refusal := command.LocalRefusal(1, diag.String())
		return encodeRequest{}, &refusal
	}
	input, err := io.ReadAll(encodeStdin)
	if err != nil {
		diag.Str("error reading stdin: ").Err(err).Byte('\n')
		refusal := command.LocalRefusal(1, diag.String())
		return encodeRequest{}, &refusal
	}
	request.route = strings.TrimSpace(string(input))
	if request.route == "" {
		diag.Str("error: missing route command\n")
		writeEncodeUsage(&diag)
		refusal := command.LocalRefusal(1, diag.String())
		return encodeRequest{}, &refusal
	}
	return request, nil
}

// applyEncodeLeaves sets the options the keyword leaves stand for. A choice
// given both as a keyword and as an option is refused rather than one of the
// two silently winning.
func applyEncodeLeaves(fs *flag.FlagSet, validated command.ValidatedArgs) *command.LocalAnswer {
	given := givenFlags(fs)
	type binding struct {
		leaf  string
		flags []string
		set   func(value string) map[string]string
	}
	bindings := []binding{
		{leafEncodeFamily, []string{"f"}, func(v string) map[string]string { return map[string]string{"f": v} }},
		{leafEncodeLocalAS, []string{"a"}, func(v string) map[string]string { return map[string]string{"a": v} }},
		{leafEncodePeerAS, []string{"z"}, func(v string) map[string]string { return map[string]string{"z": v} }},
		{leafEncodePathID, []string{"i"}, func(v string) map[string]string {
			return map[string]string{"i": strconv.FormatBool(v == "include")}
		}},
		{leafEncodePart, []string{"n", "no-header"}, func(v string) map[string]string {
			return map[string]string{
				"n":         strconv.FormatBool(v == encodePartNLRI),
				"no-header": strconv.FormatBool(v == encodePartBody),
			}
		}},
		{leafEncodeASNWidth, []string{"asn4"}, func(v string) map[string]string {
			return map[string]string{"asn4": strconv.FormatBool(v == "four-octet")}
		}},
	}
	for _, b := range bindings {
		value, found := validated.Keyed(b.leaf)
		if !found {
			continue
		}
		for _, name := range b.flags {
			if given[name] {
				return givenTwice(b.leaf)
			}
		}
		for name, flagValue := range b.set(value) {
			if err := fs.Set(name, flagValue); err != nil {
				return setRefusal(b.leaf, err)
			}
		}
	}
	return nil
}

// writeEncodeUsage writes the encode help page into diag, uncolored, as it
// always was.
func writeEncodeUsage(diag *textbuf.Buffer) {
	p := helpfmt.Page{
		Command:   "ze bgp encode",
		ShortHelp: "Encode API route command to BGP message hex. Route command can be provided as argument or via stdin",
		Usage:     []string{"ze bgp encode [options] [route-command]"},
		Sections: []helpfmt.HelpSection{
			{Title: helpSectionOptions, Entries: []helpfmt.HelpEntry{
				{Name: "-f <family>", Desc: "Address family (default: ipv4/unicast) (e.g., 'ipv4/unicast', 'ipv6/unicast', 'l2vpn/evpn')"},
				{Name: "-a <asn>", Desc: "Local AS number (default: 65533)"},
				{Name: "-z <asn>", Desc: "Peer AS number (default: 65533)"},
				{Name: "-i", Desc: "Enable ADD-PATH (include path-id)"},
				{Name: "-n", Desc: "Output only NLRI bytes"},
				{Name: "--no-header", Desc: "Exclude 19-byte BGP header"},
				{Name: "--asn4", Desc: "Use 4-byte ASN encoding (default: true)"},
			}},
		},
		Examples: []string{
			"# IPv4/IPv6 Unicast",
			`ze bgp encode "route 10.0.0.0/24 next-hop 192.168.1.1"`,
			`ze bgp encode -f "ipv6/unicast" "route 2001:db8::/32 next-hop 2001:db8::1"`,
			"",
			"# L3VPN (mpls-vpn)",
			`ze bgp encode -f "ipv4/mpls-vpn" "10.0.0.0/24 rd 100:1 next-hop 1.2.3.4 label 100"`,
			`ze bgp encode -f "ipv4/mpls-vpn" "10.0.0.0/24 rd 1.2.3.4:100 next-hop 1.2.3.4 label 100"`,
			"",
			"# Labeled Unicast (mpls-label)",
			`ze bgp encode -f "ipv4/mpls-label" "10.0.0.0/24 next-hop 1.2.3.4 label 100"`,
			"",
			"# EVPN",
			`ze bgp encode -f "l2vpn/evpn" "mac-ip rd 100:1 esi 0 etag 0 mac 00:11:22:33:44:55 label 100 next-hop 1.2.3.4"`,
			`ze bgp encode -f "l2vpn/evpn" "ip-prefix rd 100:1 esi 0 etag 0 prefix 10.0.0.0/24 gateway 0.0.0.0 label 100 next-hop 1.2.3.4"`,
			`ze bgp encode -f "l2vpn/evpn" "multicast rd 100:1 etag 0 next-hop 1.2.3.4 extended-community target:100:1"`,
			"",
			"# Output options",
			`ze bgp encode -n "route 10.0.0.0/24 next-hop 1.2.3.4"                NLRI only`,
			`ze bgp encode --no-header "route 10.0.0.0/24 next-hop 1.2.3.4"       No BGP header`,
			`echo "route 10.0.0.0/24 next-hop 1.2.3.4" | ze bgp encode            stdin`,
		},
	}
	p.WriteTo(diag, false)
}

// parseEncodingFamily parses family string to AFI/SAFI.
// Requires "afi/safi" format (e.g., "ipv4/unicast").
func parseEncodingFamily(famName string) (family.AFI, family.SAFI, error) {
	f, ok := family.LookupFamily(strings.ToLower(famName))
	if !ok {
		return 0, 0, fmt.Errorf("unknown family: %s (expected afi/safi format)", famName)
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
	update, err := ub.BuildUnicast(&params)
	if err != nil {
		return nil, nil, err
	}

	// Extract NLRI bytes
	var nlriBytes []byte
	if isIPv6 {
		// For IPv6, NLRI is in MP_REACH_NLRI - extract from path attributes
		// For now, just pack the prefix directly
		inet := nlri.NewINET(family.IPv6Unicast, parsed.Route.Prefix, 0, false)
		nlriLen := nlri.LenWithContext(inet, addPath)
		nlriBytes = make([]byte, nlriLen)
		nlri.WriteNLRI(inet, nlriBytes, 0, addPath)
	} else {
		// For IPv4, NLRI is inline
		inet := nlri.NewINET(family.IPv4Unicast, parsed.Route.Prefix, 0, false)
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
		Prefix:             r.Prefix,
		NextHop:            r.NextHop.Addr, // Extract address from RouteNextHop
		Origin:             attrs.Origin,
		LocalPreference:    attrs.LocalPreference,
		HasLocalPreference: attrs.HasLocalPreference,
		MED:                attrs.MED,
		HasMED:             attrs.HasMED,
		ASPath:             attrs.ASPath,
		Communities:        attrs.Communities,
		LargeCommunities:   attrs.LargeCommunities,
		ExtCommunityBytes:  attrs.ExtCommunityBytes,
	}
}
