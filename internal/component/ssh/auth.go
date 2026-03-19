// Design: (none -- new SSH server component)
// Overview: ssh.go -- SSH server lifecycle

package ssh

import (
	"crypto/subtle"

	"golang.org/x/crypto/bcrypt"
)

// dummyHash is a pre-computed bcrypt hash used for timing-safe authentication.
// When a username is not found, we still run bcrypt against this hash to prevent
// timing side-channel attacks that could enumerate valid usernames.
var dummyHash = []byte("$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234") //nolint:gosec // not a credential

// UserConfig holds a configured SSH user's credentials.
type UserConfig struct {
	Name string
	Hash string // bcrypt hash of the user's credential
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
