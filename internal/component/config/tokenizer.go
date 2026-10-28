// Design: docs/architecture/config/syntax.md — config parsing and loading
// Overview: parser.go — config parser core

package config

import (
	"errors"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// tokenType represents the type of token.
type tokenType int

const (
	tokenEOF tokenType = iota
	tokenWord
	tokenString
	tokenLBrace
	tokenRBrace
	tokenLBracket
	tokenRBracket
	tokenLParen
	tokenRParen
	tokenSemicolon
)

func (t tokenType) String() string {
	switch t {
	case tokenEOF:
		return "EOF"
	case tokenWord:
		return "WORD"
	case tokenString:
		return "STRING"
	case tokenLBrace:
		return "LBRACE"
	case tokenRBrace:
		return "RBRACE"
	case tokenLBracket:
		return "LBRACKET"
	case tokenRBracket:
		return "RBRACKET"
	case tokenLParen:
		return "LPAREN"
	case tokenRParen:
		return "RPAREN"
	case tokenSemicolon:
		return "SEMICOLON"
	default:
		panic("BUG: unknown config token type")
	}
}

// token represents a lexical token. comments holds the `#` comments read
// since the token before it, in source order, and blankBefore whether a blank
// line separates the token from the last of them (or from the token before it
// when there is none). A synthetic semicolon carries no comment: the comments
// it crosses ride on the next real token.
type token struct {
	kind        tokenType
	value       string
	line        int
	col         int
	comments    []comment
	blankBefore bool
}

// comment is one `#` comment, kept as trivia for the parser to attach to the
// statement it belongs to rather than discarded. text is every byte after the
// `#` to the end of its line, the CR of a CRLF line ending excluded. sameLine
// is true when the comment follows a token on that token's line; blankBefore
// when a blank line separates it from the token or comment before it.
type comment struct {
	text        string
	line        int
	sameLine    bool
	blankBefore bool
}

// tokenizer breaks input into tokens.
// Automatic semicolon insertion adds a synthetic semicolon at a newline or EOF,
// or before a closing brace, after a value token (word, string, ], or )).
//
// Comments become trivia (see comment): trivia collects them until the next
// real token carries them, newlines counts the newlines since the last token
// or comment (two make a blank line), and scanned records that a real token
// was read, so a comment on the first line is never "after a token". taken
// collects the comments of every token the parser consumed with next; the
// parser drains it (takeConsumed) at statement boundaries, and takes the
// comments of a token it only peeked with peekTrivia.
type tokenizer struct {
	input      string
	pos        int
	line       int
	col        int
	peeked     *token
	insertSemi bool // next newline, EOF, or closing brace should produce a semicolon
	trivia     []comment
	newlines   int
	scanned    bool
	taken      []comment
	// unterminated holds the refusal of the first quoted string that reached
	// the end of the input with no closing quote (readString). The token is
	// still produced so a caller that only scans (probe, mask) keeps going;
	// a caller that builds a config from the tokens MUST refuse when it is set.
	unterminated error
}

// newTokenizer creates a new tokenizer for the given input.
func newTokenizer(input string) *tokenizer {
	return &tokenizer{
		input: input,
		pos:   0,
		line:  1,
		col:   1,
	}
}

// peek returns the next token without consuming it.
func (t *tokenizer) peek() token {
	if t.peeked != nil {
		return *t.peeked
	}
	tok := t.read()
	t.peeked = &tok
	return tok
}

// peekTrivia returns the next token without consuming it, and hands its
// comments to the caller: the token keeps none, so consuming it later adds
// nothing to taken.
func (t *tokenizer) peekTrivia() (token, []comment) {
	tok := t.peek()
	comments := t.peeked.comments
	t.peeked.comments = nil
	tok.comments = nil
	return tok, comments
}

// next returns the next token and advances the tokenizer. The token's
// comments join taken.
func (t *tokenizer) next() token {
	var tok token
	if t.peeked != nil {
		tok = *t.peeked
		t.peeked = nil
	} else {
		tok = t.read()
	}
	t.taken = append(t.taken, tok.comments...)
	return tok
}

// takenMark answers the position in taken a later takeConsumed starts from.
func (t *tokenizer) takenMark() int {
	return len(t.taken)
}

// takeConsumed removes and answers the comments of the tokens consumed since
// mark (takenMark), in source order.
func (t *tokenizer) takeConsumed(mark int) []comment {
	if mark >= len(t.taken) {
		return nil
	}
	consumed := slices.Clone(t.taken[mark:])
	t.taken = t.taken[:mark]
	return consumed
}

// read scans the next token and arms automatic semicolon insertion, which
// only value tokens do.
func (t *tokenizer) read() token {
	tok := t.scan()
	t.insertSemi = tok.kind == tokenWord || tok.kind == tokenString ||
		tok.kind == tokenRBracket || tok.kind == tokenRParen
	return tok
}

// scan produces the next raw token, including synthetic semicolons. A real
// token takes the comments read since the token before it.
func (t *tokenizer) scan() token {
	semiLine, semiCol := t.line, t.col
	newlineSeen := t.skipWhitespaceAndComments()

	if t.insertSemi && (newlineSeen ||
		t.pos >= len(t.input) ||
		(t.pos < len(t.input) && t.input[t.pos] == '}')) {
		return token{kind: tokenSemicolon, value: ";", line: semiLine, col: semiCol}
	}

	tok := t.scanToken()
	tok.comments, tok.blankBefore = t.trivia, t.newlines >= 2
	t.trivia, t.newlines, t.scanned = nil, 0, true
	return tok
}

// scanToken reads the real token at t.pos, whitespace and comments skipped.
func (t *tokenizer) scanToken() token {
	if t.pos >= len(t.input) {
		return token{kind: tokenEOF, line: t.line, col: t.col}
	}

	ch := t.input[t.pos]
	startLine, startCol := t.line, t.col

	switch ch {
	case '{':
		t.advance()
		return token{kind: tokenLBrace, value: "{", line: startLine, col: startCol}
	case '}':
		t.advance()
		return token{kind: tokenRBrace, value: "}", line: startLine, col: startCol}
	case '[':
		t.advance()
		return token{kind: tokenLBracket, value: "[", line: startLine, col: startCol}
	case ']':
		t.advance()
		return token{kind: tokenRBracket, value: "]", line: startLine, col: startCol}
	case '(':
		t.advance()
		return token{kind: tokenLParen, value: "(", line: startLine, col: startCol}
	case ')':
		t.advance()
		return token{kind: tokenRParen, value: ")", line: startLine, col: startCol}
	case ';':
		t.advance()
		return token{kind: tokenSemicolon, value: ";", line: startLine, col: startCol}
	case '"', '\'':
		return t.readString(ch, startLine, startCol)
	}
	return t.readWord(startLine, startCol)
}

// all returns all tokens.
func (t *tokenizer) all() []token {
	var tokens []token
	for {
		tok := t.next()
		tokens = append(tokens, tok)
		if tok.kind == tokenEOF {
			break
		}
	}
	return tokens
}

// advance moves to the next character.
func (t *tokenizer) advance() {
	if t.pos < len(t.input) {
		if t.input[t.pos] == '\n' {
			t.line++
			t.col = 1
		} else {
			t.col++
		}
		t.pos++
	}
}

// skipWhitespaceAndComments skips whitespace and keeps each # comment as
// trivia (readComment). Returns true if a newline was crossed (used for
// auto-semicolon insertion).
func (t *tokenizer) skipWhitespaceAndComments() bool {
	newlineSeen := false
	for t.pos < len(t.input) {
		ch := t.input[t.pos]

		if ch == ' ' || ch == '\t' || ch == '\r' {
			t.advance()
			continue
		}

		if ch == '\n' {
			newlineSeen = true
			t.newlines++
			t.advance()
			continue
		}

		if ch == '#' {
			// Comments end at newline, which counts as a newline crossing
			t.readComment()
			continue
		}

		break
	}
	return newlineSeen
}

// readComment reads the comment at t.pos up to its newline, which it leaves
// for the caller, and appends it to trivia.
func (t *tokenizer) readComment() {
	line := t.line
	t.advance() // skip '#'
	start := t.pos
	for t.pos < len(t.input) && t.input[t.pos] != '\n' {
		t.advance()
	}
	text := strings.TrimSuffix(t.input[start:t.pos], "\r")
	t.trivia = append(t.trivia, comment{
		text:        text,
		line:        line,
		sameLine:    t.scanned && t.newlines == 0 && len(t.trivia) == 0,
		blankBefore: t.newlines >= 2,
	})
	t.newlines = 0
}

// readString reads a quoted string. A string that reaches the end of the
// input before its closing quote records the refusal in t.unterminated: the
// grammar has no string that runs to the end of the file, and reading one
// would swallow every statement after the quote into one value.
func (t *tokenizer) readString(quote byte, startLine, startCol int) token {
	t.advance() // skip opening quote

	var value []byte
	closed := false
	for t.pos < len(t.input) {
		ch := t.input[t.pos]
		if ch == quote {
			t.advance() // skip closing quote
			closed = true
			break
		}
		if ch == '\\' && t.pos+1 < len(t.input) {
			t.advance()
			ch = t.input[t.pos]
			// Handle escape sequences
			switch ch {
			case 'n':
				value = append(value, '\n')
			case 't':
				value = append(value, '\t')
			case '\\':
				value = append(value, '\\')
			case '"':
				value = append(value, '"')
			case '\'':
				value = append(value, '\'')
			default:
				value = append(value, ch)
			}
			t.advance()
			continue
		}
		value = append(value, ch)
		t.advance()
	}

	if !closed && t.unterminated == nil {
		var b textbuf.Buffer
		t.unterminated = errors.New(b.Str("line ").Int(int64(startLine)).
			Str(": unterminated ").Byte(quote).Str(" quote at column ").Int(int64(startCol)).
			Str(": the string has no closing ").Byte(quote).
			Str(", so it would run to the end of the file\n  add the closing ").Byte(quote).
			Str(" where the value ends").String())
	}
	return token{kind: tokenString, value: string(value), line: startLine, col: startCol}
}

// readWord reads an unquoted word.
//
// A `[` straight after a `:` inside a word opens an address, not a list, and
// stays in the word through its `]`: `redirect:[2001:db8::1]:100` is the RFC
// 8956 Section 6.1 IPv6 route-target redirect, the form ze prints for a received
// one (attribute.IPv6ExtendedCommunity.AppendDecoded), and splitting it would
// refuse the spelling ze writes. Only the colon opens one, because an ExaBGP
// config read by migration glues a list to a word elsewhere: the SRv6 SID
// structure `0x48[64,24,16,0,0,0]` is still a word and a list.
func (t *tokenizer) readWord(startLine, startCol int) token {
	start := t.pos

	for t.pos < len(t.input) {
		ch := t.input[t.pos]
		if ch == '[' && t.pos > start && t.input[t.pos-1] == ':' {
			if end, closed := t.wordAddressEnd(); closed {
				for t.pos < end {
					t.advance()
				}
				continue
			}
		}
		if endsWord(ch) {
			break
		}
		t.advance()
	}

	return token{kind: tokenWord, value: t.input[start:t.pos], line: startLine, col: startCol}
}

// wordAddressEnd answers the offset just past the `]` that closes the `[` at
// t.pos, when the bracket holds at least one character and none that ends a
// word. closed is false otherwise, and the `[` is then a list as before.
func (t *tokenizer) wordAddressEnd() (end int, closed bool) {
	for i := t.pos + 1; i < len(t.input); i++ {
		ch := t.input[i]
		if ch == ']' {
			return i + 1, i > t.pos+1
		}
		if endsWord(ch) {
			return 0, false
		}
	}
	return 0, false
}

// bareWordReadsBack reports whether the tokenizer reads s, written unquoted,
// back as one word token equal to s. It asks the tokenizer itself rather than
// a list of characters, so the printer's quoting cannot drift from what ends
// a word or opens another token: whitespace, braces, brackets outside a
// `:[...]` address, `;`, quotes and `#` anywhere, and a leading `(` or `)`.
// The empty string reads back as no token at all, so it is never bare.
func bareWordReadsBack(s string) bool {
	t := tokenizer{input: s, line: 1, col: 1}
	tok := t.next()
	if tok.kind != tokenWord {
		return false
	}
	// A word token is a slice of the input, so equal text means the word
	// started at offset 0 and ran to the end: nothing was left to re-read.
	return tok.value == s
}

// endsWord reports whether ch ends an unquoted word.
func endsWord(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' ||
		ch == '{' || ch == '}' || ch == '[' || ch == ']' || ch == ';' ||
		ch == '"' || ch == '\'' || ch == '#'
}
