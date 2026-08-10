// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec NLRI plugin
// RFC: rfc/short/rfc5575.md
// Detail: plugin_decode.go — wire-to-JSON decoding and formatting
// Detail: plugin_encode_text.go — text-to-wire encoding
// Detail: plugin_protocol.go — stdin/stdout protocol dispatch
//
// Package flowspec implements a FlowSpec family plugin for ze.
// It handles decoding of FlowSpec NLRI (RFC 8955, 8956) for the decode mode protocol.
//
// RFC 8955: Dissemination of Flow Specification Rules (IPv4 FlowSpec)
// RFC 8956: Dissemination of Flow Specification Rules for IPv6
package flowspec

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var errNoValidFlowspecDecoded = errors.New("no valid FlowSpec decoded")

// flowLogger is the package-level logger, disabled by default.
var flowLogger = slogutil.DiscardLogger()

// setFlowSpecLogger sets the package-level logger.
// Called by cmd/ze/bgp/plugin_flowspec.go with slogutil.PluginLogger().
func setFlowSpecLogger(l *slog.Logger) {
	if l != nil {
		flowLogger = l
	}
}

// runFlowSpecPlugin runs the FlowSpec plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func runFlowSpecPlugin(conn net.Conn) int {
	flowLogger.Debug("flowspec plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-nlri-flowspec", conn)
	defer func() { _ = p.Close() }()

	p.OnEncodeNLRI(EncodeNLRIHex)
	p.OnDecodeNLRI(DecodeNLRIHex)

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		Families: []sdk.FamilyDecl{
			{Name: "ipv4/flow", Mode: "both", AFI: 1, SAFI: 133},
			{Name: "ipv6/flow", Mode: "both", AFI: 2, SAFI: 133},
			{Name: "ipv4/flow-vpn", Mode: "both", AFI: 1, SAFI: 134},
			{Name: "ipv6/flow-vpn", Mode: "both", AFI: 2, SAFI: 134},
		},
	})
	if err != nil {
		flowLogger.Error("flowspec plugin failed", "error", err)
		return 1
	}

	return 0
}

// DecodeNLRIHex decodes FlowSpec NLRI from hex bytes, returning a data structure.
// This is the in-process fast path registered in the plugin registry.
// Same logic as the OnDecodeNLRI SDK callback but callable without RPC.
func DecodeNLRIHex(family, hexStr string) (any, error) {
	if !isValidFlowSpecFamily(family) {
		return nil, fmt.Errorf("unsupported family: %s", family)
	}

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}

	result := decodeFlowSpecNLRI(family, data)
	if result == nil {
		return nil, errNoValidFlowspecDecoded
	}

	return result, nil
}

// EncodeNLRIHex encodes FlowSpec NLRI from text args, returning hex bytes.
// This is the in-process fast path registered in the plugin registry.
// Same logic as the OnEncodeNLRI SDK callback but callable without RPC.
func EncodeNLRIHex(famName string, args []string) (string, error) {
	if !isValidFlowSpecFamily(famName) {
		return "", fmt.Errorf("invalid family: %s", famName)
	}

	fam, ok := family.LookupFamily(famName)
	if !ok {
		return "", fmt.Errorf("unknown family: %s", famName)
	}

	wireBytes, err := EncodeFlowSpecComponents(fam, args)
	if err != nil {
		return "", err
	}

	return textbuf.StringHexUpper(wireBytes), nil
}

// getFlowSpecYANG returns the embedded YANG schema for the flowspec plugin.
// FlowSpec plugin doesn't augment config schema, returns empty.
func getFlowSpecYANG() string {
	return ""
}

// RunCLIDecode decodes FlowSpec NLRI from hex string for CLI mode.
// This is for direct CLI invocation: ze plugin flowspec --nlri <hex>
// Output is plain JSON or text (no "decoded json" prefix).
// Errors go to errOut (typically stderr), results go to output (typically stdout).
func RunCLIDecode(hexData, family string, textOutput bool, output, errOut io.Writer) int {
	data, err := hex.DecodeString(hexData)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "error: invalid hex: %v\n", err) //nolint:errcheck // output
		return 1
	}

	if !isValidFlowSpecFamily(family) {
		_, _ = fmt.Fprintf(errOut, "error: invalid family: %s\n", family) //nolint:errcheck // output
		return 1
	}

	result := decodeFlowSpecNLRI(family, data)
	if result == nil {
		_, _ = fmt.Fprintln(errOut, "error: no valid FlowSpec decoded") //nolint:errcheck // output
		return 1
	}

	if textOutput {
		text := formatFlowSpecText(result)
		_, _ = fmt.Fprintln(output, text) //nolint:errcheck // output
		return 0
	}

	// JSON output (default)
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "error: JSON encoding failed: %v\n", err) //nolint:errcheck // output
		return 1
	}
	_, _ = fmt.Fprintln(output, string(jsonBytes)) //nolint:errcheck // output
	return 0
}

// flowSpecFamilies returns the address families this plugin can decode.
func flowSpecFamilies() []string {
	return []string{
		"ipv4/flow",
		"ipv6/flow",
		"ipv4/flow-vpn",
		"ipv6/flow-vpn",
	}
}
