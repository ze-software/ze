// Design: docs/architecture/web-interface.md -- Graceful listener migration on config reload
// Related: main_reload.go -- doReload calls listenerMigrator.reloadListeners

package hub

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
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

// tlsUpdatable is implemented by any server that can replace its serving
// certificate without rebinding. Declared here, alongside Reconfigurable and for
// the same reason: always-on hub code must be able to rotate the web server's
// certificate without importing the compile-out-able web package
// (//go:build ze_web).
type tlsUpdatable interface {
	UpdateTLSCertificate(certPEM, keyPEM []byte) error
}

// authReporter is implemented by a server whose request gate follows live
// accepted authentication state. The migrator uses it for exposure checks
// without installing candidate credential material in the server.
type authReporter interface {
	Authenticated() bool
}

// authIntent is the authentication mode a reloaded config asks one management
// service to serve after final acceptance. Candidate credential material never
// crosses the listener-migration seam.
type authIntent struct {
	authenticated bool
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

// listenerMigrator coordinates listener reconfiguration across services on
// config reload. It detects cross-service address conflicts and sequences
// migrations to minimize downtime.
type listenerMigrator struct {
	web    Reconfigurable
	webTLS tlsUpdatable
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
	// A service that implements authReporter answers from its live accepted
	// request gate. Other services keep the boot classification.
	authAtBoot map[string]bool

	authReporters map[string]authReporter

	// authReloaders resolves each service's configured authentication from a
	// reloaded config tree.
	authReloaders map[string]authReloader
}

// markAuthenticated records that the boot guard classified the named service as
// gating every request. Recording the positive matters as much as the negative:
// a service the reload guard has no record for is outside the guard's scope,
// not inside it and authenticated.
func (m *listenerMigrator) markAuthenticated(name string) { m.markAuth(name, true) }

// markUnauthenticated records that the named service was built without
// authentication, so reloadListeners refuses any migration that would move it
// to a non-loopback address (AC-7 of the management-listener guard).
func (m *listenerMigrator) markUnauthenticated(name string) { m.markAuth(name, false) }

func (m *listenerMigrator) markAuth(name string, authenticated bool) {
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
func (m *listenerMigrator) hasService(name string) bool {
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

// setAuthReloader registers how the named service's configured authentication
// is resolved from a reloaded config tree.
func (m *listenerMigrator) setAuthReloader(name string, r authReloader) {
	if m.authReloaders == nil {
		m.authReloaders = make(map[string]authReloader)
	}
	m.authReloaders[name] = r
}

// registerAuthReporter records a server whose live request gate can report its
// current accepted authentication mode.
func (m *listenerMigrator) registerAuthReporter(name string, srv Reconfigurable) {
	reporter, ok := srv.(authReporter)
	if !ok {
		delete(m.authReporters, name)
		return
	}
	if m.authReporters == nil {
		m.authReporters = make(map[string]authReporter)
	}
	m.authReporters[name] = reporter
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
func (m *listenerMigrator) runningAuth(name string) (authenticated, known bool) {
	if reporter, ok := m.authReporters[name]; ok && reporter != nil {
		return reporter.Authenticated(), true
	}
	v, ok := m.authAtBoot[name]
	return v, ok
}

// newListenerMigrator creates a migrator with no services attached. Each service
// registers itself later through its setter method as it starts.
//
// It takes no web argument on purpose. Each setter both assigns the reference
// and registers any live authentication reporter.
func newListenerMigrator() *listenerMigrator {
	return &listenerMigrator{
		logger: slogutil.Logger("hub.listener"),
	}
}

// setWeb updates the web server reference.
func (m *listenerMigrator) setWeb(web Reconfigurable) {
	m.web = web
	m.registerAuthReporter(svcWeb, web)
}

// setWebTLS updates the web server's certificate-rotation reference. Takes
// tlsUpdatable (not *web.WebServer) so always-on code never imports the web
// package, the same reason setLG takes Reconfigurable.
func (m *listenerMigrator) setWebTLS(s tlsUpdatable) { m.webTLS = s }

// updateWebCertificate re-resolves certName in the PKI store and installs the
// resulting chain on the running web server, without rebinding its listeners.
// Called on reload AFTER the new store is installed, so a commit that rotates
// the certificate's material takes effect on the next handshake (AC-9).
//
// An empty certName means the operator configured no reference: the self-signed
// certificate keeps serving and nothing is touched. A non-empty name that no
// longer resolves is an ERROR that fails the reload, never a silent downgrade to
// the self-signed certificate the listener may still be holding (R-5).
func (m *listenerMigrator) updateWebCertificate(certName string) error {
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

// setLG updates the looking glass server reference.
func (m *listenerMigrator) setLG(s Reconfigurable) {
	m.lg = s
	m.registerAuthReporter(svcLG, s)
}

// setMCP lives in register_mcp.go, under //go:build ze_mcp. Its only caller is
// that file's registration, so an always-on copy is a method a build without
// ze_mcp can never reach. setWeb and setLG stay here because untagged tests
// call them.

// setREST updates the REST API server reference.
func (m *listenerMigrator) setREST(s Reconfigurable) {
	m.rest = s
	m.registerAuthReporter(svcREST, s)
}

// setGRPC updates the gRPC API server reference.
func (m *listenerMigrator) setGRPC(s Reconfigurable) {
	m.grpc = s
	m.registerAuthReporter(svcGRPC, s)
}

func noListenerRestore() error { return nil }

// resolvedAuth pairs a service with the authentication its reloaded config asks
// for.
type resolvedAuth struct {
	name   string
	intent authIntent
}

// reloadListeners validates candidate authentication modes, re-runs the
// exposure guard over the final (address, mode) pair, and migrates listeners.
// It returns an undo for every listener that may still be on its candidate
// address. This includes a failed migration whose internal rollback was
// incomplete, so the reload transaction can retry restoration before making
// accepted credentials visible again.
func (m *listenerMigrator) reloadListeners(ctx context.Context, tree *zeconfig.Tree) (func() error, error) {
	changes := m.buildChanges(tree)
	intents, err := m.resolveAuthIntents(tree)
	if err != nil {
		return noListenerRestore, err
	}
	if err := m.checkAuthRebuildable(intents); err != nil {
		return noListenerRestore, err
	}
	if err := m.checkReloadExposure(changes, intents); err != nil {
		return noListenerRestore, err
	}
	if len(changes) == 0 {
		return noListenerRestore, nil
	}
	return m.migrateListeners(ctx, changes)
}

// resolveAuthIntents asks each registered reloader what the reloaded config
// wants, in a stable order, and reads nothing into the running servers.
//
// A service with no running handle, and a service the exposure guard never
// classified, are both skipped, and their reloaders are never CALLED. That is
// the load-bearing half: registerMgmtAuthReloaders registers web, mcp, rest and
// grpc unconditionally, so on a binary built without one of them a reloader
// that fails because the live API users cannot be resolved would fail every
// reload over a service that is not in the binary. Skipping the CHECK alone
// would not prevent that; skipping the call does.
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
func (m *listenerMigrator) resolveAuthIntents(tree *zeconfig.Tree) ([]resolvedAuth, error) {
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
func (m *listenerMigrator) checkAuthRebuildable(intents []resolvedAuth) error {
	for _, ra := range intents {
		if reporter, ok := m.authReporters[ra.name]; ok && reporter != nil {
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

// checkReloadExposure evaluates final addresses against candidate modes. A
// service with no candidate auth intent keeps its running mode, which remains
// load-bearing for fixed-auth services such as web.
func (m *listenerMigrator) checkReloadExposure(changes []serviceChange, intents []resolvedAuth) error {
	candidate := make(map[string]bool, len(intents))
	for _, resolved := range intents {
		candidate[resolved.name] = resolved.intent.authenticated
	}

	checked := make(map[string]bool, len(changes))
	for _, change := range changes {
		authenticated, known := m.runningAuth(change.name)
		if value, ok := candidate[change.name]; ok {
			authenticated, known = value, true
		}
		if err := checkServiceExposure(change.name, change.newAddr, authenticated, known); err != nil {
			return err
		}
		checked[change.name] = true
	}

	for _, resolved := range intents {
		if checked[resolved.name] {
			continue
		}
		if err := checkServiceExposure(resolved.name, m.serviceAddresses(resolved.name),
			resolved.intent.authenticated, true); err != nil {
			return err
		}
	}
	return nil
}

func (m *listenerMigrator) serviceAddresses(name string) []string {
	switch name {
	case svcWeb:
		return m.web.Addresses()
	case svcMCP:
		return m.mcp.Addresses()
	case svcREST:
		return m.rest.Addresses()
	case svcGRPC:
		return m.grpc.Addresses()
	default:
		return nil
	}
}

func checkServiceExposure(name string, addrs []string, authenticated, known bool) error {
	if !known || authenticated {
		return nil
	}
	for _, addr := range addrs {
		if listenAddrIsNonLoopback(addr) {
			return fmt.Errorf("%s listener %q cannot serve a non-loopback address without authentication", name, addr)
		}
	}
	return nil
}

// buildChanges collects the per-service listen-address change set the reloaded
// tree asks for. A service the tree does not describe keeps its listeners.
func (m *listenerMigrator) buildChanges(tree *zeconfig.Tree) []serviceChange {
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

// migrateListeners applies an already-guarded change set. It rolls back every
// listener applied before an in-transaction migration failure. If that
// rollback is incomplete, the returned undo retries the same accepted
// addresses at the outer reload boundary.
func (m *listenerMigrator) migrateListeners(ctx context.Context, changes []serviceChange) (func() error, error) {
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
			if rollbackErr != nil {
				retryRollback := func() error {
					return m.rollbackAppliedListeners(ctx, applied)
				}
				return retryRollback, fmt.Errorf("reconfigure %s: %w (listener rollback failed: %w)", label, err, rollbackErr)
			}
			return noListenerRestore, fmt.Errorf("reconfigure %s: %w", label, err)
		}
		applied = append(applied, change)
	}

	return func() error {
		return m.rollbackAppliedListeners(ctx, applied)
	}, nil
}

func (m *listenerMigrator) rollbackAppliedListeners(ctx context.Context, applied []serviceChange) error {
	var rollbackErrs []error
	for _, change := range slices.Backward(applied) {
		m.logger.Warn("rolling back listener migration", "service", change.name, "addr", change.oldAddr)
		if err := change.server.Reconfigure(ctx, change.oldAddr); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("rollback %s: %w", change.name, err))
		}
	}
	return errors.Join(rollbackErrs...)
}

func (m *listenerMigrator) buildChange(name string, srv Reconfigurable, newAddrs []string) (serviceChange, bool) {
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
