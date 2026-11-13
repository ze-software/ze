// Design: docs/architecture/api/commands.md -- operational command quoting and pipe boundaries.

package command

import (
	"errors"
	"strings"
	"unicode"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ErrBackslashInInvocation refuses an escape outside a quoted operational
// value, or an escape the quoted grammar does not define.
var ErrBackslashInInvocation = errors.New("backslash is not allowed in commands")

// ScanInvocation reads operational command words, the byte offset of the first
// unquoted pipe (-1 when absent), and whether a double quote remains open.
// Dispatcher, pipeline entry and the input key handler use this same grammar.
// A quoted value decodes escaped quotes, backslashes, n, r and t; an unquoted
// backslash remains refused. An unfinished quote retains the existing partial
// word behavior, which completion and the input key handler also need.
// Input length bounds the scan and its output; every returned word is owned.
// On error an earlier pipe offset remains available: the operator surface owns
// the filter grammar after that boundary, which need not use invocation escapes.
func ScanInvocation(input string) (tokens []string, pipeAt int, quoteOpen bool, err error) {
	trimmed := strings.TrimLeftFunc(input, unicode.IsSpace)
	lead := len(input) - len(trimmed)
	trimmed = strings.TrimRightFunc(trimmed, unicode.IsSpace)
	pipeAt = -1
	var current textbuf.Buffer
	for i := 0; i < len(trimmed); i++ {
		c := trimmed[i]
		if c == '\\' {
			if !quoteOpen || i+1 == len(trimmed) {
				return nil, pipeAt, quoteOpen, ErrBackslashInInvocation
			}
			i++
			switch trimmed[i] {
			case '"', '\\':
				current.Byte(trimmed[i])
			case 'n':
				current.Byte('\n')
			case 'r':
				current.Byte('\r')
			case 't':
				current.Byte('\t')
			default:
				return nil, pipeAt, quoteOpen, ErrBackslashInInvocation
			}
			continue
		}
		if c == '"' {
			quoteOpen = !quoteOpen
			continue
		}
		if c == '|' && !quoteOpen && pipeAt == -1 {
			pipeAt = lead + i
		}
		if (c == ' ' || c == '\t') && !quoteOpen {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
			}
			continue
		}
		current.Byte(c)
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens, pipeAt, quoteOpen, nil
}
