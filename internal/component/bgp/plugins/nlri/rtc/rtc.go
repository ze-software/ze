// Design: docs/architecture/wire/nlri.md — route target constraint plugin
// RFC: rfc/short/rfc4684.md
//
// Package bgp_rtc implements a Route Target Constraint family plugin for ze.
// It handles RTC NLRI (RFC 4684, SAFI 132).
package rtc

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

const (
	// familyIPv4RTC is the address family name this plugin registers and
	// decodes. The plugin registry, the CLI and the reactor all match a family
	// by exact string.
	familyIPv4RTC = "ipv4/rtc" // AFI 1, SAFI 132

	// familyModeDecode declares a family this plugin decodes but never encodes.
	// The plugin server reads it as sdk.FamilyDecl.Mode.
	familyModeDecode = "decode"
)

var logger = slogutil.DiscardLogger()

// SetLogger sets the package-level logger.
func SetLogger(l *slog.Logger) {
	if l != nil {
		logger = l
	}
}

// runRTCPlugin runs the RTC plugin using the SDK RPC protocol.
func runRTCPlugin(conn net.Conn) int {
	logger.Debug("rtc plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-nlri-rtc", conn)
	defer func() { _ = p.Close() }()

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		Families: []sdk.FamilyDecl{
			{Name: familyIPv4RTC, Mode: familyModeDecode, AFI: 1, SAFI: 132},
		},
	})
	if err != nil {
		logger.Error("rtc plugin failed", "error", err)
		return 1
	}

	return 0
}

// DecodeNLRIHex decodes RTC NLRI from hex bytes, returning a data structure.
// This is the in-process fast path registered in the plugin registry.
func DecodeNLRIHex(family, hexStr string) (any, error) {
	if family != familyIPv4RTC {
		return nil, fmt.Errorf("unsupported family: %s", family)
	}

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}

	rtc, _, err := parseRTC(data)
	if err != nil {
		return nil, fmt.Errorf("parse RTC failed: %w", err)
	}

	return rtcToJSON(rtc), nil
}

// RunCLIDecode decodes RTC NLRI from hex string for CLI mode.
// This is for direct CLI invocation: ze plugin bgp-rtc --nlri.
func RunCLIDecode(hexData, family string, textOutput bool, output, errOut io.Writer) int {
	writeErr := func(format string, args ...any) {
		_, e := fmt.Fprintf(errOut, format, args...) //nolint:errcheck // output
		_ = e
	}
	writeOut := func(s string) {
		_, e := fmt.Fprintln(output, s) //nolint:errcheck // output
		_ = e
	}

	if family != familyIPv4RTC {
		writeErr("error: invalid family: %s (expected ipv4/rtc)\n", family)
		return 1
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeErr("error: invalid hex: %v\n", err)
		return 1
	}

	rtc, _, err := parseRTC(data)
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
		if len(parts) >= 4 && parts[0] == "decode" && parts[1] == "nlri" {
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

// rtcToJSON converts a parsed RTC NLRI to a JSON-friendly map.
func rtcToJSON(r *RTC) map[string]any {
	return map[string]any{
		"origin-as":    r.OriginAS(),
		"route-target": r.routeTargetValue().String(),
		"is-default":   r.isDefault(),
	}
}
