// Design: docs/architecture/fleet-interface.md -- fleet route registration
// Related: source.go -- the slot whose state decides whether the routes mount
// Related: handler_shell.go, handler_fragment.go, handler_events.go -- the handlers the routes build
// Related: pages.go -- the fragment registry the shell's own fragments join
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

// The ServeMux patterns of the fleet interface. The mux prefers the longest
// pattern, so the assets, the fragments and the event stream are answered by
// their own handlers and every other address under the prefix by the page
// handler.
const (
	shellPattern    = "GET " + fleetPath
	assetsPattern   = "GET " + fleetAssetsPath
	fragmentPattern = "GET " + fleetPath + fragmentSegment + "/"
	eventsPattern   = "GET " + EventsPath
	stepPattern     = "POST " + stepPath
	holdPattern     = "POST " + holdPath
)

func init() {
	registerShellFragments()

	web.RegisterWebRoute(web.WebRoute{
		Pattern: assetsPattern,
		Wrap:    web.WrapAuth,
		Enabled: sourceRegistered,
		Build:   func(web.RouteDeps) http.Handler { return newAssetHandler() },
	})
	web.RegisterWebRoute(web.WebRoute{
		Pattern: fragmentPattern,
		Wrap:    web.WrapAuth,
		Enabled: sourceRegistered,
		Build:   buildFragmentHandler,
	})
	web.RegisterWebRoute(web.WebRoute{
		Pattern: eventsPattern,
		Wrap:    web.WrapAuth,
		Enabled: sourceRegistered,
		Build:   buildEventsHandler,
	})
	web.RegisterWebRoute(web.WebRoute{
		Pattern: stepPattern,
		Wrap:    web.WrapMutation,
		Enabled: stepperRegistered,
		Build:   buildStepHandler,
	})
	web.RegisterWebRoute(web.WebRoute{
		Pattern: holdPattern,
		Wrap:    web.WrapMutation,
		Enabled: stepperRegistered,
		Build:   buildHoldHandler,
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

// sourceRegistered is the Enabled function of every fleet route but the two
// prototype-only ones. The hub calls it once, when the web server starts, which is after
// every init() has run, so the order in which packages register does not
// matter.
func sourceRegistered() bool {
	_, ok := buildSource.registered()
	return ok
}

// stepperRegistered is the Enabled function of the two prototype-only routes:
// a source is registered and its events follow a script that can be stepped,
// which only the sample source's do. A build with the real source mounts
// neither.
func stepperRegistered() bool {
	source, ok := buildSource.registered()
	if !ok {
		return false
	}

	_, steps := source.(Stepper)

	return steps
}

// mountedSource answers the registered source for a route the hub is
// building. The hub builds only a route whose Enabled answered true, so an
// empty slot here is a defect.
func mountedSource() Source {
	source, ok := buildSource.registered()
	if !ok {
		panic("BUG: fleetui: a fleet route was built with no data source registered")
	}

	return source
}

// buildShellHandler builds the page handler over the registered source.
func buildShellHandler(web.RouteDeps) http.Handler {
	return newFleetHandler(mountedSource())
}

// buildFragmentHandler builds the fragment handler over the registered
// source.
func buildFragmentHandler(web.RouteDeps) http.Handler {
	return http.HandlerFunc(newFleetHandler(mountedSource()).serveFragment)
}

// buildEventsHandler builds the live event stream over the registered
// source.
func buildEventsHandler(web.RouteDeps) http.Handler {
	return newEventStream(mountedSource())
}

// mountedStepper answers the registered source as a Stepper, which
// stepperRegistered found it to be before the hub built the route.
func mountedStepper() Stepper {
	stepper, steps := mountedSource().(Stepper)
	if !steps {
		panic("BUG: fleetui: a prototype-only route was built over a source that cannot be stepped")
	}

	return stepper
}

// buildStepHandler builds the step route over the registered source.
func buildStepHandler(web.RouteDeps) http.Handler {
	return newStepHandler(mountedStepper())
}

// buildHoldHandler builds the hold route over the registered source.
func buildHoldHandler(web.RouteDeps) http.Handler {
	return newHoldHandler(mountedStepper())
}
