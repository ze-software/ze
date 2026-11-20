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
// A quoted value decodes escaped quotes, backslashes, n, r and t. Before the
// first unquoted pipe a backslash outside quotes is refused; after it, in the
// pipe chain, it is a literal character, as scanChain reads it. The words cover
// the whole line and read a pipe outside quotes as a word character, so
// accounting and the authorizer see the chain as typed. An unfinished quote
// retains the existing partial word behavior, which completion and the input
// key handler also need.
// Input length bounds the scan and its output; every returned word is owned.
// On error the offset of a pipe read before the fault remains available.
func ScanInvocation(input string) (tokens []string, pipeAt int, quoteOpen bool, err error) {
	result, err := scanInvocation(input, false)
	return result.tokens, result.pipeAt, result.quoteAt >= 0, err
}

// ScanExecution reads a complete operational invocation and answers the words
// of its command part, before the first unquoted pipe. The pipe chain after
// that pipe is read under the same quote grammar (scanChain), so an unfinished
// quoted value anywhere on the line is refused before execution, with the
// one-based column of its opening quote, and so is a quoted escape the grammar
// does not define.
// Completion MUST use ScanInvocation instead, because its input is incomplete.
func ScanExecution(input string) (tokens []string, pipeAt int, err error) {
	result, _, err := scanExecution(input)
	if err != nil {
		return nil, result.pipeAt, err
	}
	return result.tokens, result.pipeAt, nil
}

// scanExecution is ScanExecution answering the pipe chain it read too, so a
// caller that needs the operations reads the line once.
func scanExecution(input string) (invocationScan, chainScan, error) {
	result, err := scanInvocation(input, true)
	if err != nil {
		return result, chainScan{quoteAt: -1}, err
	}
	if result.quoteAt >= 0 {
		return result, chainScan{quoteAt: -1}, unfinishedQuote(input, result.quoteAt)
	}
	if result.pipeAt < 0 {
		return result, chainScan{quoteAt: -1}, nil
	}
	chainStart := result.pipeAt + 1
	chain, err := scanChain(input[chainStart:])
	if err != nil {
		return result, chainScan{quoteAt: -1}, err
	}
	if chain.quoteAt >= 0 {
		return result, chainScan{quoteAt: -1}, unfinishedQuote(input, chainStart+chain.quoteAt)
	}
	return result, chain, nil
}

// unfinishedQuote refuses text whose double quote at byte offset quoteAt never
// closes. It names the quote's one-based character column and never echoes the
// value, which may hold a credential.
func unfinishedQuote(text string, quoteAt int) error {
	var problem textbuf.Buffer
	problem.Str("The command has an unfinished quote at column ").
		Int(int64(utf8.RuneCountInString(text[:quoteAt]) + 1)).Byte('.')
	return opererr.New(problem.String(),
		"No command ran. Add the closing double quote before trying again.", "help")
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
				if result.pipeAt >= 0 {
					// The pipe chain keeps a backslash outside quotes as a
					// literal character (scanChain); only the command part
					// refuses it.
					current.Byte(c)
					wordStarted = true
					continue
				}
				result.tokens = nil
				return result, ErrBackslashInInvocation
			}
			decoded, ok := quotedEscape(trimmed, i)
			if !ok {
				result.tokens = nil
				return result, ErrBackslashInInvocation
			}
			current.Byte(decoded)
			i++
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

// quotedEscape decodes the escape whose backslash is at text[i] inside a quoted
// value: an escaped quote or backslash, n, r or t. ok is false for any other
// escape and for a backslash that ends the text, which the caller refuses.
func quotedEscape(text string, i int) (decoded byte, ok bool) {
	if i+1 == len(text) {
		return 0, false
	}
	switch text[i+1] {
	case '"', '\\':
		return text[i+1], true
	case 'n':
		return '\n', true
	case 'r':
		return '\r', true
	case 't':
		return '\t', true
	}
	return 0, false
}

// chainScan is a pipe chain as scanChain read it. operations holds each
// operation's decoded words, its name first; a segment with no word (`| |`)
// is no operation. quoteAt is the byte offset in the scanned text of a double
// quote that never closes, or -1.
// lastStageAt is the byte offset just past the last pipe the grammar reads as a
// separator, or 0 when it reads none: where the stage being typed starts.
type chainScan struct {
	operations  [][]string
	quoteAt     int
	lastStageAt int
}

// scanChain reads the pipe chain after the first unquoted pipe under the
// invocation's quote grammar (docs/architecture/api/commands.md, "The pipe
// chain takes the invocation's quote grammar"). A pipe outside double quotes
// ends an operation and whitespace outside them ends a word, so a quoted value
// keeps its pipes and spaces. A quoted run belongs to the word it touches and
// loses its quotes: `ab"c d"e` is the one word `abc de`. Inside quotes the
// escapes are the invocation's own (quotedEscape), and any other is refused.
// A backslash outside quotes is a literal character, the one difference from
// the command part, because match patterns and paths rely on it: no escape
// exists there, so `a\"b c"` is the word `a\b c`. Only the double quote groups.
// An unfinished quote keeps its partial word and is reported in quoteAt, so
// execution refuses it while completion and recognition read on.
// The text length bounds the loop and the words it answers.
func scanChain(chain string) (chainScan, error) {
	result := chainScan{quoteAt: -1}
	var word textbuf.Buffer
	var operation []string
	wordStarted := false
	endWord := func() {
		if wordStarted {
			operation = append(operation, word.String())
			wordStarted = false
		}
	}
	endOperation := func() {
		endWord()
		if len(operation) > 0 {
			result.operations = append(result.operations, operation)
			operation = nil
		}
	}
	for i := 0; i < len(chain); {
		c := chain[i]
		if result.quoteAt >= 0 {
			switch c {
			case '\\':
				decoded, ok := quotedEscape(chain, i)
				if !ok {
					return chainScan{quoteAt: -1}, ErrBackslashInInvocation
				}
				word.Byte(decoded)
				i += 2
			case '"':
				result.quoteAt = -1
				i++
			default:
				word.Byte(c)
				i++
			}
			continue
		}
		switch c {
		case '"':
			result.quoteAt = i
			wordStarted = true
			i++
		case '|':
			endOperation()
			i++
			result.lastStageAt = i
		default:
			r, size := utf8.DecodeRuneInString(chain[i:])
			if unicode.IsSpace(r) {
				endWord()
			} else {
				word.Str(chain[i : i+size])
				wordStarted = true
			}
			i += size
		}
	}
	endOperation()
	return result, nil
}
