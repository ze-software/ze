// Design: docs/architecture/wire/nlri.md — VPLS NLRI plugin
// RFC: rfc/short/rfc4761.md
//
// Package bgp_vpls implements a VPLS family plugin for ze.
// It handles VPLS NLRI (RFC 4761, SAFI 65).
package bgp_nlri_vpls

import (
	"bufio"
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

// familyVPLS is the canonical address family string for VPLS.
const familyVPLS = "l2vpn/vpls"

var logger = slogutil.DiscardLogger()

// SetLogger sets the package-level logger.
func SetLogger(l *slog.Logger) {
	if l != nil {
		logger = l
	}
}

// RunVPLSPlugin runs the VPLS plugin using the SDK RPC protocol.
func RunVPLSPlugin(engineConn, callbackConn net.Conn) int {
	logger.Debug("vpls plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-vpls", engineConn, callbackConn)
	defer func() { _ = p.Close() }()

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		Families: []sdk.FamilyDecl{
			{Name: familyVPLS, Mode: "decode"},
		},
	})
	if err != nil {
		logger.Error("vpls plugin failed", "error", err)
		return 1
	}

	return 0
}

// DecodeNLRIHex decodes VPLS NLRI from hex bytes, returning JSON.
// This is the in-process fast path registered in the plugin registry.
func DecodeNLRIHex(family, hexStr string) (string, error) {
	if family != familyVPLS {
		return "", fmt.Errorf("unsupported family: %s", family)
	}

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return "", fmt.Errorf("invalid hex: %w", err)
	}

	vpls, _, err := ParseVPLS(data)
	if err != nil {
		return "", fmt.Errorf("parse VPLS failed: %w", err)
	}

	result := vplsToJSON(vpls)
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("JSON encoding failed: %w", err)
	}

	return string(jsonBytes), nil
}

// RunCLIDecode decodes VPLS NLRI from hex string for CLI mode.
// This is for direct CLI invocation: ze plugin bgp-vpls --nlri.
func RunCLIDecode(hexData, family string, textOutput bool, output, errOut io.Writer) int {
	writeErr := func(format string, args ...any) {
		_, e := fmt.Fprintf(errOut, format, args...)
		_ = e
	}
	writeOut := func(s string) {
		_, e := fmt.Fprintln(output, s)
		_ = e
	}

	if family != familyVPLS {
		writeErr("error: invalid family: %s (expected %s)\n", family, familyVPLS)
		return 1
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeErr("error: invalid hex: %v\n", err)
		return 1
	}

	vpls, _, err := ParseVPLS(data)
	if err != nil {
		writeErr("error: parse VPLS failed: %v\n", err)
		return 1
	}

	if textOutput {
		writeOut(vpls.String())
		return 0
	}

	result := vplsToJSON(vpls)
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
		if _, err := fmt.Fprintln(output, s); err != nil {
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
		if len(parts) >= 4 && parts[0] == "decode" && parts[1] == "nlri" {
			family := parts[2]
			hexData := parts[3]
			jsonStr, err := DecodeNLRIHex(family, hexData)
			if err == nil {
				write("decoded json " + jsonStr)
				continue
			}
		}
		write("decoded unknown")
	}
	return 0
}

// vplsToJSON converts a parsed VPLS NLRI to a JSON-friendly map.
func vplsToJSON(v *VPLS) map[string]any {
	return map[string]any{
		"rd":              v.RD().String(),
		"ve-id":           v.VEID(),
		"ve-block-offset": v.VEBlockOffset(),
		"ve-block-size":   v.VEBlockSize(),
		"label-base":      v.LabelBase(),
	}
}
