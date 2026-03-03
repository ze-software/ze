// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec NLRI decoding and JSON formatting
// RFC: rfc/short/rfc5575.md
// Overview: plugin.go — plugin entry points, CLI, families
// Related: plugin_encode_text.go — text-to-wire encoding
// Related: plugin_protocol.go — stdin/stdout protocol dispatch

package bgp_nlri_flowspec

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/nlri"
)

// isValidFlowSpecFamily checks if family is a FlowSpec family.
func isValidFlowSpecFamily(family string) bool {
	switch family {
	case "ipv4/flow", "ipv6/flow", "ipv4/flow-vpn", "ipv6/flow-vpn":
		return true
	default: // not a FlowSpec family
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

// flowSpecToJSON converts FlowSpec to ze JSON representation.
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

// componentToJSON converts a FlowComponent to ze-bgp JSON format.
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

	default: // unknown component type — format as type-N
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

// protocolNumberToName maps protocol numbers to names for output.
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

// formatNumericMatches formats numeric component matches for ze-bgp JSON.
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
	default: // unrecognized operator — treat as equality
		return "=" + value
	}
}

// tcpFlagValueToNames maps TCP flag bit values to names.
var tcpFlagValueToNames = map[uint8]string{
	0x01: "fin",
	0x02: "syn",
	0x04: "rst",
	0x08: "push", // "push" not "psh"
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
// FlowSpec uses [[a,b],[c]] for (a AND b) OR c.
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

// jsonToTextComponents converts FlowSpec JSON format to text component args.
// Input: {"destination":[["10.0.0.0/24/0"]],"protocol":[["=tcp"],["=udp"]]}
// Output: ["destination", "10.0.0.0/24", "protocol", "tcp", "udp"].
//
// For simple OR groups (each inner array has one value), all values go after
// a single keyword: "protocol tcp udp" -> one component with OR values.
//
// For complex OR-of-AND groups (inner arrays have multiple values), each OR
// group becomes a separate keyword entry: "port >80 <100 port >443 <500"
// -> two components that get merged on decode.
func jsonToTextComponents(m map[string]any) ([]string, error) {
	var args []string

	// Process components in a defined order for consistency
	// Keys match text keywords (destination-ipv6, next-header, etc.)
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
			// "protocol tcp udp" -> one component with OR values
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
			// "port >80 <100 port >443 <500" -> two components
			// The decoder merges components with same key into one entry
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
// E.g., "10.0.0.0/24/0" -> "10.0.0.0/24" (strip offset for prefixes).
// E.g., "=tcp" -> "tcp" (strip operator for protocol/next-header).
func normalizeJSONValue(key, val string) string {
	// Strip /0 offset suffix from prefixes (destination, source)
	if strings.HasPrefix(key, "destination") || strings.HasPrefix(key, "source") {
		if strings.HasSuffix(val, "/0") {
			// "10.0.0.0/24/0" -> "10.0.0.0/24"
			parts := strings.Split(val, "/")
			if len(parts) == 3 {
				return parts[0] + "/" + parts[1]
			}
		}
	}

	// Strip operator prefix for protocol/next-header (text parser expects plain value)
	if key == kwProtocol || key == "next-header" {
		// "=tcp" -> "tcp", "=6" -> "6"
		val = strings.TrimPrefix(val, "=")
		val = strings.TrimPrefix(val, "<")
		val = strings.TrimPrefix(val, ">")
		val = strings.TrimPrefix(val, "!")
		val = strings.TrimPrefix(val, "&")
	}

	return val
}
