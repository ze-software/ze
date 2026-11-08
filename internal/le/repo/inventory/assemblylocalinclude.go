// Design: docs/architecture/plugin/feature-gates.md -- a package-local assembly header is read as text at its include line.
// Related: assemblyheaders.go -- the compiler header boundary a local header never shadows.
// Related: assemblymacros.go -- the local macro rules a header's #define lines follow.
package repoinventory

import (
	"bytes"
	"fmt"
	"path"
	"strings"
)

// includeLocal applies `#include "name"` for a header other than a compiler
// header. cmd/asm (src/cmd/asm/internal/lex/input.go, Input.include) opens the
// quoted name as given before any -I directory (`fd, err := os.Open(name)`)
// and pushes a tokenizer for it onto the input stack
// (`in.Push(NewTokenizer(name, fd, fd))`), so the header's lines are read as
// if they stood at the include line. cmd/go runs the assembler in the package
// directory (src/cmd/go/internal/work/gc.go, gcToolchain.asm,
// `b.Shell(a).run(p.Dir, p.ImportPath, cfgChangedEnv, args1...)`), so a plain
// name opens the file beside the including .s file.
//
// The header is read from this snapshot's committed blob, never the host disk.
// It may hold only #define and #undef lines, which bind and remove local
// macros under the .s file's own rules. A nested #include is refused, so an
// include cycle cannot form; every other statement is refused, because its
// bytes would join a definition body that is retained only from the .s file.
func (unit *assemblyNamespace) includeLocal(name string) error {
	if name == "" {
		return fmt.Errorf("unproved assembly include %q: empty header name", name)
	}
	// A backslash is refused too: the include token is the raw Go string
	// literal, and cmd/asm unquotes it (strconv.Unquote), so an escape could
	// name another file than the text here.
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("unproved assembly include %q: a local header name holds no path separator or escape", name)
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("unproved assembly include %q: a local header name holds no \"..\" element", name)
	}
	header := path.Join(unit.pkg.dir, name)
	if !unit.headers.snapshot.localHeaders[header] {
		return fmt.Errorf("unproved assembly include %q: %s is not a committed regular file beside the including assembly", name, header)
	}
	raw := unit.headers.snapshot.blobs[header]
	// A header whose last line has no newline would let the assembler join
	// that line to the including file's next one, which this census does not
	// model.
	if len(raw) > 0 && raw[len(raw)-1] != '\n' {
		return fmt.Errorf("%s: local assembly header does not end with a newline", header)
	}
	return assemblyStatements(raw, header, func(statement assemblyStatement) error {
		if err := unit.localHeaderStatement(statement); err != nil {
			line := bytes.Count(raw[:statement.start], []byte{'\n'}) + 1
			return fmt.Errorf("%s:%d: %w", header, line, err)
		}
		return nil
	})
}

// localHeaderStatement admits one logical statement of a local header.
func (unit *assemblyNamespace) localHeaderStatement(statement assemblyStatement) error {
	tokens := statement.tokens
	if tokens[0] != "#" {
		return fmt.Errorf("local assembly header holds a statement other than #define or #undef")
	}
	if assemblyDefine(tokens) {
		return assemblyMacro(statement, unit)
	}
	if assemblyUndef(tokens) {
		return unit.undefine(tokens[2])
	}
	if len(tokens) >= 2 && tokens[1] == "include" {
		return fmt.Errorf("nested assembly include in a local header is not proved")
	}
	return fmt.Errorf("unsupported assembly preprocessing in a local header")
}
