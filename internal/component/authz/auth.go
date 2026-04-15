// Design: (none -- new authorization component)
// Overview: authz.go -- profile-based command authorization

package authz

import (
	"crypto/subtle"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// dummyHash is a pre-computed bcrypt hash used for timing-safe authentication.
// When a username is not found, we still run bcrypt against this hash to prevent
// timing side-channel attacks that could enumerate valid usernames.
var dummyHash = []byte("$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234") //nolint:gosec // not a credential

// ErrAuthRejected is returned when a backend explicitly rejects credentials.
// This is distinct from connection errors: a rejected authentication MUST NOT
// fall through to the next backend in the chain.
var ErrAuthRejected = errors.New("authentication rejected")

// UserConfig holds a configured user's credentials.
type UserConfig struct {
	Name     string
	Hash     string   // bcrypt hash of the user's credential
	Profiles []string // ze authz profile names
}

// AuthResult holds the outcome of an authentication attempt.
type AuthResult struct {
	Authenticated bool
	Profiles      []string // ze authz profiles for this user
	Source        string   // backend identifier ("local", "tacacs")
}

// Authenticator is a pluggable authentication backend.
// Implementations: LocalAuthenticator (bcrypt), TacacsAuthenticator (TACACS+).
type Authenticator interface {
	// Authenticate checks the username/password against this backend.
	// Returns (result, nil) on success or explicit rejection.
	// Returns (zero, error) on connection/infrastructure failure.
	// An explicit rejection sets Authenticated=false and returns ErrAuthRejected.
	Authenticate(username, password string) (AuthResult, error)
}

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

// ChainAuthenticator tries backends in order. It distinguishes two failure modes:
//   - Explicit rejection (ErrAuthRejected): stop immediately, do not try next backend.
//   - Connection error (any other error): try next backend.
//
// First successful authentication wins.
type ChainAuthenticator struct {
	Backends []Authenticator
}

// Authenticate tries each backend in order.
// Returns on first success or first explicit rejection (ErrAuthRejected).
// Only connection errors cause fallthrough to the next backend.
func (c *ChainAuthenticator) Authenticate(username, password string) (AuthResult, error) {
	if len(c.Backends) == 0 {
		return AuthResult{}, fmt.Errorf("no authentication backends configured")
	}
	var lastErr error
	for _, backend := range c.Backends {
		result, err := backend.Authenticate(username, password)
		if err == nil && result.Authenticated {
			return result, nil
		}
		if errors.Is(err, ErrAuthRejected) {
			// Explicit rejection: stop chain, do not try next backend.
			return result, ErrAuthRejected
		}
		// Connection/infrastructure error: try next backend.
		lastErr = err
	}
	if lastErr != nil {
		return AuthResult{}, fmt.Errorf("all authentication backends failed: %w", lastErr)
	}
	return AuthResult{}, fmt.Errorf("all authentication backends failed")
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
