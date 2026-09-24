// Design: ai/rules/evidence.md -- the exposure guard, re-run on every reload
// Related: mgmt_guard.go -- the boot-time classifier these reloaders re-answer for
// Related: listener_migrate.go -- authReloader, authReporter, reloadListeners
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
	"reflect"

	"github.com/ze-software/ze/internal/component/authz"
	zeconfig "github.com/ze-software/ze/internal/component/config"
)

// mgmtAuthInputs carries boot precedence answers a reload cannot re-derive and
// the candidate credential resolver used to classify the final API mode.
type mgmtAuthInputs struct {
	// webFollowsConfig is true when the config file's `insecure` leaf decided
	// the web authentication switch at boot. It is false when a flag or an
	// environment variable supplied the listen address, because the boot path
	// then never consults the leaf, and a reload must not consult it either.
	webFollowsConfig bool

	// mcpTokenBase is the MCP token after the flag and the environment variable
	// were applied, before the config block filled a blank.
	mcpTokenBase string
	// mcpConfigBase is the MCP settings snapshot used to construct the listener.
	// Its authentication and TLS material remain fixed until restart.
	mcpConfigBase    zeconfig.MCPListenConfig
	mcpEnabledAtBoot bool

	// apiTokenEnv is ze.api-server.token. It fills a blank config token, exactly
	// as it does at boot.
	apiTokenEnv string

	// apiCandidateUsers reads the provider only while runReload stages the
	// candidate generation. Its result selects the candidate auth mode.
	apiCandidateUsers func() ([]authz.UserConfig, error)
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
func markMgmtAuth(lm *listenerMigrator, classified map[string]bool) {
	// Map order is irrelevant: each name writes its own key.
	for name, authenticated := range classified {
		if authenticated {
			lm.markAuthenticated(name)
			continue
		}
		lm.markUnauthenticated(name)
	}
}

// registerMgmtAuthReloaders teaches the listener migrator how to re-answer each
// management surface's authentication question from a reloaded config tree.
//
// The looking glass is deliberately absent. It is an intentionally public
// surface that the boot guard never declares (internal/component/lg/server.go),
// so the reload guard has no record of it and leaves it alone, which is the
// same answer boot gives.
func registerMgmtAuthReloaders(lm *listenerMigrator, in mgmtAuthInputs) {
	lm.setAuthReloader(svcWeb, webAuthReloader(in))
	lm.setAuthReloader(svcMCP, mcpAuthReloader(in))

	// REST and gRPC read one api-server block, so they share one answer.
	api := apiAuthReloader(in)
	lm.setAuthReloader(svcREST, api)
	lm.setAuthReloader(svcGRPC, api)
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

// mcpAuthReloader rejects settings that the running MCP handler cannot apply.
// Address migration remains live; authentication and TLS require restart.
// Removing the block retains the running listener and its credentials.
func mcpAuthReloader(in mgmtAuthInputs) authReloader {
	boot := mcpFixedSettings(in.mcpConfigBase, in.mcpTokenBase)
	return func(tree *zeconfig.Tree) (authIntent, bool, error) {
		cfg, ok := zeconfig.ExtractMCPSettings(tree)
		if !ok {
			return authIntent{}, false, nil
		}
		_, enabled := zeconfig.ExtractMCPConfig(tree)
		if enabled != in.mcpEnabledAtBoot {
			return authIntent{}, false, fmt.Errorf("mcp enabled cannot change while running; restart ze to apply it")
		}
		candidate := mcpFixedSettings(cfg, in.mcpTokenBase)
		if !reflect.DeepEqual(boot, candidate) {
			return authIntent{}, false, fmt.Errorf("mcp authentication or TLS settings cannot change while running; restart ze to apply them")
		}
		return authIntent{authenticated: mcpListenerAuthenticated(true, cfg.AuthMode, candidate.Token)}, true, nil
	}
}

// mcpFixedSettings excludes the fields the live listener migrator applies and
// resolves the immutable flag/environment token with startup precedence.
func mcpFixedSettings(cfg zeconfig.MCPListenConfig, token string) zeconfig.MCPListenConfig {
	cfg = mcpEffectiveSettings(cfg, token)
	cfg.Servers = nil
	cfg.BindRemote = false
	return cfg
}

// mcpEffectiveSettings resolves the same token and inferred auth mode for boot
// validation and reload comparison. An explicit auth mode always wins.
func mcpEffectiveSettings(cfg zeconfig.MCPListenConfig, token string) zeconfig.MCPListenConfig {
	if token != "" {
		cfg.Token = token
	}
	if cfg.AuthMode == "" {
		if cfg.Token != "" {
			cfg.AuthMode = "bearer"
		}
	}
	return cfg
}

// apiAuthReloader resolves only the final authentication mode for exposure
// checks. Candidate tokens and users remain in the unpublished identity state.
// A present block re-resolves config-over-environment token precedence. An
// absent block retains the accepted token because an environment- or
// flag-started transport remains live, but still resolves candidate users so
// deleting the final user cannot silently expose that listener.
func apiAuthReloader(in mgmtAuthInputs) authReloader {
	return func(tree *zeconfig.Tree) (authIntent, bool, error) {
		cfg, present := zeconfig.ExtractAPISettings(tree)
		if in.apiCandidateUsers == nil {
			return authIntent{}, false, fmt.Errorf("resolve candidate API users: %w", errNoLiveConfigProvider)
		}
		users, err := in.apiCandidateUsers()
		if err != nil {
			return authIntent{}, false, fmt.Errorf("resolve candidate API users: %w", err)
		}

		retainedToken := ""
		if !present {
			accepted := acceptedLocalIdentity.Load()
			if accepted == nil {
				return authIntent{}, false, fmt.Errorf("resolve retained API token: %w", errNoAcceptedLocalIdentity)
			}
			retainedToken = accepted.apiToken
		}
		token := candidateAPIToken(cfg.Token, present, retainedToken, in.apiTokenEnv)
		return authIntent{authenticated: len(users) > 0 || token != ""}, true, nil
	}
}

// candidateAPIToken is shared by exposure classification and final identity
// publication so both answer from identical absent-block retention and
// config-over-environment precedence.
func candidateAPIToken(configToken string, blockPresent bool, retainedToken, environmentToken string) string {
	if !blockPresent {
		return retainedToken
	}
	if configToken != "" {
		return configToken
	}
	return environmentToken
}
