// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// Related: loader_structure.go — extensionSubstatementError asks whether a statement has a block
// RFC: rfc/short/rfc7950.md -- Section 14, the block a statement rule requires
package yang

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/openconfig/goyang/pkg/yang"
)

// statementHasBlock reports whether statement, in the module text sources
// holds for its file, closes with a "{" block rather than ";". goyang's
// parser returns the same Statement for "refine x;" and "refine x {}", so
// the text is read again from the statement's keyword: the keyword, the
// argument when there is one (quoted strings joined by "+", or one unquoted
// string), then ";" or "{" (RFC 7950 Section 6.1.3 and Section 14,
// stmtend). A text it cannot follow is an error, never a guess.
func statementHasBlock(sources map[string]string, statement *yang.Statement) (bool, error) {
	file, line, col, err := statementPosition(statement.Location())
	if err != nil {
		return false, err
	}
	text, recorded := sources[file]
	if !recorded {
		return false, fmt.Errorf("the text of %q was not recorded, so its block cannot be read", file)
	}
	at, err := runeOffset(text, line, col)
	if err != nil {
		return false, fmt.Errorf("%s: %w", statement.Location(), err)
	}
	if !strings.HasPrefix(text[at:], statement.Keyword) {
		return false, fmt.Errorf("%s: the text there is not %s", statement.Location(), statement.Keyword)
	}
	scan := sourceScan{text: text, at: at + len(statement.Keyword)}
	scan.skipSpace()
	if statement.HasArgument {
		if err := scan.skipArgument(); err != nil {
			return false, fmt.Errorf("%s: %w", statement.Location(), err)
		}
		scan.skipSpace()
	}
	if scan.at == len(text) {
		return false, fmt.Errorf("%s: the text ends before ';' or '{'", statement.Location())
	}
	if text[scan.at] == '{' {
		return true, nil
	}
	if text[scan.at] == ';' {
		return false, nil
	}
	return false, fmt.Errorf("%s: %q follows the argument, not ';' or '{'", statement.Location(), text[scan.at])
}

// statementPosition splits a goyang Statement.Location, "file:line:col" or
// "line L:C" for a text parsed with no name, into its parts.
func statementPosition(location string) (file string, line, col int, err error) {
	rest := location
	if after, unnamed := strings.CutPrefix(location, "line "); unnamed {
		rest = ":" + after
	}
	colAt := strings.LastIndexByte(rest, ':')
	if colAt < 0 {
		return "", 0, 0, fmt.Errorf("location %q names no position", location)
	}
	lineAt := strings.LastIndexByte(rest[:colAt], ':')
	if lineAt < 0 {
		return "", 0, 0, fmt.Errorf("location %q names no line", location)
	}
	line, err = strconv.Atoi(rest[lineAt+1 : colAt])
	if err != nil {
		return "", 0, 0, fmt.Errorf("location %q: %w", location, err)
	}
	col, err = strconv.Atoi(rest[colAt+1:])
	if err != nil {
		return "", 0, 0, fmt.Errorf("location %q: %w", location, err)
	}
	return rest[:lineAt], line, col, nil
}

// runeOffset answers the byte offset of line and col, both 1-based, in text.
// goyang counts a column in runes, a tab as one.
func runeOffset(text string, line, col int) (int, error) {
	at := 0
	for range line - 1 {
		newline := strings.IndexByte(text[at:], '\n')
		if newline < 0 {
			return 0, fmt.Errorf("line %d is past the end of the text", line)
		}
		at += newline + 1
	}
	for range col - 1 {
		if at >= len(text) {
			return 0, fmt.Errorf("column %d is past the end of line %d", col, line)
		}
		_, size := utf8.DecodeRuneInString(text[at:])
		at += size
	}
	return at, nil
}

// sourceScan reads YANG text from at. Every loop consumes at least one
// byte, so a scan ends within the text's length.
type sourceScan struct {
	text string
	at   int
}

// skipSpace skips whitespace and comments, "//" to the end of the line and
// "/*" to "*/" (RFC 7950 Section 6.1.1).
func (s *sourceScan) skipSpace() {
	for s.at < len(s.text) {
		rest := s.text[s.at:]
		if isSep(rest[0]) {
			s.at++
			continue
		}
		if strings.HasPrefix(rest, "//") {
			newline := strings.IndexByte(rest, '\n')
			if newline < 0 {
				s.at = len(s.text)
				return
			}
			s.at += newline + 1
			continue
		}
		if strings.HasPrefix(rest, "/*") {
			closing := strings.Index(rest[2:], "*/")
			if closing < 0 {
				s.at = len(s.text)
				return
			}
			s.at += 2 + closing + 2
			continue
		}
		return
	}
}

// skipArgument skips one argument: quoted strings joined by "+", or one
// unquoted string (RFC 7950 Section 6.1.3).
func (s *sourceScan) skipArgument() error {
	if s.at == len(s.text) {
		return errors.New("the text ends before the argument")
	}
	if s.text[s.at] != '"' && s.text[s.at] != '\'' {
		for s.at < len(s.text) && strings.IndexByte(" \t\r\n;{", s.text[s.at]) < 0 {
			s.at++
		}
		return nil
	}
	for {
		if err := s.skipQuoted(); err != nil {
			return err
		}
		mark := s.at
		s.skipSpace()
		if s.at == len(s.text) || s.text[s.at] != '+' {
			s.at = mark
			return nil
		}
		s.at++
		s.skipSpace()
	}
}

// skipQuoted skips one quoted string. A double-quoted string ends at the
// first '"' no backslash escapes; a single-quoted one at the next '\”.
func (s *sourceScan) skipQuoted() error {
	if s.at == len(s.text) {
		return errors.New("the text ends before a quoted string")
	}
	quote := s.text[s.at]
	if quote != '"' && quote != '\'' {
		return fmt.Errorf("%q follows '+', not a quoted string", quote)
	}
	s.at++
	for s.at < len(s.text) {
		c := s.text[s.at]
		if quote == '"' && c == '\\' {
			s.at += 2
			continue
		}
		s.at++
		if c == quote {
			return nil
		}
	}
	return errors.New("a quoted string is not closed")
}
