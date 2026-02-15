// Package evpn implements an EVPN family plugin for ze.
// It handles decoding of EVPN NLRI (RFC 7432, 9136) for the decode mode protocol.
package bgp_nlri_evpn

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
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

// RunEVPNPlugin runs the EVPN plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func RunEVPNPlugin(engineConn, callbackConn net.Conn) int {
	evpnLogger.Debug("evpn plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-evpn", engineConn, callbackConn)
	defer func() { _ = p.Close() }()

	p.OnDecodeNLRI(DecodeNLRIHex)

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		Families: []sdk.FamilyDecl{
			{Name: "l2vpn/evpn", Mode: "decode"},
		},
	})
	if err != nil {
		evpnLogger.Error("evpn plugin failed", "error", err)
		return 1
	}

	return 0
}

// DecodeNLRIHex decodes EVPN NLRI from hex bytes, returning JSON.
// This is the in-process fast path registered in the plugin registry.
// Same logic as the OnDecodeNLRI SDK callback but callable without RPC.
func DecodeNLRIHex(family, hexStr string) (string, error) {
	if !isValidEVPNFamily(family) {
		return "", fmt.Errorf("unsupported family: %s", family)
	}

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return "", fmt.Errorf("invalid hex: %w", err)
	}

	results := decodeEVPNNLRI(data)
	if len(results) == 0 {
		return "", fmt.Errorf("no valid EVPN routes decoded")
	}

	jsonBytes, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("JSON encoding failed: %w", err)
	}

	return string(jsonBytes), nil
}

// EncodeNLRIHex encodes EVPN NLRI from text args, returning hex bytes.
// Args format: <route-type> followed by key-value pairs:
//
//	"type2" "rd" <rd> "esi" <esi> "ethernet-tag" <tag> "mac" <mac> ["ip" <ip>] ["label" <l>]...
//	"type3" "rd" <rd> "ethernet-tag" <tag> "ip" <ip>
//	"type5" "rd" <rd> ["esi" <esi>] "ethernet-tag" <tag> "prefix" <pfx> ["gateway" <gw>] ["label" <l>]...
//
// This is the in-process fast path registered in the plugin registry.
func EncodeNLRIHex(family string, args []string) (string, error) {
	if !isValidEVPNFamily(family) {
		return "", fmt.Errorf("unsupported family: %s", family)
	}
	if len(args) == 0 {
		return "", fmt.Errorf("empty args")
	}

	routeType := args[0]
	params, err := parseEVPNEncodeArgs(args[1:])
	if err != nil {
		return "", err
	}

	evpnNLRI, err := buildEVPNFromParams(routeType, params)
	if err != nil {
		return "", err
	}

	buf := make([]byte, evpnNLRI.Len())
	evpnNLRI.WriteTo(buf, 0)
	return strings.ToUpper(hex.EncodeToString(buf)), nil
}

// evpnEncodeParams holds parsed EVPN encode parameters.
type evpnEncodeParams struct {
	rd          nlri.RouteDistinguisher
	esi         [10]byte
	ethernetTag uint32
	mac         [6]byte
	ip          netip.Addr
	prefix      netip.Prefix
	gateway     netip.Addr
	labels      []uint32
	hasRD       bool
}

// evpnEncodeKeys is the set of valid keys for EVPN encoding.
var evpnEncodeKeys = map[string]bool{
	"rd": true, "esi": true, "ethernet-tag": true, "mac": true,
	"ip": true, "prefix": true, "gateway": true, "label": true,
}

// parseEVPNEncodeArgs parses key-value pairs for EVPN encoding.
// Returns error on unknown keys (fail-on-unknown).
func parseEVPNEncodeArgs(kvArgs []string) (*evpnEncodeParams, error) {
	p := &evpnEncodeParams{}
	for i := 0; i < len(kvArgs); i++ {
		key := kvArgs[i]
		if !evpnEncodeKeys[key] {
			return nil, fmt.Errorf("unknown EVPN key: %s", key)
		}
		i++
		if i >= len(kvArgs) {
			return nil, fmt.Errorf("key %q requires value", key)
		}
		if err := p.setField(key, kvArgs[i]); err != nil {
			return nil, err
		}
	}
	if !p.hasRD {
		return nil, fmt.Errorf("rd required for EVPN")
	}
	return p, nil
}

// setField sets a single field from a key-value pair.
func (p *evpnEncodeParams) setField(key, value string) error {
	switch key {
	case "rd":
		parsed, err := nlri.ParseRDString(value)
		if err != nil {
			return fmt.Errorf("invalid rd: %w", err)
		}
		p.rd = parsed
		p.hasRD = true
	case "esi":
		parsed, err := ParseESIString(value)
		if err != nil {
			return fmt.Errorf("invalid esi: %w", err)
		}
		p.esi = [10]byte(parsed)
	case "ethernet-tag":
		v, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid ethernet-tag: %w", err)
		}
		p.ethernetTag = uint32(v)
	case "mac":
		parsed, err := evpnParseMAC(value)
		if err != nil {
			return fmt.Errorf("invalid mac: %w", err)
		}
		p.mac = parsed
	case "ip":
		parsed, err := netip.ParseAddr(value)
		if err != nil {
			return fmt.Errorf("invalid ip: %w", err)
		}
		p.ip = parsed
	case "prefix":
		parsed, err := netip.ParsePrefix(value)
		if err != nil {
			return fmt.Errorf("invalid prefix: %w", err)
		}
		p.prefix = parsed
	case "gateway":
		parsed, err := netip.ParseAddr(value)
		if err != nil {
			return fmt.Errorf("invalid gateway: %w", err)
		}
		p.gateway = parsed
	case "label":
		v, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid label: %w", err)
		}
		p.labels = append(p.labels, uint32(v))
	}
	return nil
}

// evpnRouteTypes is the set of valid EVPN route types for encoding.
var evpnRouteTypes = map[string]bool{
	"type2": true, "type3": true, "type5": true,
}

// buildEVPNFromParams creates an EVPN NLRI from parsed parameters.
// Returns error for unknown route types (fail-on-unknown).
func buildEVPNFromParams(routeType string, p *evpnEncodeParams) (EVPN, error) {
	if !evpnRouteTypes[routeType] {
		return nil, fmt.Errorf("unsupported EVPN route type: %s", routeType)
	}
	switch routeType {
	case "type2":
		return NewEVPNType2(p.rd, p.esi, p.ethernetTag, p.mac, p.ip, p.labels), nil
	case "type3":
		return NewEVPNType3(p.rd, p.ethernetTag, p.ip), nil
	case "type5":
		return NewEVPNType5(p.rd, p.esi, p.ethernetTag, p.prefix, p.gateway, p.labels), nil
	}
	return nil, fmt.Errorf("unsupported EVPN route type: %s", routeType)
}

// evpnParseMAC parses a MAC address string like "00:11:22:33:44:55".
func evpnParseMAC(s string) ([6]byte, error) {
	var mac [6]byte
	parts := strings.Split(s, ":")
	if len(parts) != 6 {
		return mac, fmt.Errorf("invalid MAC format: %s", s)
	}
	for i, p := range parts {
		v, err := strconv.ParseUint(p, 16, 8)
		if err != nil {
			return mac, fmt.Errorf("invalid MAC byte %q: %w", p, err)
		}
		mac[i] = byte(v)
	}
	return mac, nil
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
