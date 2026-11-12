// Design: docs/architecture/config/yang-config-design.md -- ze: extensions
// Related: refusal.go -- a refusal of a secret leaf shows SecretValuePlaceholder

package yang

import (
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"
)

// SecretValuePlaceholder is what every surface prints in place of a secret
// value: the editor's display, a refusal's value slot. It is declared here,
// the leaf tier both the schema and the walk import, and the config package
// names it config.SecretDataPlaceholder.
const SecretValuePlaceholder = "/* SECRET-DATA */" //nolint:gosec // not a credential

// HasSensitiveExtension reports whether entry carries ze:sensitive: its value
// is a password or a key, masked wherever it is shown.
func HasSensitiveExtension(entry *gyang.Entry) bool {
	return hasExtension(entry, "sensitive")
}

// HasBcryptExtension reports whether entry carries ze:bcrypt: its value is a
// one-way hash, or the plaintext the commit hashes.
func HasBcryptExtension(entry *gyang.Entry) bool {
	return hasExtension(entry, "bcrypt")
}

// EntryHoldsSecret reports whether entry's value is a secret a refusal must
// not echo: ze:sensitive or ze:bcrypt.
func EntryHoldsSecret(entry *gyang.Entry) bool {
	if HasSensitiveExtension(entry) {
		return true
	}
	return HasBcryptExtension(entry)
}

// hasExtension reports whether entry carries the ze extension named keyword,
// spelled with any prefix.
func hasExtension(entry *gyang.Entry, keyword string) bool {
	if entry == nil {
		return false
	}
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:"+keyword || strings.HasSuffix(ext.Keyword, ":"+keyword) {
			return true
		}
	}
	return false
}
