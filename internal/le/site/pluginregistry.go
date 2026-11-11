// Design: docs/contributing/gh-pages.md -- publish one committed Linux le population.
// Related: build.go -- refreshNativeSurfaces reads the registry before the daemon compile.
package site

import repoinventory "github.com/ze-software/ze/internal/le/repo/inventory"

// committedPluginRegistry resolves membership and metadata from one committed
// source snapshot, without executing host or cross-compiled product code.
//
// It reads the declared registrations and runs no check: building the website
// renders data. Three checks, run in CI, make that list complete: the registry
// seal (registry.Seal refuses a registration after start), the
// registration-sites check (internal/le/plugin/registrationsites) and the
// comparison of the built ze's registry with the declared list
// (internal/le/plugin/registered).
func committedPluginRegistry(root string) ([]repoinventory.Plugin, error) {
	return repoinventory.DeclaredPlugins(root)
}
