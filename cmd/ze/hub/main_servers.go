// Design: docs/architecture/hub-architecture.md -- Web, LG, and SSH server startup
// Related: main.go -- orchestration calls these, main_reload.go -- reload logic
// Detail: service_web.go -- ze_web-gated web service factory and web-only helpers

package hub

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	zeconfig "github.com/ze-software/ze/internal/component/config"
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
