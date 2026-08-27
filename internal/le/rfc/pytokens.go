// Design: docs/architecture/core-design.md -- reading Python comments without Python
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
//
// pytokens.go is the part of Python's `tokenize` that ScanPythonTags depends
// on: where the comments are, and whether the file is one the tokenizer would
// have accepted at all.
//
// Go has no Python parser, and the alternative -- scanning lines for `#` -- is
// the failure the Python reader was written to avoid. A scenario check quotes
// vtysh output, JSON bodies and protocol prefixes, and a `#` inside any of them
// is not a comment.
//
// The three states Python raises on are reproduced, because each names a file
// whose comments cannot be trusted: a string literal that never closes, a
// bracket still open at end of file, and a dedent that matches no enclosing
// block. Each answers an error, and the caller turns that into a refusal rather
// than into "this file carries no tags".
//
// WHAT IT DOES NOT REACH. A Python 3.12 f-string that nests the SAME quote
// inside its replacement field closes the literal here one quote early. The
// construct is a syntax error in every Python before 3.12 and appears nowhere
// in this repository's scenario checks; the direction of the miss is a spurious
// comment rather than a hidden one, so it reddens a run rather than passing a
// file nobody read.
package rfc

import (
	"errors"
	"strings"
)

// tabWidth is the column a tab advances to a multiple of, which is what
// Python's tokenizer uses when it compares one line's indentation to another's.
const tabWidth = 8

// The three refusals, spelled once. Each is the answer to "the comments in this
// file cannot be read", and the caller wraps it with the path.
var (
	errUnterminatedString = errors.New("unterminated string literal")
	errOpenBracket        = errors.New("EOF in multi-line statement")
	errBadDedent          = errors.New("unindent does not match any outer indentation level")
)

// pyComment is one comment token: the physical line it sits on, its own text
// from the hash onward, and what preceded it on that line.
type pyComment struct {
	line   int
	text   string
	before string
}

// closed, continues and unterminated are what a scan of one physical line
// inside a string literal can end in.
const (
	litClosed = iota
	litContinues
	litUnterminated
)

// closeOnLine scans one physical line for the end of a literal that opened
// with quote, starting at i.
//
// A backslash escapes the next character, including the newline, which is how a
// single-quoted literal legally spans two lines. A triple-quoted literal always
// continues when the line ends without its closer.
func closeOnLine(text string, start int, quote string) (int, int) {
	i := start
	for i < len(text) {
		if text[i] == '\\' {
			if i+1 >= len(text) {
				return 0, litContinues // the escape consumed the newline
			}
			i += 2
			continue
		}
		if strings.HasPrefix(text[i:], quote) {
			return i + len(quote), litClosed
		}
		i++
	}
	if len(quote) == 3 {
		return 0, litContinues
	}
	// A single-quoted literal cannot span a line without that escape.
	// Answering unterminated here rather than running to the end of the file is
	// what stops one stray quote swallowing the whole module.
	return 0, litUnterminated
}

// quoteAt answers the literal delimiter opening at i: three characters when the
// same quote is repeated three times, one otherwise.
func quoteAt(text string, i int) string {
	if i+3 <= len(text) && text[i+1] == text[i] && text[i+2] == text[i] {
		return text[i : i+3]
	}
	return text[i : i+1]
}

// indentOf answers the indentation column of a line and the rest of it, with a
// tab advancing to the next multiple of tabWidth.
func indentOf(text string) (int, string) {
	column := 0
	i := 0
	for i < len(text) {
		switch text[i] {
		case ' ':
			column++
		case '\t':
			column = (column/tabWidth + 1) * tabWidth
		case '\f':
			column = 0
		default:
			return column, text[i:]
		}
		i++
	}
	return column, ""
}

// pyComments answers every comment token in Python source, in document order.
//
// It answers an error rather than a partial list for a file Python's tokenizer
// would have refused, which is the fail-closed half of the contract: a file
// whose comments cannot be read must never be reported as carrying no tags.
func pyComments(src string) ([]pyComment, error) {
	lines := strings.Split(src, "\n")
	indents := []int{0}
	depth := 0
	continued := false
	quote := ""
	var out []pyComment

	for n, text := range lines {
		column := 0

		switch {
		case quote != "":
			end, state := closeOnLine(text, 0, quote)
			if state == litUnterminated {
				return nil, errUnterminatedString
			}
			if state == litContinues {
				continue
			}
			quote, column = "", end
		case depth == 0 && !continued:
			// A logical line starts here, so its indentation is compared
			// against the enclosing blocks. A blank line and a comment-only
			// line are neither, and neither moves the stack.
			indent, rest := indentOf(text)
			trimmed := strings.TrimSpace(rest)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				if indent > indents[len(indents)-1] {
					indents = append(indents, indent)
					break
				}
				for len(indents) > 1 && indent < indents[len(indents)-1] {
					indents = indents[:len(indents)-1]
				}
				if indent != indents[len(indents)-1] {
					return nil, errBadDedent
				}
			}
		}

		continued = false
		for column < len(text) {
			char := text[column]
			switch {
			case char == '#':
				out = append(out, pyComment{line: n + 1, text: text[column:], before: text[:column]})
				column = len(text)
			case char == '\'' || char == '"':
				opener := quoteAt(text, column)
				end, state := closeOnLine(text, column+len(opener), opener)
				switch state {
				case litClosed:
					column = end
				case litContinues:
					quote = opener
					column = len(text)
				default:
					return nil, errUnterminatedString
				}
			case char == '(' || char == '[' || char == '{':
				depth++
				column++
			case char == ')' || char == ']' || char == '}':
				if depth > 0 {
					depth--
				}
				column++
			case char == '\\' && column == len(text)-1:
				continued = true
				column++
			default:
				column++
			}
		}
	}

	if quote != "" {
		return nil, errUnterminatedString
	}
	if depth > 0 {
		return nil, errOpenBracket
	}
	return out, nil
}
