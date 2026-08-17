// Design: docs/architecture/config/syntax.md — config parsing and loading
// Overview: parser.go — config parser core

package config

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
		return "UNKNOWN"
	}
}

// token represents a lexical token.
type token struct {
	kind  tokenType
	value string
	line  int
	col   int
}

// tokenizer breaks input into tokens.
// Automatic semicolon insertion adds a synthetic semicolon at a newline or EOF,
// or before a closing brace, after a value token (word, string, ], or )).
type tokenizer struct {
	input      string
	pos        int
	line       int
	col        int
	peeked     *token
	insertSemi bool // next newline, EOF, or closing brace should produce a semicolon
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
	tok := t.next()
	t.peeked = &tok
	return tok
}

// next returns the next token and advances the tokenizer.
func (t *tokenizer) next() token {
	var tok token
	if t.peeked != nil {
		tok = *t.peeked
		t.peeked = nil
	} else {
		tok = t.scan()
	}
	// Automatic semicolon insertion is armed only by value tokens.
	t.insertSemi = tok.kind == tokenWord || tok.kind == tokenString ||
		tok.kind == tokenRBracket || tok.kind == tokenRParen
	return tok
}

// scan produces the next raw token, including synthetic semicolons.
func (t *tokenizer) scan() token {
	semiLine, semiCol := t.line, t.col
	newlineSeen := t.skipWhitespaceAndComments()

	if t.insertSemi && (newlineSeen ||
		t.pos >= len(t.input) ||
		(t.pos < len(t.input) && t.input[t.pos] == '}')) {
		return token{kind: tokenSemicolon, value: ";", line: semiLine, col: semiCol}
	}

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

// skipWhitespaceAndComments skips whitespace and # comments.
// Returns true if a newline was crossed (used for auto-semicolon insertion).
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
			t.advance()
			continue
		}

		if ch == '#' {
			// Comments end at newline, which counts as a newline crossing
			for t.pos < len(t.input) && t.input[t.pos] != '\n' {
				t.advance()
			}
			continue
		}

		break
	}
	return newlineSeen
}

// readString reads a quoted string.
func (t *tokenizer) readString(quote byte, startLine, startCol int) token {
	t.advance() // skip opening quote

	var value []byte
	for t.pos < len(t.input) {
		ch := t.input[t.pos]
		if ch == quote {
			t.advance() // skip closing quote
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

	return token{kind: tokenString, value: string(value), line: startLine, col: startCol}
}

// readWord reads an unquoted word.
func (t *tokenizer) readWord(startLine, startCol int) token {
	start := t.pos

	for t.pos < len(t.input) {
		ch := t.input[t.pos]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' ||
			ch == '{' || ch == '}' || ch == '[' || ch == ']' || ch == ';' ||
			ch == '"' || ch == '\'' || ch == '#' {
			break
		}
		t.advance()
	}

	return token{kind: tokenWord, value: t.input[start:t.pos], line: startLine, col: startCol}
}
