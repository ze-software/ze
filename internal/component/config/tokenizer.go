// Design: docs/architecture/config/syntax.md — config parsing and loading
// Overview: parser.go — config parser core

package config

// TokenType represents the type of token.
type TokenType int

const (
	TokenEOF TokenType = iota
	TokenWord
	TokenString
	TokenLBrace
	TokenRBrace
	TokenLBracket
	TokenRBracket
	TokenLParen
	TokenRParen
	TokenSemicolon
)

func (t TokenType) String() string {
	switch t {
	case TokenEOF:
		return "EOF"
	case TokenWord:
		return "WORD"
	case TokenString:
		return "STRING"
	case TokenLBrace:
		return "LBRACE"
	case TokenRBrace:
		return "RBRACE"
	case TokenLBracket:
		return "LBRACKET"
	case TokenRBracket:
		return "RBRACKET"
	case TokenLParen:
		return "LPAREN"
	case TokenRParen:
		return "RPAREN"
	case TokenSemicolon:
		return "SEMICOLON"
	default:
		return "UNKNOWN"
	}
}

// Token represents a lexical token.
type Token struct {
	Type  TokenType
	Value string
	Line  int
	Col   int
}

// Tokenizer breaks input into tokens.
// Automatic semicolon insertion: a newline (or EOF) after a value token
// (word, string, ], )) inserts a synthetic semicolon — same approach as Go's lexer.
type Tokenizer struct {
	input      string
	pos        int
	line       int
	col        int
	peeked     *Token
	insertSemi bool // next newline/EOF should produce a semicolon
}

// NewTokenizer creates a new tokenizer for the given input.
func NewTokenizer(input string) *Tokenizer {
	return &Tokenizer{
		input: input,
		pos:   0,
		line:  1,
		col:   1,
	}
}

// Peek returns the next token without consuming it.
func (t *Tokenizer) Peek() Token {
	if t.peeked != nil {
		return *t.peeked
	}
	tok := t.Next()
	t.peeked = &tok
	return tok
}

// Next returns the next token and advances the tokenizer.
func (t *Tokenizer) Next() Token {
	var tok Token
	if t.peeked != nil {
		tok = *t.peeked
		t.peeked = nil
	} else {
		tok = t.scan()
	}
	// Auto-semicolon: value tokens cause the next newline/EOF to produce a semicolon.
	t.insertSemi = tok.Type == TokenWord || tok.Type == TokenString ||
		tok.Type == TokenRBracket || tok.Type == TokenRParen
	return tok
}

// scan produces the next raw token, including synthetic semicolons.
func (t *Tokenizer) scan() Token {
	semiLine, semiCol := t.line, t.col
	newlineSeen := t.skipWhitespaceAndComments()

	if t.insertSemi && (newlineSeen || t.pos >= len(t.input)) {
		return Token{Type: TokenSemicolon, Value: ";", Line: semiLine, Col: semiCol}
	}

	if t.pos >= len(t.input) {
		return Token{Type: TokenEOF, Line: t.line, Col: t.col}
	}

	ch := t.input[t.pos]
	startLine, startCol := t.line, t.col

	switch ch {
	case '{':
		t.advance()
		return Token{Type: TokenLBrace, Value: "{", Line: startLine, Col: startCol}
	case '}':
		t.advance()
		return Token{Type: TokenRBrace, Value: "}", Line: startLine, Col: startCol}
	case '[':
		t.advance()
		return Token{Type: TokenLBracket, Value: "[", Line: startLine, Col: startCol}
	case ']':
		t.advance()
		return Token{Type: TokenRBracket, Value: "]", Line: startLine, Col: startCol}
	case '(':
		t.advance()
		return Token{Type: TokenLParen, Value: "(", Line: startLine, Col: startCol}
	case ')':
		t.advance()
		return Token{Type: TokenRParen, Value: ")", Line: startLine, Col: startCol}
	case ';':
		t.advance()
		return Token{Type: TokenSemicolon, Value: ";", Line: startLine, Col: startCol}
	case '"', '\'':
		return t.readString(ch, startLine, startCol)
	}
	return t.readWord(startLine, startCol)
}

// All returns all tokens.
func (t *Tokenizer) All() []Token {
	var tokens []Token
	for {
		tok := t.Next()
		tokens = append(tokens, tok)
		if tok.Type == TokenEOF {
			break
		}
	}
	return tokens
}

// advance moves to the next character.
func (t *Tokenizer) advance() {
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
func (t *Tokenizer) skipWhitespaceAndComments() bool {
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
func (t *Tokenizer) readString(quote byte, startLine, startCol int) Token {
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

	return Token{Type: TokenString, Value: string(value), Line: startLine, Col: startCol}
}

// readWord reads an unquoted word.
func (t *Tokenizer) readWord(startLine, startCol int) Token {
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

	return Token{Type: TokenWord, Value: t.input[start:t.pos], Line: startLine, Col: startCol}
}
