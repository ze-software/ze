// Package vpn implements a VPN family plugin for ze.
// It handles decoding of VPN NLRI (RFC 4364, 4659) for the decode mode protocol.
package vpn

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// vpnLogger is the package-level logger, disabled by default.
var vpnLogger = slogutil.DiscardLogger()

// SetVPNLogger sets the package-level logger.
// Called by cmd/ze/bgp/plugin_vpn.go with slogutil.PluginLogger().
func SetVPNLogger(l *slog.Logger) {
	if l != nil {
		vpnLogger = l
	}
}

// VPNPlugin implements a VPN family plugin.
// For now, it only supports decode mode (started with --decode).
type VPNPlugin struct {
	input  *bufio.Scanner
	output io.Writer
}

// MaxLineSize is the maximum size of a single input line (1MB).
const MaxLineSize = 1024 * 1024

// NewVPNPlugin creates a new VPN Plugin.
func NewVPNPlugin(input io.Reader, output io.Writer) *VPNPlugin {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, MaxLineSize), MaxLineSize)
	return &VPNPlugin{
		input:  scanner,
		output: output,
	}
}

// Run starts the vpn plugin in normal mode.
func (p *VPNPlugin) Run() int {
	vpnLogger.Debug("vpn plugin starting")
	p.doStartupProtocol()
	vpnLogger.Debug("vpn plugin startup complete, entering event loop")
	p.eventLoop()
	return 0
}

// doStartupProtocol performs the 5-stage plugin registration protocol.
func (p *VPNPlugin) doStartupProtocol() {
	// Stage 1: Declaration - claim VPN family decode
	p.send("declare family ipv4 vpn decode")
	p.send("declare family ipv6 vpn decode")
	p.send("declare rfc 4364")
	p.send("declare rfc 4659")
	p.send("declare encoding hex")
	p.send("declare done")

	// Stage 2: Parse config (VPN plugin doesn't need config)
	p.waitForLine("config done")

	// Stage 3: No explicit capability injection needed.
	p.send("capability done")

	// Stage 4: Wait for registry
	p.waitForLine("registry done")

	// Stage 5: Ready
	p.send("ready")
}

// eventLoop handles decode requests from the engine.
func (p *VPNPlugin) eventLoop() {
	for p.input.Scan() {
		line := p.input.Text()
		if len(line) == 0 {
			continue
		}

		vpnLogger.Debug("received", "line", line[:min(80, len(line))])

		serial, command := parseSerialPrefix(line)
		response := p.handleRequest(command)
		if response != "" {
			if serial != "" {
				p.send(fmt.Sprintf("@%s %s", serial, response))
			} else {
				p.send(response)
			}
		}
	}
}

// parseSerialPrefix extracts "#serial" prefix from a line.
func parseSerialPrefix(line string) (string, string) {
	if len(line) > 0 && line[0] == '#' {
		idx := strings.IndexByte(line, ' ')
		if idx > 1 {
			return line[1:idx], line[idx+1:]
		}
	}
	return "", line
}

// Protocol constants.
const (
	cmdDecode       = "decode"
	objTypeNLRI     = "nlri"
	fmtJSON         = "json"
	fmtText         = "text"
	respDecodedUnk  = "decoded unknown"
	respDecodedJSON = "decoded json "
)

// handleRequest processes a single request and returns the response.
func (p *VPNPlugin) handleRequest(line string) string {
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return ""
	}

	cmd := parts[0]
	objType := parts[1]

	if cmd == cmdDecode && objType == objTypeNLRI {
		return p.handleDecodeRequest(parts)
	}

	return ""
}

// handleDecodeRequest handles: decode nlri <family> <hex>.
func (p *VPNPlugin) handleDecodeRequest(parts []string) string {
	if len(parts) < 4 {
		return respDecodedUnk
	}

	family := strings.ToLower(parts[2])
	hexData := parts[3]

	if !isValidVPNFamily(family) {
		return respDecodedUnk
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		return respDecodedUnk
	}

	results := decodeVPNNLRI(family, data)
	if len(results) == 0 {
		return respDecodedUnk
	}

	// Return single object for single NLRI (matches flowspec pattern),
	// array for multiple NLRIs.
	var jsonBytes []byte
	if len(results) == 1 {
		jsonBytes, err = json.Marshal(results[0])
	} else {
		jsonBytes, err = json.Marshal(results)
	}
	if err != nil {
		return respDecodedUnk
	}

	return respDecodedJSON + string(jsonBytes)
}

// waitForLine reads lines until one matches the expected line.
func (p *VPNPlugin) waitForLine(expected string) {
	for p.input.Scan() {
		line := p.input.Text()
		if line == expected {
			return
		}
	}
}

// send sends raw output to ze.
// Write errors are logged but not propagated - if the pipe is broken, the plugin exits anyway.
func (p *VPNPlugin) send(msg string) {
	_, err := fmt.Fprintf(p.output, "%s\n", msg)
	if err != nil {
		vpnLogger.Debug("write error", "err", err)
	}
}

// GetVPNYANG returns the embedded YANG schema for the vpn plugin.
// VPN plugin doesn't augment config schema, returns empty.
func GetVPNYANG() string {
	return ""
}

// RunCLIDecode decodes VPN NLRI from hex string for CLI mode.
// This is for direct CLI invocation: ze bgp plugin vpn --nlri <hex>
// Output is plain JSON array or text (no "decoded json" prefix).
// Errors go to errOut (typically stderr), results go to output (typically stdout).
func RunCLIDecode(hexData, family string, textOutput bool, output, errOut io.Writer) int {
	writeErr := func(format string, args ...any) {
		n, e := io.WriteString(errOut, fmt.Sprintf(format, args...))
		_ = n
		_ = e
	}
	writeOut := func(s string) {
		n, e := io.WriteString(output, s+"\n")
		_ = n
		_ = e
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeErr("error: invalid hex: %v\n", err)
		return 1
	}

	if !isValidVPNFamily(family) {
		writeErr("error: invalid family: %s (expected ipv4/vpn or ipv6/vpn)\n", family)
		return 1
	}

	results := decodeVPNNLRI(family, data)
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

// RunVPNDecode runs the plugin in decode mode for ze bgp decode (engine protocol).
func RunVPNDecode(input io.Reader, output io.Writer) int {
	writeUnknown := func() {
		_, err := fmt.Fprintln(output, "decoded unknown")
		if err != nil {
			vpnLogger.Debug("write error", "err", err)
		}
	}

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
	return 0
}

// handleDecodeNLRI handles: decode nlri <family> <hex>.
func handleDecodeNLRI(parts []string, format string, output io.Writer, writeUnknown func()) {
	if len(parts) < 4 {
		writeUnknown()
		return
	}

	family := strings.ToLower(parts[2])
	hexData := parts[3]

	if !isValidVPNFamily(family) {
		writeUnknown()
		return
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeUnknown()
		return
	}

	results := decodeVPNNLRI(family, data)
	if len(results) == 0 {
		writeUnknown()
		return
	}

	if format == fmtText {
		var texts []string
		for _, r := range results {
			texts = append(texts, formatVPNTextSingle(r))
		}
		_, err := fmt.Fprintln(output, "decoded text "+strings.Join(texts, "; "))
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
	_, err = fmt.Fprintln(output, "decoded json "+string(jsonBytes))
	if err != nil {
		vpnLogger.Debug("write error", "err", err)
	}
}

// isValidVPNFamily checks if family is a VPN family.
func isValidVPNFamily(family string) bool {
	return family == "ipv4/vpn" || family == "ipv6/vpn"
}

// decodeVPNNLRI decodes VPN NLRI wire bytes to array of JSON maps.
// MP_REACH/MP_UNREACH can contain multiple packed NLRIs.
func decodeVPNNLRI(family string, data []byte) []map[string]any {
	var results []map[string]any
	remaining := data

	// Determine AFI from family
	afi := AFIIPv4
	if strings.HasPrefix(family, "ipv6") {
		afi = AFIIPv6
	}

	for len(remaining) > 0 {
		v, rest, err := ParseVPN(afi, SAFIVPN, remaining, false)
		if err != nil {
			vpnLogger.Debug("parse vpn failed", "err", err)
			// Add as unparsed
			results = append(results, map[string]any{
				"parsed": false,
				"raw":    fmt.Sprintf("%X", remaining),
			})
			break
		}
		results = append(results, vpnToJSON(v))
		remaining = rest
	}

	return results
}

// vpnToJSON converts VPN route to JSON representation.
// Format: {"rd": "...", "prefix": "...", "labels": [[n], ...]}.
func vpnToJSON(v *VPN) map[string]any {
	result := map[string]any{
		"rd":     v.rd.String(),
		"prefix": v.prefix.String(),
	}

	// Format labels as nested array [[label1], [label2], ...]
	if len(v.labels) > 0 {
		labels := make([][]int, len(v.labels))
		for i, l := range v.labels {
			labels[i] = []int{int(l)}
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
	var parts []string

	// Determine family from prefix
	family := "VPNv4"
	if prefix, ok := result["prefix"].(string); ok && strings.Contains(prefix, ":") {
		family = "VPNv6"
	}
	parts = append(parts, family)

	if v, ok := result["rd"].(string); ok {
		parts = append(parts, "rd="+v)
	}
	if v, ok := result["prefix"].(string); ok {
		parts = append(parts, "prefix="+v)
	}
	if v, ok := result["labels"].([][]int); ok && len(v) > 0 {
		var labelStrs []string
		for _, l := range v {
			if len(l) > 0 {
				labelStrs = append(labelStrs, fmt.Sprintf("%d", l[0]))
			}
		}
		parts = append(parts, "label="+strings.Join(labelStrs, ","))
	}
	if v, ok := result["path-id"].(uint32); ok && v != 0 {
		parts = append(parts, fmt.Sprintf("path-id=%d", v))
	}

	if len(parts) == 0 {
		return "(empty)"
	}
	return strings.Join(parts, " ")
}

// ParseFamily wraps nlri.ParseFamily for use by this package.
func ParseFamily(s string) (Family, bool) {
	return nlri.ParseFamily(s)
}
