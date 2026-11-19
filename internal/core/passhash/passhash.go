// Design: docs/guide/authentication.md -- a migrated account keeps its SHA-crypt hash
// Related: passhash_test.go -- Check and Verify over every accepted format

// Package passhash answers for a stored password hash in every format a
// ze:bcrypt config leaf accepts: bcrypt, which Ze writes, and SHA-crypt ($5$,
// $6$), which Ze only reads, so an account migrated from another system keeps
// its password. Every consumer of a ze:bcrypt leaf, the config validator
// included, asks this package, so a value the validator accepts is a value
// every login check can verify.
package passhash

import (
	"errors"

	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/core/redact"
	"github.com/ze-software/ze/internal/core/shacrypt"
)

// ErrNotPasswordHash reports a value in none of the accepted formats, such as a
// plaintext password written where the hash belongs.
var ErrNotPasswordHash = errors.New("not a bcrypt hash or a SHA-crypt hash ($5$, $6$)")

// Check returns nil when stored is a hash Verify can answer for. A value that
// names SHA-crypt and is malformed returns the reason; any other value that is
// not a bcrypt hash returns ErrNotPasswordHash. No error carries the value.
func Check(stored string) error {
	_, err := shacrypt.Parse(stored)
	if !errors.Is(err, shacrypt.ErrNotSHACrypt) {
		return err
	}
	if redact.IsBcryptHash(stored) {
		return nil
	}
	return ErrNotPasswordHash
}

// Verify reports whether password matches stored, in the format the hash
// names by its prefix. A value that names SHA-crypt is verified as SHA-crypt,
// a malformed one included, which matches nothing. Every other value goes to
// bcrypt, which refuses what is not a bcrypt hash. Both comparisons take
// constant time over the digest.
//
// The time a call takes depends on the format and its cost: a bcrypt hash at
// its cost, a SHA-crypt hash at its rounds. A caller that hides whether an
// account exists cannot do it by timing for an account whose hash costs a
// different time than the caller's dummy comparison.
func Verify(stored, password string) bool {
	parsed, err := shacrypt.Parse(stored)
	if errors.Is(err, shacrypt.ErrNotSHACrypt) {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)) == nil
	}
	if err != nil {
		return false
	}
	return parsed.Matches([]byte(password))
}
