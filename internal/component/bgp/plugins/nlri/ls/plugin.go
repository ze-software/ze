// Design: docs/architecture/wire/nlri-bgpls.md — BGP-LS NLRI plugin
// RFC: rfc/short/rfc7752.md

package ls

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

	"github.com/ze-software/ze/internal/core/bgp/nlri/bgpls"

	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var errNoValidBgpLsNlrisDecoded = errors.New("no valid BGP-LS NLRIs decoded")

// bgplsLogger is the package-level logger, disabled by default.
var bgplsLogger = slogutil.DiscardLogger()

// setBGPLSLogger sets the package-level logger.
// Called by cmd/ze/bgp/plugin_bgpls.go with slogutil.PluginLogger().
func setBGPLSLogger(l *slog.Logger) {
	if l != nil {
		bgplsLogger = l
	}
}

// runBGPLSPlugin runs the BGP-LS plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func runBGPLSPlugin(conn net.Conn) int {
	bgplsLogger.Debug("bgpls plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-nlri-ls", conn)
	defer func() { _ = p.Close() }()

	p.OnDecodeNLRI(func(family string, hexStr string, addPath bool, _ bool) (any, error) {
		if !isValidBGPLSFamily(family) {
			return nil, fmt.Errorf("unsupported family: %s", family)
		}

		data, err := hex.DecodeString(hexStr)
		if err != nil {
			return nil, fmt.Errorf("invalid hex: %w", err)
		}

		results := decodeBGPLSNLRI(data, addPath)
		if len(results) == 0 {
			return nil, errNoValidBgpLsNlrisDecoded
		}

		if len(results) == 1 {
			return results[0], nil
		}
		return results, nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, &sdk.Registration{
		Families: []sdk.FamilyDecl{
			familyDecl(BGPLSFamily),
			familyDecl(BGPLSVPNFamily),
		},
	})
	if err != nil {
		bgplsLogger.Error("bgpls plugin failed", "error", err)
		return 1
	}

	return 0
}

// familyModeDecode declares a family this plugin decodes but never encodes.
// The plugin server reads it as sdk.FamilyDecl.Mode.
//
// The family names are not declared here. family.MustRegister in register.go joins
// each one from its AFI and SAFI parts, and bgpLSFamilies reads them back.
const familyModeDecode = "decode"

// bgpLSFamilies returns the registry's own name for each family this plugin
// decodes. The plugin registration and the CLI help both read it.
func bgpLSFamilies() []string {
	return []string{BGPLSFamily.String(), BGPLSVPNFamily.String()}
}

// familyDecl declares fam to the plugin server, taking its name and its AFI and
// SAFI numbers from the registration in register.go. Nothing here repeats them.
func familyDecl(fam family.Family) sdk.FamilyDecl {
	return sdk.FamilyDecl{
		Name: fam.String(),
		Mode: familyModeDecode,
		AFI:  uint16(fam.AFI),
		SAFI: uint8(fam.SAFI),
	}
}

// JSON keys of the decoded NLRI objects this plugin emits.
const (
	// jsonKeyParsed reports whether the NLRI could be decoded, and jsonKeyRaw
	// carries the undecoded wire bytes that go with a false.
	jsonKeyParsed = "parsed"
	jsonKeyRaw    = "raw"
)

// Protocol constants.
const (
	cmdDecode       = "decode"
	objTypeNLRI     = "nlri"
	fmtJSON         = "json"
	fmtText         = "text"
	respDecodedUnk  = "decoded unknown"
	respDecodedJSON = "decoded json "
)

// getBGPLSYANG returns the embedded YANG schema for the bgpls plugin.
// BGP-LS plugin doesn't augment config schema, returns empty.
func getBGPLSYANG() string {
	return ""
}

// runBGPLSCLIDecode decodes BGP-LS NLRI from hex string for CLI mode.
// This is for direct CLI invocation: ze plugin bgpls --nlri <hex>
// Output is plain JSON or text (no "decoded json" prefix).
// Errors go to errOut (typically stderr), results go to output (typically stdout).
func runBGPLSCLIDecode(hexData, family string, textOutput bool, output, errOut io.Writer) int {
	writeErr := func(format string, args ...any) {
		_, e := fmt.Fprintf(errOut, format, args...) //nolint:errcheck // output
		_ = e                                        // CLI output - pipe failure is unrecoverable
	}
	writeOut := func(s string) {
		_, e := fmt.Fprintln(output, s) //nolint:errcheck // output
		_ = e                           // CLI output - pipe failure is unrecoverable
	}

	if !isValidBGPLSFamily(family) {
		writeErr("error: invalid family: %s (expected bgp-ls/bgp-ls or bgp-ls/bgp-ls-vpn)\n", family)
		return 1
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeErr("error: invalid hex: %v\n", err)
		return 1
	}

	// The CLI and the plugin text command both hand over NLRI octets with no
	// negotiation behind them, so no Path Identifier precedes them
	// (RFC 7911 Section 3 puts one there only when ADD-PATH is negotiated).
	results := decodeBGPLSNLRI(data, false)
	if len(results) == 0 {
		writeErr("error: no valid BGP-LS routes decoded\n")
		return 1
	}

	if textOutput {
		writeOut(formatBGPLSText(results))
		return 0
	}

	// JSON output (default) - single object for single NLRI, array for multiple
	var jsonBytes []byte
	if len(results) == 1 {
		jsonBytes, err = json.MarshalIndent(results[0], "", "  ")
	} else {
		jsonBytes, err = json.MarshalIndent(results, "", "  ")
	}
	if err != nil {
		writeErr("error: JSON encoding failed: %v\n", err)
		return 1
	}
	writeOut(string(jsonBytes))
	return 0
}

// runBGPLSDecode runs the plugin in decode mode for ze bgp decode (engine protocol).
func runBGPLSDecode(input io.Reader, output io.Writer) int {
	write := func(s string) {
		if _, err := fmt.Fprintln(output, s); err != nil { //nolint:errcheck // output
			bgplsLogger.Debug("write error", "err", err)
		}
	}
	writeUnknown := func() { write("decoded unknown") }

	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 {
			writeUnknown()
			continue
		}

		cmd := parts[0]
		objType := parts[1]

		// Handle format specifier
		format := fmtJSON
		if objType == fmtJSON || objType == fmtText {
			format = objType
			if len(parts) < 4 {
				writeUnknown()
				continue
			}
			objType = parts[2]
			parts = append([]string{cmd, objType}, parts[3:]...)
		}

		if cmd == cmdDecode && objType == objTypeNLRI {
			handleDecodeNLRI(parts, format, output, writeUnknown)
		} else if cmd == cmdDecode {
			writeUnknown()
		}
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

// handleDecodeNLRI handles: decode nlri <family> <hex>.
func handleDecodeNLRI(parts []string, format string, output io.Writer, writeUnknown func()) {
	if len(parts) < 4 {
		writeUnknown()
		return
	}

	fam := strings.ToLower(parts[2])
	hexData := parts[3]

	if !isValidBGPLSFamily(fam) {
		writeUnknown()
		return
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeUnknown()
		return
	}

	// The CLI and the plugin text command both hand over NLRI octets with no
	// negotiation behind them, so no Path Identifier precedes them
	// (RFC 7911 Section 3 puts one there only when ADD-PATH is negotiated).
	results := decodeBGPLSNLRI(data, false)
	if len(results) == 0 {
		writeUnknown()
		return
	}

	if format == fmtText {
		if _, err := fmt.Fprintln(output, "decoded text "+formatBGPLSText(results)); err != nil { //nolint:errcheck // output
			bgplsLogger.Debug("write error", "err", err)
		}
		return
	}

	// Single object for single NLRI, array for multiple (matches VPN pattern)
	var jsonBytes []byte
	if len(results) == 1 {
		jsonBytes, err = json.Marshal(results[0])
	} else {
		jsonBytes, err = json.Marshal(results)
	}
	if err != nil {
		writeUnknown()
		return
	}
	if _, err := fmt.Fprintln(output, "decoded json "+string(jsonBytes)); err != nil { //nolint:errcheck // output
		bgplsLogger.Debug("write error", "err", err)
	}
}

// isValidBGPLSFamily reports whether name is one of this plugin's families. The
// registry resolves the name first, so only its own spelling is accepted.
func isValidBGPLSFamily(name string) bool {
	fam, ok := family.LookupFamily(name)
	if !ok {
		return false
	}
	return fam == BGPLSFamily || fam == BGPLSVPNFamily
}

// decodeBGPLSNLRI decodes BGP-LS NLRI wire bytes to array of JSON maps.
// MP_REACH/MP_UNREACH can contain multiple packed NLRIs.
func decodeBGPLSNLRI(data []byte, addPath bool) []map[string]any {
	var results []map[string]any

	// Handle empty/truncated data
	if len(data) < 4 {
		results = append(results, map[string]any{
			jsonKeyParsed: false,
			jsonKeyRaw:    strings.ToUpper(textbuf.StringHex(data)),
		})
		return results
	}

	remaining := data
	for len(remaining) > 0 {
		// RFC 7911 Section 3: "the NLRI encoding MUST be extended by prepending
		// the Path Identifier field, which is of four octets." Split it before
		// the NLRI header, so ParseBGPLSWithRest reads the type and length at
		// the right offset.
		pathID, body, pathErr := nlri.SplitPathID(remaining, addPath)
		if pathErr != nil {
			results = append(results, map[string]any{
				jsonKeyParsed: false,
				jsonKeyRaw:    strings.ToUpper(textbuf.StringHex(remaining)),
			})
			break
		}
		remaining = body

		parsed, rest, err := bgpls.ParseBGPLSWithRest(remaining)
		if err != nil {
			bgplsLogger.Debug("parse bgpls failed", "err", err)
			if rest == nil {
				// The FRAMING is broken (truncated header or a Total NLRI Length that
				// overruns), so the next boundary cannot be found. Everything left is one
				// opaque blob, and the loop has to stop.
				results = append(results, map[string]any{
					jsonKeyParsed: false,
					jsonKeyRaw:    strings.ToUpper(textbuf.StringHex(remaining)),
				})
				break
			}
			// The framing is intact and only THIS NLRI could not be parsed -- an
			// unrecognized NLRI type is the common case, and RFC 9552 Section 5.1 requires
			// those to be preserved rather than treated as an error. Report just this one
			// as unparsed and keep decoding.
			//
			// This used to blob the whole remainder and break, so a single unknown type
			// early in a densely packed UPDATE erased the structured view of every NLRI
			// after it, for known types too.
			results = append(results, map[string]any{
				jsonKeyParsed: false,
				jsonKeyRaw:    strings.ToUpper(textbuf.StringHex(remaining[:len(remaining)-len(rest)])),
			})
			remaining = rest
			continue
		}
		route := bgplsToJSON(parsed, remaining[:len(remaining)-len(rest)])
		if addPath {
			route["path-id"] = pathID
		}
		results = append(results, route)
		remaining = rest
	}

	return results
}

// bgplsToJSON converts a BGP-LS NLRI to JSON format.
// RFC 7752 defines the NLRI structure.
func bgplsToJSON(n bgpls.BGPLSNLRI, data []byte) map[string]any {
	result := map[string]any{
		"ls-nlri-type":        bgpls.BGPLSNLRITypeString(uint16(n.NLRIType())),
		"l3-routing-topology": n.Identifier(),
		"protocol-id":         int(n.ProtocolID()),
	}

	// Type-specific fields based on NLRI type - parse from wire bytes
	switch n.NLRIType() {
	case bgpls.BGPLSNodeNLRI:
		result["node-descriptors"] = bgpls.ParseBGPLSNodeTLVs(data)

	case bgpls.BGPLSLinkNLRI:
		localDescs, remoteDescs, info := bgpls.ParseBGPLSLinkTLVs(data)
		result["local-node-descriptors"] = localDescs
		result["remote-node-descriptors"] = remoteDescs
		result["interface-addresses"] = info.IfAddrs
		result["neighbor-addresses"] = info.NeighAddrs
		result["multi-topology-ids"] = info.MTIDs
		result["link-identifiers"] = info.LinkIDs

	case bgpls.BGPLSPrefixV4NLRI, bgpls.BGPLSPrefixV6NLRI:
		nodeDescs, prefixInfo := bgpls.ParseBGPLSPrefixTLVs(data, n.NLRIType())
		result["node-descriptors"] = nodeDescs
		if prefixInfo.Prefix != "" {
			result["ip-reachability-tlv"] = prefixInfo.Prefix
			result["ip-reach-prefix"] = prefixInfo.Prefix
		}
		result["multi-topology-ids"] = prefixInfo.MTIDs

	case bgpls.BGPLSSRv6SIDNLRI:
		result["node-descriptors"] = bgpls.ParseBGPLSNodeTLVs(data)
		if v, ok := n.(*bgpls.BGPLSSRv6SID); ok && len(v.SRv6SID.SRv6SID) > 0 {
			result["srv6-sid"] = bgpls.FormatIPv6Compressed(v.SRv6SID.SRv6SID)
		}
	default:
		// The interface admits an open type set; retain only the common keys.
	}

	return result
}

// formatBGPLSText formats BGP-LS NLRI results as human-readable text.
func formatBGPLSText(results []map[string]any) string {
	texts := make([]string, 0, len(results))
	for _, r := range results {
		texts = append(texts, formatBGPLSTextSingle(r))
	}
	return textbuf.Join(texts, "; ")
}

// formatBGPLSTextSingle formats a single BGP-LS NLRI as human-readable text.
func formatBGPLSTextSingle(result map[string]any) string {
	var parts []string

	if v, ok := result["ls-nlri-type"].(string); ok {
		parts = append(parts, v)
	}
	if v, ok := result["protocol-id"].(int); ok {
		var b textbuf.Buffer
		parts = append(parts, b.Reset().Str("proto=").Int(int64(v)).String())
	}
	if v, ok := result["l3-routing-topology"].(uint64); ok {
		var b textbuf.Buffer
		parts = append(parts, b.Reset().Str("id=").Uint(v).String())
	}

	if len(parts) == 0 {
		return "(empty)"
	}
	return textbuf.Join(parts, " ")
}
