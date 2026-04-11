// Design: docs/architecture/api/process-protocol.md — plugin process management

package plugin

import (
	"net"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// CanonicalSubsystemName converts a plugin registry name (hyphen-separated,
// e.g. "bgp-gr", "bgp-filter-community", "iface-dhcp") to the canonical
// dot-separated slog subsystem name the engine uses everywhere else
// (e.g. "bgp.gr", "bgp.filter.community", "iface.dhcp").
//
// This keeps plugin registration names, env var keys, config file keys,
// and stderr output tags aligned so a user writing
// `environment.log { bgp.gr debug; }` in the config actually routes
// to the bgp.gr plugin logger. Without this transform the engine logger
// would be registered under the raw registry name "bgp-gr" and require
// the unintuitive `environment.log { bgp-gr debug; }` form.
//
// Idempotent for names that already contain no hyphens (e.g. "rib",
// "interface", "bgp.reactor").
func CanonicalSubsystemName(registryName string) string {
	return strings.ReplaceAll(registryName, "-", ".")
}

// InternalPluginRunner is a function that runs a plugin in-process using SDK RPC.
// conn is the single bidirectional connection for all RPCs.
type InternalPluginRunner func(conn net.Conn) int

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
	return func(conn net.Conn) int {
		if reg.ConfigureEngineLogger != nil {
			// Pass the canonical dot-separated subsystem name so the
			// plugin's engine logger matches the rest of the slogutil
			// hierarchy (bgp.gr, bgp.filter.community, etc.) and the
			// config file key `bgp.gr debug` routes here via
			// ApplyLogConfig -> ze.log.bgp.gr -> getLogEnv("bgp.gr").
			reg.ConfigureEngineLogger(CanonicalSubsystemName(name))
		}
		if reg.ConfigureMetrics != nil {
			if mr := registry.GetMetricsRegistry(); mr != nil {
				reg.ConfigureMetrics(mr)
			}
		}
		if reg.ConfigureEventBus != nil {
			if eb := registry.GetEventBus(); eb != nil {
				reg.ConfigureEventBus(eb)
			}
		}
		if reg.ConfigurePluginServer != nil {
			if s := registry.GetPluginServer(); s != nil {
				reg.ConfigurePluginServer(s)
			}
		}
		return reg.RunEngine(conn)
	}
}

// GetPluginForFamily returns the internal plugin name that handles a family.
// Returns empty string if no plugin is known for the family.
func GetPluginForFamily(family string) string {
	return registry.PluginForFamily(family)
}

// GetPluginForEventType returns the internal plugin name that produces an event type.
// Returns empty string if no plugin declares that event type.
func GetPluginForEventType(eventType string) string {
	return registry.PluginForEventType(eventType)
}

// GetPluginForSendType returns the internal plugin name that enables a send type.
// Returns empty string if no plugin declares that send type.
func GetPluginForSendType(sendType string) string {
	return registry.PluginForSendType(sendType)
}

// GetRequiredPlugins returns the list of internal plugins needed for the given families.
// Only returns plugins that handle known families.
// Deduplicates the result.
func GetRequiredPlugins(families []string) []string {
	return registry.RequiredPlugins(families)
}
