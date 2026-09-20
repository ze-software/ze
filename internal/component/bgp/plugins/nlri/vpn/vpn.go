// Design: docs/architecture/wire/nlri.md — VPN NLRI plugin
// RFC: rfc/short/rfc4364.md
//
// Package vpn implements a VPN family plugin for ze.
// It handles decoding of VPN NLRI (RFC 4364, 4659) for the decode mode protocol.
package vpn

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var (
	errNoValidVpnRoutesDecoded = errors.New("no valid VPN routes decoded")
	errRdRequiresValue         = errors.New("rd requires value")
	errLabelRequiresValue      = errors.New("label requires value")
	errPrefixRequiresValue     = errors.New("prefix requires value")
	errPathIdRequiresValue     = errors.New("path-id requires value")
	errRdRequiredForVpn        = errors.New("rd required for VPN")
	errLabelRequiredForVpn     = errors.New("label required for VPN")
	errPrefixRequiredForVpn    = errors.New("prefix required for VPN")
)

// vpnLogger is the package-level logger, disabled by default.
var vpnLogger = slogutil.DiscardLogger()

// setVPNLogger sets the package-level logger.
// Called by cmd/ze/bgp/plugin_vpn.go with slogutil.PluginLogger().
func setVPNLogger(l *slog.Logger) {
	if l != nil {
		vpnLogger = l
	}
}

// runVPNPlugin runs the VPN plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func runVPNPlugin(conn net.Conn) int {
	vpnLogger.Debug("vpn plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-nlri-vpn", conn)
	defer func() { _ = p.Close() }()

	p.OnDecodeNLRI(DecodeNLRIHex)

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		Families: []sdk.FamilyDecl{
			familyDecl(IPv4VPN),
			familyDecl(IPv6VPN),
		},
	})
	if err != nil {
		vpnLogger.Error("vpn plugin failed", "error", err)
		return 1
	}

	return 0
}

// DecodeNLRIHex decodes VPN NLRI from hex bytes, returning a data structure.
// This is the in-process fast path registered in the plugin registry.
// Same logic as the OnDecodeNLRI SDK callback but callable without RPC.
//
// addPath states whether each NLRI in the section carries a 4-octet Path
// Identifier ahead of it (RFC 7911 Section 3). The hex alone cannot say.
func DecodeNLRIHex(family, hexStr string, addPath bool) (any, error) {
	if !isValidVPNFamily(family) {
		return nil, fmt.Errorf("unsupported family: %s", family)
	}

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}

	results := decodeVPNNLRI(family, data, addPath)
	if len(results) == 0 {
		return nil, errNoValidVpnRoutesDecoded
	}

	if len(results) == 1 {
		return results[0], nil
	}
	return results, nil
}

// EncodeNLRIHex encodes VPN NLRI from text args, returning hex bytes.
// Args format: "rd" <rd> "label" <label>... "prefix" <prefix> ["path-id" <id>]
// This is the in-process fast path registered in the plugin registry.
func EncodeNLRIHex(famName string, args []string) (string, error) {
	fam, ok := family.LookupFamily(famName)
	if !ok {
		return "", fmt.Errorf("unknown family: %s", famName)
	}
	if !isValidVPNFamily(famName) {
		return "", fmt.Errorf("unsupported family: %s", famName)
	}

	var rd RouteDistinguisher
	var labels []uint32
	var prefix netip.Prefix
	var pathID uint32
	var hasRD, hasPrefix bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "rd":
			i++
			if i >= len(args) {
				return "", errRdRequiresValue
			}
			parsed, err := ParseRDString(args[i])
			if err != nil {
				return "", fmt.Errorf("invalid rd: %w", err)
			}
			rd = parsed
			hasRD = true
		case "label":
			i++
			if i >= len(args) {
				return "", errLabelRequiresValue
			}
			v, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil {
				return "", fmt.Errorf("invalid label: %w", err)
			}
			labels = append(labels, uint32(v))
		case "prefix":
			i++
			if i >= len(args) {
				return "", errPrefixRequiresValue
			}
			p, err := netip.ParsePrefix(args[i])
			if err != nil {
				return "", fmt.Errorf("invalid prefix: %w", err)
			}
			prefix = p
			hasPrefix = true
		case "path-id":
			i++
			if i >= len(args) {
				return "", errPathIdRequiresValue
			}
			v, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil {
				return "", fmt.Errorf("invalid path-id: %w", err)
			}
			pathID = uint32(v)
		}
	}

	if !hasRD {
		return "", errRdRequiredForVpn
	}
	if len(labels) == 0 {
		return "", errLabelRequiredForVpn
	}
	if !hasPrefix {
		return "", errPrefixRequiredForVpn
	}

	v := NewVPN(fam, rd, labels, prefix, pathID)
	nlriBytes := v.Bytes()

	return textbuf.StringHexUpper(nlriBytes), nil
}

// familyModeDecode declares a family this plugin decodes but never encodes.
// The plugin server reads it as sdk.FamilyDecl.Mode.
//
// The family names are not declared here. family.MustRegister in types.go joins
// each one from its AFI and SAFI parts, and vPNFamilies reads them back.
const familyModeDecode = "decode"

// Protocol constants.
const (
	cmdDecode       = "decode"
	objTypeNLRI     = "nlri"
	fmtJSON         = "json"
	fmtText         = "text"
	respDecodedUnk  = "decoded unknown"
	respDecodedJSON = "decoded json "
)

// getVPNYANG returns the embedded YANG schema for the vpn plugin.
// VPN plugin doesn't augment config schema, returns empty.
func getVPNYANG() string {
	return ""
}

// RunCLIDecode decodes VPN NLRI from hex string for CLI mode.
// This is for direct CLI invocation: ze plugin vpn --nlri <hex>
// Output is plain JSON array or text (no "decoded json" prefix).
// Errors go to errOut (typically stderr), results go to output (typically stdout).
func RunCLIDecode(hexData, family string, textOutput bool, output, errOut io.Writer) int {
	writeErr := func(format string, args ...any) {
		if _, err := fmt.Fprintf(errOut, format, args...); err != nil { //nolint:errcheck // output
			return
		}
	}
	writeOut := func(s string) {
		_, err := io.WriteString(output, s+"\n")
		if err != nil {
			return
		}
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeErr("error: invalid hex: %v\n", err)
		return 1
	}

	if !isValidVPNFamily(family) {
		writeErr("error: invalid family: %s (expected ipv4/mpls-vpn or ipv6/mpls-vpn)\n", family)
		return 1
	}

	// The CLI and the plugin text command both hand over NLRI octets with no
	// negotiation behind them, so no Path Identifier precedes them
	// (RFC 7911 Section 3 puts one there only when ADD-PATH is negotiated).
	results := decodeVPNNLRI(family, data, false)
	if len(results) == 0 {
		writeErr("error: no valid VPN routes decoded\n")
		return 1
	}

	if textOutput {
		for _, r := range results {
			writeOut(formatVPNTextSingle(r))
		}
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

// runVPNDecode runs the plugin in decode mode for ze bgp decode (engine protocol).
func runVPNDecode(input io.Reader, output io.Writer) int {
	write := func(s string) {
		_, err := fmt.Fprintln(output, s) //nolint:errcheck // output
		if err != nil {
			vpnLogger.Debug("write error", "err", err)
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

	if !isValidVPNFamily(fam) {
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
	results := decodeVPNNLRI(fam, data, false)
	if len(results) == 0 {
		writeUnknown()
		return
	}

	if format == fmtText {
		var texts []string
		for _, r := range results {
			texts = append(texts, formatVPNTextSingle(r))
		}
		_, err := fmt.Fprintln(output, "decoded text "+textbuf.Join(texts, "; ")) //nolint:errcheck // output
		if err != nil {
			vpnLogger.Debug("write error", "err", err)
		}
		return
	}

	// Single object for single NLRI, array for multiple
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
	_, err = fmt.Fprintln(output, "decoded json "+string(jsonBytes)) //nolint:errcheck // output
	if err != nil {
		vpnLogger.Debug("write error", "err", err)
	}
}

// familyDecl declares fam to the plugin server, taking its name and its AFI and
// SAFI numbers from the registration in types.go. Nothing here repeats them.
func familyDecl(fam Family) sdk.FamilyDecl {
	return sdk.FamilyDecl{
		Name: fam.String(),
		Mode: familyModeDecode,
		AFI:  uint16(fam.AFI),
		SAFI: uint8(fam.SAFI),
	}
}

// isValidVPNFamily reports whether name is one of this plugin's families. The
// registry resolves the name first, so only its own spelling is accepted.
func isValidVPNFamily(name string) bool {
	fam, ok := family.LookupFamily(name)
	if !ok {
		return false
	}
	return fam == IPv4VPN || fam == IPv6VPN
}

// decodeVPNNLRI decodes VPN NLRI wire bytes to array of JSON maps.
// MP_REACH/MP_UNREACH can contain multiple packed NLRIs.
func decodeVPNNLRI(family string, data []byte, addPath bool) []map[string]any {
	var results []map[string]any
	remaining := data

	// Determine AFI from family
	afi := AFIIPv4
	if strings.HasPrefix(family, "ipv6") {
		afi = AFIIPv6
	}

	for len(remaining) > 0 {
		// RFC 7911 Section 3: "the NLRI encoding MUST be extended by prepending
		// the Path Identifier field, which is of four octets." ParseVPN consumes
		// that field, and vpnToJSON publishes it, so the walk stays one call.
		v, rest, err := ParseVPN(afi, SAFIVPN, remaining, addPath)
		if err != nil {
			vpnLogger.Debug("parse vpn failed", "err", err)
			// Add as unparsed
			results = append(results, map[string]any{
				"parsed": false,
				"raw":    textbuf.StringHexUpper(remaining),
			})
			break
		}
		route := vpnToJSON(v)
		if addPath {
			// RFC 7911 Section 3 gives the Path Identifier four octets and
			// reserves no value, so zero is an identifier rather than its
			// absence. Publish it whenever ADD-PATH put one on the wire.
			route["path-id"] = v.PathID()
		}
		results = append(results, route)
		remaining = rest
	}

	return results
}

// vpnToJSON converts VPN route to JSON representation.
// Format: {"rd": "...", "prefix": "...", "labels": [[label, entry], ...]}.
func vpnToJSON(v *VPN) map[string]any {
	result := map[string]any{
		"rd":     v.rd.String(),
		"prefix": v.prefix.String(),
	}

	// Each member is the label and the stack entry it came from, the pair
	// AppendJSON writes and ExaBGP publishes.
	if entries := v.LabelEntries(); len(entries) > 0 {
		labels := make([][]int, len(entries))
		for i, entry := range entries {
			labels[i] = []int{int(nlri.LabelValue(entry))}
			if entry != 0 {
				labels[i] = append(labels[i], int(entry))
			}
		}
		result["labels"] = labels
	}

	if v.pathID != 0 {
		result["path-id"] = v.pathID
	}

	return result
}

// formatVPNTextSingle formats a single VPN route as human-readable text.
func formatVPNTextSingle(result map[string]any) string {
	var b textbuf.Buffer

	fam := "VPNv4"
	if prefix, ok := result["prefix"].(string); ok && strings.Contains(prefix, ":") {
		fam = "VPNv6"
	}
	b.Reset().Str(fam)

	if v, ok := result["rd"].(string); ok {
		b.Str(" rd=").Str(v)
	}
	if v, ok := result["prefix"].(string); ok {
		b.Str(" prefix=").Str(v)
	}
	if v, ok := result["labels"].([][]int); ok && len(v) > 0 {
		b.Str(" label=")
		first := true
		for _, l := range v {
			if len(l) > 0 {
				if !first {
					b.Byte(',')
				}
				b.Int(int64(l[0]))
				first = false
			}
		}
	}
	if v, ok := result["path-id"].(uint32); ok && v != 0 {
		b.Str(" path-id=").Uint32(v)
	}

	return b.String()
}

// LookupFamily wraps family.LookupFamily for use by this package.
func LookupFamily(s string) (Family, bool) {
	return family.LookupFamily(s)
}
