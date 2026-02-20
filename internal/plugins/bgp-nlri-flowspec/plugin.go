// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec NLRI plugin
// Design: rfc/short/rfc5575.md
//
// Package flowspec implements a FlowSpec family plugin for ze.
// It handles decoding of FlowSpec NLRI (RFC 8955, 8956) for the decode mode protocol.
//
// RFC 8955: Dissemination of Flow Specification Rules (IPv4 FlowSpec)
// RFC 8956: Dissemination of Flow Specification Rules for IPv6
package bgp_nlri_flowspec

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
	"slices"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// flowLogger is the package-level logger, disabled by default.
var flowLogger = slogutil.DiscardLogger()

// SetFlowSpecLogger sets the package-level logger.
// Called by cmd/ze/bgp/plugin_flowspec.go with slogutil.PluginLogger().
func SetFlowSpecLogger(l *slog.Logger) {
	if l != nil {
		flowLogger = l
	}
}

// RunFlowSpecPlugin runs the FlowSpec plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func RunFlowSpecPlugin(engineConn, callbackConn net.Conn) int {
	flowLogger.Debug("flowspec plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-flowspec", engineConn, callbackConn)
	defer func() { _ = p.Close() }()

	p.OnEncodeNLRI(EncodeNLRIHex)
	p.OnDecodeNLRI(DecodeNLRIHex)

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		Families: []sdk.FamilyDecl{
			{Name: "ipv4/flow", Mode: "both"},
			{Name: "ipv6/flow", Mode: "both"},
			{Name: "ipv4/flow-vpn", Mode: "both"},
			{Name: "ipv6/flow-vpn", Mode: "both"},
		},
	})
	if err != nil {
		flowLogger.Error("flowspec plugin failed", "error", err)
		return 1
	}

	return 0
}

// DecodeNLRIHex decodes FlowSpec NLRI from hex bytes, returning JSON.
// This is the in-process fast path registered in the plugin registry.
// Same logic as the OnDecodeNLRI SDK callback but callable without RPC.
func DecodeNLRIHex(family, hexStr string) (string, error) {
	if !isValidFlowSpecFamily(family) {
		return "", fmt.Errorf("unsupported family: %s", family)
	}

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return "", fmt.Errorf("invalid hex: %w", err)
	}

	result := decodeFlowSpecNLRI(family, data)
	if result == nil {
		return "", fmt.Errorf("no valid FlowSpec decoded")
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("JSON encoding failed: %w", err)
	}

	return string(jsonBytes), nil
}

// EncodeNLRIHex encodes FlowSpec NLRI from text args, returning hex bytes.
// This is the in-process fast path registered in the plugin registry.
// Same logic as the OnEncodeNLRI SDK callback but callable without RPC.
func EncodeNLRIHex(family string, args []string) (string, error) {
	if !isValidFlowSpecFamily(family) {
		return "", fmt.Errorf("invalid family: %s", family)
	}

	fam, ok := nlri.ParseFamily(family)
	if !ok {
		return "", fmt.Errorf("unknown family: %s", family)
	}

	wireBytes, err := EncodeFlowSpecComponents(fam, args)
	if err != nil {
		return "", err
	}

	return strings.ToUpper(hex.EncodeToString(wireBytes)), nil
}

// Protocol constants for request/response handling.
const (
	cmdEncode       = "encode"
	cmdDecode       = "decode"
	objTypeNLRI     = "nlri"
	fmtJSON         = "json"
	fmtText         = "text"
	respDecodedUnk  = "decoded unknown"
	respEncodedErr  = "encoded error "
	respEncodedHex  = "encoded hex "
	respDecodedJSON = "decoded json "
	respDecodedText = "decoded text "
)

// GetFlowSpecYANG returns the embedded YANG schema for the flowspec plugin.
// FlowSpec plugin doesn't augment config schema, returns empty.
func GetFlowSpecYANG() string {
	return ""
}

// RunCLIDecode decodes FlowSpec NLRI from hex string for CLI mode.
// This is for direct CLI invocation: ze plugin flowspec --nlri <hex>
// Output is plain JSON or text (no "decoded json" prefix).
// Errors go to errOut (typically stderr), results go to output (typically stdout).
func RunCLIDecode(hexData, family string, textOutput bool, output, errOut io.Writer) int {
	data, err := hex.DecodeString(hexData)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "error: invalid hex: %v\n", err)
		return 1
	}

	if !isValidFlowSpecFamily(family) {
		_, _ = fmt.Fprintf(errOut, "error: invalid family: %s\n", family)
		return 1
	}

	result := decodeFlowSpecNLRI(family, data)
	if result == nil {
		_, _ = fmt.Fprintln(errOut, "error: no valid FlowSpec decoded")
		return 1
	}

	if textOutput {
		text := formatFlowSpecText(result)
		_, _ = fmt.Fprintln(output, text)
		return 0
	}

	// JSON output (default)
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "error: JSON encoding failed: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintln(output, string(jsonBytes))
	return 0
}

// FlowSpecFamilies returns the address families this plugin can decode.
func FlowSpecFamilies() []string {
	return []string{
		"ipv4/flow",
		"ipv6/flow",
		"ipv4/flow-vpn",
		"ipv6/flow-vpn",
	}
}

// RunFlowSpecDecode runs the plugin in decode/encode mode for ze bgp decode/encode.
// Handles both decode and encode requests on stdin, writes responses to stdout.
//
// Decode formats:
//   - "decode nlri <family> <hex>" → JSON (default)
//   - "decode json nlri <family> <hex>" → JSON (explicit)
//   - "decode text nlri <family> <hex>" → human-readable text
//
// Encode formats:
//   - "encode nlri <family> <components...>" → text input (default)
//   - "encode text nlri <family> <components...>" → text input (explicit)
//   - "encode json nlri <family> <json>" → JSON input
//
// Response: "encoded hex <hex>" or "encoded error <msg>".
func RunFlowSpecDecode(input io.Reader, output io.Writer) int {
	// Response writers - use io.WriteString, errors discarded for protocol writes.
	writeResponse := func(s string) {
		_, err := io.WriteString(output, s)
		_ = err // Protocol writes - pipe failure causes exit
	}
	writeUnknown := func() { writeResponse("decoded unknown\n") }
	writeError := func(msg string) { writeResponse("encoded error " + msg + "\n") }

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

		// Handle format specifier for decode/encode
		// Decode: default=json, Encode: default=text
		format := fmtJSON
		if cmd == cmdEncode {
			format = fmtText // encode defaults to text input
		}

		if objType == fmtJSON || objType == fmtText {
			format = objType
			// Shift parts: cmd format nlri → cmd nlri (with format stored)
			if len(parts) < 4 {
				if cmd == cmdDecode {
					writeUnknown()
				} else {
					writeError("missing arguments")
				}
				continue
			}
			objType = parts[2]
			parts = append([]string{cmd, objType}, parts[3:]...)
		}

		switch {
		case cmd == cmdDecode && objType == objTypeNLRI:
			handleDecodeNLRI(parts, format, output, writeUnknown)
		case cmd == cmdEncode && objType == objTypeNLRI:
			if format == fmtJSON {
				handleEncodeNLRIFromJSON(parts, output, writeError)
			} else {
				handleEncodeNLRI(parts, output, writeError)
			}
		case cmd == cmdDecode:
			writeUnknown()
		case cmd == cmdEncode:
			writeError("unsupported object type")
		}
	}
	return 0
}

// handleDecodeNLRI handles: decode nlri <family> <hex>.
// Format parameter determines output: "json" or "text".
func handleDecodeNLRI(parts []string, format string, output io.Writer, writeUnknown func()) {
	if len(parts) < 4 {
		writeUnknown()
		return
	}

	family := strings.ToLower(parts[2])
	hexData := parts[3]

	if !isValidFlowSpecFamily(family) {
		writeUnknown()
		return
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeUnknown()
		return
	}

	result := decodeFlowSpecNLRI(family, data)
	if result == nil {
		writeUnknown()
		return
	}

	// Response writers - use io.WriteString, errors discarded for protocol writes.
	writeResponseLine := func(s string) {
		_, err := io.WriteString(output, s)
		_ = err // Protocol writes - pipe failure causes exit
	}

	if format == "text" {
		text := formatFlowSpecText(result)
		writeResponseLine("decoded text " + text + "\n")
		return
	}

	// Default: JSON
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		writeUnknown()
		return
	}
	writeResponseLine("decoded json " + string(jsonBytes) + "\n")
}

// formatFlowSpecText formats FlowSpec components as human-readable text.
// Output is single-line, space-separated component descriptions.
func formatFlowSpecText(result map[string]any) string {
	var parts []string

	// Order components logically: destination, source, protocol, ports, etc.
	componentOrder := []string{
		"destination", "source", "protocol",
		"port", "destination-port", "source-port",
		"icmp-type", "icmp-code", "tcp-flags", "packet-length", "dscp",
		"fragment", "flow-label", "rd",
	}

	for _, key := range componentOrder {
		if val, ok := result[key]; ok {
			formatted := formatComponentValue(key, val)
			if formatted != "" {
				parts = append(parts, key+" "+formatted)
			}
		}
	}

	// Add any remaining keys not in the order list
	for key, val := range result {
		if !contains(componentOrder, key) {
			formatted := formatComponentValue(key, val)
			if formatted != "" {
				parts = append(parts, key+" "+formatted)
			}
		}
	}

	if len(parts) == 0 {
		return "(empty)"
	}
	return strings.Join(parts, " ")
}

// formatComponentValue formats a single FlowSpec component value for text output.
func formatComponentValue(_ string, val any) string {
	switch v := val.(type) {
	case string:
		return v
	case []any:
		// FlowSpec uses nested arrays for OR/AND grouping
		return formatNestedValues(v)
	case []string:
		return strings.Join(v, ",")
	case float64:
		return fmt.Sprintf("%.0f", v)
	case int:
		return fmt.Sprintf("%d", v)
	}
	return fmt.Sprintf("%v", val)
}

// formatNestedValues formats FlowSpec nested array values.
// FlowSpec JSON uses [[a,b],[c]] for (a AND b) OR c.
func formatNestedValues(vals []any) string {
	parts := make([]string, 0, len(vals))
	for _, v := range vals {
		switch inner := v.(type) {
		case []any:
			// Inner array - AND group
			andParts := make([]string, 0, len(inner))
			for _, item := range inner {
				andParts = append(andParts, fmt.Sprintf("%v", item))
			}
			parts = append(parts, strings.Join(andParts, "&"))
		case string:
			parts = append(parts, inner)
		}
	}
	return strings.Join(parts, "|")
}

// contains checks if a string slice contains a value.
func contains(slice []string, val string) bool {
	return slices.Contains(slice, val)
}

// handleEncodeNLRIFromJSON handles: encode json nlri <family> <json>.
// JSON format matches decode output: {"destination":[["10.0.0.0/24/0"]],...}.
func handleEncodeNLRIFromJSON(parts []string, output io.Writer, writeError func(string)) {
	if len(parts) < 4 {
		writeError("missing family or JSON")
		return
	}

	family := strings.ToLower(parts[2])
	if !isValidFlowSpecFamily(family) {
		writeError("invalid family: " + family)
		return
	}

	// JSON is the remaining part (may have been split by Fields if it had spaces)
	jsonStr := strings.Join(parts[3:], " ")

	// Parse JSON
	var jsonMap map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &jsonMap); err != nil {
		writeError("invalid JSON: " + err.Error())
		return
	}

	// Convert JSON to text components
	textArgs, err := jsonToTextComponents(jsonMap)
	if err != nil {
		writeError(err.Error())
		return
	}

	// Parse family
	fam, ok := nlri.ParseFamily(family)
	if !ok {
		writeError("unknown family: " + family)
		return
	}

	// Encode using existing text encoder
	wireBytes, err := EncodeFlowSpecComponents(fam, textArgs)
	if err != nil {
		writeError(err.Error())
		return
	}

	writeHex := func(s string) {
		_, err := io.WriteString(output, s)
		_ = err // Protocol writes - pipe failure causes exit
	}
	writeHex("encoded hex " + strings.ToUpper(hex.EncodeToString(wireBytes)) + "\n")
}

// jsonToTextComponents converts JSON FlowSpec format to text component args.
// JSON: {"destination":[["10.0.0.0/24/0"]],"protocol":[["=tcp"],["=udp"]]}
// Text: ["destination", "10.0.0.0/24", "protocol", "tcp", "udp"].
//
// For simple OR groups (each inner array has one value), all values go after
// a single keyword: "protocol tcp udp" → one component with OR values.
//
// For complex OR-of-AND groups (inner arrays have multiple values), each OR
// group becomes a separate keyword entry: "port >80 <100 port >443 <500"
// → two components that get merged on decode.
func jsonToTextComponents(m map[string]any) ([]string, error) {
	var args []string

	// Process components in a defined order for consistency
	// JSON keys match text keywords (destination-ipv6, next-header, etc.)
	// RD must come first for VPN families (parsed before components)
	componentOrder := []string{
		"rd", // Route Distinguisher - must be first for VPN families
		"destination", "destination-ipv6", "source", "source-ipv6",
		"protocol", "next-header", "port", "destination-port", "source-port",
		"icmp-type", "icmp-code", "tcp-flags", "packet-length", "dscp",
		"fragment", "flow-label",
	}

	for _, key := range componentOrder {
		val, ok := m[key]
		if !ok {
			continue
		}

		// Handle RD specially - it's a simple string, not an array
		if key == "rd" {
			if rdStr, ok := val.(string); ok {
				args = append(args, "rd", rdStr)
			}
			continue
		}

		// Handle nested array format: [[val1, val2], [val3]]
		// Outer array = OR, Inner array = AND
		arr, ok := val.([]any)
		if !ok {
			continue
		}

		// Check if this is simple OR (each inner array has exactly one value)
		// or complex OR-of-AND (some inner arrays have multiple values)
		isSimpleOR := true
		for _, orGroup := range arr {
			if innerArr, ok := orGroup.([]any); ok && len(innerArr) > 1 {
				isSimpleOR = false
				break
			}
		}

		if isSimpleOR {
			// Simple OR: collect all values, emit once with keyword
			// "protocol tcp udp" → one component with OR values
			var values []string
			for _, orGroup := range arr {
				switch g := orGroup.(type) {
				case []any:
					for _, v := range g {
						if s, ok := v.(string); ok {
							values = append(values, normalizeJSONValue(key, s))
						}
					}
				case string:
					values = append(values, normalizeJSONValue(key, g))
				}
			}
			if len(values) > 0 {
				args = append(args, key)
				args = append(args, values...)
			}
		} else {
			// Complex OR-of-AND: emit each OR group with its own keyword
			// "port >80 <100 port >443 <500" → two components
			// The decoder merges components with same key into one JSON entry
			for _, orGroup := range arr {
				innerArr, ok := orGroup.([]any)
				if !ok {
					if s, ok := orGroup.(string); ok {
						args = append(args, key, normalizeJSONValue(key, s))
					}
					continue
				}
				var values []string
				for _, v := range innerArr {
					if s, ok := v.(string); ok {
						values = append(values, normalizeJSONValue(key, s))
					}
				}
				if len(values) > 0 {
					args = append(args, key)
					args = append(args, values...)
				}
			}
		}
	}

	if len(args) == 0 {
		return nil, fmt.Errorf("no valid components in JSON")
	}

	return args, nil
}

// normalizeJSONValue converts JSON value format to text format.
// E.g., "10.0.0.0/24/0" → "10.0.0.0/24" (strip offset for prefixes).
// E.g., "=tcp" → "tcp" (strip operator for protocol/next-header).
func normalizeJSONValue(key, val string) string {
	// Strip /0 offset suffix from prefixes (destination, source)
	if strings.HasPrefix(key, "destination") || strings.HasPrefix(key, "source") {
		if strings.HasSuffix(val, "/0") {
			// "10.0.0.0/24/0" → "10.0.0.0/24"
			parts := strings.Split(val, "/")
			if len(parts) == 3 {
				return parts[0] + "/" + parts[1]
			}
		}
	}

	// Strip operator prefix for protocol/next-header (text parser expects plain value)
	if key == kwProtocol || key == "next-header" {
		// "=tcp" → "tcp", "=6" → "6"
		val = strings.TrimPrefix(val, "=")
		val = strings.TrimPrefix(val, "<")
		val = strings.TrimPrefix(val, ">")
		val = strings.TrimPrefix(val, "!")
		val = strings.TrimPrefix(val, "&")
	}

	return val
}

// handleEncodeNLRI handles: encode nlri <family> <components...>
// Components: destination <prefix> | source <prefix> | protocol <num> | port <op><num> | ...
func handleEncodeNLRI(parts []string, output io.Writer, writeError func(string)) {
	if len(parts) < 4 {
		writeError("missing family or components")
		return
	}

	family := strings.ToLower(parts[2])
	if !isValidFlowSpecFamily(family) {
		writeError("invalid family: " + family)
		return
	}

	// Parse family using nlri.ParseFamily (still in nlri package)
	fam, ok := nlri.ParseFamily(family)
	if !ok {
		writeError("unknown family: " + family)
		return
	}

	// Parse components from remaining args
	args := parts[3:]
	wireBytes, err := EncodeFlowSpecComponents(fam, args)
	if err != nil {
		writeError(err.Error())
		return
	}

	_, _ = fmt.Fprintf(output, "encoded hex %s\n", strings.ToUpper(hex.EncodeToString(wireBytes)))
}

// isValidFlowSpecFamily checks if family is a FlowSpec family.
func isValidFlowSpecFamily(family string) bool {
	switch family {
	case "ipv4/flow", "ipv6/flow", "ipv4/flow-vpn", "ipv6/flow-vpn":
		return true
	default:
		return false
	}
}

// decodeFlowSpecNLRI decodes FlowSpec NLRI wire bytes to JSON map.
func decodeFlowSpecNLRI(family string, data []byte) map[string]any {
	isVPN := strings.HasSuffix(family, "-vpn")

	// Determine Family from family string
	fam, ok := nlri.ParseFamily(family)
	if !ok {
		flowLogger.Debug("unknown family", "family", family)
		return nil
	}

	var fs *FlowSpec
	var rd *RouteDistinguisher

	if isVPN {
		fsv, err := ParseFlowSpecVPN(fam, data)
		if err != nil {
			flowLogger.Debug("parse flowspec vpn failed", "err", err)
			return nil
		}
		fs = fsv.FlowSpec()
		rdVal := fsv.RD()
		rd = &rdVal
	} else {
		var err error
		fs, err = ParseFlowSpec(fam, data)
		if err != nil {
			flowLogger.Debug("parse flowspec failed", "err", err)
			return nil
		}
	}

	return flowSpecToJSON(fs, family, rd)
}

// flowSpecToJSON converts FlowSpec to JSON representation.
// Format: {"rd": "...", "destination-ipv6": [["prefix/len/offset"]], ...}.
// Note: "family" is NOT included since it's already in the JSON path when embedded.
//
// Multiple components of the same type are merged into a single key with
// combined OR groups. This enables round-trip: if two "port" components
// exist in wire format, they become one "port" key with multiple OR groups.
func flowSpecToJSON(fs *FlowSpec, family string, rd *RouteDistinguisher) map[string]any {
	result := make(map[string]any)

	// Add RD for VPN families
	if rd != nil {
		result["rd"] = rd.String()
	}

	// Determine IPv4 vs IPv6 from family string
	isIPv6 := strings.Contains(family, "ipv6")

	for _, comp := range fs.Components() {
		key, values := componentToJSON(comp, isIPv6)
		// Merge with existing values if key already exists (multiple components of same type)
		if existing, ok := result[key]; ok {
			if existingSlice, ok := existing.([][]string); ok {
				result[key] = append(existingSlice, values...)
			} else {
				result[key] = values
			}
		} else {
			result[key] = values
		}
	}

	return result
}

// componentToJSON converts a FlowComponent to ExaBGP JSON format.
// Returns the key name and nested array values.
func componentToJSON(comp FlowComponent, isIPv6 bool) (string, [][]string) {
	compType := comp.Type()

	switch compType {
	case FlowDestPrefix:
		key := "destination"
		if isIPv6 {
			key = "destination-ipv6"
		}
		prefix := formatPrefixWithOffset(comp)
		return key, [][]string{{prefix}}

	case FlowSourcePrefix:
		key := "source"
		if isIPv6 {
			key = "source-ipv6"
		}
		prefix := formatPrefixWithOffset(comp)
		return key, [][]string{{prefix}}

	case FlowIPProtocol:
		key := "protocol"
		if isIPv6 {
			key = "next-header"
		}
		return key, formatNumericMatches(comp, compType)

	case FlowPort:
		return "port", formatNumericMatches(comp, compType)

	case FlowDestPort:
		return "destination-port", formatNumericMatches(comp, compType)

	case FlowSourcePort:
		return "source-port", formatNumericMatches(comp, compType)

	case FlowICMPType:
		return "icmp-type", formatNumericMatches(comp, compType)

	case FlowICMPCode:
		return "icmp-code", formatNumericMatches(comp, compType)

	case FlowTCPFlags:
		return "tcp-flags", formatBitmaskMatches(comp, tcpFlagValueToNames)

	case FlowPacketLength:
		return "packet-length", formatNumericMatches(comp, compType)

	case FlowDSCP:
		return "dscp", formatNumericMatches(comp, compType)

	case FlowFragment:
		return "fragment", formatBitmaskMatches(comp, fragmentFlagValueToNames)

	case FlowFlowLabel:
		return "flow-label", formatNumericMatches(comp, compType)

	default:
		return fmt.Sprintf("type-%d", compType), [][]string{}
	}
}

// formatPrefixWithOffset formats a prefix component as "prefix/length/offset".
func formatPrefixWithOffset(comp FlowComponent) string {
	prefix := ""
	offset := uint8(0)

	if pc, ok := comp.(interface{ Prefix() netip.Prefix }); ok {
		prefix = pc.Prefix().String()
	}
	if oc, ok := comp.(interface{ Offset() uint8 }); ok {
		offset = oc.Offset()
	}

	return fmt.Sprintf("%s/%d", prefix, offset)
}

// protocolNumberToName maps protocol numbers to names for ExaBGP output.
var protocolNumberToName = map[uint8]string{
	1:   "icmp",
	2:   "igmp",
	6:   "tcp",
	17:  "udp",
	47:  "gre",
	58:  "icmpv6",
	89:  "ospf",
	132: "sctp",
}

// formatNumericMatches formats numeric component matches for ExaBGP JSON.
// Returns nested arrays: [[value1], [value2]] for OR logic.
func formatNumericMatches(comp FlowComponent, compType FlowComponentType) [][]string {
	nc, ok := comp.(interface{ Matches() []FlowMatch })
	if !ok {
		return [][]string{}
	}

	matches := nc.Matches()
	result := make([][]string, 0, len(matches))
	var andGroup []string

	for _, m := range matches {
		valStr := formatNumericValue(m, compType)

		if m.And && len(andGroup) > 0 {
			// Continue AND group
			andGroup = append(andGroup, valStr)
		} else {
			// Start new OR group (flush previous AND group if any)
			if len(andGroup) > 0 {
				result = append(result, andGroup)
			}
			andGroup = []string{valStr}
		}
	}

	// Flush final group
	if len(andGroup) > 0 {
		result = append(result, andGroup)
	}

	return result
}

// formatNumericValue formats a single numeric match value.
func formatNumericValue(m FlowMatch, compType FlowComponentType) string {
	// For protocol, try to use name
	if compType == FlowIPProtocol {
		if name, ok := protocolNumberToName[uint8(m.Value)]; ok { //nolint:gosec // Protocol values are 8-bit
			return formatWithOperator(name, m.Op)
		}
	}

	// Format with operator prefix
	return formatWithOperator(fmt.Sprintf("%d", m.Value), m.Op)
}

// formatWithOperator adds operator prefix to a value string.
func formatWithOperator(value string, op FlowOperator) string {
	// Mask out non-comparison bits
	compOp := op &^ (FlowOpEnd | FlowOpAnd | FlowOpLenMask)

	switch compOp { //nolint:exhaustive // Masked bits cannot match
	case FlowOpEqual:
		return "=" + value
	case FlowOpGreater:
		return ">" + value
	case FlowOpLess:
		return "<" + value
	case FlowOpGreater | FlowOpEqual:
		return ">=" + value
	case FlowOpLess | FlowOpEqual:
		return "<=" + value
	case FlowOpNotEq:
		return "!=" + value
	default:
		return "=" + value
	}
}

// tcpFlagValueToNames maps TCP flag bit values to names.
var tcpFlagValueToNames = map[uint8]string{
	0x01: "fin",
	0x02: "syn",
	0x04: "rst",
	0x08: "push", // ExaBGP uses "push" not "psh"
	0x10: "ack",
	0x20: "urg",
	0x40: "ece",
	0x80: "cwr",
}

// fragmentFlagValueToNames maps fragment flag bit values to names.
var fragmentFlagValueToNames = map[uint8]string{
	0x01: "dont-fragment",
	0x02: "is-fragment",
	0x04: "first-fragment",
	0x08: "last-fragment",
}

// formatBitmaskMatches formats bitmask component matches (TCP flags, Fragment).
// Returns nested arrays with combined flag names.
func formatBitmaskMatches(comp FlowComponent, flagMap map[uint8]string) [][]string {
	nc, ok := comp.(interface{ Matches() []FlowMatch })
	if !ok {
		return [][]string{}
	}

	matches := nc.Matches()
	result := make([][]string, 0, len(matches))

	// Each FlowMatch becomes its own inner array
	// E.g., "=ack+cwr" and "!fin+ece" become [["=ack","cwr"],["!fin","ece"]]
	for _, m := range matches {
		valStrs := formatBitmaskValue(m, flagMap)
		result = append(result, valStrs)
	}

	return result
}

// formatBitmaskValue formats a bitmask value as separate flag elements.
// Returns ["=ack", "cwr"] for ack+cwr with match operator.
// The operator prefix (= or !) is only on the first flag.
func formatBitmaskValue(m FlowMatch, flagMap map[uint8]string) []string {
	// Build prefix from operator
	var prefix string
	if m.Op&FlowOpNot != 0 {
		prefix = "!"
	}
	if m.Op&FlowOpMatch != 0 {
		prefix += "="
	}
	if prefix == "" {
		prefix = "=" // Default to match
	}

	flags := uint8(m.Value) //nolint:gosec // Bitmask values are 8-bit
	var names []string

	// Check each bit in order
	for bit := uint8(0x01); bit != 0; bit <<= 1 {
		if flags&bit != 0 {
			if name, ok := flagMap[bit]; ok {
				names = append(names, name)
			}
		}
	}

	if len(names) == 0 {
		return []string{fmt.Sprintf("%s%d", prefix, flags)}
	}

	// Prefix only on first flag, rest are bare names
	result := make([]string, len(names))
	result[0] = prefix + names[0]
	for i := 1; i < len(names); i++ {
		result[i] = names[i]
	}
	return result
}

// ============================================================================
// Encoding: Text → Wire bytes
// ============================================================================

// FlowSpec component keywords.
const (
	kwDestination     = "destination"      // Type 1 (IPv4)
	kwDestinationIPv6 = "destination-ipv6" // Type 1 (IPv6)
	kwSource          = "source"           // Type 2 (IPv4)
	kwSourceIPv6      = "source-ipv6"      // Type 2 (IPv6)
	kwProtocol        = "protocol"         // Type 3 (IPv4)
	kwNextHeader      = "next-header"      // Type 3 (IPv6)
	kwPort            = "port"             // Type 4
	kwDestPort        = "destination-port" // Type 5
	kwSourcePort      = "source-port"      // Type 6
	kwICMPType        = "icmp-type"        // Type 7
	kwICMPCode        = "icmp-code"        // Type 8
	kwTCPFlags        = "tcp-flags"        // Type 9
	kwPacketLength    = "packet-length"    // Type 10
	kwDSCP            = "dscp"             // Type 11
	kwFragment        = "fragment"         // Type 12
	kwFlowLabel       = "flow-label"       // Type 13 (IPv6 only)
	kwRD              = "rd"               // Route Distinguisher (VPN)
)

// protocolNameToNumber maps protocol names to numbers.
// IANA Protocol Numbers: https://www.iana.org/assignments/protocol-numbers
var protocolNameToNumber = map[string]uint8{
	"icmp":   1,
	"igmp":   2,
	"tcp":    6,
	"udp":    17,
	"gre":    47,
	"icmpv6": 58,
	"ospf":   89,
	"sctp":   132,
}

// tcpFlagNameToValue maps TCP flag names to values.
// RFC 8955 Section 4.2.2.9.
var tcpFlagNameToValue = map[string]uint8{
	"fin":  0x01,
	"syn":  0x02,
	"rst":  0x04,
	"psh":  0x08,
	"push": 0x08, // alias for psh
	"ack":  0x10,
	"urg":  0x20,
	"ece":  0x40,
	"cwr":  0x80,
}

// fragmentFlagNameToValue maps fragment flag names to values.
// RFC 8955 Section 4.2.2.12.
var fragmentFlagNameToValue = map[string]uint8{
	"dont-fragment":  0x01,
	"is-fragment":    0x02,
	"first-fragment": 0x04,
	"last-fragment":  0x08,
	"df":             0x01, // alias
	"isf":            0x02, // alias
	"ff":             0x04, // alias
	"lf":             0x08, // alias
}

// EncodeFlowSpecComponents parses text components and returns wire bytes.
// Format: <component>+ where component is one of:
//   - destination <prefix>
//   - source <prefix>
//   - protocol <num|name>+
//   - port <op><num>+
//   - rd <type:admin:value> (for VPN families)
//   - etc.
//
// This function is used by the engine to delegate FlowSpec text parsing to the plugin.
func EncodeFlowSpecComponents(family Family, args []string) ([]byte, error) {
	isVPN := family.SAFI == SAFIFlowSpecVPN

	var fs *FlowSpec
	var fsv *FlowSpecVPN
	var rd RouteDistinguisher

	// Parse RD first if VPN family
	if isVPN {
		var consumed int
		var err error
		rd, consumed, err = parseRDFromArgs(args)
		if err != nil {
			return nil, err
		}
		args = args[consumed:]
		fsv = NewFlowSpecVPN(family, rd)
	} else {
		fs = NewFlowSpec(family)
	}

	addComponent := func(c FlowComponent) {
		if fsv != nil {
			fsv.AddComponent(c)
		} else {
			fs.AddComponent(c)
		}
	}

	// Parse components
	i := 0
	for i < len(args) {
		comp, consumed, err := parseComponentText(args[i:], family)
		if err != nil {
			return nil, err
		}
		addComponent(comp)
		i += consumed
	}

	// Return wire bytes
	if fsv != nil {
		if len(fsv.Components()) == 0 {
			return nil, fmt.Errorf("flowspec requires at least one component")
		}
		return fsv.Bytes(), nil
	}
	if len(fs.Components()) == 0 {
		return nil, fmt.Errorf("flowspec requires at least one component")
	}
	return fs.Bytes(), nil
}

// parseRDFromArgs parses "rd <value>" from args.
func parseRDFromArgs(args []string) (RouteDistinguisher, int, error) {
	for i := range len(args) - 1 {
		if args[i] == kwRD {
			rd, err := nlri.ParseRDString(args[i+1])
			if err != nil {
				return RouteDistinguisher{}, 0, fmt.Errorf("invalid rd: %w", err)
			}
			return rd, 2, nil
		}
	}
	return RouteDistinguisher{}, 0, fmt.Errorf("rd required for VPN family")
}

// parseComponentText parses a single FlowSpec component from args.
// Named differently from parseFlowComponent in types.go (which parses wire format).
func parseComponentText(args []string, family Family) (FlowComponent, int, error) {
	if len(args) == 0 {
		return nil, 0, fmt.Errorf("expected component")
	}

	keyword := strings.ToLower(args[0])

	switch keyword {
	case kwDestination, kwDestinationIPv6:
		return parsePrefixComponentText(args, FlowDestPrefix, family)
	case kwSource, kwSourceIPv6:
		return parsePrefixComponentText(args, FlowSourcePrefix, family)
	case kwProtocol, kwNextHeader:
		return parseProtocolComponentText(args[1:])
	case kwPort:
		return parseNumericComponentText(args[1:], FlowPort)
	case kwDestPort:
		return parseNumericComponentText(args[1:], FlowDestPort)
	case kwSourcePort:
		return parseNumericComponentText(args[1:], FlowSourcePort)
	case kwICMPType:
		return parseNumericComponentText(args[1:], FlowICMPType)
	case kwICMPCode:
		return parseNumericComponentText(args[1:], FlowICMPCode)
	case kwTCPFlags:
		return parseTCPFlagsComponentText(args[1:])
	case kwPacketLength:
		return parseNumericComponentText(args[1:], FlowPacketLength)
	case kwDSCP:
		return parseNumericComponentText(args[1:], FlowDSCP)
	case kwFragment:
		return parseFragmentComponentText(args[1:])
	case kwFlowLabel:
		return parseNumericComponentText(args[1:], FlowFlowLabel)
	case kwRD:
		// Skip rd - already parsed
		return nil, 2, nil
	}

	// Unknown keyword - return error (not silent ignore)
	return nil, 0, fmt.Errorf("unknown component: %s", keyword)
}

// parsePrefixComponentText parses destination or source prefix from text.
func parsePrefixComponentText(args []string, compType FlowComponentType, family Family) (FlowComponent, int, error) {
	if len(args) < 2 {
		return nil, 0, fmt.Errorf("%s requires prefix", args[0])
	}

	prefix, err := netip.ParsePrefix(args[1])
	if err != nil {
		return nil, 0, fmt.Errorf("invalid prefix: %w", err)
	}

	// Validate AFI match
	if prefix.Addr().Is4() && family.AFI != AFIIPv4 {
		return nil, 0, fmt.Errorf("IPv4 prefix for IPv6 flowspec")
	}
	if prefix.Addr().Is6() && family.AFI != AFIIPv6 {
		return nil, 0, fmt.Errorf("IPv6 prefix for IPv4 flowspec")
	}

	if compType == FlowDestPrefix {
		return NewFlowDestPrefixComponent(prefix), 2, nil
	}
	return NewFlowSourcePrefixComponent(prefix), 2, nil
}

// parseProtocolComponentText parses protocol values (names or numbers).
func parseProtocolComponentText(args []string) (FlowComponent, int, error) {
	var protocols []uint8
	consumed := 0

	for i := range args {
		token := strings.ToLower(args[i])
		if isComponentKeyword(token) {
			break
		}

		// Try name first
		if num, ok := protocolNameToNumber[token]; ok {
			protocols = append(protocols, num)
			consumed++
			continue
		}

		// Try number
		num, err := parseUint8(token)
		if err != nil {
			if consumed == 0 {
				return nil, 0, fmt.Errorf("invalid protocol: %s", token)
			}
			break
		}
		protocols = append(protocols, num)
		consumed++
	}

	if len(protocols) == 0 {
		return nil, 0, fmt.Errorf("protocol requires value")
	}

	return NewFlowIPProtocolComponent(protocols...), consumed + 1, nil
}

// parseNumericComponentText parses numeric component with operators.
func parseNumericComponentText(args []string, compType FlowComponentType) (FlowComponent, int, error) {
	var matches []FlowMatch
	consumed := 0

	maxValue := componentMaxValue(compType)

	for i := range args {
		token := args[i]
		if isComponentKeyword(strings.ToLower(token)) {
			break
		}

		op, value, err := parseOperatorValue(token)
		if err != nil {
			if consumed == 0 {
				return nil, 0, fmt.Errorf("invalid %s value: %w", compType, err)
			}
			break
		}

		if value > maxValue {
			return nil, 0, fmt.Errorf("%s value %d exceeds max %d", compType, value, maxValue)
		}

		matches = append(matches, FlowMatch{
			Op:    op,
			Value: value,
			And:   consumed > 0,
		})
		consumed++
	}

	if len(matches) == 0 {
		return nil, 0, fmt.Errorf("%s requires value", compType)
	}

	return NewFlowNumericComponent(compType, matches), consumed + 1, nil
}

// parseTCPFlagsComponentText parses TCP flags with bitmask operators.
func parseTCPFlagsComponentText(args []string) (FlowComponent, int, error) {
	var matches []FlowMatch
	consumed := 0

	for i := range args {
		token := strings.ToLower(args[i])
		if isComponentKeyword(token) {
			break
		}

		// Parse modifiers and flags
		op, flags, hasAndPrefix, err := parseBitmaskValue(token, tcpFlagNameToValue)
		if err != nil {
			if consumed == 0 {
				return nil, 0, fmt.Errorf("invalid tcp-flags: %w", err)
			}
			break
		}

		matches = append(matches, FlowMatch{
			Op:    op,
			Value: uint64(flags),
			And:   hasAndPrefix, // AND only if explicit & prefix
		})
		consumed++
	}

	if len(matches) == 0 {
		return nil, 0, fmt.Errorf("tcp-flags requires value")
	}

	return NewFlowTCPFlagsMatchComponent(matches), consumed + 1, nil
}

// parseFragmentComponentText parses fragment flags.
func parseFragmentComponentText(args []string) (FlowComponent, int, error) {
	var matches []FlowMatch
	consumed := 0

	for i := range args {
		token := strings.ToLower(args[i])
		if isComponentKeyword(token) {
			break
		}

		op, flags, hasAndPrefix, err := parseBitmaskValue(token, fragmentFlagNameToValue)
		if err != nil {
			if consumed == 0 {
				return nil, 0, fmt.Errorf("invalid fragment: %w", err)
			}
			break
		}

		matches = append(matches, FlowMatch{
			Op:    op,
			Value: uint64(flags),
			And:   hasAndPrefix, // AND only if explicit & prefix
		})
		consumed++
	}

	if len(matches) == 0 {
		return nil, 0, fmt.Errorf("fragment requires value")
	}

	return NewFlowFragmentMatchComponent(matches), consumed + 1, nil
}

// isComponentKeyword checks if token is a component keyword.
func isComponentKeyword(token string) bool {
	switch token {
	case kwDestination, kwDestinationIPv6, kwSource, kwSourceIPv6,
		kwProtocol, kwNextHeader, kwPort, kwDestPort, kwSourcePort,
		kwICMPType, kwICMPCode, kwTCPFlags, kwPacketLength, kwDSCP,
		kwFragment, kwFlowLabel, kwRD:
		return true
	}
	return false
}

// componentMaxValue returns max valid value for component type.
func componentMaxValue(compType FlowComponentType) uint64 {
	switch compType { //nolint:exhaustive // Only numeric types
	case FlowIPProtocol, FlowICMPType, FlowICMPCode:
		return 255
	case FlowPort, FlowDestPort, FlowSourcePort, FlowPacketLength:
		return 65535
	case FlowDSCP:
		return 63
	default:
		return 0xFFFFFFFF
	}
}

// parseOperatorValue parses "<op><value>" like "=80", ">100", "80".
func parseOperatorValue(token string) (FlowOperator, uint64, error) {
	op := FlowOpEqual
	s := token

	// Parse operator prefix
	switch {
	case strings.HasPrefix(s, ">="):
		op = FlowOpGreater | FlowOpEqual
		s = s[2:]
	case strings.HasPrefix(s, "<="):
		op = FlowOpLess | FlowOpEqual
		s = s[2:]
	case strings.HasPrefix(s, "!="):
		op = FlowOpNotEq
		s = s[2:]
	case strings.HasPrefix(s, ">"):
		op = FlowOpGreater
		s = s[1:]
	case strings.HasPrefix(s, "<"):
		op = FlowOpLess
		s = s[1:]
	case strings.HasPrefix(s, "="):
		op = FlowOpEqual
		s = s[1:]
	}

	value, err := parseUint64(s)
	if err != nil {
		return 0, 0, err
	}

	return op, value, nil
}

// parseBitmaskValue parses bitmask with modifiers like "!syn", "=ack", "syn&ack", "&!is-fragment".
// Returns operator, flags value, whether token had AND prefix, and error.
func parseBitmaskValue(token string, nameToValue map[string]uint8) (FlowOperator, uint8, bool, error) {
	var op FlowOperator
	s := token

	// Handle leading & (AND connector with previous) - strip it first
	hasAndPrefix := strings.HasPrefix(s, "&")
	s = strings.TrimPrefix(s, "&")

	// Parse modifiers (!, =) that may come after & prefix
	if strings.HasPrefix(s, "!") {
		op |= FlowOpNot
		s = s[1:]
	}
	if strings.HasPrefix(s, "=") {
		op |= FlowOpMatch
		s = s[1:]
	}

	// Parse flag names (may be combined with &)
	var flags uint8
	for part := range strings.SplitSeq(s, "&") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if val, ok := nameToValue[part]; ok {
			flags |= val
		} else {
			// Try numeric
			num, err := parseUint8(part)
			if err != nil {
				return 0, 0, false, fmt.Errorf("unknown flag: %s", part)
			}
			flags |= num
		}
	}

	return op, flags, hasAndPrefix, nil
}

func parseUint8(s string) (uint8, error) {
	v, err := parseUint64(s)
	if err != nil {
		return 0, err
	}
	if v > 255 {
		return 0, fmt.Errorf("value %d exceeds uint8", v)
	}
	return uint8(v), nil //nolint:gosec // bounds checked
}

func parseUint64(s string) (uint64, error) {
	// Handle hex
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return strconv.ParseUint(s[2:], 16, 64)
	}
	return strconv.ParseUint(s, 10, 64)
}
