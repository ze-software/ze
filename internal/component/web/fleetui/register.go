// Design: docs/architecture/fleet-interface.md -- fleet route registration
// Related: source.go -- the slot whose state decides whether the routes mount
// Related: handler_shell.go, handler_fragment.go, handler_events.go -- the handlers the routes build
// Related: pages.go -- the fragment registry the shell's own fragments join
// Related: handler_action.go -- the action registry the action route answers
// Related: handler_audit.go -- the audit export the export route answers
// Related: register_device_pages.go -- the console route, registered by its family
//
// The fleet routes register here. Each is declared once, in declaredRoutes, and
// read two ways: the daemon mounts them through the web route registry when
// the web server starts, over the build's registered source, and NewRoutes
// mounts them on one handler over a source it is given, for a caller that
// serves the interface in process. The registered routes mount only when a
// data source is registered, so a build that links this package and no
// source serves no fleet route and lists no fleet entry in the portal menu.
//
// A handler takes its source from its builder, never from the slot, so two
// sources can be served in one process. The slot only feeds the daemon's
// registration, and keeps its source for the life of the process: Source has
// no Close, and nothing stops it. A source that runs a worker, as the
// sample's clock does, runs it until the process ends.

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
	fragmentPattern = "GET " + fragmentBase
	eventsPattern   = "GET " + EventsPath
	stepPattern     = "POST " + stepPath
	holdPattern     = "POST " + holdPath
	// actionPattern is every prototype action: the action registry answers
	// it, under the mutation wrap, because an action is a state change in
	// the functional interface even though the prototype changes nothing.
	actionPattern = "POST " + actionPath
	// auditExportPattern is the audit log export, a read.
	auditExportPattern = "GET " + AuditExportPath
)

// fleetRoute is one route of the fleet interface: its pattern, the wrap the
// hub applies, and the builder of its handler over a source. stepped marks a
// prototype-only route, mounted only over a source that implements Stepper.
type fleetRoute struct {
	pattern string
	wrap    web.WrapKind
	stepped bool
	build   func(Source) http.Handler
	// portal is the portal entry the route carries, or nil.
	portal *web.PortalService
}

// declaredRoutes is every route of the interface. A family adds its own from
// its register file through mustRegisterRoute, and nothing appends to it
// after init, so it is read without a lock.
var declaredRoutes []fleetRoute

// mustRegisterRoute adds route to the interface and registers it with the web
// route registry, enabled while the build's slot holds a source, or a source
// that can be stepped for a stepped route. It MUST be called from init().
func mustRegisterRoute(route fleetRoute) {
	declaredRoutes = append(declaredRoutes, route)

	enabled := sourceRegistered
	if route.stepped {
		enabled = stepperRegistered
	}

	web.RegisterWebRoute(web.WebRoute{
		Pattern: route.pattern,
		Wrap:    route.wrap,
		Enabled: enabled,
		Build:   func(web.RouteDeps) http.Handler { return route.build(mountedSource()) },
		Portal:  route.portal,
	})
}

// NewRoutes answers every fleet route over source on one handler: the pages,
// the assets, the fragments, the event stream, the actions, the console and
// the audit export, and the two prototype-only routes when source implements
// Stepper. Every request carries account as its account name, as
// web.InsecureMiddleware gives the name "insecure", and no request is
// authenticated or checked for its origin. It is for a caller that serves the
// interface in process, a recording of the pages or a test; the daemon mounts
// the same routes through the web route registry, under its own wraps.
// source MUST NOT be nil.
func NewRoutes(source Source, account string) http.Handler {
	if source == nil {
		panic("BUG: fleetui: NewRoutes was given no data source")
	}

	_, steps := source.(Stepper)
	mux := http.NewServeMux()

	for _, route := range declaredRoutes {
		if route.stepped && !steps {
			continue
		}
		mux.Handle(route.pattern, route.build(source))
	}

	return web.AccountMiddleware(account, mux)
}

func init() {
	registerShellFragments()

	mustRegisterRoute(fleetRoute{
		pattern: assetsPattern,
		wrap:    web.WrapAuth,
		build:   func(Source) http.Handler { return newAssetHandler() },
	})
	mustRegisterRoute(fleetRoute{
		pattern: fragmentPattern,
		wrap:    web.WrapAuth,
		build:   func(source Source) http.Handler { return http.HandlerFunc(newFleetHandler(source).serveFragment) },
	})
	mustRegisterRoute(fleetRoute{
		pattern: eventsPattern,
		wrap:    web.WrapAuth,
		build:   func(source Source) http.Handler { return newEventStream(source) },
	})
	mustRegisterRoute(fleetRoute{
		pattern: actionPattern,
		wrap:    web.WrapMutation,
		build:   func(source Source) http.Handler { return newActionHandler(source) },
	})
	mustRegisterRoute(fleetRoute{
		pattern: auditExportPattern,
		wrap:    web.WrapAuth,
		build:   newAuditExportHandler,
	})
	mustRegisterRoute(fleetRoute{
		pattern: stepPattern,
		wrap:    web.WrapMutation,
		stepped: true,
		build:   func(source Source) http.Handler { return newStepHandler(stepperOf(source)) },
	})
	mustRegisterRoute(fleetRoute{
		pattern: holdPattern,
		wrap:    web.WrapMutation,
		stepped: true,
		build:   func(source Source) http.Handler { return newHoldHandler(stepperOf(source)) },
	})
	mustRegisterRoute(fleetRoute{
		pattern: shellPattern,
		wrap:    web.WrapAuth,
		build:   func(source Source) http.Handler { return newFleetHandler(source) },
		// Direct, because the fleet interface has its own shell and the web
		// server forbids framing a page.
		portal: &web.PortalService{Key: "fleet", Title: "Fleet", Path: fleetPath, Direct: true},
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

// stepperOf answers source as a Stepper for a prototype-only route. Both
// mounting paths build such a route only over a source they found to be one,
// stepperRegistered for the hub and NewRoutes for an in-process caller.
func stepperOf(source Source) Stepper {
	stepper, steps := source.(Stepper)
	if !steps {
		panic("BUG: fleetui: a prototype-only route was built over a source that cannot be stepped")
	}

	return stepper
}
