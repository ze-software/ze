// Design: docs/architecture/api/architecture.md -- startup registration of compiled-bus implied and graph edge providers.
// Related: autoload_reasons.go -- provideImpliedBus and provideGraphBusEdges.

package server

import "github.com/ze-software/ze/internal/component/config"

func init() {
	config.RegisterImpliedBusProvider(provideImpliedBus)
	config.RegisterGraphBusEdgeProvider(provideGraphBusEdges)
}
