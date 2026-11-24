// Design: docs/architecture/api/architecture.md -- endpoint name resolution by registration identity
// Related: bus_requirements.go -- unified BGP feature-requirement validator over compiled bus filters

package bgpconfig

import (
	"slices"

	"github.com/ze-software/ze/internal/component/plugin"
)

// processNameFor returns the configured endpoint name running registryName, or
// registryName itself when auto-loaded without an explicit bus > endpoint
// alias.
func processNameFor(plugins []plugin.PluginConfig, registryName string) string {
	for _, declared := range plugins {
		if declared.CompiledIdentity() == registryName {
			return declared.Name
		}
		if slices.Contains(plugin.RegistryNames(declared), registryName) {
			return declared.Name
		}
	}
	return registryName
}
