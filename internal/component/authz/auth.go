// Design: (none -- new authorization component)
// Overview: authz.go -- profile-based command authorization
// Related: register.go -- registers the local AAA backend with aaa.Default

package authz

import (
	"crypto/subtle"

	"golang.org/x/crypto/bcrypt"

	"codeberg.org/thomas-mangin/ze/internal/component/aaa"
)

// Type aliases: the AAA interface layer lives in internal/component/aaa.
// authz keeps these names as aliases so existing call sites (ssh, web,
// tacacs, tests) compile unchanged. Only the ownership moved.
type (
	UserConfig         = aaa.UserCredential
	AuthResult         = aaa.AuthResult
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
	Users []UserConfig
}

// Authenticate checks username/password against local bcrypt user list.
// Returns (result, nil) on success, (result, ErrAuthRejected) on failure.
// Never returns a connection error (local auth has no infrastructure failures).
// Timing-safe: invokes bcrypt even for unknown users.
func (a *LocalAuthenticator) Authenticate(username, password string) (AuthResult, error) {
	if username == "" {
		return AuthResult{Source: "local"}, ErrAuthRejected
	}
	found := false
	for _, u := range a.Users {
		if u.Name == username {
			found = true
			if CheckPassword(u.Hash, password) {
				return AuthResult{
					Authenticated: true,
					Profiles:      u.Profiles,
					Source:        "local",
				}, nil
			}
		}
	}
	if !found {
		// Timing-safe: always run bcrypt even for unknown users.
		bcrypt.CompareHashAndPassword(dummyHash, []byte(password)) //nolint:errcheck // result intentionally ignored
	}
	return AuthResult{Source: "local"}, ErrAuthRejected
}

// CheckPassword validates a credential against a stored bcrypt hash.
// Supports two modes:
//   - Hash-as-token: credential is the bcrypt hash itself (ze cli sends the hash
//     stored in zefs). Matched by constant-time string comparison.
//   - Plaintext: credential is the user's password (interactive SSH terminal).
//     Matched by bcrypt comparison.
//
// Returns false for empty hash or empty credential.
func CheckPassword(hash, credential string) bool {
	if hash == "" || credential == "" {
		return false
	}
	// Hash-as-token: ze cli sends the bcrypt hash read from zefs.
	if subtle.ConstantTimeCompare([]byte(hash), []byte(credential)) == 1 {
		return true
	}
	// Plaintext: interactive user typed their password.
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(credential)) == nil
}

// AuthenticateUser checks username/credential against the configured user list.
// Tries all matching entries (a user may appear in both config and zefs with
// different bcrypt hashes). Returns true on first match.
// When no username matches, bcrypt is still invoked against a dummy hash
// to prevent timing side-channel attacks on username enumeration.
func AuthenticateUser(users []UserConfig, username, credential string) bool {
	if username == "" {
		return false
	}
	found := false
	for _, u := range users {
		if u.Name == username {
			found = true
			if CheckPassword(u.Hash, credential) {
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
