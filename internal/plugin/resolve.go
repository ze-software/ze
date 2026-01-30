// Package plugin provides plugin resolution and registry.
package plugin

import (
	"errors"
	"path/filepath"
	"sort"
	"strings"
)

// PluginType represents the type of plugin resolution.
type PluginType int

const (
	// PluginTypeInternal is a built-in plugin (ze.X).
	PluginTypeInternal PluginType = iota
	// PluginTypeExternal is an external plugin (fork/exec).
	PluginTypeExternal
	// PluginTypeAuto triggers auto-discovery of all plugins.
	PluginTypeAuto
)

// ResolvedPlugin contains the resolved plugin information.
type ResolvedPlugin struct {
	Type    PluginType
	Name    string   // Plugin name (e.g., "rib", "gr")
	Command []string // For external: binary and args to exec
}

// PluginInfo contains metadata about an internal plugin.
type PluginInfo struct {
	Name         string   `json:"name"`                   // Plugin name (e.g., "flowspec")
	Description  string   `json:"description"`            // Human-readable description
	RFCs         []string `json:"rfcs,omitempty"`         // Related RFCs
	Capabilities []int    `json:"capabilities,omitempty"` // Capability codes this plugin handles
	Families     []string `json:"families,omitempty"`     // Address families this plugin handles
}

// internalPluginInfo contains metadata for each internal plugin.
var internalPluginInfo = map[string]PluginInfo{
	"flowspec": {
		Name:        "flowspec",
		Description: "FlowSpec NLRI encoding/decoding",
		RFCs:        []string{"8955", "8956"},
		Families:    []string{"ipv4/flow", "ipv6/flow", "ipv4/flow-vpn", "ipv6/flow-vpn"},
	},
	"gr": {
		Name:        "gr",
		Description: "Graceful Restart state management",
		RFCs:        []string{"4724"},
	},
	"hostname": {
		Name:         "hostname",
		Description:  "FQDN capability decoding",
		RFCs:         []string{"5765"},
		Capabilities: []int{73},
	},
	"rib": {
		Name:        "rib",
		Description: "Route Information Base storage",
		RFCs:        []string{"4271"},
	},
	"rr": {
		Name:        "rr",
		Description: "Route Reflector / Route Server",
		RFCs:        []string{"4456"},
	},
}

// AvailableInternalPlugins returns the list of internal plugin names.
// Used by `ze --plugin` to list available plugins.
// Uses internalPluginRunners from inprocess.go as single source of truth.
func AvailableInternalPlugins() []string {
	plugins := make([]string, 0, len(internalPluginRunners))
	for name := range internalPluginRunners {
		plugins = append(plugins, name)
	}
	// Sort for stable output
	sort.Strings(plugins)
	return plugins
}

// InternalPluginInfo returns metadata for all internal plugins.
// Returns a slice sorted by plugin name.
func InternalPluginInfo() []PluginInfo {
	names := AvailableInternalPlugins()
	result := make([]PluginInfo, 0, len(names))
	for _, name := range names {
		if info, ok := internalPluginInfo[name]; ok {
			result = append(result, info)
		} else {
			// Fallback for plugins without metadata
			result = append(result, PluginInfo{Name: name})
		}
	}
	return result
}

// ErrEmptyPlugin is returned when an empty plugin string is provided.
var ErrEmptyPlugin = errors.New("empty plugin string")

// ErrUnknownInternalPlugin is returned when ze.X refers to unknown plugin.
var ErrUnknownInternalPlugin = errors.New("unknown internal plugin")

// ResolvePlugin parses a plugin string and returns resolved information.
//
// Resolution rules:
//   - "ze.X" -> internal plugin (no fork)
//   - "./path" -> fork local binary
//   - "/path" -> fork absolute path binary
//   - "auto" -> auto-discover all plugins
//   - "cmd args..." -> fork command with args
func ResolvePlugin(s string) (*ResolvedPlugin, error) {
	if s == "" {
		return nil, ErrEmptyPlugin
	}

	// Auto discovery
	if s == "auto" {
		return &ResolvedPlugin{Type: PluginTypeAuto}, nil
	}

	// Internal plugin (ze.X)
	if strings.HasPrefix(s, "ze.") {
		name := strings.TrimPrefix(s, "ze.")
		if !IsInternalPlugin(name) {
			return nil, ErrUnknownInternalPlugin
		}
		return &ResolvedPlugin{
			Type: PluginTypeInternal,
			Name: name,
		}, nil
	}

	// External plugin - parse command
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return nil, ErrEmptyPlugin
	}

	// Derive name from last component of first part (binary) or last arg
	name := deriveName(parts)

	return &ResolvedPlugin{
		Type:    PluginTypeExternal,
		Name:    name,
		Command: parts,
	}, nil
}

// deriveName extracts a plugin name from command parts.
// Uses last argument if it looks like a plugin name, otherwise basename of binary.
func deriveName(parts []string) string {
	// If command is "ze bgp plugin X", use X as name
	if len(parts) >= 4 && parts[0] == "ze" && parts[1] == "bgp" && parts[2] == "plugin" {
		return parts[3]
	}

	// Otherwise use basename of binary
	return filepath.Base(parts[0])
}

// IsInternalPlugin checks if a name is a registered internal plugin.
// Uses internalPluginRunners from inprocess.go as single source of truth.
func IsInternalPlugin(name string) bool {
	return GetInternalPluginRunner(name) != nil
}

// InternalPluginCommand returns the command to run an internal plugin.
// For internal plugins, this is "ze bgp plugin <name>".
func InternalPluginCommand(name string) []string {
	return []string{"ze", "bgp", "plugin", name}
}
