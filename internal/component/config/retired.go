// Design: docs/architecture/config/syntax.md — config parsing and loading
// Related: parser.go — hierarchical parse, unknown-field refusal
// Related: setparser.go — set-format parse, unknown-field refusal

package config

import "github.com/ze-software/ze/internal/core/textbuf"

// retiredKeywords maps a config keyword ze no longer accepts to the spelling
// that replaces it. A retired keyword parses as an unknown field, and the field
// name alone does not tell an operator what to write instead.
//
// The parser accepts ONE spelling and this table is the MESSAGE, never a second
// grammar. Ze rewrites no file: the operator edits the keyword, which is why the
// replacement has to be in the refusal.
var retiredKeywords = map[string]string{
	"process": "attach process",
	// Administrative distance had two containers declaring the same numbers,
	// `bgp { admin-distance }` and `rib { admin-distance }`, and which one
	// decided depended on whether the rib block existed. One declaration
	// survives, under `rib`, and it is named for the quantity rather than for
	// its adjective. The YANG validator cannot deliver this message: walkTree
	// (yang/validator.go) iterates the SCHEMA's children and never the data, so
	// it emits nothing for a key it does not know.
	"admin-distance": "distance, inside rib { distance { ... } }, which now declares every protocol",
	// A pin names one certificate and dies with it. The client now names the
	// hub's certificate authority instead, so a reissued leaf still validates.
	"certificate-fingerprint": "ca <pki-ca-name>, naming the certificate authority root exported from the hub",
}

// RetiredKeywordHint returns the sentence a parse error carries when an unknown
// field names a retired keyword, or "" when the name was never a keyword ze
// accepted.
func RetiredKeywordHint(name string) string {
	replacement, ok := retiredKeywords[name]
	if !ok {
		return ""
	}
	var b textbuf.Buffer
	return b.Str(" (").Str(name).Str(" is retired: write ").Str(replacement).
		Str(" instead)").String()
}
