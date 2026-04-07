// Design: docs/architecture/core-design.md — BGP CLI commands
// Overview: decode.go — top-level decode dispatch
// Related: decode_update.go — UPDATE message decoding calls MP_REACH/MP_UNREACH parsers
// Related: decode_plugin.go — plugin invocation for NLRI decoding
// Related: decode_human.go — human-readable NLRI formatters

package bgp

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// buildMPReachZe builds Ze format NLRI operations from MP_REACH_NLRI.
func buildMPReachZe(mpReach []byte) (string, []map[string]any) {
	if len(mpReach) < 5 {
		return "", nil
	}

	afi := family.AFI(binary.BigEndian.Uint16(mpReach[0:2]))
	safi := family.SAFI(mpReach[2])
	nhLen := int(mpReach[3])

	if len(mpReach) < 4+nhLen+1 {
		return "", nil
	}

	nhData := mpReach[4 : 4+nhLen]
	nextHop := parseNextHop(nhData, afi)

	nlriOffset := 4 + nhLen + 1
	if nlriOffset >= len(mpReach) {
		return "", nil
	}

	nlriData := mpReach[nlriOffset:]
	familyKey := formatFamily(afi, safi)

	routes := parseNLRIByFamily(nlriData, afi, safi, false)
	if len(routes) == 0 {
		return "", nil
	}

	// Ze format: array of operations with action/next-hop/nlri
	op := map[string]any{
		"next-hop": nextHop,
		"action":   "add",
		"nlri":     routes,
	}

	return familyKey, []map[string]any{op}
}

// buildMPUnreachZe builds Ze format NLRI operations from MP_UNREACH_NLRI.
func buildMPUnreachZe(mpUnreach []byte) (string, []map[string]any) {
	if len(mpUnreach) < 3 {
		return "", nil
	}

	afi := family.AFI(binary.BigEndian.Uint16(mpUnreach[0:2]))
	safi := family.SAFI(mpUnreach[2])

	if len(mpUnreach) <= 3 {
		return "", nil
	}

	nlriData := mpUnreach[3:]
	familyKey := formatFamily(afi, safi)

	routes := parseNLRIByFamily(nlriData, afi, safi, true)
	if len(routes) == 0 {
		return "", nil
	}

	// Ze format: withdraw operation
	op := map[string]any{
		"action": "del",
		"nlri":   routes,
	}

	return familyKey, []map[string]any{op}
}

// parseIPv4Prefixes parses IPv4 NLRI prefixes.
func parseIPv4Prefixes(data []byte) []string {
	var prefixes []string
	offset := 0

	for offset < len(data) {
		if offset >= len(data) {
			break
		}

		prefixLen := int(data[offset])
		offset++

		byteLen := (prefixLen + 7) / 8
		if offset+byteLen > len(data) {
			break
		}

		prefixBytes := make([]byte, 4)
		copy(prefixBytes, data[offset:offset+byteLen])

		prefix := fmt.Sprintf("%d.%d.%d.%d/%d",
			prefixBytes[0], prefixBytes[1], prefixBytes[2], prefixBytes[3], prefixLen)
		prefixes = append(prefixes, prefix)

		offset += byteLen
	}

	return prefixes
}

// parseNextHop parses the next-hop from MP_REACH_NLRI.
func parseNextHop(data []byte, _ family.AFI) string {
	switch {
	case len(data) == 4:
		return fmt.Sprintf("%d.%d.%d.%d", data[0], data[1], data[2], data[3])
	case len(data) == 16:
		addr := netip.AddrFrom16([16]byte(data))
		return addr.String()
	case len(data) == 32: // IPv6 with link-local
		addr := netip.AddrFrom16([16]byte(data[:16]))
		return addr.String()
	case len(data) == 0:
		return "no-nexthop"
	default:
		return fmt.Sprintf("%x", data)
	}
}

// formatFamily returns the family string for JSON output.
func formatFamily(afi family.AFI, safi family.SAFI) string {
	// Use afi/safi format
	return family.Family{AFI: afi, SAFI: safi}.String()
}

// parseNLRIByFamily parses NLRI based on address family.
func parseNLRIByFamily(data []byte, afi family.AFI, safi family.SAFI, _ bool) []any {
	var routes []any

	switch {
	case afi == family.AFIL2VPN && safi == family.SAFIEVPN:
		// EVPN decoding delegated to plugin
		famStr := family.Family{AFI: afi, SAFI: safi}.String()
		hexData := fmt.Sprintf("%X", data)
		result := invokePluginNLRIDecode("bgp-nlri-evpn", famStr, hexData)
		if result != nil {
			// Result can be array (multiple NLRIs) or map (single NLRI)
			if arr, ok := result.([]any); ok {
				routes = arr
			} else {
				routes = []any{result}
			}
		} else {
			// Plugin failed or unavailable - return raw bytes
			routes = []any{map[string]any{"parsed": false, "raw": hexData}}
		}
	case safi == family.SAFIFlowSpec || safi == family.SAFIFlowSpecVPN:
		// FlowSpec decoding delegated to plugin
		famStr := family.Family{AFI: afi, SAFI: safi}.String()
		hexData := fmt.Sprintf("%X", data)
		result := invokePluginNLRIDecode("bgp-nlri-flowspec", famStr, hexData)
		if result != nil {
			// Result can be array (multiple NLRIs) or map (single NLRI)
			if arr, ok := result.([]any); ok {
				routes = arr
			} else {
				routes = []any{result}
			}
		} else {
			// Plugin failed or unavailable - return raw bytes
			routes = []any{map[string]any{"parsed": false, "raw": hexData}}
		}
	case afi == family.AFIBGPLS:
		// BGP-LS decoding delegated to plugin
		famStr := family.Family{AFI: afi, SAFI: safi}.String()
		hexData := fmt.Sprintf("%X", data)
		result := invokePluginNLRIDecode("bgp-nlri-ls", famStr, hexData)
		if result != nil {
			if arr, ok := result.([]any); ok {
				routes = arr
			} else {
				routes = []any{result}
			}
		} else {
			routes = []any{map[string]any{"parsed": false, "raw": hexData}}
		}
	case safi == family.SAFIVPN:
		// VPN decoding delegated to plugin (RFC 4364, 4659)
		famStr := family.Family{AFI: afi, SAFI: safi}.String()
		hexData := fmt.Sprintf("%X", data)
		result := invokePluginNLRIDecode("bgp-nlri-vpn", famStr, hexData)
		if result != nil {
			if arr, ok := result.([]any); ok {
				routes = arr
			} else {
				routes = []any{result}
			}
		} else {
			routes = []any{map[string]any{"parsed": false, "raw": hexData}}
		}
	default: // IPv4/IPv6 unicast/multicast - simple prefix format
		routes = parseGenericNLRI(data, afi)
	}

	return routes
}

// parseGenericNLRI parses generic NLRI (IPv4/IPv6 prefixes).
// Returns a slice of prefix strings (e.g., ["10.0.0.0/24", "2001::1/128"]).
func parseGenericNLRI(data []byte, afi family.AFI) []any {
	var routes []any
	offset := 0

	for offset < len(data) {
		prefixLen := int(data[offset])
		offset++

		byteLen := (prefixLen + 7) / 8
		if offset+byteLen > len(data) {
			break
		}

		var prefix string
		if afi == family.AFIIPv6 {
			prefixBytes := make([]byte, 16)
			copy(prefixBytes, data[offset:offset+byteLen])
			addr := netip.AddrFrom16([16]byte(prefixBytes))
			prefix = fmt.Sprintf("%s/%d", addr, prefixLen)
		} else {
			prefixBytes := make([]byte, 4)
			copy(prefixBytes, data[offset:offset+byteLen])
			prefix = fmt.Sprintf("%d.%d.%d.%d/%d",
				prefixBytes[0], prefixBytes[1], prefixBytes[2], prefixBytes[3], prefixLen)
		}

		// Return plain prefix string (consistent with IPv4 unicast format)
		routes = append(routes, prefix)
		offset += byteLen
	}

	return routes
}

// decodeNLRIOnly decodes NLRI without envelope.
// If a matching plugin is enabled, it will be invoked for decoding.
// If outputJSON is false, returns human-readable format.
func decodeNLRIOnly(data []byte, family string, outputJSON bool) (string, error) {
	// Validate family against known AFI/SAFI combinations
	if err := validateDecodeFamily(family); err != nil {
		return "", err
	}

	// Try plugin decode first if plugin is enabled for this family
	pluginName := lookupFamilyPlugin(family)
	if pluginName != "" {
		hexData := fmt.Sprintf("%X", data)
		result := invokePluginNLRIDecode(pluginName, family, hexData)
		if result != nil {
			if !outputJSON {
				// Handle both array and map results
				if mapResult, ok := result.(map[string]any); ok {
					return formatNLRIHuman(mapResult, family), nil
				}
				if arrResult, ok := result.([]any); ok && len(arrResult) > 0 {
					if firstMap, ok := arrResult[0].(map[string]any); ok {
						return formatNLRIHuman(firstMap, family), nil
					}
				}
			}
			jsonData, err := json.Marshal(result)
			if err != nil {
				return "", fmt.Errorf("json marshal: %w", err)
			}
			return string(jsonData), nil
		}
		// Plugin failed, fall through to built-in decode
	}

	// Plugin failed or unknown family - return raw bytes
	result := map[string]any{
		"parsed": false,
		"raw":    fmt.Sprintf("%X", data),
	}

	// Human-readable output
	if !outputJSON {
		return formatNLRIHuman(result, family), nil
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("json marshal: %w", err)
	}

	return string(jsonData), nil
}
