// Design: docs/architecture/api/commands.md -- operational command quoting and pipe boundaries.

package command

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/opererr"
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
	result, err := scanInvocation(input, false)
	return result.tokens, result.pipeAt, result.quoteAt >= 0, err
}

// ScanExecution reads a complete operational invocation. An unfinished quoted
// value is refused before execution, with its one-based opening column. It
// stops at the first unquoted pipe: the caller owns the filter grammar beyond
// that boundary, including escapes that invocation values do not support.
// Completion MUST use ScanInvocation instead, because its input is incomplete.
func ScanExecution(input string) (tokens []string, pipeAt int, err error) {
	result, err := scanInvocation(input, true)
	if err != nil {
		return nil, result.pipeAt, err
	}
	if result.quoteAt >= 0 {
		var problem textbuf.Buffer
		problem.Str("The command has an unfinished quote at column ").
			Int(int64(utf8.RuneCountInString(input[:result.quoteAt]) + 1)).Byte('.')
		return nil, result.pipeAt, opererr.New(problem.String(),
			"No command ran. Add the closing double quote before trying again.", "help")
	}
	return result.tokens, result.pipeAt, nil
}

// invocationScan keeps source offsets beside decoded tokens. A quote offset
// of -1 means no value is open; both public readers use this one grammar.
type invocationScan struct {
	tokens  []string
	pipeAt  int
	quoteAt int
}

func scanInvocation(input string, stopAtPipe bool) (invocationScan, error) {
	trimmed := strings.TrimLeftFunc(input, unicode.IsSpace)
	lead := len(input) - len(trimmed)
	result := invocationScan{pipeAt: -1, quoteAt: -1}
	var current textbuf.Buffer
	wordStarted := false
	for i := 0; i < len(trimmed); i++ {
		c := trimmed[i]
		if c == '\\' {
			if result.quoteAt < 0 {
				result.tokens = nil
				return result, ErrBackslashInInvocation
			}
			if i+1 == len(trimmed) {
				result.tokens = nil
				return result, ErrBackslashInInvocation
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
				result.tokens = nil
				return result, ErrBackslashInInvocation
			}
			continue
		}
		if c == '"' {
			wordStarted = true
			if result.quoteAt < 0 {
				result.quoteAt = lead + i
			} else {
				result.quoteAt = -1
			}
			continue
		}
		if c == '|' && result.quoteAt < 0 && result.pipeAt == -1 {
			result.pipeAt = lead + i
			if stopAtPipe {
				if wordStarted {
					result.tokens = append(result.tokens, current.String())
				}
				return result, nil
			}
		}
		if (c == ' ' || c == '\t') && result.quoteAt < 0 {
			if wordStarted {
				result.tokens = append(result.tokens, current.String())
				wordStarted = false
			}
			continue
		}
		current.Byte(c)
		wordStarted = true
	}
	if wordStarted {
		result.tokens = append(result.tokens, current.String())
	}
	return result, nil
}
