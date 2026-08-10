// Design: ai/rules/plugins.md -- L2TP web routes self-registration
// Related: handler_l2tp.go -- the handlers these routes serve

//go:build ze_l2tp

package web

import "net/http"

func init() {
	RegisterWebRoute(WebRoute{Pattern: "GET /l2tp", Wrap: WrapAuth, Build: func(d RouteDeps) http.Handler {
		return (&l2TPHandlers{Renderer: d.Renderer, Dispatch: d.Dispatch}).handleL2TPList()
	}})
	RegisterWebRoute(WebRoute{Pattern: "GET /l2tp/{sid}", Wrap: WrapAuth, Build: func(d RouteDeps) http.Handler {
		return (&l2TPHandlers{Renderer: d.Renderer, Dispatch: d.Dispatch}).handleL2TPDetail()
	}})
	RegisterWebRoute(WebRoute{Pattern: "POST /l2tp/{sid}/disconnect", Wrap: WrapMutation, Build: func(d RouteDeps) http.Handler {
		return (&l2TPHandlers{Renderer: d.Renderer, Dispatch: d.Dispatch}).handleL2TPDisconnect()
	}})
	RegisterWebRoute(WebRoute{Pattern: "GET /l2tp/{login}/samples", Wrap: WrapAuth, Build: func(RouteDeps) http.Handler {
		return handleL2TPSamplesJSON()
	}})
	RegisterWebRoute(WebRoute{Pattern: "GET /l2tp/{login}/samples.csv", Wrap: WrapAuth, Build: func(RouteDeps) http.Handler {
		return handleL2TPSamplesCSV()
	}})
	RegisterWebRoute(WebRoute{Pattern: "GET /l2tp/{login}/samples/stream", Wrap: WrapAuth, Build: func(RouteDeps) http.Handler {
		return HandleL2TPSamplesSSE()
	}})
}
