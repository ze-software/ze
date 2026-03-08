// Design: docs/architecture/wire/nlri.md — MVPN NLRI plugin
//
// Package bgp_mvpn implements a Multicast VPN family plugin for ze.
// It handles MVPN NLRI (RFC 6514, SAFI 5).
package mvpn

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

var logger = slogutil.DiscardLogger()

// SetLogger sets the package-level logger.
func SetLogger(l *slog.Logger) {
	if l != nil {
		logger = l
	}
}

// RunMVPNPlugin runs the MVPN plugin using the SDK RPC protocol.
func RunMVPNPlugin(engineConn, callbackConn net.Conn) int {
	logger.Debug("mvpn plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-mvpn", engineConn, callbackConn)
	defer func() { _ = p.Close() }()

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		Families: []sdk.FamilyDecl{
			{Name: "ipv4/mvpn", Mode: "decode"},
			{Name: "ipv6/mvpn", Mode: "decode"},
		},
	})
	if err != nil {
		logger.Error("mvpn plugin failed", "error", err)
		return 1
	}

	return 0
}

// DecodeNLRIHex decodes MVPN NLRI from hex bytes, returning JSON.
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

	mvpn, _, err := ParseMVPN(afi, data)
	if err != nil {
		return "", fmt.Errorf("parse MVPN failed: %w", err)
	}

	result := mvpnToJSON(mvpn)
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("JSON encoding failed: %w", err)
	}

	return string(jsonBytes), nil
}

// RunCLIDecode decodes MVPN NLRI from hex string for CLI mode.
// This is for direct CLI invocation: ze plugin bgp-mvpn --nlri.
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
		writeErr("error: invalid family: %s (expected ipv4/mvpn or ipv6/mvpn)\n", family)
		return 1
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeErr("error: invalid hex: %v\n", err)
		return 1
	}

	mvpn, _, err := ParseMVPN(afi, data)
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

// mvpnToJSON converts a parsed MVPN NLRI to a JSON-friendly map.
func mvpnToJSON(m *MVPN) map[string]any {
	return map[string]any{
		"route-type": int(m.RouteType()),
		"rd":         m.RD().String(),
	}
}

// familyToAFI maps family string to AFI constant.
func familyToAFI(family string) (AFI, error) {
	lower := strings.ToLower(family)
	if lower == "ipv4/mvpn" {
		return AFIIPv4, nil
	}
	if lower == "ipv6/mvpn" {
		return AFIIPv6, nil
	}
	return 0, fmt.Errorf("unsupported family: %s", family)
}
