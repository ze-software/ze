// Design: docs/architecture/container/image-store.md -- the fetch: the registry a functional test fetches from
// Related: registry_config.go -- the shipped variant
// Related: registry_list.go -- the registry list and listedClientConfig
// Related: ../../../test/fixture/container_registry.go -- the registry the key points at

//go:build zetest

package imagestore

import (
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/plugins/container/registry"
)

// testRegistryPortKey exists only in a zetest build, the functional-test
// daemon, so no shipped ze registers or reads it. When it is set, every
// registry is reached through a mirror on that loopback port over plain
// HTTP: the compiled registry fixture serves the image there, and the test
// names the image as an operator would, a short name of the public hub
// included. A registry with an entry in the registry list is reached as its
// entry says, whatever the key holds: the configuration wins.
const testRegistryPortKey = "ze.test.container.registry.port"

var _ = env.MustRegister(env.EnvEntry{
	Key:         testRegistryPortKey,
	Type:        "string",
	Description: "Test DUT builds only: the loopback port of the plain-HTTP registry mirror every image fetch asks first, the compiled registry fixture (test infrastructure)",
	Private:     true,
})

// engineClientConfig answers how the registry list of the running config
// says the registry is reached, as the shipped variant does. For a registry
// with no entry it answers the default with the test registry as the mirror,
// when the key names its port.
func engineClientConfig(reference registry.Reference) (registry.Config, error) {
	config, listed, err := listedClientConfig(appliedRegistries.current(), reference)
	if err != nil {
		return registry.Config{}, err
	}
	if listed {
		return config, nil
	}
	port := env.Get(testRegistryPortKey)
	if port == "" {
		return config, nil
	}
	config.Mirror = "127.0.0.1:" + port
	config.Transport = registry.TransportHTTP
	return config, nil
}
