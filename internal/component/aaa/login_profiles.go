// Design: docs/architecture/core-design.md -- AAA login-resolved profiles
// Related: types.go -- AuthResult.Profiles, the value recorded here
// Related: ../authz/authz.go -- Store.Authorize consumes this as a fallback
//
// Why this exists: a backend decides which authz profiles a user holds at
// AUTHENTICATION time. The local backend reads them from
// system.authentication.user[*].profile, but TACACS+ derives them from the
// server's priv-lvl reply via the tacacs-profile mapping -- a value that exists
// nowhere in config keyed by username.
//
// Authorization runs later, on a different call, and receives only the username.
// Without somewhere to put the login-time answer, authz.Store could only look up
// its static config assignments, find nothing for a TACACS+ user, and fall
// through to the built-in admin profile: the priv-lvl mapping was logged at login
// and then ignored, so every TACACS+ user was authorized as admin.
//
// Only profile NAMES are recorded, never a resolved Profile. Authorization looks
// each name up in the live store, so a reload that changes what "read-only" means
// takes effect on the next command rather than being pinned at login.

package aaa

import (
	"slices"
	"sync/atomic"
)

// acceptedLocalUserGenerations maps each accepted local user name to the
// per-user generations (UserCredential.UserGeneration) its records carry. A
// name can repeat in the accepted list, hence a slice. nil means no local
// identity is accepted, so every recovery grant is refused.
var acceptedLocalUserGenerations atomic.Pointer[map[string][]uint64]

// SetAcceptedLocalUsers publishes the per-user generations of the accepted
// local credentials, against which local recovery grants are valid. The hub
// calls it each time it accepts a local identity, and with nil when it retires
// one, which revokes every grant. Remote login-resolved profiles are not
// generation-bound.
func SetAcceptedLocalUsers(users []UserCredential) {
	if users == nil {
		acceptedLocalUserGenerations.Store(nil)
		return
	}
	generations := make(map[string][]uint64, len(users))
	for _, user := range users {
		generations[user.Name] = append(generations[user.Name], user.UserGeneration)
	}
	acceptedLocalUserGenerations.Store(&generations)
}

// acceptedLocalUserHolds reports whether the accepted local identity still
// declares username at generation, the per-user generation that granted a
// recovery session. It is the check the web session store makes for a
// locally anchored cookie (web.SessionStore.localUserSessionStatus): a change
// to another user leaves it true, and a change to this user's own hash,
// profiles or keys, or this user's removal, makes it false.
//
// Generation numbers are never shared between two user records, so a grant
// presented under another username matches nothing and is refused.
func acceptedLocalUserHolds(username string, generation uint64) bool {
	// refuse-by-default: guards acceptedGenerationAuthorizer.Authorize -- a
	// result no local record stamped carries generation 0 and grants nothing.
	if generation == 0 {
		return false
	}
	generations := acceptedLocalUserGenerations.Load()
	// refuse-by-default: guards acceptedGenerationAuthorizer.Authorize -- no
	// accepted identity (before boot publishes, after shutdown) grants nothing.
	if generations == nil {
		return false
	}
	return slices.Contains((*generations)[username], generation)
}

// acceptedGenerationAuthorizer is the break-glass recovery grant one local
// login holds. It is valid only while the accepted local identity declares the
// session's user at the per-user generation that authenticated it.
type acceptedGenerationAuthorizer struct {
	generation uint64
	next       Authorizer
}

func (a acceptedGenerationAuthorizer) Authorize(username, remoteAddr, command string, isReadOnly bool) bool {
	if !acceptedLocalUserHolds(username, a.generation) {
		return false
	}
	if a.next == nil {
		return true
	}
	return a.next.Authorize(username, remoteAddr, command, isReadOnly)
}

func (a acceptedGenerationAuthorizer) AuthorizeCommandArgs(
	username, remoteAddr, command string,
	args []string,
	peer string,
	isReadOnly bool,
) bool {
	if !acceptedLocalUserHolds(username, a.generation) {
		return false
	}
	if a.next == nil {
		return true
	}
	if typed, ok := a.next.(CommandArgsAuthorizer); ok {
		return typed.AuthorizeCommandArgs(username, remoteAddr, command, args, peer, isReadOnly)
	}
	return a.next.Authorize(username, remoteAddr, CanonicalCommand(command, args, peer), isReadOnly)
}

// AuthorizeLocalScope implements LocalScopeAuthorizer. No positive scope
// survives an authorization generation change of the authenticated local user,
// and an unsupported underlying authorizer fails closed.
func (a acceptedGenerationAuthorizer) AuthorizeLocalScope(
	username, command string,
	args []string,
	isReadOnly bool,
) bool {
	if !acceptedLocalUserHolds(username, a.generation) {
		return false
	}
	if a.next == nil {
		return true
	}
	allowed, supported := AuthorizeLocalScope(a.next, username, command, args, isReadOnly)
	if !supported {
		return false
	}
	return allowed
}

// resultAuthorizingAuthenticator rejects reserved identities and binds
// authorization to each successful authentication result.
type resultAuthorizingAuthenticator struct {
	next       Authenticator
	authorizer Authorizer
}

// WithProfileAuthorizer applies the common authentication choke point and
// binds authorizer to each successful result's resolved profiles.
func WithProfileAuthorizer(next Authenticator, authorizer Authorizer) Authenticator {
	return resultAuthorizingAuthenticator{next: next, authorizer: authorizer}
}

// AuthorizerForResult returns the authorization view for one successful
// authentication result. Local users retain live assignment behavior across
// reloads. Remote profiles are bound to the result. A recovery profile is valid
// only while its own user's accepted generation (result.UserGeneration) remains
// current: a change to any other local user leaves it valid.
func AuthorizerForResult(authorizer Authorizer, result AuthResult) Authorizer {
	if result.Source == SourceLocal {
		if !slices.Contains(result.Profiles, ReservedRecoveryProfile) {
			return authorizer
		}
		return acceptedGenerationAuthorizer{
			generation: result.UserGeneration,
			next:       BindProfiles(authorizer, []string{ReservedRecoveryProfile}),
		}
	}

	profiles := make([]string, 0, len(result.Profiles))
	for _, profile := range result.Profiles {
		if profile != ReservedRecoveryProfile {
			profiles = append(profiles, profile)
		}
	}
	return BindProfiles(authorizer, profiles)
}

func (p resultAuthorizingAuthenticator) Authenticate(request AuthRequest) (AuthResult, error) {
	// Fail closed: no externally-supplied username may bear the reserved prefix.
	// Such a username is a bug or an attempt to spoof a server-injected trusted
	// identity that authz.Store.Authorize permits, or a reserved recovery
	// profile. This wrapper is the one authentication choke point every surface
	// (ssh, web, api) passes through, so rejecting here stops any backend,
	// including a hostile remote TACACS+/RADIUS server, from ever making such a
	// username Authenticated. Server-injected identities never pass through
	// authentication, so they are unaffected.
	if IsReservedName(request.Username) {
		return AuthResult{}, ErrAuthRejected
	}
	result, err := p.next.Authenticate(request)
	if err == nil && result.Authenticated {
		result.Authorizer = AuthorizerForResult(p.authorizer, result)
	}
	return result, err
}
