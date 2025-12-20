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
type Tokenizer struct {
	input  string
	pos    int
	line   int
	col    int
	peeked *Token
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
	if t.peeked != nil {
		tok := *t.peeked
		t.peeked = nil
		return tok
	}

	t.skipWhitespaceAndComments()

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
	default:
		return t.readWord(startLine, startCol)
	}
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
func (t *Tokenizer) skipWhitespaceAndComments() {
	for t.pos < len(t.input) {
		ch := t.input[t.pos]

		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			t.advance()
			continue
		}

		if ch == '#' {
			// Skip to end of line
			for t.pos < len(t.input) && t.input[t.pos] != '\n' {
				t.advance()
			}
			continue
		}

		break
	}
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
