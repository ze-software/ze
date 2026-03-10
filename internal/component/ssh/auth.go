// Design: (none -- new SSH server component)
// Overview: ssh.go -- SSH server lifecycle

package ssh

import (
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

// CheckPassword validates a plaintext password against a bcrypt hash.
// Returns false for empty hash or empty password.
func CheckPassword(hash, password string) bool {
	if hash == "" || password == "" {
		return false
	}
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// AuthenticateUser checks username/password against the configured user list.
// Returns true if the username exists and the password matches the bcrypt hash.
// When the username is not found, bcrypt is still invoked against a dummy hash
// to prevent timing side-channel attacks on username enumeration.
func AuthenticateUser(users []UserConfig, username, password string) bool {
	if username == "" {
		return false
	}
	for _, u := range users {
		if u.Name == username {
			return CheckPassword(u.Hash, password)
		}
	}
	// Timing-safe: always run bcrypt even for unknown users.
	bcrypt.CompareHashAndPassword(dummyHash, []byte(password)) //nolint:errcheck // result intentionally ignored
	return false
}
