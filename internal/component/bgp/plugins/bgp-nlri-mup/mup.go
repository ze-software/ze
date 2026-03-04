// Design: docs/architecture/wire/nlri.md — MUP NLRI plugin
// RFC: rfc/short/draft-ietf-bess-mup-safi.md
//
// Package bgp_mup implements a Mobile User Plane family plugin for ze.
// It handles MUP NLRI (draft-mpmz-bess-mup-safi, SAFI 85).
package bgp_nlri_mup

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

var logger = slogutil.DiscardLogger()

// SetLogger sets the package-level logger.
func SetLogger(l *slog.Logger) {
	if l != nil {
		logger = l
	}
}

// RunMUPPlugin runs the MUP plugin using the SDK RPC protocol.
func RunMUPPlugin(engineConn, callbackConn net.Conn) int {
	logger.Debug("mup plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-mup", engineConn, callbackConn)
	defer func() { _ = p.Close() }()

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		Families: []sdk.FamilyDecl{
			{Name: "ipv4/mup", Mode: "decode"},
			{Name: "ipv6/mup", Mode: "decode"},
		},
	})
	if err != nil {
		logger.Error("mup plugin failed", "error", err)
		return 1
	}

	return 0
}

// DecodeNLRIHex decodes MUP NLRI from hex bytes, returning JSON.
// This is the in-process fast path registered in the plugin registry.
func DecodeNLRIHex(family, hexStr string) (string, error) {
	afi, err := familyToAFI(family)
	if err != nil {
		return "", err
	}

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return "", fmt.Errorf("invalid hex: %w", err)
	}

	mup, _, err := ParseMUP(afi, data)
	if err != nil {
		return "", fmt.Errorf("parse MUP failed: %w", err)
	}

	result := mupToJSON(mup)
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("JSON encoding failed: %w", err)
	}

	return string(jsonBytes), nil
}

// RunCLIDecode decodes MUP NLRI from hex string for CLI mode.
// This is for direct CLI invocation: ze plugin bgp-mup --nlri <hex>.
func RunCLIDecode(hexData, family string, textOutput bool, output, errOut io.Writer) int {
	writeErr := func(format string, args ...any) {
		_, e := fmt.Fprintf(errOut, format, args...)
		_ = e
	}
	writeOut := func(s string) {
		_, e := fmt.Fprintln(output, s)
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

// mupToJSON converts a parsed MUP NLRI to a JSON-friendly map.
func mupToJSON(m *MUP) map[string]any {
	return map[string]any{
		"route-type": int(m.RouteType()),
		"arch-type":  int(m.ArchType()),
		"rd":         m.RD().String(),
		"raw":        fmt.Sprintf("%X", m.Bytes()),
	}
}

// familyToAFI maps family string to AFI constant.
func familyToAFI(family string) (AFI, error) {
	lower := strings.ToLower(family)
	if lower == "ipv4/mup" {
		return AFIIPv4, nil
	}
	if lower == "ipv6/mup" {
		return AFIIPv6, nil
	}
	return 0, fmt.Errorf("unsupported family: %s", family)
}
