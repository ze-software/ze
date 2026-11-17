// Design: docs/architecture/container/image-store.md -- the fetch: how a registry is reached before the registry list exists
// Related: registry_config_zetest.go -- the functional-test daemon's variant
// Related: fetch_steps.go -- defaultClientConfig

//go:build !zetest

package imagestore

import "github.com/ze-software/ze/internal/plugins/container/registry"

// engineClientConfig answers how the store reaches the registry a reference
// names, in a shipped ze: by the default of defaultClientConfig, since no
// registry configuration exists yet.
func engineClientConfig(reference registry.Reference) (registry.Config, error) {
	return defaultClientConfig(reference)
}
