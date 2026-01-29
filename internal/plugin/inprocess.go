package plugin

import (
	"io"

	"codeberg.org/thomas-mangin/ze/internal/plugin/gr"
	"codeberg.org/thomas-mangin/ze/internal/plugin/hostname"
	"codeberg.org/thomas-mangin/ze/internal/plugin/rib"
	"codeberg.org/thomas-mangin/ze/internal/plugin/rr"
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
