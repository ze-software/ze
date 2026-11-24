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

func (a boundStoreAuthorizer) AuthorizeCommandArgs(username, _, command string, args []string, peer string, isReadOnly bool) bool {
	if a.store == nil {
		return authorizeWithNilStore(username, aaa.CanonicalCommand(command, args, peer), isReadOnly)
	}
	return a.store.AuthorizeWithProfiles(username, a.profiles, aaa.CanonicalCommand(command, args, peer), isReadOnly) != Deny
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
	if a.Store == nil {
		return authorizeWithNilStore(username, aaa.CanonicalCommand(command, args, peer), isReadOnly)
	}
	return a.Store.Authorize(username, aaa.CanonicalCommand(command, args, peer), isReadOnly) != Deny
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
