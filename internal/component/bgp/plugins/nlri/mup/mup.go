// Design: docs/architecture/wire/nlri.md — MUP NLRI plugin
// RFC: rfc/short/draft-ietf-bess-mup-safi.md
//
// Package bgp_mup implements a Mobile User Plane family plugin for ze.
// It handles MUP NLRI (draft-ietf-bess-mup-safi, SAFI 85).
package mup

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// Address family names this plugin registers and decodes. The plugin registry,
// the CLI and the reactor all match a family by exact string.
const (
	familyIPv4MUP = "ipv4/mup" // AFI 1, SAFI 85
	familyIPv6MUP = "ipv6/mup" // AFI 2, SAFI 85

	// familyModeDecode declares a family this plugin decodes but never encodes.
	// The plugin server reads it as sdk.FamilyDecl.Mode.
	familyModeDecode = "decode"

	// cmdDecode is the text-protocol verb this plugin answers on stdin.
	cmdDecode = "decode"

	// kwRouteType is the keyword that introduces the route type in the
	// CLI-style argument list config.go builds and encode.go parses.
	kwRouteType = "route-type"

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

// runMUPPlugin runs the MUP plugin using the SDK RPC protocol.
func runMUPPlugin(conn net.Conn) int {
	logger.Debug("mup plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-nlri-mup", conn)
	defer func() { _ = p.Close() }()

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		Families: []sdk.FamilyDecl{
			{Name: familyIPv4MUP, Mode: familyModeDecode, AFI: 1, SAFI: 85},
			{Name: familyIPv6MUP, Mode: familyModeDecode, AFI: 2, SAFI: 85},
		},
	})
	if err != nil {
		logger.Error("mup plugin failed", "error", err)
		return 1
	}

	return 0
}

// DecodeNLRIHex decodes MUP NLRI from hex bytes, returning a data structure.
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

	mup, _, err := ParseMUP(afi, data)
	if err != nil {
		return nil, fmt.Errorf("parse MUP failed: %w", err)
	}

	return mupToJSON(mup), nil
}

// RunCLIDecode decodes MUP NLRI from hex string for CLI mode.
// This is for direct CLI invocation: ze plugin bgp-mup --nlri <hex>.
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
		writeErr("error: invalid family: %s (expected ipv4/mup or ipv6/mup)\n", family)
		return 1
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeErr("error: invalid hex: %v\n", err)
		return 1
	}

	mup, _, err := ParseMUP(afi, data)
	if err != nil {
		writeErr("error: parse MUP failed: %v\n", err)
		return 1
	}

	if textOutput {
		writeOut(mup.String())
		return 0
	}

	result := mupToJSON(mup)
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

// mupToJSON converts a parsed MUP NLRI to a JSON-friendly map.
func mupToJSON(m *MUP) map[string]any {
	return map[string]any{
		jsonKeyRouteType: int(m.RouteType()),
		"arch-type":      int(m.ArchType()),
		"rd":             m.RD().String(),
	}
}

// familyToAFI maps family string to AFI constant.
func familyToAFI(family string) (AFI, error) {
	lower := strings.ToLower(family)
	if lower == familyIPv4MUP {
		return AFIIPv4, nil
	}
	if lower == familyIPv6MUP {
		return AFIIPv6, nil
	}
	return 0, fmt.Errorf("unsupported family: %s", family)
}
