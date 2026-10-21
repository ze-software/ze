// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec NLRI decoding and JSON formatting
// RFC: rfc/short/rfc5575.md
// Overview: plugin.go — plugin entry points, CLI, families
// Related: plugin_encode_text.go — text-to-wire encoding
// Related: plugin_protocol.go — stdin/stdout protocol dispatch

package flowspec

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/nlri/flowspec"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var errNoValidComponentsInJson = errors.New("no valid components in JSON")

// isValidFlowSpecFamily reports whether name is one of this plugin's families.
// The name is resolved through the registry first, so only the registry's own
// spelling is accepted and an unregistered name is refused rather than guessed.
func isValidFlowSpecFamily(name string) bool {
	fam, ok := family.LookupFamily(name)
	if !ok {
		return false
	}
	return slices.Contains(flowSpecFamilySet, fam)
}

// decodeFlowSpecNLRI decodes FlowSpec NLRI wire bytes to JSON map.
func decodeFlowSpecNLRI(famName string, data []byte) map[string]any {
	isVPN := strings.HasSuffix(famName, "-vpn")

	// Determine Family from family string
	fam, ok := family.LookupFamily(famName)
	if !ok {
		flowLogger.Debug("unknown family", "family", famName)
		return nil
	}

	var fs *flowspec.FlowSpec
	var rd *flowspec.RouteDistinguisher

	if isVPN {
		fsv, err := flowspec.ParseFlowSpecVPN(fam, data)
		if err != nil {
			flowLogger.Debug("parse flowspec vpn failed", "err", err)
			return nil
		}
		fs = fsv.FlowSpec()
		rdVal := fsv.RD()
		rd = &rdVal
	} else {
		var err error
		fs, err = flowspec.ParseFlowSpec(fam, data)
		if err != nil {
			flowLogger.Debug("parse flowspec failed", "err", err)
			return nil
		}
	}

	return flowSpecToJSON(fs, famName, rd)
}

// flowSpecToJSON converts FlowSpec to ze JSON representation.
// Format: {"rd": "...", "destination-ipv6": [["prefix/len/offset"]], ...}.
// Note: "family" is NOT included since it's already in the JSON path when embedded.
//
// One component becomes one key. A component type appears at most once in a
// FlowSpec (RFC 8955 Section 4.2, enforced by FlowSpec.AddComponent and
// FlowSpec.parseComponents), so each key is written once and the OR groups of a
// type all come from FormatNumericMatches over that one component.
func flowSpecToJSON(fs *flowspec.FlowSpec, family string, rd *flowspec.RouteDistinguisher) map[string]any {
	result := make(map[string]any)

	// Add RD for VPN families
	if rd != nil {
		result[flowspec.KwRD] = rd.String()
	}

	// Determine IPv4 vs IPv6 from family string
	isIPv6 := strings.Contains(family, "ipv6")

	for _, comp := range fs.Components() {
		key, values := componentToJSON(comp, isIPv6)
		result[key] = values
	}

	return result
}

// componentToJSON converts a FlowComponent to ze-bgp JSON format.
// Returns the key name and nested array values.
func componentToJSON(comp flowspec.FlowComponent, isIPv6 bool) (string, [][]string) {
	compType := comp.Type()

	switch compType {
	case flowspec.FlowDestPrefix:
		key := flowspec.KwDestinationIPv4
		if isIPv6 {
			key = flowspec.KwDestinationIPv6
		}
		prefix := flowspec.FormatPrefixWithOffset(comp)
		return key, [][]string{{prefix}}

	case flowspec.FlowSourcePrefix:
		key := flowspec.KwSourceIPv4
		if isIPv6 {
			key = flowspec.KwSourceIPv6
		}
		prefix := flowspec.FormatPrefixWithOffset(comp)
		return key, [][]string{{prefix}}

	case flowspec.FlowIPProtocol:
		key := flowspec.KwProtocol
		if isIPv6 {
			key = flowspec.KwNextHeader
		}
		return key, flowspec.FormatNumericMatches(comp, compType)

	case flowspec.FlowPort:
		return flowspec.KwPort, flowspec.FormatNumericMatches(comp, compType)

	case flowspec.FlowDestPort:
		return flowspec.KwDestPort, flowspec.FormatNumericMatches(comp, compType)

	case flowspec.FlowSourcePort:
		return flowspec.KwSourcePort, flowspec.FormatNumericMatches(comp, compType)

	case flowspec.FlowICMPType:
		return flowspec.KwICMPType, flowspec.FormatNumericMatches(comp, compType)

	case flowspec.FlowICMPCode:
		return flowspec.KwICMPCode, flowspec.FormatNumericMatches(comp, compType)

	case flowspec.FlowTCPFlags:
		return flowspec.KwTCPFlags, flowspec.FormatBitmaskMatches(comp, flowspec.TCPFlagValueToNames)

	case flowspec.FlowPacketLength:
		return flowspec.KwPacketLength, flowspec.FormatNumericMatches(comp, compType)

	case flowspec.FlowDSCP:
		return flowspec.KwDSCP, flowspec.FormatNumericMatches(comp, compType)

	case flowspec.FlowFragment:
		return flowspec.KwFragment, flowspec.FormatBitmaskMatches(comp, flowspec.FragmentFlagValueToNames)

	case flowspec.FlowFlowLabel:
		return flowspec.KwFlowLabel, flowspec.FormatNumericMatches(comp, compType)

	default: // Component implementations form an open set; retain numeric keys.
		var b textbuf.Buffer
		return b.Reset().Str("type-").Uint8(uint8(compType)).String(), [][]string{}
	}
}

// formatFlowSpecText formats FlowSpec components as human-readable text.
// Output is single-line, space-separated component descriptions.
func formatFlowSpecText(result map[string]any) string {
	var parts []string

	// Order components logically: destination, source, protocol, ports, etc.
	componentOrder := []string{
		flowspec.KwDestinationIPv4, flowspec.KwDestinationIPv6, flowspec.KwSourceIPv4, flowspec.KwSourceIPv6, flowspec.KwProtocol,
		flowspec.KwPort, flowspec.KwDestPort, flowspec.KwSourcePort,
		flowspec.KwICMPType, flowspec.KwICMPCode, flowspec.KwTCPFlags, flowspec.KwPacketLength, flowspec.KwDSCP,
		flowspec.KwFragment, flowspec.KwFlowLabel, flowspec.KwRD,
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
	return textbuf.Join(parts, " ")
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
		return textbuf.Join(v, ",")
	case float64:
		return strconv.FormatFloat(v, 'f', 0, 64)
	case int:
		return strconv.Itoa(v)
	case uint64:
		return textbuf.StringUint(v)
	case uint32:
		return textbuf.StringUint32(v)
	case [][]string:
		var parts []string
		for _, group := range v {
			parts = append(parts, textbuf.Join(group, " "))
		}
		return textbuf.Join(parts, " ")
	default:
		return fmt.Sprint(val)
	}
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
				andParts = append(andParts, formatComponentValue("", item))
			}
			parts = append(parts, textbuf.Join(andParts, "&"))
		case string:
			parts = append(parts, inner)
		}
	}
	return textbuf.Join(parts, "|")
}

// contains checks if a string slice contains a value.
func contains(slice []string, val string) bool {
	return slices.Contains(slice, val)
}

// jsonToTextComponents converts FlowSpec JSON format to text component args.
// Input: {"destination-ipv4":[["10.0.0.0/24/0"]],"protocol":[["=tcp"],["=udp"]]}
// Output: ["destination-ipv4", "10.0.0.0/24", "protocol", "tcp", "udp"].
//
// All the groups of one component follow a single keyword. The update text
// grammar reads successive values as OR and a leading "&" as AND, so the
// outer array (OR) becomes successive values and each later value of an inner
// array (AND) carries "&": [[">80","<100"],[">443","<500"]] becomes
// "port >80 &<100 >443 &<500". Protocol and next-header values are written
// bare, as the protocol parser reads them.
func jsonToTextComponents(m map[string]any) ([]string, error) {
	var args []string

	// Process components in a defined order for consistency
	// Keys match text keywords (destination-ipv6, next-header, etc.)
	// RD must come first for VPN families (parsed before components)
	componentOrder := []string{
		flowspec.KwRD, // Route Distinguisher - must be first for VPN families
		flowspec.KwDestinationIPv4, flowspec.KwDestinationIPv6, flowspec.KwSourceIPv4, flowspec.KwSourceIPv6,
		flowspec.KwProtocol, flowspec.KwNextHeader, flowspec.KwPort, flowspec.KwDestPort, flowspec.KwSourcePort,
		flowspec.KwICMPType, flowspec.KwICMPCode, flowspec.KwTCPFlags, flowspec.KwPacketLength, flowspec.KwDSCP,
		flowspec.KwFragment, flowspec.KwFlowLabel,
	}

	for _, key := range componentOrder {
		val, ok := m[key]
		if !ok {
			continue
		}

		// Handle RD specially - it's a simple string, not an array
		if key == flowspec.KwRD {
			if rdStr, ok := val.(string); ok {
				args = append(args, flowspec.KwRD, rdStr)
			}
			continue
		}

		// Handle nested array format: [[val1, val2], [val3]]
		// Outer array = OR, Inner array = AND
		arr, ok := val.([]any)
		if !ok {
			continue
		}

		// The protocol parser reads bare values and no "&".
		joinsAnd := key != flowspec.KwProtocol && key != flowspec.KwNextHeader
		var values []string
		for _, orGroup := range arr {
			innerArr, ok := orGroup.([]any)
			if !ok {
				if s, ok := orGroup.(string); ok {
					values = append(values, normalizeJSONValue(key, s))
				}
				continue
			}
			for i, v := range innerArr {
				s, ok := v.(string)
				if !ok {
					continue
				}
				value := normalizeJSONValue(key, s)
				// RFC 8955 Section 4.2.1.1: a later term of an AND group sets the a bit.
				if joinsAnd && i > 0 {
					value = "&" + strings.TrimPrefix(value, "&")
				}
				values = append(values, value)
			}
		}
		if len(values) > 0 {
			args = append(args, key)
			args = append(args, values...)
		}
	}

	if len(args) == 0 {
		return nil, errNoValidComponentsInJson
	}

	return args, nil
}

// normalizeJSONValue converts JSON value format to text format.
// E.g., "10.0.0.0/24/0" -> "10.0.0.0/24" (strip offset for prefixes).
// E.g., "=tcp" -> "tcp" (strip operator for protocol/next-header).
func normalizeJSONValue(key, val string) string {
	// Strip the /0 offset suffix a prefix component carries. The four prefix
	// keywords are named rather than matched on a shared first word, so
	// destination-port never reaches a branch written for an address.
	if isPrefixComponentKey(key) {
		if strings.HasSuffix(val, "/0") {
			// "10.0.0.0/24/0" -> "10.0.0.0/24"
			parts := strings.Split(val, "/")
			if len(parts) == 3 {
				var tb textbuf.Buffer
				return tb.Str(parts[0]).Byte('/').Str(parts[1]).String()
			}
		}
	}

	// Strip operator prefix for protocol/next-header (text parser expects plain value)
	if key == flowspec.KwProtocol || key == flowspec.KwNextHeader {
		// "=tcp" -> "tcp", "=6" -> "6"
		val = strings.TrimPrefix(val, "=")
		val = strings.TrimPrefix(val, "<")
		val = strings.TrimPrefix(val, ">")
		val = strings.TrimPrefix(val, "!")
		val = strings.TrimPrefix(val, "&")
	}

	return val
}

// isPrefixComponentKey answers whether a JSON key names one of the four prefix
// components, which are the only ones whose value carries the RFC 8956 offset
// suffix.
func isPrefixComponentKey(key string) bool {
	switch key {
	case flowspec.KwDestinationIPv4, flowspec.KwDestinationIPv6, flowspec.KwSourceIPv4, flowspec.KwSourceIPv6:
		return true
	}
	return false
}
