// Related: password_strength.go -- PasswordWeakness, the policy under test

package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPasswordStrengthShort: a password under the minimum length warns, and the
// boundary is exact.
//
// VALIDATES: AC-1 -- PasswordWeakness returns a reason below PasswordMinLength
// and returns none at it.
//
// PREVENTS: an off-by-one that warns on every password, or on none.
func TestPasswordStrengthShort(t *testing.T) {
	cases := []struct {
		name      string
		plaintext string
		weak      bool
	}{
		{"one character", "a", true},
		{"one below the minimum", "abcdefg", true},
		{"exactly the minimum", "abcdefgh", false},
		{"one above the minimum", "abcdefghi", false},
		{"seven multi-byte characters", strings.Repeat("é", 7), true},
		{"eight multi-byte characters", strings.Repeat("é", 8), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reason := PasswordWeakness(c.plaintext)
			if !c.weak {
				assert.Empty(t, reason, "password meeting the policy must produce no reason")
				return
			}
			require.NotEmpty(t, reason, "short password must produce a reason")
			assert.Contains(t, reason, "shorter than")
		})
	}
}

// TestPasswordStrengthDenylist: a denylisted password warns whatever its case,
// and a long one warns too.
//
// VALIDATES: AC-2 and AC-3 -- exact, case-insensitive match against the
// embedded list, independent of the length rule.
//
// PREVENTS: a case-sensitive match letting "PASSWORD" through, and a substring
// match warning on a strong password that merely contains a listed word.
func TestPasswordStrengthDenylist(t *testing.T) {
	cases := []struct {
		name      string
		plaintext string
		weak      bool
	}{
		{"lowercase entry", "password", true},
		{"uppercase entry", "PASSWORD", true},
		{"mixed case entry", "LetMeIn", true},
		{"digits entry", "12345678", true},
		{"long entry", "changeme", true},
		{"listed word as a substring", "password-of-the-day", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reason := PasswordWeakness(c.plaintext)
			if !c.weak {
				assert.Empty(t, reason, "a password that merely contains a listed word is not on the list")
				return
			}
			require.NotEmpty(t, reason, "denylisted password must produce a reason")
			assert.Contains(t, reason, "most common")
		})
	}
}

// TestPasswordStrengthStrongNoReason: a long password outside the list is
// silent, and so is the empty value.
//
// VALIDATES: AC-4 and AC-5 -- no reason for a strong password, and none for an
// empty one, which is not a password the operator chose.
//
// PREVENTS: a warning on every commit, which operators learn to ignore.
func TestPasswordStrengthStrongNoReason(t *testing.T) {
	for _, plaintext := range []string{
		"correct horse battery staple",
		"Tr0ub4dor&3!x",
		strings.Repeat("x", 72),
		"",
	} {
		assert.Empty(t, PasswordWeakness(plaintext), "no reason expected for %q", plaintext)
	}
}

// TestPasswordWeaknessNeverEchoesPlaintext: the reason names the rule, never
// the password.
//
// VALIDATES: R-3 -- the reason travels to a log line, a commit status and
// stderr, so it must carry no part of the secret.
//
// PREVENTS: a helpful "abc is too short" message writing the password into the
// journal of every box that loads the config.
func TestPasswordWeaknessNeverEchoesPlaintext(t *testing.T) {
	for _, plaintext := range []string{"hunter2", "s3kr3t", "Qwerty", "LETMEIN"} {
		reason := PasswordWeakness(plaintext)
		require.NotEmpty(t, reason, "%q is weak and must produce a reason", plaintext)
		assert.NotContains(t, strings.ToLower(reason), strings.ToLower(plaintext),
			"the reason must never echo the password")
	}
}
