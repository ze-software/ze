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

import "sync"

// loginProfiles maps username -> authz profile names resolved at authentication.
//
// Keyed by username alone, not by session: authorizers receive a username and a
// remote address, but the remote address of a command is not the address of the
// login for every surface. A username is one identity here -- two sessions for
// the same name are the same user -- so the last successful authentication wins.
//
// Entries are not evicted on logout. The map is bounded by the number of distinct
// usernames that have ever authenticated successfully, an entry costs a name and
// a short slice, and a stale entry cannot outlive its meaning: it holds names,
// and a name that no longer resolves in the live store contributes nothing.
var loginProfiles sync.Map // string -> []string

// RecordLoginProfiles stores the authz profile names a backend resolved for a
// successful authentication. Build wraps the composed authenticator so this is
// called for every surface (ssh, web, api) rather than at each call site.
//
// An authentication that resolves no profiles records nothing: it must not erase
// a mapping from an earlier login that did resolve some, and an empty entry is
// indistinguishable from "never seen" to the reader anyway.
func RecordLoginProfiles(username string, profiles []string) {
	if username == "" || len(profiles) == 0 {
		return
	}
	// Copy: the caller's slice belongs to an AuthResult that it may reuse.
	stored := make([]string, len(profiles))
	copy(stored, profiles)
	loginProfiles.Store(username, stored)
}

// LoginProfiles returns the profile names recorded for username at its last
// successful authentication. The returned slice is read-only; callers must not
// mutate it.
func LoginProfiles(username string) ([]string, bool) {
	v, ok := loginProfiles.Load(username)
	if !ok {
		return nil, false
	}
	profiles, ok := v.([]string)
	if !ok || len(profiles) == 0 {
		return nil, false
	}
	return profiles, true
}

// ForgetLoginProfilesForTest drops the recorded profiles for username. Exported for
// tests, which must not leak identities into each other through this map.
func ForgetLoginProfilesForTest(username string) {
	loginProfiles.Delete(username)
}

// profileRecordingAuthenticator wraps an authenticator so successful
// authentication publishes its resolved profiles to authorization.
type profileRecordingAuthenticator struct {
	next Authenticator
}

// WithProfileRecording wraps next with the common authentication choke point
// that rejects reserved usernames and publishes successful login profiles.
// Registry-built authentication and direct local API authentication must both
// use this function so neither behavior can drift between construction paths.
func WithProfileRecording(next Authenticator) Authenticator {
	return profileRecordingAuthenticator{next: next}
}

func (p profileRecordingAuthenticator) Authenticate(request AuthRequest) (AuthResult, error) {
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
		// NOTE: do NOT FilterReservedNames(result.Profiles) here. The trusted local
		// backend legitimately delivers the reserved break-glass recovery profile
		// through this exact path (UserCredential.Profiles -> AuthResult.Profiles;
		// cmd/ze/hub/main_servers.go usersFromZefsDB), so a central strip here would
		// erase the recovery grant and lock out the bootstrap admin. Reserved-name
		// filtering therefore lives in each UNTRUSTED wire backend (radius mapProfiles,
		// tacacs handlePass), which never has a legitimate reason to emit one.
		RecordLoginProfiles(request.Username, result.Profiles)
	}
	return result, err
}
