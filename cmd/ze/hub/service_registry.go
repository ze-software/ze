// Design: ai/rules/plugins.md -- compile-out-able services (feature-gate)
//
// Construction registry for optional, compile-out-able daemon services. A
// feature registers a serviceFactory from an init() guarded by its
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

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/command"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/component/resolve"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// Service is one optional daemon service built through the construction
// registry. It reuses the existing Reconfigurable listener-migration contract
// (so the listenerMigrator can drive it unchanged) and adds a name (for routing
// to the right migrator slot) and shutdown.
type Service interface {
	Reconfigurable // Addresses() []string; Reconfigure(ctx, []string) error
	Name() string
	Shutdown(ctx context.Context) error
}

// serviceDeps carries the generic runtime dependencies a factory may need. No
// field is service-specific in TYPE; per-service resolved bindings (e.g. the
// looking-glass listen addresses) are plain values resolved by the always-on
// hub and handed in. The struct grows as services are converted; it never
// imports a service package.
type serviceDeps struct {
	Store      storage.Storage
	ConfigPath string
	Resolvers  *resolve.Resolvers
	// Dispatch is the unified command dispatcher (plugin.CommandDispatcher). It
	// stays generic infrastructure -- the plugin package, not a service package
	// -- so the always-on registry still names no service type. A service adapts
	// it (rendering typed *plugin.Response at its edge via .JSON) internally.
	Dispatch plugin.CommandDispatcher

	// Looking-glass resolved binding (pilot). LGToken is a plain string (no lg
	// import): empty leaves the looking glass open, which is its default
	// posture as a public read-only surface.
	LGAddrs []string
	LGTLS   bool
	// LGTLSExplicit reports that the operator wrote the tls setting, rather than
	// inheriting the default-on. It decides what a missing certificate store
	// means: a demand that cannot be met (error), or a default that yields to
	// the prior plaintext behavior with a warning.
	LGTLSExplicit bool
	LGToken       string

	// Web resolved bindings. These stay generic: no internal/component/web type
	// crosses the always-on registry boundary.
	WebEnabled  bool
	WebAddrs    []string
	InsecureWeb bool
	// WebCertificate names an entry in the PKI store to serve on HTTPS. Empty
	// selects the self-signed certificate. A plain string (no pki type crosses
	// this boundary); the factory resolves it through pki.ServerTLSMaterial.
	WebCertificate string
	Authorizer     aaa.Authorizer
	Recorder       audit.Recorder
	CommitHook     func() error
	// PowerUsers is the zefs break-glass account, read once by the hub. Those
	// credentials live in the blob store, so no reload changes them and a
	// snapshot is correct. It reaches the factory rather than being re-read
	// there because the hub already merged it into LocalUsersLive below: two
	// reads of one database can disagree, and a factory that granted a login
	// from its own read and then revoked it against the hub's would lock the
	// break-glass account out of the surface it exists to recover. A factory
	// uses it to NAME those accounts, never to decide who may log in.
	PowerUsers []authz.UserConfig
	// LocalUsersLive returns the local credentials the daemon accepts RIGHT NOW:
	// PowerUsers merged with the users the RUNNING config declares, read per call
	// rather than snapshotted. A reload can delete a user, and a snapshot would
	// keep letting them in until the daemon restarted. An error means the
	// running config could not be read, and the caller MUST deny rather than
	// fall back to a snapshot.
	//
	// It answers the serve-or-not question too, asked once at construction:
	// "does this configuration authenticate anybody". One source answers both,
	// so a surface cannot decide to serve on a user list it will not then admit.
	//
	// It is the SAME closure the AAA chain's local backend answers from
	// (liveLocalUsers in main.go), which is what stops a session and a password
	// disagreeing about who exists.
	LocalUsersLive    func() ([]authz.UserConfig, error)
	EventRing         *pluginserver.EventRing
	WebPortalServices []webPortalService
	// WebCommands sources plugin-registered commands (Hidden excluded) for the
	// web terminal's tab-completion. A lazy func because the web service is built
	// before plugins finish registering; the web factory resolves it on first
	// completion request. Generic type (command, not a web type) so the always-on
	// registry boundary is preserved. nil leaves web completion YANG-only.
	WebCommands func() []command.CommandEntry

	// MCP resolved bindings + command source (all generic types). Consumed only
	// by the ze_mcp-gated factory (service_mcp.go); populated always-on so a
	// no-mcp build neither names a zemcp type nor leaves an unused local. A
	// pointer so serviceDeps stays small (the by-value struct trips hugeParam,
	// learned 981); a nil MCP is the not-configured skip.
	MCP *mcpServiceDeps
}

// mcpServiceDeps carries everything the MCP factory needs, all generic-typed so
// no internal/component/mcp type crosses the always-on registry boundary. The
// always-on hub resolves these (listen addrs, token, the YANG MCPListenConfig,
// the MCP-surface dispatcher, the neutral command metadata source, and the
// audit recorder); the gated factory converts them into zemcp types.
type mcpServiceDeps struct {
	Addrs    []string
	Token    string
	Config   zeconfig.MCPListenConfig
	ConfigOK bool
	Dispatch plugin.CommandDispatcher
	Commands func() []commandMeta
	Recorder audit.Recorder
}

// serviceFactory builds (and starts) one service from deps. It returns a nil
// Service when the service is not configured/enabled -- that is NOT an error.
// A non-nil error means the build failed unexpectedly; the hub logs and skips.
type serviceFactory func(deps serviceDeps) (Service, error)

type serviceMigratorWire func(*listenerMigrator, Service)

type namedFactory struct {
	name         string
	factory      serviceFactory
	wireMigrator serviceMigratorWire
}

type builtService struct {
	Service
	wireMigrator serviceMigratorWire
}

// serviceFactories is appended to by registerService from build-tag-gated
// init() functions. Order is registration order (deterministic per build).
var serviceFactories []namedFactory

// registerService records a factory under name. Called from a
// //go:build ze_<feature> init(); absent that tag, the call is not compiled.
// The registration owns any listener-migrator wiring, so the always-on registry
// does not grow a switch over every optional service.
func registerService(name string, f serviceFactory, wireMigrator serviceMigratorWire) {
	serviceFactories = append(serviceFactories, namedFactory{name: name, factory: f, wireMigrator: wireMigrator})
}

// buildServices builds every registered service. Factories returning a nil
// Service (not configured) are skipped; build errors are logged and skipped,
// matching the prior best-effort service startup.
func buildServices(deps serviceDeps) []builtService {
	logger := slogutil.Logger("hub.services")
	var built []builtService
	for _, nf := range serviceFactories {
		svc, err := nf.factory(deps)
		if err != nil {
			logger.Warn("service build failed", "service", nf.name, "error", err)
			continue
		}
		if svc == nil {
			continue
		}
		built = append(built, builtService{Service: svc, wireMigrator: nf.wireMigrator})
	}
	return built
}

// registerBuiltService wires a built service into the listenerMigrator through
// the hook supplied by that service's build-tag-gated registration.
func registerBuiltService(lm *listenerMigrator, svc builtService) {
	if svc.wireMigrator != nil {
		svc.wireMigrator(lm, svc.Service)
	}
}
