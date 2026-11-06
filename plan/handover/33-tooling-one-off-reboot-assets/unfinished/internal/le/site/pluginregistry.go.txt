// Design: docs/contributing/gh-pages.md -- publish one committed Linux le population.
// Related: build.go -- registry refusal precedes the daemon compile.
package site

import repoinventory "github.com/ze-software/ze/internal/le/repo/inventory"

// committedPluginRegistry resolves membership and metadata from one committed
// source snapshot, without executing host or cross-compiled product code.
func committedPluginRegistry(root string) ([]repoinventory.Plugin, error) {
	return repoinventory.Plugins(root)
}
