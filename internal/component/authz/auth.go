// Design: (none -- new authorization component)
// Overview: authz.go -- profile-based command authorization
// Related: register.go -- registers the local AAA backend with aaa.Default

package authz

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/component/aaa"
)

// Type aliases: the AAA interface layer lives in internal/component/aaa.
// authz keeps these names as aliases so existing call sites (ssh, web,
// tacacs, tests) compile unchanged. Only the ownership moved.
type (
	UserConfig         = aaa.UserCredential
	SSHPublicKey       = aaa.SSHPublicKey
	AuthResult         = aaa.AuthResult
	AuthRequest        = aaa.AuthRequest
	Authenticator      = aaa.Authenticator
	ChainAuthenticator = aaa.ChainAuthenticator
)

// ErrAuthRejected re-exports aaa.ErrAuthRejected so callers that check
// errors.Is(err, authz.ErrAuthRejected) keep working without an edit.
var ErrAuthRejected = aaa.ErrAuthRejected

// dummyHash is a pre-computed bcrypt hash used for timing-safe authentication.
// When a username is not found, we still run bcrypt against this hash to prevent
// timing side-channel attacks that could enumerate valid usernames.
var dummyHash = []byte("$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234") //nolint:gosec // not a credential

// LocalAuthenticator wraps the existing bcrypt-based user list authentication.
type LocalAuthenticator struct {
	// Users is a fixed credential list, correct only where the set cannot
	// change while the process runs.
	Users []UserConfig

	// UsersFunc returns the credentials that are valid RIGHT NOW. When set it
	// REPLACES Users rather than adding to it: a caller whose user set follows
	// the running configuration must not also carry a snapshot, because the
	// snapshot is exactly the stale answer that has to stop being given. A
	// daemon built one credential list at startup and kept consulting it, so a
	// user the operator deleted and reloaded went on logging in until restart.
	UsersFunc func() ([]UserConfig, error)
}

// users returns the credential list this authenticator answers from.
func (a *LocalAuthenticator) users() ([]UserConfig, error) {
	if a.UsersFunc != nil {
		return a.UsersFunc()
	}
	return a.Users, nil
}

// Authenticate checks username/password against the local bcrypt user list.
// Returns (result, nil) on success, (result, ErrAuthRejected) on failure.
// Timing-safe: invokes bcrypt even for unknown users.
//
// With UsersFunc set it can also return that function's error. An unreadable
// user list is NOT an empty one: treating it as empty would reject every login
// while claiming the credentials were simply wrong, so the cause is returned
// instead (ai/rules/evidence.md, "a guard must fail closed or say something").
func (a *LocalAuthenticator) Authenticate(request aaa.AuthRequest) (AuthResult, error) {
	username := request.Username
	password := request.Password

	if username == "" {
		return AuthResult{Source: aaa.SourceLocal}, ErrAuthRejected
	}
	users, err := a.users()
	if err != nil {
		return AuthResult{Source: aaa.SourceLocal}, fmt.Errorf("local: read users: %w", err)
	}
	found := false
	for _, u := range users {
		if u.Name == username {
			found = true
			if CheckPassword(u.Hash, password, request.Local) {
				return AuthResult{
					Authenticated: true,
					Profiles:      u.Profiles,
					Source:        aaa.SourceLocal,
				}, nil
			}
		}
	}
	if !found {
		// Timing-safe: always run bcrypt even for unknown users.
		bcrypt.CompareHashAndPassword(dummyHash, []byte(password)) //nolint:errcheck // result intentionally ignored
	}
	return AuthResult{Source: aaa.SourceLocal}, ErrAuthRejected
}

// CheckPassword validates a credential against a stored bcrypt hash.
// Supports two modes:
//   - Hash-as-token: credential is the bcrypt hash itself (ze cli sends the hash
//     stored in zefs). Matched by constant-time string comparison. Only tried
//     when allowHashToken is true — i.e. the connection is trusted-local
//     (loopback TCP or unix socket). Over any remote transport the hash is NOT a
//     credential, so a leaked config backup cannot be replayed as a password.
//   - Plaintext: credential is the user's password (interactive SSH terminal,
//     web login, API bearer). Matched by bcrypt comparison. Always tried,
//     regardless of transport.
//
// Returns false for empty hash or empty credential. allowHashToken defaults to
// the restrictive value at every call site because it is threaded from
// AuthRequest.Local, whose zero value is remote (fail-closed).
func CheckPassword(hash, credential string, allowHashToken bool) bool {
	if hash == "" || credential == "" {
		return false
	}
	if allowHashToken {
		// Hash-as-token: the local ze cli sends the bcrypt hash read from zefs.
		gotHash := sha256.Sum256([]byte(hash))
		credHash := sha256.Sum256([]byte(credential))
		if subtle.ConstantTimeCompare(gotHash[:], credHash[:]) == 1 {
			return true
		}
	}
	// Plaintext: interactive user typed their password.
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(credential)) == nil
}

// AuthenticateUser checks username/credential against the configured user list.
// Tries all matching entries (a user may appear in both config and zefs with
// different bcrypt hashes). Returns true on first match.
// When no username matches, bcrypt is still invoked against a dummy hash
// to prevent timing side-channel attacks on username enumeration.
//
// allowHashToken threads the same trusted-local gate as CheckPassword: the
// bcrypt-hash-as-token credential path is tried only when true. It has no
// non-test caller today, but carries the restriction so a future caller that
// wires it cannot fail open (the parameter is not optional; a caller must state
// the transport class explicitly).
func AuthenticateUser(users []UserConfig, username, credential string, allowHashToken bool) bool {
	if username == "" {
		return false
	}
	found := false
	for _, u := range users {
		if u.Name == username {
			found = true
			if CheckPassword(u.Hash, credential, allowHashToken) {
				return true
			}
		}
	}
	if !found {
		// Timing-safe: always run bcrypt even for unknown users.
		bcrypt.CompareHashAndPassword(dummyHash, []byte(credential)) //nolint:errcheck // result intentionally ignored
	}
	return false
}
