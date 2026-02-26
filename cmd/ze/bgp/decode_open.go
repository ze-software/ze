// Design: docs/architecture/core-design.md — BGP CLI commands
// Related: decode.go — top-level decode dispatch
// Related: decode_plugin.go — plugin invocation for capability decoding

package bgp

import (
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
)

// pluginCapabilityMap maps capability codes to plugin names.
// Populated from plugin registry at init time.
var pluginCapabilityMap = registry.CapabilityMap()

// pluginFamilyMap maps address families to plugin names for CLI decode.
// Populated from plugin registry at init time.
var pluginFamilyMap = registry.FamilyMap()

// decodeOpenMessage decodes a BGP OPEN message and returns Ze format.
func decodeOpenMessage(data []byte, hasHeader bool, plugins []string) (map[string]any, error) {
	body := data
	if hasHeader {
		if len(data) < message.HeaderLen {
			return nil, fmt.Errorf("data too short for header")
		}
		body = data[message.HeaderLen:]
	}

	open, err := message.UnpackOpen(body)
	if err != nil {
		return nil, fmt.Errorf("unpack open: %w", err)
	}

	// Parse capabilities
	caps := capability.ParseFromOptionalParams(open.OptionalParams)

	// Determine ASN (use ASN4 if available)
	asn := uint32(open.MyAS)
	for _, c := range caps {
		if asn4, ok := c.(*capability.ASN4); ok {
			asn = asn4.ASN
			break
		}
	}

	// Ze format: capabilities as array of objects with code, name, value
	capsArray := make([]map[string]any, 0, len(caps))
	for _, c := range caps {
		capJSON := capabilityToZeJSON(c, plugins)
		capsArray = append(capsArray, capJSON)
	}

	// Ze format: open event content
	openContent := map[string]any{
		"asn":          asn,
		"router-id":    open.RouterID(),
		"hold-time":    open.HoldTime,
		"capabilities": capsArray,
	}

	return map[string]any{"open": openContent}, nil
}

// capabilityToZeJSON converts a capability to Ze ze-bgp JSON format.
// Ze format: {"code": N, "name": "...", "value": "..."}.
func capabilityToZeJSON(c capability.Capability, plugins []string) map[string]any {
	code := int(c.Code())

	switch cap := c.(type) {
	case *capability.Multiprotocol:
		return map[string]any{"code": code, "name": "multiprotocol", "value": cap.AFI.String() + "/" + cap.SAFI.String()}
	case *capability.ASN4:
		return map[string]any{"code": code, "name": "asn4", "value": fmt.Sprintf("%d", cap.ASN)}
	case *capability.ExtendedMessage:
		return map[string]any{"code": code, "name": "extended-message"}
	case *capability.AddPath:
		families := make([]string, len(cap.Families))
		for i, f := range cap.Families {
			families[i] = fmt.Sprintf("%s/%s", f.AFI.String(), f.SAFI.String())
		}
		return map[string]any{"code": code, "name": "add-path", "value": families}
	}
	// Unknown or plugin-decoded capability type - try plugin decode or return raw
	return unknownCapabilityZe(c, plugins)
}

// unknownCapabilityZe returns Ze format JSON for an unrecognized/plugin-required capability.
// Auto-invokes registered plugins for known capability codes (consistent with NLRI family auto-lookup).
// Falls back to raw hex if no plugin is registered or decode fails.
func unknownCapabilityZe(c capability.Capability, plugins []string) map[string]any {
	code := int(c.Code())
	raw := make([]byte, c.Len())
	c.WriteTo(raw, 0)
	var rawHex string
	if len(raw) >= 2 {
		rawHex = fmt.Sprintf("%X", raw[2:])
	}

	// Auto-invoke registered plugin for known capability codes.
	pluginName, hasPlugin := pluginCapabilityMap[uint8(c.Code())]
	if hasPlugin {
		result := invokePluginDecode(pluginName, uint8(c.Code()), rawHex)
		if result != nil {
			result["code"] = code
			return result
		}
	}

	return map[string]any{"code": code, "name": "unknown", "raw": rawHex}
}
