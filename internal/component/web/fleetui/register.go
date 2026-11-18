// Design: docs/architecture/fleet-interface.md -- fleet route registration
// Related: source.go -- the slot whose state decides whether the routes mount
// Related: handler_shell.go -- the handlers the routes build
//
// The fleet routes register here and the hub mounts them when the web server
// starts. They mount only when a data source is registered, so a build that
// links this package and no source serves no fleet route and lists no fleet
// entry in the portal menu.

package fleetui

import (
	"net/http"

	"github.com/ze-software/ze/internal/component/web"
)

// The two ServeMux patterns of the fleet interface. The assets pattern is the
// longer one, so the mux prefers it for an asset request.
const (
	shellPattern  = "GET " + fleetPath
	assetsPattern = "GET " + fleetAssetsPath
)

func init() {
	web.RegisterWebRoute(web.WebRoute{
		Pattern: assetsPattern,
		Wrap:    web.WrapAuth,
		Enabled: sourceRegistered,
		Build:   func(web.RouteDeps) http.Handler { return newAssetHandler() },
	})
	web.RegisterWebRoute(web.WebRoute{
		Pattern: shellPattern,
		Wrap:    web.WrapAuth,
		Enabled: sourceRegistered,
		Build:   buildShellHandler,
		// Direct, because the fleet interface has its own shell and the web
		// server forbids framing a page.
		Portal: &web.PortalService{Key: "fleet", Title: "Fleet", Path: fleetPath, Direct: true},
	})
}

// sourceRegistered is the Enabled function of every fleet route. The hub calls
// it once, when the web server starts, which is after every init() has run, so
// the order in which packages register does not matter.
func sourceRegistered() bool {
	_, ok := buildSource.registered()
	return ok
}

// buildShellHandler builds the estate page handler over the registered source.
// The hub calls it only for a route whose Enabled answered true.
func buildShellHandler(web.RouteDeps) http.Handler {
	source, ok := buildSource.registered()
	if !ok {
		panic("BUG: fleetui: the shell route was built with no data source registered")
	}

	return newShellHandler(source)
}
