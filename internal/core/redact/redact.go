// Design: plan/learned/1181-fixit-bcrypt-hash-credential.md -- credential-token redaction for logs

// Package redact scrubs credential-bearing tokens from strings before they are
// logged. It owns the canonical bcrypt-shape regex (config.IsBcryptHash
// delegates here) so the pattern has a single home, and it is a leaf package
// (no ze imports) so any tier may use it.
package redact

import (
	"bytes"
	"encoding/json"
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

// isSecretConfigKey reports whether a config leaf name holds a secret value.
// It is the one predicate behind both command-token redaction and JSON payload
// redaction, so a leaf that is secret in a `set` line is secret in a captured
// config payload too.
//
// The bare word "key" is deliberately absent: it names a key-chain entry id and
// a YANG list key far more often than it names a secret, and every real secret
// leaf in the tree spells the suffix (`md5-key`, `auth-key`, `pre-shared-key`).
func isSecretConfigKey(name string) bool {
	lower := strings.ToLower(name)
	switch lower {
	case "password", "secret", "passphrase", "psk", "md5", "token":
		return true
	}
	return strings.HasSuffix(lower, "-password") ||
		strings.HasSuffix(lower, "-secret") ||
		strings.HasSuffix(lower, "-passphrase") ||
		strings.HasSuffix(lower, "-key")
}

// isCredentialKey reports whether a command token names a secret-bearing leaf
// whose following token is the secret value. It is isSecretConfigKey plus the
// command-line-only "plaintext-*" prefix (which covers "plaintext-password").
func isCredentialKey(token string) bool {
	return isSecretConfigKey(token) || strings.HasPrefix(strings.ToLower(token), "plaintext-")
}

// JSON returns payload with every secret-bearing value replaced by Placeholder:
// any value whose key satisfies isSecretConfigKey, at any depth, plus any string
// value that is bcrypt-shaped wherever it appears.
//
// It fails CLOSED. When payload is not valid JSON there is no way to tell a
// secret from anything else, so the returned bytes are a bare Placeholder string
// and the error is non-nil. A caller that ignores the error still cannot leak,
// because the returned bytes never carry input content.
func JSON(payload []byte) ([]byte, error) {
	var tree any
	if err := json.Unmarshal(payload, &tree); err != nil {
		return failClosed(), err
	}
	// HTML escaping is off deliberately: it would emit the placeholder as
	// "<redacted>", which reads as noise in an operator-facing file and
	// disagrees with every other encoder in the capture path.
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(redactValue(tree, false)); err != nil {
		return failClosed(), err
	}
	return bytes.TrimRight(out.Bytes(), "\n"), nil
}

// placeholderJSON is Placeholder as a JSON string, folded at compile time so
// there is no second spelling of the placeholder text.
const placeholderJSON = `"` + Placeholder + `"`

// failClosed is the output when nothing about the input can be trusted: a bare
// placeholder string, carrying none of the input.
func failClosed() []byte { return []byte(placeholderJSON) }

// redactValue walks a decoded JSON tree. secret is true when the value arrived
// under a secret-bearing key, in which case the whole subtree is replaced: a
// secret spelled as an object or a list must not survive by changing shape.
func redactValue(v any, secret bool) any {
	if secret {
		return Placeholder
	}
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, child := range t {
			out[k] = redactValue(child, isSecretConfigKey(k))
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, child := range t {
			out[i] = redactValue(child, false)
		}
		return out
	case string:
		if IsBcryptHash(t) {
			return Placeholder
		}
		return t
	default:
		return v
	}
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
