// Design: plan/learned/1181-fixit-bcrypt-hash-credential.md -- credential-token redaction for logs

// Package redact scrubs credential-bearing tokens from strings before they are
// logged. It owns the canonical bcrypt-shape regex (config.IsBcryptHash
// delegates here) so the pattern has a single home, and it is a leaf package
// (no ze imports) so any tier may use it.
package redact

import (
	"regexp"
	"strings"
)

// Placeholder replaces a redacted credential token in a log-safe string.
const Placeholder = "<redacted>"

// bcryptFormat matches a canonical bcrypt hash:
// $2[aby]$<cost>$<22-char salt + 31-char hash, base64url> (60 chars total).
var bcryptFormat = regexp.MustCompile(`^\$2[aby]\$\d{2}\$[./A-Za-z0-9]{53}$`)

// IsBcryptHash reports whether s is a syntactically valid bcrypt hash.
func IsBcryptHash(s string) bool {
	return bcryptFormat.MatchString(s)
}

// isCredentialKey reports whether a command token names a password-family leaf
// whose following token is the secret value: "password", any "*-password", or
// any "plaintext-*" (which covers "plaintext-password").
func isCredentialKey(token string) bool {
	lower := strings.ToLower(token)
	return lower == "password" ||
		strings.HasSuffix(lower, "-password") ||
		strings.HasPrefix(lower, "plaintext-")
}

// Command returns a copy of a command line safe to log: every bcrypt-shaped
// token is replaced with Placeholder, and the value token immediately following
// a password-family key is replaced with Placeholder even when it is a
// non-bcrypt plaintext secret. A command with no credential token is returned
// unchanged (whitespace preserved). Redaction runs on the full command; callers
// truncate the result AFTER, so a secret straddling a truncation boundary can
// never half-leak.
func Command(cmd string) string {
	fields := strings.Fields(cmd)
	changed := false
	prevKey := false
	for i, tok := range fields {
		switch {
		case IsBcryptHash(tok):
			fields[i] = Placeholder
			changed = true
		case prevKey:
			fields[i] = Placeholder
			changed = true
		}
		// Classify from the original token so a redacted value never becomes a key.
		prevKey = isCredentialKey(tok)
	}
	if !changed {
		return cmd
	}
	return strings.Join(fields, " ")
}
