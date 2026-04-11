// Design: docs/architecture/config/syntax.md -- BGP config registration hooks

package bgpconfig

import (
	"fmt"
	"os"

	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

func init() {
	zeconfig.RegisterPluginExtractor(extractBGPInlinePlugins)
	registry.RegisterReactorFactory(createReactorFromCoordinator)
}

// createReactorFromCoordinator builds a BGP reactor using config state stored
// in the coordinator by the hub. This keeps bgp/config imports out of the hub.
func createReactorFromCoordinator(coord registry.CoordinatorAccessor) (registry.BGPReactorHandle, error) {
	configPath, _ := coord.GetExtra("bgp.configPath").(string)
	cliPlugins, _ := coord.GetExtra("bgp.cliPlugins").([]string)
	configData, _ := coord.GetExtra("bgp.configData").([]byte)

	storeAny := coord.GetExtra("bgp.store")
	if storeAny == nil {
		return nil, fmt.Errorf("bgp: coordinator missing bgp.store")
	}
	store, ok := storeAny.(storage.Storage)
	if !ok {
		return nil, fmt.Errorf("bgp: bgp.store has unexpected type %T", storeAny)
	}

	// Re-read config from disk for reload support. Stdin uses captured data.
	// Mirrors the hub's initial-load fallback (cmd/ze/hub/main.go Run): try the
	// blob store first, and if the store is blob-only (e.g., gokrazy read-only
	// root) fall back to a direct filesystem read. Without this fallback, all
	// encode/plugin .ci tests that pass a /tmp/... config path via `ze <file>`
	// fail with "read file/active/...: file does not exist" because the
	// filesystem path is not a valid blob key.
	var data []byte
	if configPath != "" && configPath != "-" && store != nil {
		var err error
		data, err = store.ReadFile(configPath)
		if err != nil && storage.IsBlobStorage(store) {
			data, err = os.ReadFile(configPath) //nolint:gosec // path supplied by the daemon operator
		}
		if err != nil {
			return nil, fmt.Errorf("re-read config for reactor: %w", err)
		}
	} else {
		data = configData
	}

	// Set YANG validator for runtime attribute validation (origin enum, med/local-pref ranges).
	pluginYANG := plugin.CollectPluginYANG(cliPlugins)
	if v, vErr := zeconfig.YANGValidatorWithPlugins(pluginYANG); vErr == nil && v != nil {
		plugin.SetYANGValidator(v)
	}

	result, err := zeconfig.LoadConfig(string(data), configPath, cliPlugins)
	if err != nil {
		return nil, fmt.Errorf("parse config for reactor: %w", err)
	}

	r, err := CreateReactor(result, configPath, store)
	if err != nil {
		return nil, err
	}

	// Chaos injection from hub-stored config.
	injectChaos(r, coord)

	// GR marker from storage (RFC 4724 Section 4.1).
	readGRMarker(r, store)

	return r, nil
}
