// Design: docs/architecture/wire/nlri.md — route target constraint plugin
// RFC: rfc/short/rfc4684.md
//
// Package bgp_rtc implements a Route Target Constraint family plugin for ze.
// It handles RTC NLRI (RFC 4684, SAFI 132).
package bgp_nlri_rtc

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"

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

// RunRTCPlugin runs the RTC plugin using the SDK RPC protocol.
func RunRTCPlugin(engineConn, callbackConn net.Conn) int {
	logger.Debug("rtc plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-rtc", engineConn, callbackConn)
	defer func() { _ = p.Close() }()

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		Families: []sdk.FamilyDecl{
			{Name: "ipv4/rtc", Mode: "decode"},
		},
	})
	if err != nil {
		logger.Error("rtc plugin failed", "error", err)
		return 1
	}

	return 0
}

// DecodeNLRIHex decodes RTC NLRI from hex bytes, returning JSON.
// This is the in-process fast path registered in the plugin registry.
func DecodeNLRIHex(family, hexStr string) (string, error) {
	if family != "ipv4/rtc" {
		return "", fmt.Errorf("unsupported family: %s", family)
	}

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return "", fmt.Errorf("invalid hex: %w", err)
	}

	rtc, _, err := ParseRTC(data)
	if err != nil {
		return "", fmt.Errorf("parse RTC failed: %w", err)
	}

	result := rtcToJSON(rtc)
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("JSON encoding failed: %w", err)
	}

	return string(jsonBytes), nil
}

// RunCLIDecode decodes RTC NLRI from hex string for CLI mode.
// This is for direct CLI invocation: ze plugin bgp-rtc --nlri.
func RunCLIDecode(hexData, family string, textOutput bool, output, errOut io.Writer) int {
	writeErr := func(format string, args ...any) {
		_, e := fmt.Fprintf(errOut, format, args...)
		_ = e
	}
	writeOut := func(s string) {
		_, e := fmt.Fprintln(output, s)
		_ = e
	}

	if family != "ipv4/rtc" {
		writeErr("error: invalid family: %s (expected ipv4/rtc)\n", family)
		return 1
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeErr("error: invalid hex: %v\n", err)
		return 1
	}

	rtc, _, err := ParseRTC(data)
	if err != nil {
		writeErr("error: parse RTC failed: %v\n", err)
		return 1
	}

	if textOutput {
		writeOut(rtc.String())
		return 0
	}

	result := rtcToJSON(rtc)
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		writeErr("error: JSON encoding failed: %v\n", err)
		return 1
	}
	writeOut(string(jsonBytes))
	return 0
}

// rtcToJSON converts a parsed RTC NLRI to a JSON-friendly map.
func rtcToJSON(r *RTC) map[string]any {
	result := map[string]any{
		"origin-as":    r.OriginAS(),
		"route-target": r.RouteTargetValue().String(),
		"is-default":   r.IsDefault(),
		"raw":          fmt.Sprintf("%X", r.Bytes()),
	}
	return result
}
