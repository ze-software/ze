// Design: docs/architecture/container/image-store.md -- how the store reaches the plugin
// Related: store.go -- the store the factory builds
// Related: registry_list.go -- the config sibling registered here
//
// Package imagestore is the container plugin's image store: it fetches OCI
// images, keeps each in one folder named by its digest, and gives each
// container its writable layer and its volumes. It runs in the plugin's
// process and fills the plugin's store seam from its own init(); the
// plugin's runtime package never imports it.
package imagestore

import (
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/internal/plugins/container"
	"github.com/ze-software/ze/pkg/zefs"
)

//nolint:gochecknoinits // registration: the store fills the plugin's seam and is discovered through the generated imports
func init() {
	container.SetStoreFactory(newStore)
	// The engine client is the store's one handle on the state store: the
	// fetch writes the bindings through it and the resolve reads them and
	// records the generations.
	container.RegisterEngineClientReceiver(receivedEngine.receive)
	// The registry list is the store's part of the container config: the
	// parse judges it at every verify and the apply makes it the list each
	// fetch reads.
	container.RegisterConfigSibling(registrySiblingName, parseRegistrySections, appliedRegistries.apply)
	// The plugin persists its bindings, generations and project-id counter
	// through the SDK state calls, which the daemon answers only for keys its
	// owner was granted.
	statestore.RegisterPluginKeys(container.PluginName,
		zefs.KeyContainerBinding,
		zefs.KeyContainerGeneration,
		zefs.KeyContainerProjectID,
	)
}
