// Design: docs/architecture/api/process-protocol.md — plugin process management

package plugin

import (
	"net"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// InternalPluginRunner is a function that runs a plugin in-process using SDK RPC.
// engineConn = Socket A plugin side (plugin → engine RPCs).
// callbackConn = Socket B plugin side (engine → plugin callbacks).
type InternalPluginRunner func(engineConn, callbackConn net.Conn) int

// GetInternalPluginWantsConfig returns the config roots an internal plugin wants.
// Returns nil if the plugin doesn't declare any config roots.
func GetInternalPluginWantsConfig(name string) []string {
	reg := registry.Lookup(name)
	if reg == nil {
		return nil
	}
	return reg.ConfigRoots
}

// GetInternalPluginYANG returns the YANG schema for an internal plugin.
// Returns empty string if the plugin doesn't provide YANG.
func GetInternalPluginYANG(name string) string {
	reg := registry.Lookup(name)
	if reg == nil {
		return ""
	}
	return reg.YANG
}

// GetAllInternalPluginYANG returns all internal plugin YANG schemas.
// Used to load all plugin YANG schemas before config parsing.
func GetAllInternalPluginYANG() map[string]string {
	schemas := registry.YANGSchemas()
	result := make(map[string]string, len(schemas))
	for name, yang := range schemas {
		moduleName := "ze-" + name + ".yang"
		result[moduleName] = yang
	}
	return result
}

// CollectPluginYANG returns YANG schemas for the specified plugins.
// Each entry maps module name (e.g., "ze-hostname") to YANG content.
// Only returns entries for plugins that have YANG schemas.
func CollectPluginYANG(plugins []string) map[string]string {
	result := make(map[string]string)
	for _, p := range plugins {
		// Extract plugin name from "ze.X" format.
		name := p
		if len(p) > 3 && p[:3] == "ze." {
			name = p[3:]
		}

		yang := GetInternalPluginYANG(name)
		if yang != "" {
			moduleName := "ze-" + name + ".yang"
			result[moduleName] = yang
		}
	}
	return result
}

// GetInternalPluginRunner returns the runner function for an internal plugin.
// Returns nil if the plugin is not found.
// The returned runner configures the plugin's engine logger before running.
func GetInternalPluginRunner(name string) InternalPluginRunner {
	reg := registry.Lookup(name)
	if reg == nil {
		return nil
	}
	return func(engineConn, callbackConn net.Conn) int {
		if reg.ConfigureEngineLogger != nil {
			reg.ConfigureEngineLogger(name)
		}
		if reg.ConfigureMetrics != nil {
			if mr := registry.GetMetricsRegistry(); mr != nil {
				reg.ConfigureMetrics(mr)
			}
		}
		return reg.RunEngine(engineConn, callbackConn)
	}
}

// GetPluginForFamily returns the internal plugin name that handles a family.
// Returns empty string if no plugin is known for the family.
func GetPluginForFamily(family string) string {
	return registry.PluginForFamily(family)
}

// GetRequiredPlugins returns the list of internal plugins needed for the given families.
// Only returns plugins that handle known families.
// Deduplicates the result.
func GetRequiredPlugins(families []string) []string {
	return registry.RequiredPlugins(families)
}
