// Design: docs/architecture/web-interface.md -- Graceful listener migration on config reload
// Related: main_reload.go -- doReload calls ListenerMigrator.ReloadListeners

package hub

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	zepki "github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// The migrator's name for each management surface. The boot classification
// (markMgmtAuth), the reloaders (registerMgmtAuthReloaders), the change set and
// the handle lookup all key on these, so one spelling per surface is what keeps
// a surface inside the reload guard.
const (
	svcWeb  = "web"
	svcLG   = "lg"
	svcMCP  = "mcp"
	svcREST = "rest"
	svcGRPC = "grpc"
)

// Reconfigurable is implemented by any server that supports live listener migration.
type Reconfigurable interface {
	Addresses() []string
	Reconfigure(ctx context.Context, newAddrs []string) error
}

// TLSUpdatable is implemented by any server that can replace its serving
// certificate without rebinding. Declared here, alongside Reconfigurable and for
// the same reason: always-on hub code must be able to rotate the web server's
// certificate without importing the compile-out-able web package
// (//go:build ze_web).
type TLSUpdatable interface {
	UpdateTLSCertificate(certPEM, keyPEM []byte) error
}

// AuthUpdatable is implemented by any management server that can replace the
// credentials gating its requests without rebinding its listeners. Declared
// here for the same reason as TLSUpdatable: always-on hub code must rebuild a
// server's authentication on reload without importing the compile-out-able
// package that constructs it.
//
// A server that implements it answers for its LIVE authentication, which is
// what lets the reload guard classify what a server serves rather than the mode
// it was built with. A server that does not implement it keeps the
// authentication it was constructed with until the daemon restarts, and
// ReloadListeners refuses that service's listener change instead of applying an
// address under authentication the reloaded config no longer describes.
type AuthUpdatable interface {
	// Authenticated reports whether every request is gated right now.
	Authenticated() bool
	// UpdateAuth installs reloaded credentials and returns a function that puts
	// the previous ones back. Restoring a value the server already held cannot
	// fail, so the returned function reports nothing.
	UpdateAuth(token string, authenticator func(authHeader string) (username string, ok bool)) (restore func(), err error)
}

// authIntent is the authentication a reloaded config asks one management
// service to serve. token and authenticator are the material an AuthUpdatable
// server installs; authenticated is what the exposure guard classifies, and it
// is the only field a service whose authentication is fixed at construction
// uses.
type authIntent struct {
	authenticated bool
	token         string
	authenticator func(authHeader string) (username string, ok bool)
}

// authReloader resolves one service's authIntent from a reloaded config tree.
// It is registered by the boot code that owns that service's flag, environment,
// and config precedence, so the precedence has exactly one implementation.
//
// ok reports whether the reloaded config describes the service at all. A false
// ok with a nil error means the config is silent, and the service keeps the
// authentication it is serving: removing a config block must never strip the
// credentials off a server that is still listening. A non-nil error means the
// authentication could not be determined, and the reload fails closed.
type authReloader func(tree *zeconfig.Tree) (intent authIntent, ok bool, err error)

// serviceChange describes a single service's listener migration.
type serviceChange struct {
	name    string
	server  Reconfigurable
	oldAddr []string
	newAddr []string
	add     []string
	remove  []string
}

// ListenerMigrator coordinates listener reconfiguration across services on
// config reload. It detects cross-service address conflicts and sequences
// migrations to minimize downtime.
type ListenerMigrator struct {
	web    Reconfigurable
	webTLS TLSUpdatable
	lg     Reconfigurable
	mcp    Reconfigurable
	rest   Reconfigurable
	grpc   Reconfigurable
	logger *slog.Logger

	// authAtBoot records, per service name, whether the boot guard
	// (mgmt_guard.go) classified that surface as gating every request. It is
	// the reload guard's source for a service whose authentication is fixed at
	// construction. A service absent from the map is one the boot guard never
	// classified -- the looking glass, an intentionally public surface -- and
	// the reload guard leaves it alone exactly as the boot guard does.
	//
	// A service that implements AuthUpdatable is asked directly instead, because
	// a reload changes its answer and this record would go stale.
	authAtBoot map[string]bool

	// authUpdaters holds the services that can rebuild their authentication in
	// place. Populated from the Set* methods by type assertion.
	authUpdaters map[string]AuthUpdatable

	// authReloaders resolves each service's configured authentication from a
	// reloaded config tree.
	authReloaders map[string]authReloader
}

// MarkAuthenticated records that the boot guard classified the named service as
// gating every request. Recording the positive matters as much as the negative:
// a service the reload guard has no record for is outside the guard's scope,
// not inside it and authenticated.
func (m *ListenerMigrator) MarkAuthenticated(name string) { m.markAuth(name, true) }

// MarkUnauthenticated records that the named service was built without
// authentication, so ReloadListeners refuses any migration that would move it
// to a non-loopback address (AC-7 of the management-listener guard).
func (m *ListenerMigrator) MarkUnauthenticated(name string) { m.markAuth(name, false) }

func (m *ListenerMigrator) markAuth(name string, authenticated bool) {
	if m.authAtBoot == nil {
		m.authAtBoot = make(map[string]bool)
	}
	m.authAtBoot[name] = authenticated
}

// hasService reports whether a server handle was built for the named surface.
// It is what resolveAuthIntents asks before resolving one: a surface with no
// handle is a server the daemon never started, and producing an intent for it
// makes checkAuthRebuildable refuse a whole reload over a config edit no
// running server has to follow.
func (m *ListenerMigrator) hasService(name string) bool {
	switch name {
	case svcWeb:
		return m.web != nil
	case svcLG:
		return m.lg != nil
	case svcMCP:
		return m.mcp != nil
	case svcREST:
		return m.rest != nil
	case svcGRPC:
		return m.grpc != nil
	default:
		return false
	}
}

// SetAuthReloader registers how the named service's configured authentication
// is resolved from a reloaded config tree.
func (m *ListenerMigrator) SetAuthReloader(name string, r authReloader) {
	if m.authReloaders == nil {
		m.authReloaders = make(map[string]authReloader)
	}
	m.authReloaders[name] = r
}

// registerAuthUpdater records a server that can rebuild its authentication in
// place. A server that does not implement AuthUpdatable is deliberately not
// recorded: ReloadListeners then reads the boot guard's classification for it,
// which stays correct precisely because such a server's authentication cannot
// change while it runs.
func (m *ListenerMigrator) registerAuthUpdater(name string, srv Reconfigurable) {
	au, ok := srv.(AuthUpdatable)
	if !ok {
		delete(m.authUpdaters, name)
		return
	}
	if m.authUpdaters == nil {
		m.authUpdaters = make(map[string]AuthUpdatable)
	}
	m.authUpdaters[name] = au
}

// runningAuth reports whether the named service gates every request right now,
// and whether the guard has any record of it. A server that can rebuild its
// authentication is asked directly; every other service answers from the boot
// guard's record.
//
// A false known means the service is outside the exposure guard's scope, not
// that it is unauthenticated. The looking glass is the case: it is an
// intentionally public surface that the boot guard never declares, and the
// reload guard must not start refusing its migrations.
func (m *ListenerMigrator) runningAuth(name string) (authenticated, known bool) {
	if au, ok := m.authUpdaters[name]; ok && au != nil {
		return au.Authenticated(), true
	}
	v, ok := m.authAtBoot[name]
	return v, ok
}

// NewListenerMigrator creates a migrator. Pass nil for services that are not running.
func NewListenerMigrator(web Reconfigurable) *ListenerMigrator {
	return &ListenerMigrator{
		web:    web,
		logger: slogutil.Logger("hub.listener"),
	}
}

// SetWeb updates the web server reference.
func (m *ListenerMigrator) SetWeb(web Reconfigurable) {
	m.web = web
	m.registerAuthUpdater(svcWeb, web)
}

// SetWebTLS updates the web server's certificate-rotation reference. Takes
// TLSUpdatable (not *web.WebServer) so always-on code never imports the web
// package, the same reason SetLG takes Reconfigurable.
func (m *ListenerMigrator) SetWebTLS(s TLSUpdatable) { m.webTLS = s }

// UpdateWebCertificate re-resolves certName in the PKI store and installs the
// resulting chain on the running web server, without rebinding its listeners.
// Called on reload AFTER the new store is installed, so a commit that rotates
// the certificate's material takes effect on the next handshake (AC-9).
//
// An empty certName means the operator configured no reference: the self-signed
// certificate keeps serving and nothing is touched. A non-empty name that no
// longer resolves is an ERROR that fails the reload, never a silent downgrade to
// the self-signed certificate the listener may still be holding (R-5).
func (m *ListenerMigrator) UpdateWebCertificate(certName string) error {
	if m.webTLS == nil || certName == "" {
		return nil
	}
	certPEM, keyPEM, err := zepki.ServerTLSMaterial(certName)
	if err != nil {
		return fmt.Errorf("web tls certificate %q: %w", certName, err)
	}
	if err := m.webTLS.UpdateTLSCertificate(certPEM, keyPEM); err != nil {
		return fmt.Errorf("web tls certificate %q: %w", certName, err)
	}
	return nil
}

// SetLG updates the looking glass server reference. Takes Reconfigurable (not
// *lg.LGServer) so always-on code never imports the lg package: lg is built
// through the construction registry and may be compiled out (//go:build ze_lg).
//
// The registerAuthUpdater call below is a live dependency, not boilerplate. No
// looking-glass type implements AuthUpdatable today, so runningAuth("lg")
// answers known=false and the reload guard leaves the looking glass alone,
// which is what registerMgmtAuthReloaders documents. The day an LG type gains
// UpdateAuth, this line makes runningAuth answer known=true from the live
// server, and a looking glass with no token starts having its non-loopback
// migrations REFUSED -- the opposite of the intentionally public posture.
// Whoever implements AuthUpdatable on lg owns that decision here.
func (m *ListenerMigrator) SetLG(s Reconfigurable) {
	m.lg = s
	m.registerAuthUpdater(svcLG, s)
}

// SetMCP updates the MCP server reference. Takes Reconfigurable (not
// *mcpServerHandle) so always-on code never imports the mcp package: mcp is
// built through the construction registry and may be compiled out
// (//go:build ze_mcp).
func (m *ListenerMigrator) SetMCP(s Reconfigurable) {
	m.mcp = s
	m.registerAuthUpdater(svcMCP, s)
}

// SetREST updates the REST API server reference. Takes Reconfigurable (not
// *rest.RESTServer) so always-on code never imports the api/rest package: the
// API servers are built through the ze_api seam and may be compiled out.
func (m *ListenerMigrator) SetREST(s Reconfigurable) {
	m.rest = s
	m.registerAuthUpdater(svcREST, s)
}

// SetGRPC updates the gRPC API server reference. Takes Reconfigurable (see
// SetREST) so always-on code never imports the api/grpc package.
func (m *ListenerMigrator) SetGRPC(s Reconfigurable) {
	m.grpc = s
	m.registerAuthUpdater(svcGRPC, s)
}

// noAuthRestore is the undo returned when a reload installed no credentials.
func noAuthRestore() {}

// resolvedAuth pairs a service with the authentication its reloaded config asks
// for.
type resolvedAuth struct {
	name   string
	intent authIntent
}

// ReloadListeners applies a reloaded config to the running management services:
// it rebuilds the authentication of every server that can take a new one,
// re-runs the exposure guard over the (address, authentication) pair the reload
// produces, and then migrates listen addresses.
//
// It returns an undo for the credentials it installed. The caller MUST run that
// undo if any LATER step of the reload fails, because a reload the operator is
// told failed must not leave a listener authenticated differently from the
// config the daemon rolled back to (runReload, main_reload.go). On its own
// error paths ReloadListeners has already undone everything itself, and the
// returned undo does nothing.
//
// Order matters, and refusals come first. A service that must change its
// authentication and cannot is refused BEFORE anything is applied, so the
// daemon keeps every listener and every credential it has. Authentication is
// then rebuilt BEFORE any address moves, so the exposure guard classifies the
// pair the reload produces rather than the one the servers were constructed
// with.
func (m *ListenerMigrator) ReloadListeners(ctx context.Context, tree *zeconfig.Tree) (func(), error) {
	changes := m.buildChanges(tree)

	intents, err := m.resolveAuthIntents(tree)
	if err != nil {
		return noAuthRestore, err
	}

	if err := m.checkAuthRebuildable(intents); err != nil {
		return noAuthRestore, err
	}

	restoreAuth, err := m.applyAuthIntents(intents)
	if err != nil {
		return noAuthRestore, err
	}

	if err := m.checkReloadExposure(changes); err != nil {
		restoreAuth()
		return noAuthRestore, err
	}

	if len(changes) > 0 {
		if err := m.migrateListeners(ctx, changes, restoreAuth); err != nil {
			return noAuthRestore, err
		}
	}

	return restoreAuth, nil
}

// resolveAuthIntents asks each registered reloader what the reloaded config
// wants, in a stable order, and reads nothing into the running servers.
//
// A service with no running handle, and a service the exposure guard never
// classified, are both skipped, and their reloaders are never CALLED. That is
// the load-bearing half: registerMgmtAuthReloaders registers web, mcp, rest and
// grpc unconditionally, so on a binary built without one of them a reloader
// that fails -- apiAuthReloader refuses when the power-user credentials stop
// being readable -- would fail every reload over a service that is not in the
// binary. Skipping the CHECK alone would not prevent that; skipping the call
// does.
//
// The handle test is also what lets markMgmtAuth classify every surface at boot
// rather than only the built ones. One `api-server` block answers for REST and
// gRPC together, so a config enabling REST alone classifies a gRPC server the
// daemon never started; dropping it here means no intent exists for it and
// checkAuthRebuildable can never refuse a reload over it (AC-5). Doing it here
// rather than in checkAuthRebuildable is what makes the boot ordering
// irrelevant, and the reloader call is skipped as well as the check.
//
// The classification test below reaches no service today, and it is a guard
// rather than dead weight. registerMgmtAuthReloaders registers web, mcp, rest
// and grpc, and markMgmtAuth classifies those same four names, so every name
// this loop visits already has a record. The looking glass never enters the
// loop at all: it gets no reloader, so its absent record is never asked for.
//
// Registering a reloader for a name markMgmtAuth does not classify makes the
// branch live again. It then skips that name, which keeps a surface outside the
// exposure guard's scope from producing an intent checkAuthRebuildable could
// refuse a whole reload over.
func (m *ListenerMigrator) resolveAuthIntents(tree *zeconfig.Tree) ([]resolvedAuth, error) {
	names := make([]string, 0, len(m.authReloaders))
	for name := range m.authReloaders {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]resolvedAuth, 0, len(names))
	for _, name := range names {
		if !m.hasService(name) {
			continue
		}
		if _, known := m.runningAuth(name); !known {
			continue
		}
		intent, ok, err := m.authReloaders[name](tree)
		if err != nil {
			// The configured authentication could not be determined. Refusing
			// is the only safe answer: treating an unresolved mode as "no
			// authentication" is the permissive no-op a guard must never make
			// (ai/rules/evidence.md).
			return nil, fmt.Errorf("resolve %s authentication: %w", name, err)
		}
		if !ok {
			// The reloaded config does not describe this service. It keeps the
			// authentication it is serving; a deleted config block must not
			// strip the credentials off a server that is still listening.
			continue
		}
		out = append(out, resolvedAuth{name: name, intent: intent})
	}
	return out, nil
}

// checkAuthRebuildable refuses the reload when a service must change its
// authentication and cannot, before anything has been applied.
//
// The refusal has to reach the operator's exit status, not just a log line.
// runReload discards the candidate config on any error and promotes it only on
// success, so returning an error here is what stops `ze config commit`
// reporting success over a web server that is still serving unauthenticated.
//
// Refusing the whole reload is deliberate, and narrower than it looks. The
// alternative -- migrate the other services and report the refusal afterwards
// -- would leave those listeners on addresses the rolled-back config no longer
// describes, and runReload's rollback has no way to move them back.
//
// It tests no handle of its own: resolveAuthIntents produces an intent only for
// a service that has one, so a surface the daemon never built never reaches
// here (AC-5).
func (m *ListenerMigrator) checkAuthRebuildable(intents []resolvedAuth) error {
	for _, ra := range intents {
		if au, ok := m.authUpdaters[ra.name]; ok && au != nil {
			continue
		}
		running, known := m.runningAuth(ra.name)
		if !known || running == ra.intent.authenticated {
			continue
		}
		return fmt.Errorf("%s cannot change its authentication while running: it is serving %s and the config asks for %s; restart ze to apply it",
			ra.name, authWord(running), authWord(ra.intent.authenticated))
	}
	return nil
}

// authWord names an authentication state for an operator-facing message.
func authWord(authenticated bool) string {
	if authenticated {
		return "authenticated"
	}
	return "unauthenticated"
}

// applyAuthIntents installs each resolved intent on the server that can take
// it, and returns a function restoring every server it changed.
func (m *ListenerMigrator) applyAuthIntents(intents []resolvedAuth) (func(), error) {
	var restores []func()
	restoreAll := func() {
		for i := len(restores) - 1; i >= 0; i-- {
			restores[i]()
		}
	}

	for _, ra := range intents {
		au, updatable := m.authUpdaters[ra.name]
		if !updatable || au == nil {
			// checkAuthRebuildable already refused any such service whose mode
			// must change, so reaching here means its mode is already correct.
			continue
		}
		// Always applied, never only on a changed mode: a rotated token or an
		// edited user list changes the material while the mode stays the same,
		// and the reloaded config is what the server must serve.
		restore, err := au.UpdateAuth(ra.intent.token, ra.intent.authenticator)
		if err != nil {
			restoreAll()
			return nil, fmt.Errorf("rebuild %s authentication: %w", ra.name, err)
		}
		restores = append(restores, restore)
	}

	return restoreAll, nil
}

// checkReloadExposure re-runs the boot guard's AUTHENTICATION classification
// over the (address, authentication) pair each service holds once this reload
// applies. It refuses before anything is rebound, so the daemon keeps every
// listener it has.
//
// It reads the LIVE authentication, so a server whose authentication this
// reload just rebuilt is classified on the new mode. That is the whole point:
// the boot-time record would still call such a server authenticated because
// that is what it was when it started.
//
// An unknown service is SKIPPED, which is the permissive branch, and it is
// correct only for a surface the boot guard genuinely never classified (the
// looking glass). So markMgmtAuth runs before any handle reaches the migrator:
// a classified surface whose record has not landed yet would take this branch
// and migrate unauthenticated to a public address, and the web server carries
// no loopback rule of its own to catch it.
//
// Authentication is all it judges, and that is deliberate rather than
// complete. A transport requirement belongs to the server that knows its own
// transport: gRPC refuses a non-loopback address without TLS in
// checkGRPCListenAddr, on the same reload, and this function cannot see a
// certificate. Do not read a pass here as "the listener is safe to expose".
func (m *ListenerMigrator) checkReloadExposure(changes []serviceChange) error {
	for i := range changes {
		authenticated, known := m.runningAuth(changes[i].name)
		if !known || authenticated {
			continue
		}
		for _, addr := range changes[i].add {
			if listenAddrIsNonLoopback(addr) {
				return fmt.Errorf("refusing to migrate %s to non-loopback listener %q without authentication", changes[i].name, addr)
			}
		}
	}
	return nil
}

// buildChanges collects the per-service listen-address change set the reloaded
// tree asks for. A service the tree does not describe keeps its listeners.
func (m *ListenerMigrator) buildChanges(tree *zeconfig.Tree) []serviceChange {
	var changes []serviceChange

	if m.web != nil {
		if webCfg, ok := zeconfig.ExtractWebConfig(tree); ok {
			if sc, ok := m.buildChange(svcWeb, m.web, endpointsToAddrs(webCfg.Servers)); ok {
				changes = append(changes, sc)
			}
		}
	}

	if m.lg != nil {
		if lgCfg, ok := zeconfig.ExtractLGConfig(tree); ok {
			if sc, ok := m.buildChange(svcLG, m.lg, endpointsToAddrs(lgCfg.Servers)); ok {
				changes = append(changes, sc)
			}
		}
	}

	if m.mcp != nil {
		if mcpCfg, ok := zeconfig.ExtractMCPConfig(tree); ok {
			if sc, ok := m.buildChange(svcMCP, m.mcp, endpointsToAddrs(mcpCfg.Servers)); ok {
				changes = append(changes, sc)
			}
		}
	}

	if m.rest != nil || m.grpc != nil {
		if apiCfg, ok := zeconfig.ExtractAPIConfig(tree); ok {
			if m.rest != nil && apiCfg.RESTOn {
				if sc, ok := m.buildChange(svcREST, m.rest, apiListenToAddrs(apiCfg.REST)); ok {
					changes = append(changes, sc)
				}
			}
			if m.grpc != nil && apiCfg.GRPCOn {
				if sc, ok := m.buildChange(svcGRPC, m.grpc, apiListenToAddrs(apiCfg.GRPC)); ok {
					changes = append(changes, sc)
				}
			}
		}
	}

	return changes
}

// migrateListeners applies an already-guarded change set. restoreAuth puts back
// the authentication every server started the reload with; it runs on any
// failure, alongside the address rollback, so a reload that fails part-way
// leaves neither the addresses nor the authentication half-applied.
func (m *ListenerMigrator) migrateListeners(ctx context.Context, changes []serviceChange, restoreAuth func()) error {
	conflicts := detectConflicts(changes)

	ordered := make([]serviceChange, 0, len(changes))
	for i := range changes {
		if !conflicts[changes[i].name] {
			ordered = append(ordered, changes[i])
		}
	}
	for i := range changes {
		if conflicts[changes[i].name] {
			ordered = append(ordered, changes[i])
		}
	}

	var applied []serviceChange
	for i := range ordered {
		change := ordered[i]
		label := change.name
		if conflicts[change.name] {
			label += " (conflicting)"
			m.logger.Warn("sequenced listener migration (brief gap expected)",
				"service", change.name,
				"add", change.add, "remove", change.remove)
		} else {
			m.logger.Info("reconfiguring listeners", "service", change.name,
				"add", change.add, "remove", change.remove)
		}
		if err := change.server.Reconfigure(ctx, change.newAddr); err != nil {
			rollbackErr := m.rollbackAppliedListeners(ctx, applied)
			restoreAuth()
			if rollbackErr != nil {
				return fmt.Errorf("reconfigure %s: %w (listener rollback failed: %w)", label, err, rollbackErr)
			}
			return fmt.Errorf("reconfigure %s: %w", label, err)
		}
		applied = append(applied, change)
	}

	return nil
}

func (m *ListenerMigrator) rollbackAppliedListeners(ctx context.Context, applied []serviceChange) error {
	var rollbackErrs []error
	for i := len(applied) - 1; i >= 0; i-- {
		change := applied[i]
		m.logger.Warn("rolling back listener migration", "service", change.name, "addr", change.oldAddr)
		if err := change.server.Reconfigure(ctx, change.oldAddr); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("rollback %s: %w", change.name, err))
		}
	}
	return errors.Join(rollbackErrs...)
}

func (m *ListenerMigrator) buildChange(name string, srv Reconfigurable, newAddrs []string) (serviceChange, bool) {
	oldAddrs := srv.Addresses()
	_, add, remove := listenerDiff(oldAddrs, newAddrs)
	if len(add) == 0 && len(remove) == 0 {
		return serviceChange{}, false
	}
	return serviceChange{
		name:    name,
		server:  srv,
		oldAddr: oldAddrs,
		newAddr: newAddrs,
		add:     add,
		remove:  remove,
	}, true
}

func listenerDiff(oldAddrs, newAddrs []string) (keep, add, remove []string) {
	oldSet := make(map[string]struct{}, len(oldAddrs))
	for _, a := range oldAddrs {
		oldSet[a] = struct{}{}
	}
	newSet := make(map[string]struct{}, len(newAddrs))
	for _, a := range newAddrs {
		newSet[a] = struct{}{}
	}
	for _, a := range newAddrs {
		if _, exists := oldSet[a]; exists {
			keep = append(keep, a)
		} else {
			add = append(add, a)
		}
	}
	for _, a := range oldAddrs {
		if _, exists := newSet[a]; !exists {
			remove = append(remove, a)
		}
	}
	return keep, add, remove
}

func apiListenToAddrs(configs []zeconfig.APIListenConfig) []string {
	out := make([]string, 0, len(configs))
	for _, c := range configs {
		out = append(out, c.Listen())
	}
	return out
}

// detectConflicts returns a set of service names that have address conflicts
// with other services. A conflict occurs when an address in one service's
// "remove" set appears in another service's "add" set.
func detectConflicts(changes []serviceChange) map[string]bool {
	addSets := make(map[string]map[string]bool, len(changes))
	for _, c := range changes {
		s := make(map[string]bool, len(c.add))
		for _, a := range c.add {
			s[a] = true
		}
		addSets[c.name] = s
	}

	conflicted := make(map[string]bool)
	for _, c := range changes {
		for _, removed := range c.remove {
			for otherName, otherAdd := range addSets {
				if otherName == c.name {
					continue
				}
				if otherAdd[removed] {
					conflicted[c.name] = true
					conflicted[otherName] = true
				}
			}
		}
	}
	return conflicted
}
