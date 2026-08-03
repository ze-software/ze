// Design: docs/architecture/config/yang-config-design.md — hub handler claim surface
// Related: main.go -- buildSchemaRegistry, the producer this reads

package cli

import (
	"sort"
)

// ConfigHandlerPaths returns the handler paths the internal schema registry
// binds, sorted.
//
// Hub.RouteCommand resolves a config path to a subsystem with
// SchemaRegistry.FindHandler (internal/component/plugin/server/hub.go,
// schema.go), so a path under one of these reaches a handler even when no
// plugin declares it as a config root. That makes this the second claim surface
// a config-completeness check has to union in.
//
// The paths are DERIVED from buildSchemaRegistry, the same producer behind
// `ze schema handlers`. Nothing here restates them.
func ConfigHandlerPaths() ([]string, error) {
	registry, err := buildSchemaRegistry(nil)
	if err != nil {
		return nil, err
	}
	handlers := registry.ListHandlers()
	paths := make([]string, 0, len(handlers))
	for path := range handlers {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}
