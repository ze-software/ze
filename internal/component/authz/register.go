// Design: .claude/patterns/registration.md -- AAA registry (VFS-like)
// Overview: auth.go -- LocalAuthenticator implementation
// Related: ../aaa/aaa.go -- AAA interfaces this backend implements

package authz

import (
	"codeberg.org/thomas-mangin/ze/internal/component/aaa"
)

// StoreAuthorizer adapts *Store to aaa.Authorizer (bool return). It maps the
// profile-based Action verdict to a boolean: any decision other than Deny is
// treated as allowed. A nil Store allows everything (matching the existing
// "nil authorizer = allow all" convention in the dispatcher).
type StoreAuthorizer struct {
	Store *Store
}

// Authorize implements aaa.Authorizer.
func (a StoreAuthorizer) Authorize(username, command string, isReadOnly bool) bool {
	if a.Store == nil {
		return true
	}
	return a.Store.Authorize(username, command, isReadOnly) != Deny
}

// localBackend is the AAA backend for built-in bcrypt user authentication.
type localBackend struct{}

// Name returns the backend identifier matching AuthResult.Source.
func (localBackend) Name() string { return "local" }

// Priority 200 places local after tacacs (priority 100) in the chain:
// tacacs is tried first; local is the fallback when tacacs is unreachable.
func (localBackend) Priority() int { return 200 }

// Build returns a Contribution with a LocalAuthenticator bound to
// params.LocalUsers and the hub-supplied Authorizer (if any). Empty user
// list yields an authenticator that rejects every login (timing-safe),
// matching prior behavior.
//
// Authorizer is only contributed when params.LocalAuthorizer is non-nil.
// A nil LocalAuthorizer means "no local RBAC configured" and the dispatcher
// falls back to its own nil-authorizer semantics (allow all). Contributing
// a StoreAuthorizer{Store: nil} here would lie about the configured state.
func (localBackend) Build(params aaa.BuildParams) (aaa.Contribution, error) {
	contrib := aaa.Contribution{
		Authenticator: &LocalAuthenticator{Users: params.LocalUsers},
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
