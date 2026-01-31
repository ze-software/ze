// Package evpn implements an EVPN family plugin for ze.
// It handles decoding of EVPN NLRI (RFC 7432, 9136) for the decode mode protocol.
package evpn

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// evpnLogger is the package-level logger, disabled by default.
var evpnLogger = slogutil.DiscardLogger()

// SetEVPNLogger sets the package-level logger.
// Called by cmd/ze/bgp/plugin_evpn.go with slogutil.PluginLogger().
func SetEVPNLogger(l *slog.Logger) {
	if l != nil {
		evpnLogger = l
	}
}

// EVPNPlugin implements an EVPN family plugin.
// For now, it only supports decode mode (started with --decode).
type EVPNPlugin struct {
	input  *bufio.Scanner
	output io.Writer
}

// MaxLineSize is the maximum size of a single input line (1MB).
const MaxLineSize = 1024 * 1024

// NewEVPNPlugin creates a new EVPN Plugin.
func NewEVPNPlugin(input io.Reader, output io.Writer) *EVPNPlugin {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, MaxLineSize), MaxLineSize)
	return &EVPNPlugin{
		input:  scanner,
		output: output,
	}
}

// Run starts the evpn plugin in normal mode.
func (p *EVPNPlugin) Run() int {
	evpnLogger.Debug("evpn plugin starting")
	p.doStartupProtocol()
	evpnLogger.Debug("evpn plugin startup complete, entering event loop")
	p.eventLoop()
	return 0
}

// doStartupProtocol performs the 5-stage plugin registration protocol.
func (p *EVPNPlugin) doStartupProtocol() {
	// Stage 1: Declaration - claim EVPN family decode
	p.send("declare family l2vpn evpn decode")
	p.send("declare rfc 7432")
	p.send("declare rfc 9136")
	p.send("declare encoding hex")
	p.send("declare done")

	// Stage 2: Parse config (EVPN plugin doesn't need config)
	p.waitForLine("config done")

	// Stage 3: No explicit capability injection needed.
	p.send("capability done")

	// Stage 4: Wait for registry
	p.waitForLine("registry done")

	// Stage 5: Ready
	p.send("ready")
}

// eventLoop handles decode requests from the engine.
func (p *EVPNPlugin) eventLoop() {
	for p.input.Scan() {
		line := p.input.Text()
		if len(line) == 0 {
			continue
		}

		evpnLogger.Debug("received", "line", line[:min(80, len(line))])

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
func (p *EVPNPlugin) handleRequest(line string) string {
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
func (p *EVPNPlugin) handleDecodeRequest(parts []string) string {
	if len(parts) < 4 {
		return respDecodedUnk
	}

	family := strings.ToLower(parts[2])
	hexData := parts[3]

	if !isValidEVPNFamily(family) {
		return respDecodedUnk
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		return respDecodedUnk
	}

	results := decodeEVPNNLRI(data)
	if len(results) == 0 {
		return respDecodedUnk
	}

	jsonBytes, err := json.Marshal(results)
	if err != nil {
		return respDecodedUnk
	}

	return respDecodedJSON + string(jsonBytes)
}

// waitForLine reads lines until one matches the expected line.
func (p *EVPNPlugin) waitForLine(expected string) {
	for p.input.Scan() {
		line := p.input.Text()
		if line == expected {
			return
		}
	}
}

// send sends raw output to ze.
// Write errors are logged but not propagated - if the pipe is broken, the plugin exits anyway.
func (p *EVPNPlugin) send(msg string) {
	if _, err := fmt.Fprintf(p.output, "%s\n", msg); err != nil {
		evpnLogger.Debug("write error", "err", err)
	}
}

// GetEVPNYANG returns the embedded YANG schema for the evpn plugin.
// EVPN plugin doesn't augment config schema, returns empty.
func GetEVPNYANG() string {
	return ""
}

// RunCLIDecode decodes EVPN NLRI from hex string for CLI mode.
// This is for direct CLI invocation: ze bgp plugin evpn --nlri <hex>
// Output is plain JSON array or text (no "decoded json" prefix).
// Errors go to errOut (typically stderr), results go to output (typically stdout).
func RunCLIDecode(hexData, family string, textOutput bool, output, errOut io.Writer) int {
	writeErr := func(format string, args ...any) {
		_, e := fmt.Fprintf(errOut, format, args...)
		_ = e // CLI output - pipe failure is unrecoverable
	}
	writeOut := func(s string) {
		_, e := fmt.Fprintln(output, s)
		_ = e // CLI output - pipe failure is unrecoverable
	}

	if !isValidEVPNFamily(family) {
		writeErr("error: invalid family: %s (expected l2vpn/evpn)\n", family)
		return 1
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeErr("error: invalid hex: %v\n", err)
		return 1
	}

	results := decodeEVPNNLRI(data)
	if len(results) == 0 {
		writeErr("error: no valid EVPN routes decoded\n")
		return 1
	}

	if textOutput {
		for _, r := range results {
			writeOut(formatEVPNTextSingle(r))
		}
		return 0
	}

	// JSON output (default)
	jsonBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		writeErr("error: JSON encoding failed: %v\n", err)
		return 1
	}
	writeOut(string(jsonBytes))
	return 0
}

// RunEVPNDecode runs the plugin in decode mode for ze bgp decode (engine protocol).
func RunEVPNDecode(input io.Reader, output io.Writer) int {
	writeUnknown := func() {
		if _, err := fmt.Fprintln(output, "decoded unknown"); err != nil {
			evpnLogger.Debug("write error", "err", err)
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

	if !isValidEVPNFamily(family) {
		writeUnknown()
		return
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeUnknown()
		return
	}

	results := decodeEVPNNLRI(data)
	if len(results) == 0 {
		writeUnknown()
		return
	}

	if format == fmtText {
		var texts []string
		for _, r := range results {
			texts = append(texts, formatEVPNTextSingle(r))
		}
		if _, err := fmt.Fprintln(output, "decoded text "+strings.Join(texts, "; ")); err != nil {
			evpnLogger.Debug("write error", "err", err)
		}
		return
	}

	jsonBytes, err := json.Marshal(results)
	if err != nil {
		writeUnknown()
		return
	}
	if _, err := fmt.Fprintln(output, "decoded json "+string(jsonBytes)); err != nil {
		evpnLogger.Debug("write error", "err", err)
	}
}

// isValidEVPNFamily checks if family is an EVPN family.
func isValidEVPNFamily(family string) bool {
	return family == "l2vpn/evpn"
}

// decodeEVPNNLRI decodes EVPN NLRI wire bytes to array of JSON maps.
// MP_REACH/MP_UNREACH can contain multiple packed NLRIs.
func decodeEVPNNLRI(data []byte) []map[string]any {
	var results []map[string]any
	remaining := data

	for len(remaining) >= 2 {
		routeType := remaining[0]
		routeLen := int(remaining[1])

		if len(remaining) < 2+routeLen {
			// Truncated - add as unparsed
			results = append(results, map[string]any{
				"code":   int(routeType),
				"parsed": false,
				"raw":    fmt.Sprintf("%X", remaining),
			})
			break
		}

		routeData := remaining[:2+routeLen]

		// Parse single NLRI
		evpn, _, err := ParseEVPN(routeData, false)
		if err != nil {
			evpnLogger.Debug("parse evpn failed", "err", err)
			results = append(results, map[string]any{
				"code":   int(routeType),
				"parsed": false,
				"raw":    fmt.Sprintf("%X", routeData),
			})
		} else {
			results = append(results, evpnToJSON(evpn, routeData))
		}

		remaining = remaining[2+routeLen:]
	}

	return results
}

// evpnToJSON converts EVPN route to JSON representation.
// rawData is included in output as "raw" field for debugging.
func evpnToJSON(e EVPN, rawData []byte) map[string]any {
	result := make(map[string]any)

	// For unparsed routes (EVPNGeneric), only output code, parsed, raw
	if _, ok := e.(*EVPNGeneric); ok {
		result["code"] = int(e.RouteType())
		result["parsed"] = false
		result["raw"] = fmt.Sprintf("%X", rawData)
		return result
	}

	// Match expected format: code, parsed, raw, name, rd, etc.
	result["code"] = int(e.RouteType())
	result["parsed"] = true
	result["raw"] = fmt.Sprintf("%X", rawData)
	result["name"] = evpnRouteName(e.RouteType())
	result["rd"] = e.RD().String()

	switch v := e.(type) {
	case *EVPNType1:
		result["esi"] = formatESIForJSON(v.ESI())
		result["ethernet-tag"] = v.EthernetTag()
		result["label"] = formatLabelsForJSON(v.Labels())

	case *EVPNType2:
		result["esi"] = formatESIForJSON(v.ESI())
		result["ethernet-tag"] = v.EthernetTag()
		result["mac"] = formatMACUpper(v.MAC())
		if v.IP().IsValid() {
			result["ip"] = v.IP().String()
		}
		result["label"] = formatLabelsForJSON(v.Labels())

	case *EVPNType3:
		result["ethernet-tag"] = v.EthernetTag()
		result["originator"] = v.OriginatorIP().String()

	case *EVPNType4:
		result["esi"] = formatESIForJSON(v.ESI())
		result["originator"] = v.OriginatorIP().String()

	case *EVPNType5:
		result["esi"] = formatESIForJSON(v.ESI())
		result["ethernet-tag"] = v.EthernetTag()
		result["prefix"] = v.Prefix().String()
		if v.Gateway().IsValid() && !v.Gateway().IsUnspecified() {
			result["gateway"] = v.Gateway().String()
		}
		result["label"] = formatLabelsForJSON(v.Labels())
	}

	return result
}

// evpnRouteName returns the human-readable name for an EVPN route type.
func evpnRouteName(t EVPNRouteType) string {
	switch t {
	case EVPNRouteType1:
		return "Ethernet Auto-Discovery"
	case EVPNRouteType2:
		return "MAC/IP advertisement"
	case EVPNRouteType3:
		return "Inclusive Multicast"
	case EVPNRouteType4:
		return "Ethernet Segment"
	case EVPNRouteType5:
		return "IP Prefix"
	}
	return fmt.Sprintf("EVPN Type %d", t)
}

// formatESIForJSON formats ESI for JSON output ("-" if zero).
func formatESIForJSON(esi ESI) string {
	if esi.IsZero() {
		return "-"
	}
	return esi.String()
}

// formatMACUpper formats MAC address in uppercase.
func formatMACUpper(mac [6]byte) string {
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
		mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
}

// formatLabelsForJSON formats labels as nested array [[label1], [label2], ...].
func formatLabelsForJSON(labels []uint32) [][]int {
	if len(labels) == 0 {
		return [][]int{{0}}
	}
	result := make([][]int, len(labels))
	for i, l := range labels {
		result[i] = []int{int(l)}
	}
	return result
}

// formatMAC formats a MAC address as colon-separated hex.
func formatMAC(mac [6]byte) string {
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
}

// formatEVPNTextSingle formats a single EVPN route as human-readable text.
func formatEVPNTextSingle(result map[string]any) string {
	var parts []string

	if v, ok := result["name"].(string); ok {
		parts = append(parts, v)
	}
	if v, ok := result["rd"].(string); ok {
		parts = append(parts, "rd="+v)
	}
	if v, ok := result["esi"].(string); ok && v != "00:00:00:00:00:00:00:00:00:00" {
		parts = append(parts, "esi="+v)
	}
	if v, ok := result["mac"].(string); ok {
		parts = append(parts, "mac="+v)
	}
	if v, ok := result["ip"].(string); ok {
		parts = append(parts, "ip="+v)
	}
	if v, ok := result["prefix"].(string); ok {
		parts = append(parts, "prefix="+v)
	}
	if v, ok := result["originator"].(string); ok {
		parts = append(parts, "originator="+v)
	}
	if v, ok := result["gateway"].(string); ok {
		parts = append(parts, "gateway="+v)
	}
	if v, ok := result["ethernet-tag"].(uint32); ok && v != 0 {
		parts = append(parts, fmt.Sprintf("etag=%d", v))
	}
	if v, ok := result["labels"].([]uint32); ok && len(v) > 0 {
		labels := make([]string, len(v))
		for i, l := range v {
			labels[i] = fmt.Sprintf("%d", l)
		}
		parts = append(parts, "labels="+strings.Join(labels, ","))
	}

	if len(parts) == 0 {
		return "(empty)"
	}
	return strings.Join(parts, " ")
}
