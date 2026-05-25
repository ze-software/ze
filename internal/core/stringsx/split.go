// Design: docs/architecture/core-design.md -- shared core helpers
package stringsx

import (
	"strings"
	"unicode/utf8"
)

// SplitCount returns the substrings of s separated by sep and their count.
// The result matches strings.Split(s, sep), but it finds separators only once.
func SplitCount(s, sep string) ([]string, int) {
	if sep == "" {
		parts := make([]string, 0, len(s))
		for s != "" {
			_, size := utf8.DecodeRuneInString(s)
			parts = append(parts, s[:size])
			s = s[size:]
		}
		return parts, len(parts)
	}

	parts := make([]string, 0, 4)
	for {
		i := strings.Index(s, sep)
		if i < 0 {
			parts = append(parts, s)
			return parts, len(parts)
		}
		parts = append(parts, s[:i])
		s = s[i+len(sep):]
	}
}
