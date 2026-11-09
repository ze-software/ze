// Design: docs/contributing/gh-pages.md -- publish one committed Linux le population.
// Related: build.go -- refreshNativeSurfaces reads the registry before the daemon compile.
package site

import repoinventory "github.com/ze-software/ze/internal/le/repo/inventory"

// committedPluginRegistry resolves membership and metadata from one committed
// source snapshot, without executing host or cross-compiled product code.
//
// It reads the declared registrations and never runs the external-effect
// proof: building the website renders data and runs no check. The proof that
// no other registration exists is repoinventory.Plugins, which the plugin
// gated-table and coupling checks and the inventory's own tests run in CI.
func committedPluginRegistry(root string) ([]repoinventory.Plugin, error) {
	return repoinventory.DeclaredPlugins(root)
}
