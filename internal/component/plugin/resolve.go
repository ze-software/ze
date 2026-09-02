// Design: docs/architecture/api/process-protocol.md — plugin process management
//
// Package plugin provides plugin resolution and registry.
package plugin

import (
	"errors"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/events"
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

// defaultEventNamespace holds the event namespace under which plugin-declared
// EventTypes and namespace-less RPC event subscriptions are registered. The
// protocol component that owns the plugin host registers it from its
// register.go init() (BGP registers "bgp"); the host itself stays
// protocol-neutral. Empty until registered: consumers fail closed with an
// error log instead of guessing a namespace.
var (
	defaultEventNamespaceMu sync.RWMutex
	defaultEventNamespace   string
)

// RegisterDefaultEventNamespace sets the namespace for plugin-declared event
// types and namespace-less plugin RPC subscriptions. Must be called from a
// protocol component's init() (register.go). Calling twice with the same
// value is idempotent; a conflicting second registration is a programmer
// error and panics.
func RegisterDefaultEventNamespace(namespace string) {
	if namespace == "" {
		panic("BUG: plugin: RegisterDefaultEventNamespace called with empty namespace")
	}
	defaultEventNamespaceMu.Lock()
	defer defaultEventNamespaceMu.Unlock()
	if defaultEventNamespace != "" && defaultEventNamespace != namespace {
		panic("BUG: plugin: conflicting default event namespace registration")
	}
	defaultEventNamespace = namespace
}

// DefaultEventNamespace returns the registered default event namespace, or
// "" when no protocol component has registered one.
func DefaultEventNamespace() string {
	defaultEventNamespaceMu.RLock()
	defer defaultEventNamespaceMu.RUnlock()
	return defaultEventNamespace
}

// registerEventTypesOnce ensures plugin event types are registered exactly once.
var registerEventTypesOnce sync.Once

// RegisterPluginEventTypes iterates all registered plugins and registers
// their declared EventTypes into ValidEvents. Safe to call multiple times
// (idempotent via sync.Once). Called from PeersFromTree (config parsing)
// and NewServer (startup).
func RegisterPluginEventTypes() {
	registerEventTypesOnce.Do(func() {
		// Plugin event types go into the default event namespace registered
		// by the owning protocol component ("bgp" today). If a future plugin
		// needs another namespace, EventTypes would need namespace info.
		namespace := DefaultEventNamespace()
		for _, reg := range registry.All() {
			for _, et := range reg.EventTypes {
				if namespace == "" {
					slog.Error("register plugin event type failed: no default event namespace registered (call plugin.RegisterDefaultEventNamespace from the protocol component's register.go)",
						"plugin", reg.Name, "event", et)
					continue
				}
				if err := events.RegisterEventType(namespace, et); err != nil {
					slog.Error("register plugin event type failed", "plugin", reg.Name, "event", et, "error", err)
				}
			}
		}
	})
}

// registerSendTypesOnce ensures plugin send types are registered exactly once.
var registerSendTypesOnce sync.Once

// RegisterPluginSendTypes iterates all registered plugins and registers
// their declared SendTypes into ValidSendTypes. Safe to call multiple times
// (idempotent via sync.Once). Called from PeersFromTree (config parsing)
// and NewServer (startup).
func RegisterPluginSendTypes() {
	registerSendTypesOnce.Do(func() {
		for _, reg := range registry.All() {
			for _, st := range reg.SendTypes {
				if err := events.RegisterSendType(st); err != nil {
					slog.Error("register plugin send type failed", "plugin", reg.Name, "send-type", st, "error", err)
				}
			}
		}
	})
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

// RegistryNames returns the registry names one plugin config can name, the
// implementation first and the process name second.
//
// The operator names the PROCESS and the run/use spelling names the
// IMPLEMENTATION, so `plugin { internal rs { use bgp-rs } }` gives
// PluginConfig{Name: "rs", Run: "bgp-rs"} and only "bgp-rs" is a registry key. A
// lookup that starts from the process name alone finds nothing whenever the
// operator renamed the implementation, and the caller then reads the absence as
// "this plugin declares nothing".
//
// ResolvePlugin owns every legal spelling, which is why this function is one
// call: `use bgp-rib`, `use ze.bgp-rib`, `run ze.bgp-rib` and
// `run ze plugin bgp-rib` all answer "bgp-rib". An external command line
// answers the binary's base name, which the registry does not hold, and that is
// what leaves the process name as the answer for a genuinely external plugin.
//
// The process name stays in the list because a config that carries no spelling
// is filed under it: `plugin { internal bgp-rib { } }`, and every auto-loaded
// plugin, for which getConfigPathPlugins (config/loader.go) builds
// PluginConfig{Name: <registry name>} with no Run.
//
// The names are CANDIDATES, so a caller that needs the one row the registry
// holds calls RegistryName rather than this.
func RegistryNames(p PluginConfig) []string {
	var names []string
	if p.Run != "" {
		if resolved, err := ResolvePlugin(p.Run); err == nil && resolved.Name != "" {
			names = append(names, resolved.Name)
		}
	}
	if p.Name != "" && !slices.Contains(names, p.Name) {
		names = append(names, p.Name)
	}
	return names
}

// RegistryName returns the ONE registry row a plugin config names: the first
// candidate RegistryNames offers that the registry holds.
//
// It falls back to the process name when the registry holds no candidate, which
// is the external case: Run is a command line there, not a registry name, so
// there is no row to find and the process name is the only identity the config
// has.
func RegistryName(p PluginConfig) string {
	for _, name := range RegistryNames(p) {
		if registry.Has(name) {
			return name
		}
	}
	return p.Name
}
