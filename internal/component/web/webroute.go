// Design: ai/rules/plugins.md -- web route registry (registration over hardcoding)
// Related: handler_l2tp.go, handler_isis.go, handler_ospf.go -- in-tree feature routes that self-register
//
// WebRoute lets in-tree web features (L2TP, IS-IS, OSPF, the gokrazy portal)
// register their HTTP routes at init() instead of being hardcoded in the hub's
// startWebServer. The hub iterates RegisteredWebRoutes at server start, wraps
// each route by its WrapKind, and serves it. Deleting a feature's register_*.go
// (or the whole feature) removes its routes with no edit to shared hub code.

package web

import (
	"net/http"
	"sync"
)

// WrapKind selects the auth/mutation middleware the hub applies to a route. The
// wrap helpers close over the hub's session store, authorizer, and audit
// recorder, so they live in the hub; the web-package contract carries only the
// kind, keeping the web package free of hub internals (spec R-2).
type WrapKind int

const (
	// WrapAuth wraps a read route: an authenticated session is required.
	WrapAuth WrapKind = iota
	// WrapMutation wraps a state-changing route: auth plus same-origin check.
	WrapMutation
)

// RouteDeps carries the per-server dependencies a route builder needs. It is
// built once at server start and passed to every WebRoute.Build.
type RouteDeps struct {
	Renderer *Renderer
	Dispatch CommandDispatcher
}

// WebRoute is a self-registered HTTP route for an in-tree web feature. Feature
// packages register their routes via init() in a register_*.go file.
type WebRoute struct {
	// Pattern is the http.ServeMux pattern, e.g. "GET /l2tp".
	Pattern string
	// Wrap selects the middleware the hub applies (auth vs mutation).
	Wrap WrapKind
	// Build constructs the handler at server start from the shared deps.
	Build func(RouteDeps) http.Handler
	// Enabled, when non-nil, gates whether the route is wired at all
	// (e.g. an env-flag-gated portal). Nil means always wired.
	Enabled func() bool
	// Portal, when non-nil, is a portal/nav entry registered when the route
	// is wired, so the feature appears in the portal menu.
	Portal *PortalService
}

var (
	routeMu   sync.RWMutex
	webRoutes []WebRoute
)

// RegisterWebRoute adds a route to the registry. Call it from init() in a
// register_*.go file so the route is discovered without editing the hub.
func RegisterWebRoute(r WebRoute) {
	routeMu.Lock()
	webRoutes = append(webRoutes, r)
	routeMu.Unlock()
}

// RegisteredWebRoutes returns a copy of the registered routes for the hub to
// iterate at server start.
func RegisteredWebRoutes() []WebRoute {
	routeMu.RLock()
	out := make([]WebRoute, len(webRoutes))
	copy(out, webRoutes)
	routeMu.RUnlock()
	return out
}
