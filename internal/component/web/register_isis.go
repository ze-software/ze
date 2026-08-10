// Design: ai/rules/plugins.md -- IS-IS web routes self-registration
// Related: handler_isis.go -- the neighbor and database views these routes serve

package web

import "net/http"

func init() {
	isis := func(d RouteDeps) *ISISHandlers {
		return &ISISHandlers{Renderer: d.Renderer, Dispatch: d.Dispatch}
	}
	RegisterWebRoute(WebRoute{Pattern: "GET /isis", Wrap: WrapAuth, Build: func(d RouteDeps) http.Handler {
		return isis(d).handleISISNeighbors()
	}})
	RegisterWebRoute(WebRoute{Pattern: "GET /isis/neighbors", Wrap: WrapAuth, Build: func(d RouteDeps) http.Handler {
		return isis(d).handleISISNeighbors()
	}})
	RegisterWebRoute(WebRoute{Pattern: "GET /isis/neighbors/stream", Wrap: WrapAuth, Build: func(d RouteDeps) http.Handler {
		return isis(d).handleISISNeighborsSSE()
	}})
	RegisterWebRoute(WebRoute{Pattern: "GET /isis/database", Wrap: WrapAuth, Build: func(d RouteDeps) http.Handler {
		return isis(d).handleISISDatabase()
	}})
	RegisterWebRoute(WebRoute{Pattern: "GET /isis/database/stream", Wrap: WrapAuth, Build: func(d RouteDeps) http.Handler {
		return isis(d).handleISISDatabaseSSE()
	}})
}
