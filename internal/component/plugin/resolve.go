// Design: docs/architecture/api/process-protocol.md — plugin process management
//
// Package plugin provides plugin resolution and registry.
package plugin

import (
	"errors"
	"log/slog"
	"path/filepath"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
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

// AvailableInternalPlugins returns the list of internal plugin names.
// Used by `ze --plugin` to list available plugins.
// Uses the registry as single source of truth.
func AvailableInternalPlugins() []string {
	return registry.Names()
}

// InternalPluginInfo returns metadata for all internal plugins.
// Returns a slice sorted by plugin name.
func InternalPluginInfo() []PluginInfo {
	regs := registry.All()
	result := make([]PluginInfo, 0, len(regs))
	for _, reg := range regs {
		info := PluginInfo{
			Name:        reg.Name,
			Description: reg.Description,
			RFCs:        reg.RFCs,
			Families:    reg.Families,
		}
		// Convert uint8 capability codes to int for JSON compatibility.
		for _, c := range reg.CapabilityCodes {
			info.Capabilities = append(info.Capabilities, int(c))
		}
		result = append(result, info)
	}
	return result
}

// RegisterPluginEventTypes iterates all registered plugins and registers
// their declared EventTypes into ValidEvents. MUST be called once during
// server startup, before any subscribe-events or emit-event RPCs.
func RegisterPluginEventTypes() {
	for _, reg := range registry.All() {
		for _, et := range reg.EventTypes {
			// All plugin event types currently go into the BGP namespace.
			// If a future plugin needs RIB events, EventTypes would need namespace info.
			if err := RegisterEventType(NamespaceBGP, et); err != nil {
				slog.Error("register plugin event type failed", "plugin", reg.Name, "event", et, "error", err)
			}
		}
	}
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

	// Auto discovery.
	if s == "auto" {
		return &ResolvedPlugin{Type: PluginTypeAuto}, nil
	}

	// Internal plugin (ze.X).
	if after, ok := strings.CutPrefix(s, "ze."); ok {
		name := after
		if !IsInternalPlugin(name) {
			return nil, ErrUnknownInternalPlugin
		}
		return &ResolvedPlugin{
			Type: PluginTypeInternal,
			Name: name,
		}, nil
	}

	// External plugin - parse command.
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return nil, ErrEmptyPlugin
	}

	// Derive name from last component of first part (binary) or last arg.
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
	// If command is "ze plugin X", use X as name.
	if len(parts) >= 3 && parts[0] == "ze" && parts[1] == cmdPlugin {
		return parts[2]
	}

	// Otherwise use basename of binary.
	return filepath.Base(parts[0])
}

// IsInternalPlugin checks if a name is a registered internal plugin.
// Uses the registry as single source of truth.
func IsInternalPlugin(name string) bool {
	return registry.Has(name)
}

// InternalPluginCommand returns the command to run an internal plugin.
// For internal plugins, this is "ze plugin <name>".
func InternalPluginCommand(name string) []string {
	return []string{"ze", cmdPlugin, name}
}
