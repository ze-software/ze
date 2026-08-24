// Design: docs/architecture/hub-architecture.md -- Web, LG, and SSH server startup
// Related: main.go -- orchestration calls these, main_reload.go -- reload logic
// Detail: service_web.go -- ze_web-gated web service factory and web-only helpers

package hub

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/paths"
	"github.com/ze-software/ze/pkg/zefs"
)

// serverDispatcher builds the single unified command dispatcher for a surface.
// The returned plugin.CommandDispatcher constructs a request-scoped
// CommandContext from the caller identity and dispatches through the plugin
// server, returning the typed *plugin.Response so each surface renders at its
// own edge (text surfaces via plugin.CommandDispatcher.JSON; the API engine
// carries typed Data to the REST/gRPC transport). This replaces the two
// copy-pasted flatten adapters (serverDispatcherWithSurface + apiExecutor); the
// flatten sequence now lives once in plugin.ResponseJSON.
//
// Audit surface attribution uses caller.Surface when the caller sets it (the
// REST and gRPC transports set it per request); otherwise the fixed `surface`
// default applies (web, ssh, mcp, cli dispatchers built in main.go).
func serverDispatcher(s *pluginserver.Server, surface string) plugin.CommandDispatcher {
	return func(ctx context.Context, caller plugin.CallerIdentity, command string) (*plugin.Response, error) {
		d := s.Dispatcher()
		if d == nil {
			return nil, errServerNotReady
		}
		srf := surface
		if caller.Surface != "" {
			srf = caller.Surface
		}
		if caller.Authorizer == nil {
			caller.Authorizer = plugin.CallerAuthorizer(ctx)
		}
		cmdCtx := &pluginserver.CommandContext{
			Server:     s,
			Username:   caller.Username,
			RemoteAddr: caller.RemoteAddr,
			Surface:    srf,
			Authorizer: caller.Authorizer,
			// Every surface this dispatcher serves is an operator surface (web,
			// ssh, mcp, cli, lg, chaos, REST, gRPC). The command carries the
			// operator's own authority, which AAA checks against Username above,
			// so a peer's `attach process` block does not gate it
			// (internal/component/bgp/reactor/send_permission.go).
			Sender: plugin.OperatorSender(),
		}
		// Thread a genuine per-request context: the REST/gRPC transport passes
		// its request ctx so the command cancels with the request. Text surfaces
		// (web/mcp/lg/chaos/ssh/cli) have no request-scoped context and pass a
		// never-canceling placeholder (context.Background(), or context.TODO());
		// leaving RequestContext nil then makes CommandContext.Context() fall
		// back to the server context, so an in-flight command still cancels on
		// daemon shutdown -- matching the pre-unification
		// serverDispatcherWithSurface, which never set RequestContext.
		if ctx != nil && ctx != context.Background() && ctx != context.TODO() {
			cmdCtx.RequestContext = ctx
		}
		return d.Dispatch(cmdCtx, command)
	}
}

// endpointsToAddrs converts a slice of config.ServerEndpoint into the
// "host:port" string slice that every multi-listener binder accepts.
func endpointsToAddrs(servers []zeconfig.ServerEndpoint) []string {
	out := make([]string, 0, len(servers))
	for _, ep := range servers {
		out = append(out, ep.Listen())
	}
	return out
}

// mergeAuthUsers returns the zefs power user(s) followed by the config-file
// users. When a config user has the same name as a zefs power user, the config
// entry takes precedence and the zefs entry is dropped; this lets operators
// override the built-in password via configuration. The result is a fresh
// slice (never aliases either input).
func mergeAuthUsers(zefsUsers, configUsers []authz.UserConfig) []authz.UserConfig {
	cfgNames := make(map[string]bool, len(configUsers))
	for _, u := range configUsers {
		cfgNames[u.Name] = true
	}
	out := make([]authz.UserConfig, 0, len(zefsUsers)+len(configUsers))
	for _, u := range zefsUsers {
		if !cfgNames[u.Name] {
			out = append(out, u)
		}
	}
	out = append(out, configUsers...)
	return out
}

// errNoLiveConfigProvider reports that no ConfigProvider-backed candidate user
// source is wired. Boot and reload use that source to assemble a generation;
// request-time authentication uses liveAcceptedLocalUsers instead.
var errNoLiveConfigProvider = errors.New("no live config provider")

// errNoAcceptedLocalIdentity reports that boot has not published a complete
// users-plus-authorization generation. It is an authentication failure, never a
// valid empty user list.
var errNoAcceptedLocalIdentity = errors.New("no accepted local identity state")

// errNoSystemConfigRoot reports that the running configuration holds no
// `system` root at all, as distinct from a `system` root that declares no
// users. Both yield an empty user list, so a caller handed only that list
// cannot tell an operator's configuration from a root the daemon lost.
//
// It is not a denial: a configuration with no `system` block is legal, and the
// zefs power user must keep authenticating through it (AC-7). It exists so the
// caller says which one happened instead of treating one answer as two facts.
var errNoSystemConfigRoot = errors.New("the running configuration declares no system root")

// liveConfigUsers returns the users the ConfigProvider currently declares.
//
// During boot the provider holds the boot tree. During reload it temporarily
// holds the candidate tree so the hub can stage a replacement identity
// generation. Authentication must not call this function directly because a
// candidate remains rejectable until the reload's final publication step.
//
// A nil provider is an ERROR, not an empty user list. The caller cannot tell a
// configuration with no users from a configuration it failed to read, so this
// makes the miss explicit at the producer and lets the caller deny
// (ai/rules/evidence.md, "fail closed or say something").
//
// An absent `system` root is reported as errNoSystemConfigRoot beside an empty
// list. It is not a denial (see that error), but it is the fault mode this
// reader actually has: Provider.Get answers a missing root with an empty map and
// a nil error.
func liveConfigUsers(cp *zeconfig.Provider) ([]authz.UserConfig, error) {
	if cp == nil {
		return nil, errNoLiveConfigProvider
	}
	system, ok := cp.Root("system")
	if !ok {
		return nil, errNoSystemConfigRoot
	}
	return infra.ExtractAuthUsers(system), nil
}

// liveLocalUsers assembles a candidate local identity list from the zefs boot
// snapshot and the users currently in ConfigProvider. Config users win name
// collisions.
//
// Production calls this resolver once at boot and once while staging each
// reload. Request-time SSH, REST, gRPC, web, and AAA authentication call
// liveAcceptedLocalUsers so a rejectable candidate is never exposed.
//
// zefsUsers is a startup snapshot and correctly so. Those credentials live in
// the blob store, not the config file: no reload adds or removes them, and
// `meta/instance/admin-disabled` is written only at image assembly
// (internal/appliance/cmd_assemble.go).
//
// A read failure is logged here, at the layer that knows the read happened, and
// returned so boot or reload rejects the candidate.
func liveLocalUsers(zefsUsers []authz.UserConfig, configUsers func() ([]authz.UserConfig, error), logger *slog.Logger) func() ([]authz.UserConfig, error) {
	return func() ([]authz.UserConfig, error) {
		if configUsers == nil {
			if logger != nil {
				logger.Warn("no running-config user source wired; refusing local authentication")
			}
			return nil, errNoLiveConfigProvider
		}
		current, err := configUsers()
		switch {
		case errors.Is(err, errNoSystemConfigRoot):
			// A configuration with no `system` block declares no users. That is
			// a configuration, not a read failure, so the zefs power user keeps
			// authenticating (AC-7). Said out loud because the same empty list
			// arrives from a `system` root that declares no users, and an
			// operator reading the log needs to know which one they wrote.
			if logger != nil {
				logger.Debug("the running configuration declares no system root; only power users authenticate")
			}
			current = nil
		case err != nil:
			if logger != nil {
				logger.Warn("cannot read running config users; refusing local authentication", "error", err)
			}
			return nil, err
		}
		return mergeAuthUsers(zefsUsers, current), nil
	}
}

// resolveBootUsers is the single boot snapshot seam. Production calls the
// already-assembled live source once after ConfigProvider population. Tests
// replace this function to prove a source failure stops runYANGConfig before
// listener construction without adding another credential source.
var resolveBootUsers = func(usersLive func() ([]authz.UserConfig, error)) ([]authz.UserConfig, error) {
	if usersLive == nil {
		return nil, errNoLiveConfigProvider
	}
	return usersLive()
}

// loadZefsUsers reads credentials from the zefs database (created by ze init).
func loadZefsUsers() ([]authz.UserConfig, error) {
	dir := paths.DefaultConfigDir()
	if dir == "" {
		return nil, errCannotResolveConfigDirectory
	}
	dbPath := filepath.Join(dir, "database.zefs")
	db, err := zefs.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dbPath, err)
	}
	defer db.Close() //nolint:errcheck // read-only access
	return usersFromZefsDB(db)
}

// usersFromZefsDB reads the dedicated local power-user credentials from zefs.
// Missing or empty credentials return an error so the caller fails closed.
// When meta/instance/admin-disabled is "true", returns errAdminDisabledInZefs
// so the caller skips the built-in power user.
func usersFromZefsDB(db *zefs.BlobStore) ([]authz.UserConfig, error) {
	if disabled, err := db.ReadFile(zefs.KeyInstanceAdminDisabled.Pattern); err == nil && string(disabled) == "true" {
		return nil, errAdminDisabledInZefs
	}
	username, err := db.ReadFile(zefs.KeyLocalAdminUsername.Pattern)
	if err != nil {
		return nil, fmt.Errorf("read local username: %w", err)
	}
	hash, err := db.ReadFile(zefs.KeyLocalAdminPassword.Pattern)
	if err != nil {
		return nil, fmt.Errorf("read local password hash: %w", err)
	}
	name, err := validateLocalAdminCreds(username, hash)
	if err != nil {
		return nil, err
	}
	// Carry the reserved break-glass recovery profile. Authentication binds it
	// to AuthResult.Authorizer, never to a config assignment, so it cannot change
	// the operator's RBAC posture. The grant remains valid only for the accepted
	// local credential generation that produced the result. A strict
	// authorization default cannot lock the bootstrap admin out because
	// authz.Store honors this reserved profile.
	// meta/instance/admin-disabled (checked above) still lets an operator suppress
	// this account entirely.
	return []authz.UserConfig{{
		Name:     name,
		Hash:     string(hash),
		Profiles: []string{aaa.ReservedRecoveryProfile},
	}}, nil
}

func validateLocalAdminCreds(username, hash []byte) (string, error) {
	name := string(username)
	if name == "" {
		return "", errEmptyUsernameInZefs
	}
	if len(hash) == 0 {
		// Fail closed: never hand an empty password hash to the authorizer.
		return "", errEmptyPasswordInZefs
	}
	return name, nil
}
