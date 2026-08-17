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
	"strings"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/paths"
	"github.com/ze-software/ze/internal/core/textbuf"
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
		cmdCtx := &pluginserver.CommandContext{
			Server:     s,
			Username:   caller.Username,
			RemoteAddr: caller.RemoteAddr,
			Surface:    srf,
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

// errNoLiveConfigProvider reports that no live view of the running config is
// wired, so the current user list cannot be read at all. The reachable caller
// is liveLocalUsers with no config source threaded into it; the nil-provider
// branch of liveConfigUsers guards the same seam one layer down, for a caller
// that holds the provider rather than the func.
var errNoLiveConfigProvider = errors.New("no live config provider")

// errNoSystemConfigRoot reports that the running configuration holds no
// `system` root at all, as distinct from a `system` root that declares no
// users. Both yield an empty user list, so a caller handed only that list
// cannot tell an operator's configuration from a root the daemon lost.
//
// It is not a denial: a configuration with no `system` block is legal, and the
// zefs power user must keep authenticating through it (AC-7). It exists so the
// caller says which one happened instead of treating one answer as two facts.
var errNoSystemConfigRoot = errors.New("the running configuration declares no system root")

// liveConfigUsers returns the users the CURRENT configuration declares, read
// from the shared ConfigProvider rather than from a startup snapshot.
//
// Every applied reload refreshes that provider root by root
// (applyLoadedTreeToProvider in main_reload.go), and a rolled-back reload
// restores the prior roots, so the provider is the one in-process view of the
// config the daemon is actually running. A caller that authenticates against it
// stops accepting a user the operator deleted, without waiting for a restart.
//
// A nil provider is an ERROR, not an empty user list. The caller cannot tell a
// configuration with no users from a configuration it failed to read, so this
// makes the miss explicit at the producer and lets the caller deny
// (ai/rules/evidence.md, "fail closed or say something").
//
// An absent `system` root is reported the same way, as errNoSystemConfigRoot
// beside an empty list. It is not a denial (see that error), but it is the
// fault mode this reader actually has: Provider.Get answers a missing root with
// an empty map and a nil error, so before this branch existed the daemon losing
// the root and the operator writing no users were one indistinguishable answer.
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

// liveLocalUsers returns the local credentials the daemon accepts RIGHT NOW:
// the zefs power users merged with the users the running configuration
// declares. It is the one live source shared by AAA, API, standalone SSH, and
// web authentication, so the surfaces cannot disagree about who exists.
//
// zefsUsers is a startup snapshot and correctly so. Those credentials live in
// the blob store, not the config file: no reload adds or removes them, and
// `meta/instance/admin-disabled` is written only at image assembly
// (internal/appliance/cmd_assemble.go).
//
// A read failure is logged HERE, at the layer that knows the read happened, and
// returned so the caller denies. Every later layer sees only "no credentials
// matched", which is indistinguishable from a wrong password
// (ai/rules/evidence.md, "make the miss explicit at the producer").
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
	// Carry the reserved break-glass recovery profile. It is delivered ONLY through
	// login-resolved profiles (UserCredential.Profiles -> AuthResult.Profiles ->
	// RecordLoginProfiles), never a config assignment, so it cannot flip the
	// operator's RBAC posture. authz.Store.Authorize honors this reserved name
	// regardless of the profiles the store defines, so a strict authorization
	// default can never lock the bootstrap admin out of a box whose authorization
	// config is wrong or partial (spec-fixit-authz-admin-fallthrough O-3').
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

// resolveConfigPath returns the config file path for the editor.
func resolveConfigPath(store storage.Storage) string {
	data, err := store.ReadFile(zefs.KeyInstanceName.Pattern)
	if err == nil && len(data) > 0 {
		name := strings.TrimSpace(string(data))
		if name != "" {
			var tb textbuf.Buffer
			return tb.Str(name).Str(".conf").String()
		}
	}
	return "ze.conf"
}
