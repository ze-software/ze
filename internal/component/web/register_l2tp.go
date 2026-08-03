// Design: ai/rules/plugins.md -- L2TP web routes self-registration
// Related: handler_l2tp.go -- the handlers these routes serve

//go:build ze_l2tp

package web

import "net/http"

func init() {
	RegisterWebRoute(WebRoute{Pattern: "GET /l2tp", Wrap: WrapAuth, Build: func(d RouteDeps) http.Handler {
		return (&L2TPHandlers{Renderer: d.Renderer, Dispatch: d.Dispatch}).HandleL2TPList()
	}})
	RegisterWebRoute(WebRoute{Pattern: "GET /l2tp/{sid}", Wrap: WrapAuth, Build: func(d RouteDeps) http.Handler {
		return (&L2TPHandlers{Renderer: d.Renderer, Dispatch: d.Dispatch}).HandleL2TPDetail()
	}})
	RegisterWebRoute(WebRoute{Pattern: "POST /l2tp/{sid}/disconnect", Wrap: WrapMutation, Build: func(d RouteDeps) http.Handler {
		return (&L2TPHandlers{Renderer: d.Renderer, Dispatch: d.Dispatch}).HandleL2TPDisconnect()
	}})
	RegisterWebRoute(WebRoute{Pattern: "GET /l2tp/{login}/samples", Wrap: WrapAuth, Build: func(RouteDeps) http.Handler {
		return HandleL2TPSamplesJSON()
	}})
	RegisterWebRoute(WebRoute{Pattern: "GET /l2tp/{login}/samples.csv", Wrap: WrapAuth, Build: func(RouteDeps) http.Handler {
		return HandleL2TPSamplesCSV()
	}})
	RegisterWebRoute(WebRoute{Pattern: "GET /l2tp/{login}/samples/stream", Wrap: WrapAuth, Build: func(RouteDeps) http.Handler {
		return HandleL2TPSamplesSSE()
	}})
}
