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

var acceptedLocalProfileGeneration atomic.Uint64

// SetAcceptedLocalProfileGeneration advances the generation against which
// local recovery grants are valid. Remote login-resolved profiles are not
// generation-bound.
func SetAcceptedLocalProfileGeneration(generation uint64) {
	acceptedLocalProfileGeneration.Store(generation)
}

type acceptedGenerationAuthorizer struct {
	generation uint64
	next       Authorizer
}

func (a acceptedGenerationAuthorizer) Authorize(username, remoteAddr, command string, isReadOnly bool) bool {
	if a.generation == 0 || a.generation != acceptedLocalProfileGeneration.Load() {
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
	if a.generation == 0 || a.generation != acceptedLocalProfileGeneration.Load() {
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
// only while its accepted local credential generation remains current.
func AuthorizerForResult(authorizer Authorizer, result AuthResult) Authorizer {
	if result.Source == SourceLocal {
		if !slices.Contains(result.Profiles, ReservedRecoveryProfile) {
			return authorizer
		}
		return acceptedGenerationAuthorizer{
			generation: result.LocalGeneration,
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
