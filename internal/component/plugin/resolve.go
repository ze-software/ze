// Design: docs/architecture/api/process-protocol.md — plugin process management
//
// Package plugin provides plugin resolution and registry.
package plugin

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin/registry"
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

// PluginInfo contains metadata about an internal plugin: the descriptive
// fields of its registry.Registration, the ones an operator reads to learn what
// the plugin is, what loads it and what it needs.
//
// Each field copies one Registration field. The three booleans answer whether
// the plugin carries a YANG schema, an in-process NLRI decoder and an
// in-process NLRI encoder; they omit false so a row names only what the plugin
// has.
type PluginInfo struct {
	Name                 string   `json:"name"`                            // Plugin name (e.g., "flowspec")
	Description          string   `json:"description"`                     // Human-readable description
	RFCs                 []string `json:"rfcs,omitempty"`                  // Related RFCs
	Capabilities         []int    `json:"capabilities,omitempty"`          // Capability codes this plugin handles
	Families             []string `json:"families,omitempty"`              // Address families this plugin handles
	ConfigRoots          []string `json:"config-roots,omitempty"`          // Config roots whose presence auto-loads it
	Dependencies         []string `json:"dependencies,omitempty"`          // Plugins that MUST load with it
	OptionalDependencies []string `json:"optional-dependencies,omitempty"` // Plugins it uses when present
	StartAfter           []string `json:"start-after,omitempty"`           // Order-only prerequisites
	Features             string   `json:"features,omitempty"`              // Space-separated feature list
	YANG                 bool     `json:"yang,omitempty"`                  // It carries a YANG schema
	NLRIDecoder          bool     `json:"nlri-decoder,omitempty"`          // It decodes NLRI in process
	NLRIEncoder          bool     `json:"nlri-encoder,omitempty"`          // It encodes NLRI in process
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
		result = append(result, pluginInfo(reg))
	}
	return result
}

// pluginInfo copies the descriptive fields of one registration into its row.
func pluginInfo(reg *registry.Registration) PluginInfo {
	info := PluginInfo{
		Name:                 reg.Name,
		Description:          reg.Description,
		RFCs:                 reg.RFCs,
		Families:             reg.Families,
		ConfigRoots:          reg.ConfigRoots,
		Dependencies:         reg.Dependencies,
		OptionalDependencies: reg.OptionalDependencies,
		StartAfter:           reg.StartAfter,
		Features:             reg.Features,
		YANG:                 reg.YANG != "",
		NLRIDecoder:          reg.InProcessNLRIDecoder != nil,
		NLRIEncoder:          reg.InProcessNLRIEncoder != nil,
	}
	// Convert uint8 capability codes to int for JSON compatibility.
	for _, c := range reg.CapabilityCodes {
		info.Capabilities = append(info.Capabilities, int(c))
	}
	return info
}

// pluginEventSource is the report-bus source of a fault the plugin package
// meets: a registration refused at startup, or a reactor the coordinator
// could not attach. Each is a state only a defect produces, raised under
// report.CodeInternalDefect because this package cannot import diagnostic.
const pluginEventSource = "plugin-registry"

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

// deriveName extracts a plugin name from command parts using the basename of
// the binary.
func deriveName(parts []string) string {
	return filepath.Base(parts[0])
}

// IsInternalPlugin checks if a name is a registered internal plugin.
// Uses the registry as single source of truth.
func IsInternalPlugin(name string) bool {
	return registry.Has(name)
}

// RegistryNames returns the compiled registration identity for p as a single-
// element slice, or nil when p runs a foreign external program.
func RegistryNames(p PluginConfig) []string {
	if id := p.CompiledIdentity(); id != "" {
		return []string{id}
	}
	return nil
}

// RegistryName returns the compiled registration identity for p, or empty
// string when p runs a foreign external program.
func RegistryName(p PluginConfig) string {
	return p.CompiledIdentity()
}
