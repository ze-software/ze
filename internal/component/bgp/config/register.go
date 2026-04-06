// Design: docs/architecture/config/syntax.md -- BGP config registration hooks

package bgpconfig

import (
	"fmt"

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
	var data []byte
	if configPath != "" && configPath != "-" && store != nil {
		var err error
		data, err = store.ReadFile(configPath)
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
