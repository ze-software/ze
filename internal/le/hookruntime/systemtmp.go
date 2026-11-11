// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: bash.go -- bashSystemTmp, the guard that renders this judgment
// Related: shellline.go -- readerLine, the line judged by its words
package hookruntime

import (
	"slices"
	"strings"
)

// namesSystemTmp reports whether a line reaches the system temporary
// directory, by its words on a reader line and by its text otherwise.
func namesSystemTmp(command string) bool {
	line, reader := readerLine(command)
	if !reader {
		return systemTmp.MatchString(command)
	}
	for _, words := range line.commands {
		if slices.ContainsFunc(words, systemTmpWord) {
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
