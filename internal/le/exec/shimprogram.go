// Design: docs/guide/developer-setup.md -- le exec: the launcher the tool shims run
// Related: exec.go -- Program, the caller; resolve and IsShim, the PATH walk this mirrors
// Related: ../setup/shims.go -- shimScript, the writer of the line this file reads

package leexec

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// shimRecorded answers the tool the first PATH entry named name records, when
// that entry is a shim `le setup` wrote: the program its `exec '<le>' exec
// '<program>' "$@"` line starts (shimScript). The shim runs that path, never
// the next name on PATH, so a caller that must run the same binary as the
// shim (the post-edit lint, Program) runs it too. It answers false for a name
// holding a separator, for a first entry that is the real tool (resolve then
// finds it), and for a shim whose line it cannot read, which resolve then
// skips as it always has.
func shimRecorded(name, searchPath string) (string, bool) {
	if strings.ContainsRune(name, filepath.Separator) {
		return "", false
	}
	for _, directory := range filepath.SplitList(searchPath) {
		if directory == "" {
			continue // an empty entry means the working directory, which le never searches
		}
		candidate := filepath.Join(directory, name)
		if executableFile(candidate) != nil {
			continue
		}
		shim, err := IsShim(candidate)
		if err != nil {
			continue
		}
		if !shim {
			return "", false
		}
		return shimLineProgram(candidate)
	}
	return "", false
}

// shimLineProgram reads the program the shim at path starts: the third word
// of its `exec` line, after le's path and le's `exec`, each single-quoted as
// shimScript writes it.
func shimLineProgram(path string) (string, bool) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: a shim the PATH walk found, read to learn its tool
	if err != nil {
		return "", false
	}
	for line := range strings.Lines(string(data)) {
		rest, found := strings.CutPrefix(strings.TrimSpace(line), "exec ")
		if !found {
			continue
		}
		_, rest, read := singleQuoted(rest)
		if !read {
			return "", false
		}
		rest, found = strings.CutPrefix(rest, " exec ")
		if !found {
			return "", false
		}
		program, _, read := singleQuoted(rest)
		if !read || !filepath.IsAbs(program) {
			return "", false
		}
		return program, true
	}
	return "", false
}

// singleQuoted reads one shell word written in single quotes from the start
// of text, the way shimScript writes one (shellSingleQuote: `'...'`, with a
// quote spelled `'\”`), and answers the word, the text after it, and whether
// a word was read. Each step consumes at least one byte, so the loop is
// bounded by the text's length.
func singleQuoted(text string) (string, string, bool) {
	var word textbuf.Buffer
	at, read := 0, false
	for at < len(text) {
		if strings.HasPrefix(text[at:], `\'`) {
			word.Byte('\'')
			at += 2
			continue
		}
		if text[at] != '\'' {
			break
		}
		end := strings.IndexByte(text[at+1:], '\'')
		if end < 0 {
			return "", "", false
		}
		word.Str(text[at+1 : at+1+end])
		at += end + 2
		read = true
	}
	return word.String(), text[at:], read
}
