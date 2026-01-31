package plugin

import (
	"io"

	"codeberg.org/thomas-mangin/ze/internal/plugin/evpn"
	"codeberg.org/thomas-mangin/ze/internal/plugin/flowspec"
	"codeberg.org/thomas-mangin/ze/internal/plugin/gr"
	"codeberg.org/thomas-mangin/ze/internal/plugin/hostname"
	"codeberg.org/thomas-mangin/ze/internal/plugin/rib"
	"codeberg.org/thomas-mangin/ze/internal/plugin/rr"
	"codeberg.org/thomas-mangin/ze/internal/plugin/vpn"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// InternalPluginRunner is a function that runs a plugin in-process.
// It reads from in, writes to out, and returns an exit code.
type InternalPluginRunner func(in io.Reader, out io.Writer) int

// internalPluginRunners maps plugin names to their runner functions.
// This is the single source of truth for available internal plugins.
// Used by:
//   - IsInternalPlugin() in resolve.go
//   - AvailableInternalPlugins() in resolve.go
//   - startInternal() in process.go
var internalPluginRunners = map[string]InternalPluginRunner{
	"rib": func(in io.Reader, out io.Writer) int {
		return rib.NewRIBManager(in, out).Run()
	},
	"gr": func(in io.Reader, out io.Writer) int {
		return gr.NewGRPlugin(in, out).Run()
	},
	"rr": func(in io.Reader, out io.Writer) int {
		return rr.NewRouteServer(in, out).Run()
	},
	"hostname": func(in io.Reader, out io.Writer) int {
		// Configure logger for in-process plugin (uses ze.log.hostname env var)
		hostname.ConfigureLogger(slogutil.Logger("hostname"))
		return hostname.NewHostnamePlugin(in, out).Run()
	},
	"flowspec": func(in io.Reader, out io.Writer) int {
		// Configure logger for in-process plugin (uses ze.log.flowspec env var)
		flowspec.SetFlowSpecLogger(slogutil.Logger("flowspec"))
		return flowspec.NewFlowSpecPlugin(in, out).Run()
	},
	"evpn": func(in io.Reader, out io.Writer) int {
		// Configure logger for in-process plugin (uses ze.log.evpn env var)
		evpn.SetEVPNLogger(slogutil.Logger("evpn"))
		return evpn.NewEVPNPlugin(in, out).Run()
	},
	"vpn": func(in io.Reader, out io.Writer) int {
		// Configure logger for in-process plugin (uses ze.log.vpn env var)
		vpn.SetVPNLogger(slogutil.Logger("vpn"))
		return vpn.NewVPNPlugin(in, out).Run()
	},
}

// internalPluginYANG maps plugin names to their YANG schema getters.
// Only plugins that augment the config schema need entries here.
var internalPluginYANG = map[string]func() string{
	"hostname": hostname.GetYANG,
}

// GetInternalPluginYANG returns the YANG schema for an internal plugin.
// Returns empty string if the plugin doesn't provide YANG.
func GetInternalPluginYANG(name string) string {
	if getter, ok := internalPluginYANG[name]; ok {
		return getter()
	}
	return ""
}

// CollectPluginYANG returns YANG schemas for the specified plugins.
// Each entry maps module name (e.g., "ze-hostname") to YANG content.
// Only returns entries for plugins that have YANG schemas.
func CollectPluginYANG(plugins []string) map[string]string {
	result := make(map[string]string)
	for _, p := range plugins {
		// Extract plugin name from "ze.X" format
		name := p
		if len(p) > 3 && p[:3] == "ze." {
			name = p[3:]
		}

		yang := GetInternalPluginYANG(name)
		if yang != "" {
			// Derive module name from YANG content or use convention
			moduleName := "ze-" + name + ".yang"
			result[moduleName] = yang
		}
	}
	return result
}

// GetInternalPluginRunner returns the runner function for an internal plugin.
// Returns nil if the plugin is not found.
func GetInternalPluginRunner(name string) InternalPluginRunner {
	return internalPluginRunners[name]
}

// familyToPlugin maps address families to the internal plugin that handles them.
// Used for auto-loading plugins when a family is configured but no plugin declared.
// Key: family string (e.g., "ipv4/flow"), Value: plugin name (e.g., "flowspec").
var familyToPlugin = map[string]string{
	// FlowSpec families → flowspec plugin
	"ipv4/flow":     "flowspec",
	"ipv6/flow":     "flowspec",
	"ipv4/flow-vpn": "flowspec",
	"ipv6/flow-vpn": "flowspec",
	// EVPN family → evpn plugin
	"l2vpn/evpn": "evpn",
	// VPN families → vpn plugin
	"ipv4/vpn": "vpn",
	"ipv6/vpn": "vpn",
}

// GetPluginForFamily returns the internal plugin name that handles a family.
// Returns empty string if no plugin is known for the family.
func GetPluginForFamily(family string) string {
	return familyToPlugin[family]
}

// GetRequiredPlugins returns the list of internal plugins needed for the given families.
// Only returns plugins that are in familyToPlugin (known internal plugins).
// Deduplicates the result.
func GetRequiredPlugins(families []string) []string {
	seen := make(map[string]bool)
	var plugins []string
	for _, fam := range families {
		if p := familyToPlugin[fam]; p != "" && !seen[p] {
			seen[p] = true
			plugins = append(plugins, p)
		}
	}
	return plugins
}
