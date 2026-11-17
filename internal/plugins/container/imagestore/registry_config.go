// Design: docs/architecture/container/image-store.md -- the fetch: how a registry is reached, from the registry list
// Related: registry_config_zetest.go -- the functional-test daemon's variant
// Related: registry_list.go -- the registry list and listedClientConfig
// Related: fetch_steps.go -- defaultClientConfig

//go:build !zetest

package imagestore

import "github.com/ze-software/ze/internal/plugins/container/registry"

// engineClientConfig answers how the store reaches the registry a reference
// names, in a shipped ze: as the registry list of the running config says,
// and by the default of defaultClientConfig for a registry with no entry.
// The list is read at each call, so a config change reaches the next fetch.
func engineClientConfig(reference registry.Reference) (registry.Config, error) {
	config, _, err := listedClientConfig(appliedRegistries.current(), reference)
	return config, err
}
