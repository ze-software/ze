// Design: ai/patterns/registration.md -- AAA registry (VFS-like)
// Overview: auth.go -- LocalAuthenticator implementation
// Related: ../aaa/aaa.go -- AAA interfaces this backend implements

package authz

import (
	"strings"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// StoreAuthorizer adapts *Store to aaa.Authorizer (bool return). It maps the
// profile-based Action verdict to a boolean: any decision other than Deny is
// treated as allowed. A nil Store allows operator and internal-RPC identities
// (matching the existing "nil authorizer = allow all" convention when
// system.authorization is unconfigured), while still evaluating plugin
// identities against their compiled bus PluginCommandProfile.
type StoreAuthorizer struct {
	Store *Store
}

// authorizeWithNilStore preserves the allow-all default for human operators
// when no operator authorization Store is configured, while still enforcing
// compiled bus command profiles on plugin identities (never default-allowing a
// plugin identity).
func authorizeWithNilStore(username, command string, isReadOnly bool) bool {
	if endpoint, isPlugin := aaa.PluginEndpointFromIdentity(username); isPlugin {
		return authorizePluginIdentity(username, endpoint, command, isReadOnly) != Deny
	}
	if strings.HasPrefix(username, aaa.ReservedInternalPluginPrefix) {
		authzLogger.Warn("denied: malformed empty plugin identity",
			"identity", username, "command", command,
			"code", diagnostic.CodeDoctorPluginRequestRefused)
		return false
	}
	return true
}

// BindProfiles returns a request-carried authorizer for one authentication
// result. The profile slice is copied so backend-owned result storage cannot
// mutate an established session.
func (a StoreAuthorizer) BindProfiles(profiles []string) aaa.Authorizer {
	return boundStoreAuthorizer{
		store:    a.Store,
		profiles: append([]string(nil), profiles...),
	}
}

type boundStoreAuthorizer struct {
	store    *Store
	profiles []string
}

func (a boundStoreAuthorizer) Authorize(username, _, command string, isReadOnly bool) bool {
	if a.store == nil {
		return authorizeWithNilStore(username, command, isReadOnly)
	}
	return a.store.AuthorizeWithProfiles(username, a.profiles, command, isReadOnly) != Deny
}

// canonicalCommandForIdentity builds the authorization command string for
// username. Plugin identities match compiled bus command profiles (cli:<cmd>),
// which omit the synthetic "peer " prefix that operator RBAC adds when a peer
// selector is present.
func canonicalCommandForIdentity(username, command string, args []string, peer string) string {
	if _, isPlugin := aaa.PluginEndpointFromIdentity(username); isPlugin {
		return aaa.CanonicalCommand(command, args, "")
	}
	if strings.HasPrefix(username, aaa.ReservedInternalPluginPrefix) {
		return aaa.CanonicalCommand(command, args, "")
	}
	return aaa.CanonicalCommand(command, args, peer)
}

func (a boundStoreAuthorizer) AuthorizeCommandArgs(username, _, command string, args []string, peer string, isReadOnly bool) bool {
	canonical := canonicalCommandForIdentity(username, command, args, peer)
	if a.store == nil {
		return authorizeWithNilStore(username, canonical, isReadOnly)
	}
	return a.store.AuthorizeWithProfiles(username, a.profiles, canonical, isReadOnly) != Deny
}

// AuthorizeLocalScope implements aaa.LocalScopeAuthorizer. A missing store or
// empty profile binding fails closed rather than falling back to no-RBAC.
func (a boundStoreAuthorizer) AuthorizeLocalScope(username, command string, args []string, isReadOnly bool) bool {
	if a.store == nil {
		return false
	}
	if len(a.profiles) == 0 {
		return false
	}
	canonical := canonicalCommandForIdentity(username, command, args, "")
	return a.store.AuthorizeWithProfiles(username, a.profiles, canonical, isReadOnly) != Deny
}

func explicitTypedCommand(tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	return aaa.CanonicalCommand("", tokens, "")
}

// AuthorizeExplicit implements aaa.ExplicitAuthorizer. A missing store, empty
// profile binding, or empty token stream always fails closed.
func (a boundStoreAuthorizer) AuthorizeExplicit(
	username, _, canonicalPath string,
	tokens []string,
	isReadOnly bool,
) bool {
	if a.store == nil {
		return false
	}
	if len(a.profiles) == 0 {
		return false
	}
	typedCommand := explicitTypedCommand(tokens)
	if typedCommand == "" {
		return false
	}
	return a.store.authorizeExplicitWithProfiles(username, a.profiles, canonicalPath, typedCommand, isReadOnly) == Allow
}

// Authorize implements aaa.Authorizer.
func (a StoreAuthorizer) Authorize(username, remoteAddr, command string, isReadOnly bool) bool {
	if a.Store == nil {
		return authorizeWithNilStore(username, command, isReadOnly)
	}
	return a.Store.Authorize(username, command, isReadOnly) != Deny
}

// AuthorizeCommandArgs implements aaa.CommandArgsAuthorizer.
// It preserves typed argument boundaries while keeping the dispatcher's
// existing peer-scoped RBAC semantics through aaa.CanonicalCommand.
func (a StoreAuthorizer) AuthorizeCommandArgs(username, _, command string, args []string, peer string, isReadOnly bool) bool {
	canonical := canonicalCommandForIdentity(username, command, args, peer)
	if a.Store == nil {
		return authorizeWithNilStore(username, canonical, isReadOnly)
	}
	return a.Store.Authorize(username, canonical, isReadOnly) != Deny
}

// AuthorizeLocalScope implements aaa.LocalScopeAuthorizer. It evaluates
// canonical command-argument tokens strictly locally against a.Store and fails
// closed when a.Store is nil.
func (a StoreAuthorizer) AuthorizeLocalScope(username, command string, args []string, isReadOnly bool) bool {
	if a.Store == nil {
		return false
	}
	canonical := canonicalCommandForIdentity(username, command, args, "")
	return a.Store.Authorize(username, canonical, isReadOnly) != Deny
}

// AuthorizeExplicit implements aaa.ExplicitAuthorizer. When a.Store is nil,
// every non-plugin identity fails closed while plugin identities continue to be
// evaluated against their compiled bus command profiles under explicit-entry
// rules.
func (a StoreAuthorizer) AuthorizeExplicit(
	username, _, canonicalPath string,
	tokens []string,
	isReadOnly bool,
) bool {
	typedCommand := explicitTypedCommand(tokens)
	if typedCommand == "" {
		return false
	}
	if a.Store == nil {
		if endpoint, isPlugin := aaa.PluginEndpointFromIdentity(username); isPlugin {
			return authorizePluginIdentityExplicit(username, endpoint, canonicalPath, typedCommand, isReadOnly) == Allow
		}
		return false
	}
	return a.Store.AuthorizeExplicit(username, canonicalPath, typedCommand, isReadOnly) == Allow
}

// localBackend is the AAA backend for built-in bcrypt user authentication.
type localBackend struct{}

// Name returns the backend identifier matching AuthResult.Source.
func (localBackend) Name() string { return aaa.SourceLocal }

// Priority 200 places local after tacacs (priority 100) in the chain:
// tacacs is tried first; local is the fallback when tacacs is unreachable.
func (localBackend) Priority() int { return 200 }

// Build returns a Contribution with a LocalAuthenticator and the hub-supplied
// Authorizer (if any). Empty user list yields an authenticator that rejects
// every login (timing-safe), matching prior behavior.
//
// params.LocalUsersFunc wins over params.LocalUsers when both are supplied. A
// caller that can describe the RUNNING credentials is describing something the
// snapshot cannot: which users exist now. Passing both and preferring the
// snapshot would keep the chain authenticating deleted accounts between the
// reloads that rebuild the bundle.
//
// Authorizer is only contributed when params.LocalAuthorizer is non-nil.
// A nil LocalAuthorizer means "no local RBAC configured" and the dispatcher
// falls back to its own nil-authorizer semantics (allow all). Contributing
// a StoreAuthorizer{Store: nil} here would lie about the configured state.
func (localBackend) Build(params aaa.BuildParams) (aaa.Contribution, error) {
	contrib := aaa.Contribution{
		Authenticator: &LocalAuthenticator{
			Users:     params.LocalUsers,
			UsersFunc: params.LocalUsersFunc,
		},
	}
	if params.LocalAuthorizer != nil {
		contrib.Authorizer = params.LocalAuthorizer
	}
	return contrib, nil
}

func init() {
	if err := aaa.Default.Register(localBackend{}); err != nil {
		panic("BUG: authz: register local AAA backend: " + err.Error())
	}
}
