// Design: docs/guide/authentication.md -- the advisory password weakness warning
// Related: password_hash.go -- ApplyPasswordHashing, the walk that calls this
// Related: internal/plugins/passwd/main.go -- `ze passwd` calls this too

package config

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// PasswordMinLength is the shortest password that ze accepts without a
// weakness warning, counted in characters rather than bytes so a short
// multi-byte password cannot pass on its UTF-8 encoding length alone.
//
// The value is a policy default, not a protocol constant: raising it warns on
// more passwords and never rejects one, because the warning is advisory at
// every call site.
const PasswordMinLength = 8

// passwordDenylist holds the passwords ze warns about by name, matched
// case-insensitively and in full. It is a short fixed list of the choices an
// attacker tries first, never a dictionary: a dictionary would have to be
// embedded, kept current, and searched, and it would still miss the next
// leak. Length is the rule that covers the rest.
var passwordDenylist = [...]string{
	"password",
	"123456",
	"12345678",
	"qwerty",
	"admin",
	"letmein",
	"root",
	"changeme",
}

// PasswordWeakness returns a short operator-facing reason why plaintext is a
// weak password, and returns an empty string when it meets the policy above.
//
// The check is total: every input has a definite answer, so it reports no
// error and an empty reason means "this password met the policy" rather than
// "the check could not run". The reason describes the RULE the password
// failed and never quotes, echoes or measures the password itself, because it
// is written to a log line, a commit status and stderr.
//
// The result is advisory at every call site. A caller MUST warn on it and MUST
// NOT refuse the password, which is what keeps an existing config valid.
func PasswordWeakness(plaintext string) string {
	if plaintext == "" {
		// An empty value is not a weak password. The two callers already
		// decide what it means: the config walk hashes nothing and deletes the
		// ephemeral leaf, and `ze passwd` refuses it outright.
		return ""
	}
	for _, common := range passwordDenylist {
		if strings.EqualFold(plaintext, common) {
			return "one of the most common passwords"
		}
	}
	if utf8.RuneCountInString(plaintext) < PasswordMinLength {
		return "shorter than " + strconv.Itoa(PasswordMinLength) + " characters"
	}
	return ""
}
