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
