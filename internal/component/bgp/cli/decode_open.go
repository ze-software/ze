// Design: docs/architecture/core-design.md — BGP CLI commands
// Overview: decode.go — top-level decode dispatch
// Related: decode_plugin.go — plugin invocation for capability decoding

package cli

import (
	"errors"
	"fmt"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var errDataTooShortForHeader = errors.New("data too short for header")

// capabilityMap returns capability code to plugin name mapping.
// Lazy: all plugin init() functions must complete before first decode call.
var capabilityMap = sync.OnceValue(registry.CapabilityMap)

// decodeOpenMessage decodes a BGP OPEN message and returns Ze format.
func decodeOpenMessage(plugins decodePlugins, data []byte, hasHeader bool) (map[string]any, error) {
	body := data
	if hasHeader {
		if len(data) < message.HeaderLen {
			return nil, errDataTooShortForHeader
		}
		body = data[message.HeaderLen:]
	}

	open, err := message.UnpackOpen(body)
	if err != nil {
		return nil, fmt.Errorf("unpack open: %w", err)
	}

	// Parse capabilities
	caps, err := capability.ParseFromOptionalParams(open.OptionalParams, open.ExtendedParams)
	if err != nil {
		return nil, fmt.Errorf("parse capabilities: %w", err)
	}

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
		capJSON := capabilityToZeJSON(plugins, c)
		capsArray = append(capsArray, capJSON)
	}

	// Ze format: open event content
	openContent := map[string]any{
		"asn":          asn,
		"router-id":    open.RouterID(),
		"timer":        map[string]any{"hold-time": open.HoldTime},
		"capabilities": capsArray,
	}

	return map[string]any{"open": openContent}, nil
}

// capabilityToZeJSON converts a capability to Ze ze-bgp JSON format.
// Ze format: {"code": N, "name": "...", "value": "..."}.
// The plugins the operator named are asked first, then the registered plugin
// decoder for the code. Falls back to raw hex when none of them decodes it.
func capabilityToZeJSON(plugins decodePlugins, c capability.Capability) map[string]any {
	code := int(c.Code())
	raw := make([]byte, c.Len())
	c.WriteTo(raw, 0)
	var rawHex string
	if len(raw) >= 2 {
		rawHex = textbuf.StringHexUpper(raw[2:])
	}

	if result, decoded := plugins.capability(uint8(c.Code()), rawHex); decoded {
		result["code"] = code
		return result
	}

	// Auto-invoke registered plugin for known capability codes.
	pluginName, hasPlugin := capabilityMap()[uint8(c.Code())]
	if hasPlugin {
		result := invokePluginDecode(pluginName, uint8(c.Code()), rawHex)
		if result != nil {
			result["code"] = code
			return result
		}
	}

	return map[string]any{"code": code, "name": capNameUnknown, jsonKeyRaw: rawHex}
}
