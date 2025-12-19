package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTokenizerSimple verifies basic token extraction.
//
// VALIDATES: Tokenizer extracts words, braces, semicolons.
//
// PREVENTS: Lost or corrupted tokens.
func TestTokenizerSimple(t *testing.T) {
	input := `neighbor 192.0.2.1 { local-as 65000; }`

	tok := NewTokenizer(input)
	tokens := tok.All()

	require.Equal(t, []Token{
		{Type: TokenWord, Value: "neighbor", Line: 1, Col: 1},
		{Type: TokenWord, Value: "192.0.2.1", Line: 1, Col: 10},
		{Type: TokenLBrace, Value: "{", Line: 1, Col: 20},
		{Type: TokenWord, Value: "local-as", Line: 1, Col: 22},
		{Type: TokenWord, Value: "65000", Line: 1, Col: 31},
		{Type: TokenSemicolon, Value: ";", Line: 1, Col: 36},
		{Type: TokenRBrace, Value: "}", Line: 1, Col: 38},
		{Type: TokenEOF, Value: "", Line: 1, Col: 39},
	}, tokens)
}

// TestTokenizerMultiline verifies line tracking.
//
// VALIDATES: Line numbers are tracked across newlines.
//
// PREVENTS: Wrong line numbers in error messages.
func TestTokenizerMultiline(t *testing.T) {
	input := `neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
}`

	tok := NewTokenizer(input)
	tokens := tok.All()

	// Check line numbers
	require.Equal(t, 1, tokens[0].Line) // neighbor
	require.Equal(t, 1, tokens[2].Line) // {
	require.Equal(t, 2, tokens[3].Line) // local-as
	require.Equal(t, 3, tokens[6].Line) // peer-as
	require.Equal(t, 4, tokens[9].Line) // }
}

// TestTokenizerQuotedStrings verifies quoted string handling.
//
// VALIDATES: Quoted strings are preserved including spaces.
//
// PREVENTS: Broken strings with spaces or special chars.
func TestTokenizerQuotedStrings(t *testing.T) {
	input := `description "My BGP peer";`

	tok := NewTokenizer(input)
	tokens := tok.All()

	require.Equal(t, TokenWord, tokens[0].Type)
	require.Equal(t, "description", tokens[0].Value)
	require.Equal(t, TokenString, tokens[1].Type)
	require.Equal(t, "My BGP peer", tokens[1].Value)
	require.Equal(t, TokenSemicolon, tokens[2].Type)
}

// TestTokenizerSingleQuotes verifies single-quoted strings.
//
// VALIDATES: Single quotes work like double quotes.
//
// PREVENTS: Inconsistent string handling.
func TestTokenizerSingleQuotes(t *testing.T) {
	input := `run '/usr/bin/exabgp-api';`

	tok := NewTokenizer(input)
	tokens := tok.All()

	require.Equal(t, TokenWord, tokens[0].Type)
	require.Equal(t, "run", tokens[0].Value)
	require.Equal(t, TokenString, tokens[1].Type)
	require.Equal(t, "/usr/bin/exabgp-api", tokens[1].Value)
}

// TestTokenizerComments verifies comment handling.
//
// VALIDATES: Comments are skipped.
//
// PREVENTS: Comments being parsed as config.
func TestTokenizerComments(t *testing.T) {
	input := `# This is a comment
neighbor 192.0.2.1 {
    # Another comment
    local-as 65000;
}`

	tok := NewTokenizer(input)
	tokens := tok.All()

	// Comments should be skipped
	require.Equal(t, "neighbor", tokens[0].Value)
	require.Equal(t, 2, tokens[0].Line) // Line 2, after comment
}

// TestTokenizerNestedBraces verifies nested structure.
//
// VALIDATES: Nested braces are tokenized correctly.
//
// PREVENTS: Brace matching errors.
func TestTokenizerNestedBraces(t *testing.T) {
	input := `neighbor 192.0.2.1 {
    family {
        ipv4 unicast;
    }
}`

	tok := NewTokenizer(input)
	tokens := tok.All()

	braceCount := 0
	for _, tok := range tokens {
		if tok.Type == TokenLBrace {
			braceCount++
		} else if tok.Type == TokenRBrace {
			braceCount--
		}
	}
	require.Equal(t, 0, braceCount, "braces should be balanced")
}

// TestTokenizerNext verifies incremental tokenization.
//
// VALIDATES: Next() returns tokens one at a time.
//
// PREVENTS: Parser integration issues.
func TestTokenizerNext(t *testing.T) {
	input := `foo bar;`

	tok := NewTokenizer(input)

	token := tok.Next()
	require.Equal(t, TokenWord, token.Type)
	require.Equal(t, "foo", token.Value)

	token = tok.Next()
	require.Equal(t, TokenWord, token.Type)
	require.Equal(t, "bar", token.Value)

	token = tok.Next()
	require.Equal(t, TokenSemicolon, token.Type)

	token = tok.Next()
	require.Equal(t, TokenEOF, token.Type)

	// EOF should be repeatable
	token = tok.Next()
	require.Equal(t, TokenEOF, token.Type)
}

// TestTokenizerPeek verifies lookahead.
//
// VALIDATES: Peek() doesn't consume token.
//
// PREVENTS: Lost tokens during parsing.
func TestTokenizerPeek(t *testing.T) {
	input := `foo bar`

	tok := NewTokenizer(input)

	// Peek should not consume
	token := tok.Peek()
	require.Equal(t, "foo", token.Value)

	token = tok.Peek()
	require.Equal(t, "foo", token.Value)

	// Next should return same token
	token = tok.Next()
	require.Equal(t, "foo", token.Value)

	// Now peek should return next
	token = tok.Peek()
	require.Equal(t, "bar", token.Value)
}

// TestTokenizerEmpty verifies empty input.
//
// VALIDATES: Empty input returns EOF.
//
// PREVENTS: Panics on empty config.
func TestTokenizerEmpty(t *testing.T) {
	tok := NewTokenizer("")
	token := tok.Next()
	require.Equal(t, TokenEOF, token.Type)
}

// TestTokenizerWhitespaceOnly verifies whitespace handling.
//
// VALIDATES: Whitespace-only input returns EOF.
//
// PREVENTS: Phantom tokens from whitespace.
func TestTokenizerWhitespaceOnly(t *testing.T) {
	tok := NewTokenizer("   \n\t\n   ")
	token := tok.Next()
	require.Equal(t, TokenEOF, token.Type)
}

// TestTokenizerArray verifies array bracket tokenization.
//
// VALIDATES: [ and ] are tokenized correctly.
//
// PREVENTS: Broken array syntax parsing.
func TestTokenizerArray(t *testing.T) {
	input := `processes [ foo bar ];`

	tok := NewTokenizer(input)
	tokens := tok.All()

	require.Equal(t, TokenWord, tokens[0].Type)
	require.Equal(t, "processes", tokens[0].Value)
	require.Equal(t, TokenLBracket, tokens[1].Type)
	require.Equal(t, "[", tokens[1].Value)
	require.Equal(t, TokenWord, tokens[2].Type)
	require.Equal(t, "foo", tokens[2].Value)
	require.Equal(t, TokenWord, tokens[3].Type)
	require.Equal(t, "bar", tokens[3].Value)
	require.Equal(t, TokenRBracket, tokens[4].Type)
	require.Equal(t, "]", tokens[4].Value)
	require.Equal(t, TokenSemicolon, tokens[5].Type)
}
