// Design: ai/rules/plugins.md -- OSPF web routes self-registration
// Related: handler_ospf.go -- the neighbor and database views these routes serve
//
// Inject is never surfaced on the web (spec-ospf-ext-14 R-6): only read-only
// show commands are routed here.

package web

import "net/http"

func init() {
	ospf := func(d RouteDeps) *OSPFHandlers {
		return &OSPFHandlers{Renderer: d.Renderer, Dispatch: d.Dispatch}
	}
	RegisterWebRoute(WebRoute{Pattern: "GET /ospf", Wrap: WrapAuth, Build: func(d RouteDeps) http.Handler {
		return ospf(d).HandleOSPFNeighbors()
	}})
	RegisterWebRoute(WebRoute{Pattern: "GET /ospf/neighbors", Wrap: WrapAuth, Build: func(d RouteDeps) http.Handler {
		return ospf(d).HandleOSPFNeighbors()
	}})
	RegisterWebRoute(WebRoute{Pattern: "GET /ospf/neighbors/stream", Wrap: WrapAuth, Build: func(d RouteDeps) http.Handler {
		return ospf(d).HandleOSPFNeighborsSSE()
	}})
	RegisterWebRoute(WebRoute{Pattern: "GET /ospf/database", Wrap: WrapAuth, Build: func(d RouteDeps) http.Handler {
		return ospf(d).HandleOSPFDatabase()
	}})
	RegisterWebRoute(WebRoute{Pattern: "GET /ospf/database/stream", Wrap: WrapAuth, Build: func(d RouteDeps) http.Handler {
		return ospf(d).HandleOSPFDatabaseSSE()
	}})
	RegisterWebRoute(WebRoute{Pattern: "GET /ospf/database/opaque", Wrap: WrapAuth, Build: func(d RouteDeps) http.Handler {
		return ospf(d).HandleOSPFOpaqueDatabase()
	}})
	RegisterWebRoute(WebRoute{Pattern: "GET /ospf/database/opaque/stream", Wrap: WrapAuth, Build: func(d RouteDeps) http.Handler {
		return ospf(d).HandleOSPFOpaqueDatabaseSSE()
	}})
	RegisterWebRoute(WebRoute{Pattern: "GET /ospfv3/database", Wrap: WrapAuth, Build: func(d RouteDeps) http.Handler {
		return ospf(d).HandleOSPFv3Database()
	}})
	RegisterWebRoute(WebRoute{Pattern: "GET /ospfv3/database/stream", Wrap: WrapAuth, Build: func(d RouteDeps) http.Handler {
		return ospf(d).HandleOSPFv3DatabaseSSE()
	}})
}
