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
