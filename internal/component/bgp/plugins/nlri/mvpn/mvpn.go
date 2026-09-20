// Design: docs/architecture/wire/nlri.md — MVPN NLRI plugin
// RFC: rfc/short/rfc6514.md -- MCAST-VPN NLRI (SAFI 5), Section 4 framing
// Related: json.go -- AppendJSON, the in-process writer of the same members
//
// Package bgp_mvpn implements a Multicast VPN family plugin for ze.
// It handles MVPN NLRI (RFC 6514, SAFI 5).
package mvpn

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// mvpnFamilies returns the registry's own name for each family this plugin
// decodes. The plugin registration and the CLI help both read it.
//
// The names are not declared here. family.MustRegister in types.go joins each
// one from its AFI and SAFI parts, so no second spelling exists to drift from.
func mvpnFamilies() []string {
	return []string{IPv4MVPN.String(), IPv6MVPN.String()}
}

// familyDecl declares fam to the plugin server, taking its name and its AFI and
// SAFI numbers from the registration in types.go. Nothing here repeats them.
func familyDecl(fam family.Family) sdk.FamilyDecl {
	return sdk.FamilyDecl{
		Name: fam.String(),
		Mode: familyModeDecode,
		AFI:  uint16(fam.AFI),
		SAFI: uint8(fam.SAFI),
	}
}

const (
	// familyModeDecode declares a family this plugin decodes but never encodes.
	// The plugin server reads it as sdk.FamilyDecl.Mode.
	familyModeDecode = "decode"

	// cmdDecode is the text-protocol verb this plugin answers on stdin.
	cmdDecode = "decode"
)

// errMVPNEmptySection reports an MCAST-VPN NLRI section with no octets in it.
// A caller asking for a decode and receiving an empty list cannot tell that
// from a section it never handed over, so the walk names the case.
var errMVPNEmptySection = errors.New("mvpn: empty NLRI section")

var logger = slogutil.DiscardLogger()

// SetLogger sets the package-level logger.
func SetLogger(l *slog.Logger) {
	if l != nil {
		logger = l
	}
}

// runMVPNPlugin runs the MVPN plugin using the SDK RPC protocol.
func runMVPNPlugin(conn net.Conn) int {
	logger.Debug("mvpn plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-nlri-mvpn", conn)
	defer func() { _ = p.Close() }()

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		Families: []sdk.FamilyDecl{
			familyDecl(IPv4MVPN),
			familyDecl(IPv6MVPN),
		},
	})
	if err != nil {
		logger.Error("mvpn plugin failed", "error", err)
		return 1
	}

	return 0
}

// DecodeNLRIHex decodes an MVPN NLRI section from hex bytes, returning a data
// structure. This is the in-process fast path registered in the plugin registry.
//
// The hex is a whole MP_REACH_NLRI or MP_UNREACH_NLRI section, which packs as
// many NLRIs as fit, so every one of them is decoded. One NLRI answers as an
// object and several answer as an array, which is the shape the vpn plugin and
// decode_mp.go's nlriRoutes already agree on.
//
// addPath states whether each NLRI carries a 4-octet Path Identifier ahead of it
// (RFC 7911 Section 3). The hex alone cannot say, so the flag travels with it.
func DecodeNLRIHex(family, hexStr string, addPath bool) (any, error) {
	afi, err := familyToAFI(family)
	if err != nil {
		return nil, err
	}

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}

	routes, err := decodeMVPNSection(afi, data, addPath)
	if err != nil {
		return nil, err
	}
	return sectionJSON(routes, addPath), nil
}

// RunCLIDecode decodes MVPN NLRI from hex string for CLI mode.
// This is for direct CLI invocation: ze plugin bgp-mvpn --nlri.
func RunCLIDecode(hexData, family string, textOutput bool, output, errOut io.Writer) int {
	writeErr := func(format string, args ...any) {
		_, e := fmt.Fprintf(errOut, format, args...) //nolint:errcheck // output
		_ = e
	}
	writeOut := func(s string) {
		_, e := fmt.Fprintln(output, s) //nolint:errcheck // output
		_ = e
	}

	afi, err := familyToAFI(family)
	if err != nil {
		writeErr("error: invalid family: %s (expected ipv4/mvpn or ipv6/mvpn)\n", family)
		return 1
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeErr("error: invalid hex: %v\n", err)
		return 1
	}

	// The CLI is handed a whole NLRI section too, and `ze bgp decode` reads a hex
	// blob with no session behind it, so no Path Identifier precedes an NLRI
	// (RFC 7911 Section 3).
	routes, err := decodeMVPNSection(afi, data, false)
	if err != nil {
		// decodeMVPNSection already names the failure, so the prefix would say
		// "parse MVPN failed" twice.
		writeErr("error: %v\n", err)
		return 1
	}

	if textOutput {
		for _, route := range routes {
			writeOut(route.nlri.String())
		}
		return 0
	}

	jsonBytes, err := json.MarshalIndent(sectionJSON(routes, false), "", "  ")
	if err != nil {
		writeErr("error: JSON encoding failed: %v\n", err)
		return 1
	}
	writeOut(string(jsonBytes))
	return 0
}

// RunDecode implements the stdin/stdout decode protocol for in-process use.
// Reads lines like "decode nlri <family> <hex>", writes "decoded json <json>".
func RunDecode(input io.Reader, output io.Writer) int {
	write := func(s string) {
		if _, err := fmt.Fprintln(output, s); err != nil { //nolint:errcheck // output
			logger.Debug("write error", "err", err)
		}
	}

	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 4 && parts[0] == cmdDecode && parts[1] == "nlri" {
			fam := parts[2]
			hexData := parts[3]
			// The CLI and the plugin text command both hand over NLRI octets with no
			// negotiation behind them, so no Path Identifier precedes them
			// (RFC 7911 Section 3 puts one there only when ADD-PATH is negotiated).
			data, err := DecodeNLRIHex(fam, hexData, false)
			if err == nil {
				if raw, merr := json.Marshal(data); merr == nil {
					write("decoded json " + string(raw))
					continue
				}
			}
		}
		write("decoded unknown")
	}
	// bufio.Scanner reports a read failure and an over-long line through Err(),
	// never through Scan(). Without this the caller sees a clean, complete decode.
	if err := scanner.Err(); err != nil {
		var eb textbuf.Buffer
		write(eb.Str("decoded error ").Err(err).String())
		return 1
	}
	return 0
}

// mvpnToJSON converts a parsed MVPN NLRI to a JSON-friendly map.
//
// The members are ExaBGP's, and AppendJSON (json.go) writes the same ones for
// the same octets. Two decoders serve this family, the registry map path here
// and the in-process appender, and a reader cannot tell which one answered it.
func mvpnToJSON(m *MVPN) map[string]any {
	routeType := m.RouteType()

	// RFC 6514 Section 4 frames every NLRI as Route Type, Length and the route
	// type specific field. "raw" carries all three, so a reader can decode a
	// route type ze does not parse.
	result := map[string]any{
		"code":   int(routeType),
		"parsed": routeType.bodyParsed(),
		"raw":    textbuf.StringHexUpper(m.packed),
	}
	if !routeType.bodyParsed() {
		return result
	}

	result["name"] = routeType.name()
	result["rd"] = m.RD().String()
	result["source"] = m.source.String()
	result["group"] = m.group.String()

	// RFC 6514 Section 4.6 gives only the two C-multicast routes a Source AS,
	// and ExaBGP writes it as a decimal string rather than a number.
	if routeType.hasSourceAS() {
		result["source-as"] = textbuf.StringUint32(m.sourceAS)
	}
	return result
}

// decodeMVPNSection decodes every NLRI packed into one MCAST-VPN section.
//
// MP_REACH_NLRI carries as many NLRIs as fit into one attribute, so a decoder
// that parses the first and drops the rest publishes one route where the peer
// sent several, and no reader can tell a section of one from a section of ten
// (plan/journal/validated-value-discarded-by-its-caller.md).
//
// A remainder that will not parse is an error rather than the end of the walk.
// Ending quietly would publish the routes read so far and lose the rest with no
// word, which is the failure that reads as success (ai/rules/principles.md).
func decodeMVPNSection(afi AFI, data []byte, addPath bool) ([]mvpnRoute, error) {
	if len(data) == 0 {
		return nil, errMVPNEmptySection
	}

	var routes []mvpnRoute
	for remaining := data; len(remaining) > 0; {
		// RFC 7911 Section 3: "the NLRI encoding MUST be extended by prepending
		// the Path Identifier field, which is of four octets." The identifier
		// precedes each NLRI, so it is read inside the walk rather than once.
		pathID, rest, err := nlri.SplitPathID(remaining, addPath)
		if err != nil {
			return nil, err
		}

		mvpn, tail, err := parseMVPN(afi, rest)
		if err != nil {
			return nil, fmt.Errorf("parse MVPN failed: %w", err)
		}

		routes = append(routes, mvpnRoute{nlri: mvpn, pathID: pathID})
		remaining = tail
	}
	return routes, nil
}

// mvpnRoute is one decoded NLRI and the RFC 7911 Path Identifier that preceded
// it. The identifier is not part of the MCAST-VPN NLRI, so the MVPN itself does
// not carry it and the walk keeps the pair together.
type mvpnRoute struct {
	nlri   *MVPN
	pathID uint32
}

// sectionJSON renders a decoded section as the value the plugin registry
// publishes: one object for a section of one NLRI, an array for a section of
// several. decode_mp.go's nlriRoutes and the vpn plugin already read that shape.
func sectionJSON(routes []mvpnRoute, addPath bool) any {
	objects := make([]map[string]any, 0, len(routes))
	for _, route := range routes {
		object := mvpnToJSON(route.nlri)
		if addPath {
			// RFC 7911 Section 3 gives the Path Identifier four octets and
			// reserves no value, so zero is an identifier rather than its
			// absence. Publish it whenever ADD-PATH put one on the wire.
			object["path-id"] = route.pathID
		}
		objects = append(objects, object)
	}
	if len(objects) == 1 {
		return objects[0]
	}
	return objects
}

// familyToAFI resolves a family name to the AFI of the MVPN family it names.
// The registry resolves the name, so only its own spelling is accepted and a
// family that is not MVPN is refused rather than given an AFI.
func familyToAFI(name string) (AFI, error) {
	fam, ok := family.LookupFamily(strings.ToLower(name))
	if !ok {
		return 0, fmt.Errorf("unsupported family: %s", name)
	}
	if fam != IPv4MVPN && fam != IPv6MVPN {
		return 0, fmt.Errorf("unsupported family: %s", name)
	}
	return fam.AFI, nil
}
