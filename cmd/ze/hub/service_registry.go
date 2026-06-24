// Design: docs/architecture/cli/plugin-modes.md -- compile-out-able services (feature-gate)
//
// Construction registry for optional, compile-out-able daemon services. A
// feature registers a ServiceFactory from an init() guarded by its
// //go:build ze_<feature> tag; the hub iterates the registry at startup and
// never imports the service package directly from always-on code. With a
// feature's tag off, its registration file is not compiled, the factory is not
// registered, and the service package is linked nowhere always-on -- so the Go
// linker drops it (smaller binary, smaller attack surface).
//
// The looking-glass factory (the pilot) lives in a //go:build ze_lg file.
// See plan/spec-feature-gate-0-umbrella.md for the umbrella design and
// plan/spec-feature-gate-1-lg.md for this pilot.

package hub

import (
	"context"

	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// Service is one optional daemon service built through the construction
// registry. It reuses the existing Reconfigurable listener-migration contract
// (so the ListenerMigrator can drive it unchanged) and adds a name (for routing
// to the right migrator slot) and shutdown.
type Service interface {
	Reconfigurable // Addresses() []string; Reconfigure(ctx, []string) error
	Name() string
	Shutdown(ctx context.Context) error
}

// ServiceDeps carries the generic runtime dependencies a factory may need. No
// field is service-specific in TYPE; per-service resolved bindings (e.g. the
// looking-glass listen addresses) are plain values resolved by the always-on
// hub and handed in. The struct grows as services are converted; it never
// imports a service package.
type ServiceDeps struct {
	Store     storage.Storage
	Resolvers *resolve.Resolvers
	// Dispatch is the generic command surface (server dispatcher); a service
	// adapts it to its own narrower dispatcher type internally.
	Dispatch func(command, username, remoteAddr string) (string, error)

	// Looking-glass resolved binding (pilot). Other services add their own
	// resolved-binding fields here as they are converted.
	LGAddrs []string
	LGTLS   bool
}

// ServiceFactory builds (and starts) one service from deps. It returns a nil
// Service when the service is not configured/enabled -- that is NOT an error.
// A non-nil error means the build failed unexpectedly; the hub logs and skips.
type ServiceFactory func(deps ServiceDeps) (Service, error)

type namedFactory struct {
	name    string
	factory ServiceFactory
}

// serviceFactories is appended to by registerService from build-tag-gated
// init() functions. Order is registration order (deterministic per build).
var serviceFactories []namedFactory

// registerService records a factory under name. Called from a
// //go:build ze_<feature> init(); absent that tag, the call is not compiled.
func registerService(name string, f ServiceFactory) {
	serviceFactories = append(serviceFactories, namedFactory{name: name, factory: f})
}

// buildServices builds every registered service. Factories returning a nil
// Service (not configured) are skipped; build errors are logged and skipped,
// matching the prior best-effort service startup.
func buildServices(deps ServiceDeps) []Service {
	logger := slogutil.Logger("hub.services")
	var built []Service
	for _, nf := range serviceFactories {
		svc, err := nf.factory(deps)
		if err != nil {
			logger.Warn("service build failed", "service", nf.name, "error", err)
			continue
		}
		if svc == nil {
			continue
		}
		built = append(built, svc)
	}
	return built
}

// registerBuiltService wires a built service into the ListenerMigrator so it
// participates in graceful listener migration on config reload. As services are
// converted to the registry, add a case here.
func registerBuiltService(lm *ListenerMigrator, svc Service) {
	// As services are converted to the registry, route each to its migrator
	// slot. Pilot: looking-glass only.
	if svc.Name() == "looking-glass" {
		lm.SetLG(svc)
	}
}
