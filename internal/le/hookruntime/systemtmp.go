// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: bash.go -- bashSystemTmp, the guard that renders this judgment
// Related: readerline.go -- ctxReaderLine, the line judged by its words
package hookruntime

import "strings"

// namesSystemTmp reports whether the call's command reaches the system
// temporary directory, by its words on a reader line (ctxReaderLine) and by
// its text otherwise. On a reader line every word the walk judged counts:
// arguments, loop lists, case words, assignment values, `[[` operands and
// redirection targets, while a here-document body and a here-string are data
// (AC-17). A word the shell expands is judged by its value and by its source
// with the quotes removed, because the value drops each expansion
// (`foo$D/tmp/x` reads `foo/tmp/x`, a literal parent) while the shell may
// expand it to nothing. The text pattern (systemTmp) takes `}` as a boundary
// before /tmp, because a braced parameter the shell expands to nothing
// (`${v}/tmp`) leaves /tmp itself (spec-agent-economy-6-shell-parser AC-35).
func namesSystemTmp(ctx context) bool {
	command := stringInput(ctx.input, "command")
	line, outside := ctxReaderLine(ctx)
	if outside != "" {
		return systemTmp.MatchString(command)
	}
	for _, word := range line.words {
		if systemTmpWord(word.value.text) {
			return true
		}
		if word.value.expands && systemTmpWord(withoutQuotes(word.source)) {
			return true
		}
	}
	return false
}

// systemTmpWord reports whether a word, quotes removed, can name the system
// temporary directory: the word itself, or a value after `=`, `:` or `,`, as
// in `--dir=/tmp/x` or `PATH=/bin:/tmp`, that is /tmp or starts with /tmp/,
// or that holds a /tmp directory under a parent the shell expands (`$D/tmp`,
// `${v}/tmp`, `*/tmp`, `~/tmp`), which can expand to nothing or to `/`. A
// /tmp under a literal parent (`foo/tmp/x`, `./a/tmp`) is another directory.
func systemTmpWord(word string) bool {
	for part := range strings.FieldsFuncSeq(word, func(r rune) bool { return r == '=' || r == ':' || r == ',' }) {
		if part == "/tmp" {
			return true
		}
		if strings.HasPrefix(part, "/tmp/") {
			return true
		}
		if tmpUnderExpandedParent(part) {
			return true
		}
	}
	return false
}

// tmpUnderExpandedParent reports whether a path holds a /tmp directory whose
// parent text holds a character the shell expands. Each /tmp the path holds
// is judged, so the loop is bounded by the path's length.
func tmpUnderExpandedParent(path string) bool {
	for offset := 0; offset < len(path); {
		at := strings.Index(path[offset:], "/tmp")
		if at < 0 {
			return false
		}
		end := offset + at + len("/tmp")
		if end == len(path) || path[end] == '/' {
			if strings.ContainsAny(path[:offset+at], "$`*?[~{") {
				return true
			}
		}
		offset = end
	}
	return false
}
