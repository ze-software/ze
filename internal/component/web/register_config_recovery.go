// Design: docs/architecture/web-interface.md -- independent pending-change recovery routes.
package web

import "net/http"

func init() {
	build := func(deps RouteDeps) http.Handler {
		return HandleConfigRecovery(deps.EditorManager, deps.Renderer, deps.Authorizer, deps.Recorder)
	}
	RegisterWebRoute(WebRoute{
		Pattern: "GET /config/recover", Wrap: WrapAuth,
		Build: build,
	})
	RegisterWebRoute(WebRoute{
		Pattern: "POST /config/recover", Wrap: WrapMutation,
		Build: build,
	})
}
