// Design: ai/rules/evidence.md -- the exposure guard, re-run on every reload
// Related: mgmt_guard.go -- the boot-time classifier these reloaders re-answer for
// Related: listener_migrate.go -- authReloader, AuthUpdatable, ReloadListeners
//
// How each management surface's authentication is resolved from a RELOADED
// config tree. The boot guard (mgmt_guard.go) answers the same question once,
// from flags, environment variables, and the config file together. A SIGHUP
// reload can change only the config file, so each reloader here re-answers the
// config half against the boot answer for the other two, captured in
// mgmtAuthInputs.
//
// Keeping the precedence in one place is the point. A second implementation
// that disagreed with boot would make the migrator report changes the daemon
// would never make, or miss ones it would.

package hub

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/authz"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
)

// mgmtAuthInputs carries the boot answers a reload cannot re-derive: what a
// command-line flag or an environment variable decided, and what the daemon
// could read when it started.
type mgmtAuthInputs struct {
	// webFollowsConfig is true when the config file's `insecure` leaf decided
	// the web authentication switch at boot. It is false when a flag or an
	// environment variable supplied the listen address, because the boot path
	// then never consults the leaf, and a reload must not consult it either.
	webFollowsConfig bool

	// mcpTokenBase is the MCP token after the flag and the environment variable
	// were applied, before the config block filled a blank.
	mcpTokenBase string

	// apiTokenEnv is ze.api-server.token. It fills a blank config token, exactly
	// as it does at boot.
	apiTokenEnv string

	// apiZefsUsersOK is true when the zefs power-user credentials were readable
	// at boot. A daemon that had them and can no longer read them fails the
	// reload closed rather than rebuilding the API servers without them; a
	// daemon that never had them keeps reloading normally.
	apiZefsUsersOK bool

	// apiUsersLive returns the credentials valid right now (liveLocalUsers in
	// main.go). The rebuilt API authenticator answers from it, exactly as the
	// boot one does, so a reload that changes a listener cannot put the API
	// back on a snapshot and revive a deleted account (AC-13,
	// plan/spec-fixit-web-auth-deleted-user-survives-reload.md).
	apiUsersLive func() ([]authz.UserConfig, error)
}

// markMgmtAuth hands the boot guard's classification to the listener migrator,
// both ways. The negative stops a SIGHUP migration moving an unauthenticated
// surface to a non-loopback address. The positive matters just as much: a
// surface the migrator has no record for is one the guard never declared, and
// the reload guard leaves it alone rather than treating silence as
// authenticated.
//
// It runs BEFORE any server handle reaches the migrator, and that ordering is
// the guard's, not a convenience. checkReloadExposure SKIPS a service it has no
// record for, which is its permissive branch, so a handle installed while the
// record is still missing can be migrated to a non-loopback address with no
// authentication.
//
// A surface the daemon never builds is therefore classified here like any
// other, and dropped at RELOAD time instead: resolveAuthIntents skips a name
// with no handle, so no intent is produced for it and checkAuthRebuildable
// cannot refuse over it. The API's two transports are the case that needs it.
// One `api-server` block answers for REST and gRPC together, so a config that
// enables REST alone classifies gRPC as well; gRPC cannot rebuild its
// authentication without a handle, and an operator removing the API token was
// told "grpc cannot change its authentication while running" by a daemon
// running no gRPC server at all.
func markMgmtAuth(lm *ListenerMigrator, classified map[string]bool) {
	// Map order is irrelevant: each name writes its own key.
	for name, authenticated := range classified {
		if authenticated {
			lm.MarkAuthenticated(name)
			continue
		}
		lm.MarkUnauthenticated(name)
	}
}

// registerMgmtAuthReloaders teaches the listener migrator how to re-answer each
// management surface's authentication question from a reloaded config tree.
//
// The looking glass is deliberately absent. It is an intentionally public
// surface that the boot guard never declares (internal/component/lg/server.go),
// so the reload guard has no record of it and leaves it alone, which is the
// same answer boot gives.
func registerMgmtAuthReloaders(lm *ListenerMigrator, in mgmtAuthInputs) {
	lm.SetAuthReloader(svcWeb, webAuthReloader(in))
	lm.SetAuthReloader(svcMCP, mcpAuthReloader(in))

	// REST and gRPC read one api-server block, so they share one answer.
	api := apiAuthReloader(in)
	lm.SetAuthReloader(svcREST, api)
	lm.SetAuthReloader(svcGRPC, api)
}

// webAuthReloader resolves whether the web server the reloaded config describes
// gates every request. A secure web listener carries auth middleware and its own
// no-users refusal, so only the insecure path is unauthenticated -- the same
// rule the boot guard applies.
func webAuthReloader(in mgmtAuthInputs) authReloader {
	return func(tree *zeconfig.Tree) (authIntent, bool, error) {
		if !in.webFollowsConfig {
			// A flag or an environment variable owns this answer for the life of
			// the process. Reporting a config leaf as the new intent here would
			// announce a change the daemon would never make.
			return authIntent{}, false, nil
		}
		cfg, ok := zeconfig.ExtractWebConfig(tree)
		if !ok {
			return authIntent{}, false, nil
		}
		return authIntent{authenticated: !cfg.Insecure}, true, nil
	}
}

// mcpAuthReloader resolves whether the MCP server the reloaded config describes
// gates every request. It defers to mcpListenerAuthenticated, the one function
// that mirrors the MCP server's effective-mode precedence, so an explicit
// auth-mode of "none" reads as unauthenticated even with a token set.
func mcpAuthReloader(in mgmtAuthInputs) authReloader {
	return func(tree *zeconfig.Tree) (authIntent, bool, error) {
		cfg, ok := zeconfig.ExtractMCPSettings(tree)
		token := in.mcpTokenBase
		if ok && token == "" {
			token = cfg.Token
		}
		return authIntent{authenticated: mcpListenerAuthenticated(ok, cfg.AuthMode, token)}, true, nil
	}
}

// apiAuthReloader resolves the credentials the reloaded config gives the REST
// and gRPC servers. Both transports can install them without rebinding
// (AuthUpdatable), so this reloader returns the material as well as the mode.
func apiAuthReloader(in mgmtAuthInputs) authReloader {
	return func(tree *zeconfig.Tree) (authIntent, bool, error) {
		cfg, ok := zeconfig.ExtractAPIConfig(tree)
		if !ok {
			// The reloaded config says nothing about the API. The running
			// servers keep the credentials they have: removing a block must
			// never strip authentication off a listener that stays up.
			return authIntent{}, false, nil
		}

		token := cfg.Token
		if token == "" {
			token = in.apiTokenEnv
		}

		zefsUsers, err := loadZefsUsers()
		if err != nil {
			if in.apiZefsUsersOK {
				// These credentials authenticated API callers when the daemon
				// started. Rebuilding without them would silently drop every
				// power user, so the reload fails closed instead.
				return authIntent{}, false, fmt.Errorf("power-user credentials are no longer readable: %w", err)
			}
			zefsUsers = nil
		}

		// Config-file users authenticate alongside the power user, the same
		// merge the boot path performs.
		users := mergeAuthUsers(zefsUsers, infra.ExtractSSHConfig(tree).Users)
		return authIntent{
			authenticated: len(users) > 0 || token != "",
			token:         token,
			authenticator: buildUserAuthenticator(users, in.apiUsersLive),
		}, true, nil
	}
}
