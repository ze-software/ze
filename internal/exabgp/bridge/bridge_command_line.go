// Design: docs/architecture/exabgp-bridge.md -- the script's command line
// Related: bridge_neighbor.go -- SplitCommandLine, the inverse this pairs with
//
// A migrated ExaBGP `run` line is an argv: ExaBGP hands the program and every
// word after it to subprocess.Popen, with no shell. Ze stores the line as one
// string in the bridge's `run` leaf, so the migration writes the argv as a
// POSIX command line and the bridge runner splits it back with SplitCommandLine.

package bridge

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// JoinCommandLine writes argv as one command line that SplitCommandLine, and a
// POSIX shell, read back as the same argv. A word made only of characters no
// shell treats specially is written bare; any other word, the empty word
// included, is written in single quotes, with each apostrophe written as `'\”`.
// This is Python's shlex.quote, so the line reads the way ExaBGP users expect.
func JoinCommandLine(argv []string) string {
	var b textbuf.Buffer
	for i, word := range argv {
		if i > 0 {
			b.Byte(' ')
		}
		if isBareWord(word) {
			b.Str(word)
			continue
		}
		b.Byte('\'')
		b.Str(strings.ReplaceAll(word, `'`, `'\''`))
		b.Byte('\'')
	}
	return b.String()
}

// isBareWord reports whether word needs no quoting: it is not empty and every
// byte is in shlex.quote's safe set, letters, digits and `@%+=:,./-_`.
func isBareWord(word string) bool {
	if word == "" {
		return false
	}
	for i := range len(word) {
		if !isBareByte(word[i]) {
			return false
		}
	}
	return true
}

// isBareByte reports whether c is in shlex.quote's safe set.
func isBareByte(c byte) bool {
	if c >= 'a' && c <= 'z' {
		return true
	}
	if c >= 'A' && c <= 'Z' {
		return true
	}
	if c >= '0' && c <= '9' {
		return true
	}
	return strings.IndexByte("@%+=:,./-_", c) >= 0
}
