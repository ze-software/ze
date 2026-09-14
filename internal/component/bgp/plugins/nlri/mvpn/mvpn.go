// Design: docs/architecture/wire/nlri.md — MVPN NLRI plugin
//
// Package bgp_mvpn implements a Multicast VPN family plugin for ze.
// It handles MVPN NLRI (RFC 6514, SAFI 5).
package mvpn

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"

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

	// jsonKeyRouteType is the route type field of the decoded JSON object.
	jsonKeyRouteType = "route-type"
)

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

// DecodeNLRIHex decodes MVPN NLRI from hex bytes, returning a data structure.
// This is the in-process fast path registered in the plugin registry.
func DecodeNLRIHex(family, hexStr string) (any, error) {
	afi, err := familyToAFI(family)
	if err != nil {
		return nil, err
	}

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}

	mvpn, _, err := parseMVPN(afi, data)
	if err != nil {
		return nil, fmt.Errorf("parse MVPN failed: %w", err)
	}

	return mvpnToJSON(mvpn), nil
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

	mvpn, _, err := parseMVPN(afi, data)
	if err != nil {
		writeErr("error: parse MVPN failed: %v\n", err)
		return 1
	}

	if textOutput {
		writeOut(mvpn.String())
		return 0
	}

	result := mvpnToJSON(mvpn)
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
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
			data, err := DecodeNLRIHex(fam, hexData)
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
func mvpnToJSON(m *MVPN) map[string]any {
	return map[string]any{
		jsonKeyRouteType: int(m.RouteType()),
		"rd":             m.RD().String(),
	}
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
