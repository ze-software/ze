package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTokenizerSimple verifies basic token extraction.
//
// VALIDATES: tokenizer extracts words, braces, semicolons.
//
// PREVENTS: Lost or corrupted tokens.
func TestTokenizerSimple(t *testing.T) {
	input := `neighbor 192.0.2.1 { local-as 65000; }`

	tok := newTokenizer(input)
	tokens := tok.all()

	require.Equal(t, []token{
		{kind: tokenWord, value: "neighbor", line: 1, col: 1},
		{kind: tokenWord, value: "192.0.2.1", line: 1, col: 10},
		{kind: tokenLBrace, value: "{", line: 1, col: 20},
		{kind: tokenWord, value: "local-as", line: 1, col: 22},
		{kind: tokenWord, value: "65000", line: 1, col: 31},
		{kind: tokenSemicolon, value: ";", line: 1, col: 36},
		{kind: tokenRBrace, value: "}", line: 1, col: 38},
		{kind: tokenEOF, value: "", line: 1, col: 39},
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

	tok := newTokenizer(input)
	tokens := tok.all()

	// Check line numbers
	require.Equal(t, 1, tokens[0].line) // neighbor
	require.Equal(t, 1, tokens[2].line) // {
	require.Equal(t, 2, tokens[3].line) // local-as
	require.Equal(t, 3, tokens[6].line) // peer-as
	require.Equal(t, 4, tokens[9].line) // }
}

// TestTokenizerQuotedStrings verifies quoted string handling.
//
// VALIDATES: Quoted strings are preserved including spaces.
//
// PREVENTS: Broken strings with spaces or special chars.
func TestTokenizerQuotedStrings(t *testing.T) {
	input := `description "My BGP peer";`

	tok := newTokenizer(input)
	tokens := tok.all()

	require.Equal(t, tokenWord, tokens[0].kind)
	require.Equal(t, "description", tokens[0].value)
	require.Equal(t, tokenString, tokens[1].kind)
	require.Equal(t, "My BGP peer", tokens[1].value)
	require.Equal(t, tokenSemicolon, tokens[2].kind)
}

// TestTokenizerSingleQuotes verifies single-quoted strings.
//
// VALIDATES: Single quotes work like double quotes.
//
// PREVENTS: Inconsistent string handling.
func TestTokenizerSingleQuotes(t *testing.T) {
	input := `run '/usr/bin/exabgp-api';`

	tok := newTokenizer(input)
	tokens := tok.all()

	require.Equal(t, tokenWord, tokens[0].kind)
	require.Equal(t, "run", tokens[0].value)
	require.Equal(t, tokenString, tokens[1].kind)
	require.Equal(t, "/usr/bin/exabgp-api", tokens[1].value)
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

	tok := newTokenizer(input)
	tokens := tok.all()

	// Comments should be skipped
	require.Equal(t, "neighbor", tokens[0].value)
	require.Equal(t, 2, tokens[0].line) // Line 2, after comment
}

// TestTokenizerNestedBraces verifies nested structure.
//
// VALIDATES: Nested braces are tokenized correctly.
//
// PREVENTS: Brace matching errors.
func TestTokenizerNestedBraces(t *testing.T) {
	input := `neighbor 192.0.2.1 {
    family {
        ipv4/unicast;
    }
}`

	tok := newTokenizer(input)
	tokens := tok.all()

	braceCount := 0
	for _, tok := range tokens {
		switch tok.kind { //nolint:exhaustive // Only tracking braces
		case tokenLBrace:
			braceCount++
		case tokenRBrace:
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

	tok := newTokenizer(input)

	token := tok.next()
	require.Equal(t, tokenWord, token.kind)
	require.Equal(t, "foo", token.value)

	token = tok.next()
	require.Equal(t, tokenWord, token.kind)
	require.Equal(t, "bar", token.value)

	token = tok.next()
	require.Equal(t, tokenSemicolon, token.kind)

	token = tok.next()
	require.Equal(t, tokenEOF, token.kind)

	// EOF should be repeatable
	token = tok.next()
	require.Equal(t, tokenEOF, token.kind)
}

// TestTokenizerPeek verifies lookahead.
//
// VALIDATES: Peek() doesn't consume token.
//
// PREVENTS: Lost tokens during parsing.
func TestTokenizerPeek(t *testing.T) {
	input := `foo bar`

	tok := newTokenizer(input)

	// Peek should not consume
	token := tok.peek()
	require.Equal(t, "foo", token.value)

	token = tok.peek()
	require.Equal(t, "foo", token.value)

	// Next should return same token
	token = tok.next()
	require.Equal(t, "foo", token.value)

	// Now peek should return next
	token = tok.peek()
	require.Equal(t, "bar", token.value)
}

// TestTokenizerEmpty verifies empty input.
//
// VALIDATES: Empty input returns EOF.
//
// PREVENTS: Panics on empty config.
func TestTokenizerEmpty(t *testing.T) {
	tok := newTokenizer("")
	token := tok.next()
	require.Equal(t, tokenEOF, token.kind)
}

// TestTokenizerWhitespaceOnly verifies whitespace handling.
//
// VALIDATES: Whitespace-only input returns EOF.
//
// PREVENTS: Phantom tokens from whitespace.
func TestTokenizerWhitespaceOnly(t *testing.T) {
	tok := newTokenizer("   \n\t\n   ")
	token := tok.next()
	require.Equal(t, tokenEOF, token.kind)
}

// TestTokenizerArray verifies array bracket tokenization.
//
// VALIDATES: [ and ] are tokenized correctly.
//
// PREVENTS: Broken array syntax parsing.
func TestTokenizerArray(t *testing.T) {
	input := `processes [ foo bar ];`

	tok := newTokenizer(input)
	tokens := tok.all()

	require.Equal(t, tokenWord, tokens[0].kind)
	require.Equal(t, "processes", tokens[0].value)
	require.Equal(t, tokenLBracket, tokens[1].kind)
	require.Equal(t, "[", tokens[1].value)
	require.Equal(t, tokenWord, tokens[2].kind)
	require.Equal(t, "foo", tokens[2].value)
	require.Equal(t, tokenWord, tokens[3].kind)
	require.Equal(t, "bar", tokens[3].value)
	require.Equal(t, tokenRBracket, tokens[4].kind)
	require.Equal(t, "]", tokens[4].value)
	require.Equal(t, tokenSemicolon, tokens[5].kind)
}

// TestTokenizerAutoSemicolon verifies newlines act as implicit semicolons.
//
// VALIDATES: Newlines after value tokens insert automatic semicolons.
//
// PREVENTS: Requiring explicit semicolons when one statement per line.
func TestTokenizerAutoSemicolon(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		types  []tokenType
		values []string
	}{
		{
			name:   "newline after word",
			input:  "local-as 65000\npeer-as 65001\n",
			types:  []tokenType{tokenWord, tokenWord, tokenSemicolon, tokenWord, tokenWord, tokenSemicolon, tokenEOF},
			values: []string{"local-as", "65000", ";", "peer-as", "65001", ";", ""},
		},
		{
			name:   "EOF without newline",
			input:  "local-as 65000",
			types:  []tokenType{tokenWord, tokenWord, tokenSemicolon, tokenEOF},
			values: []string{"local-as", "65000", ";", ""},
		},
		{
			name:   "no auto-semi after open brace",
			input:  "bgp {\nlocal-as 1\n}",
			types:  []tokenType{tokenWord, tokenLBrace, tokenWord, tokenWord, tokenSemicolon, tokenRBrace, tokenEOF},
			values: []string{"bgp", "{", "local-as", "1", ";", "}", ""},
		},
		{
			name:   "auto-semi before closing brace on same line",
			input:  "edit { default-action deny }",
			types:  []tokenType{tokenWord, tokenLBrace, tokenWord, tokenWord, tokenSemicolon, tokenRBrace, tokenEOF},
			values: []string{"edit", "{", "default-action", "deny", ";", "}", ""},
		},
		{
			name:   "explicit semicolons still work",
			input:  "local-as 65000;\npeer-as 65001;\n",
			types:  []tokenType{tokenWord, tokenWord, tokenSemicolon, tokenWord, tokenWord, tokenSemicolon, tokenEOF},
			values: []string{"local-as", "65000", ";", "peer-as", "65001", ";", ""},
		},
		{
			name:   "auto-semi after closing bracket",
			input:  "processes [ foo bar ]\n",
			types:  []tokenType{tokenWord, tokenLBracket, tokenWord, tokenWord, tokenRBracket, tokenSemicolon, tokenEOF},
			values: []string{"processes", "[", "foo", "bar", "]", ";", ""},
		},
		{
			name:   "auto-semi after closing paren",
			input:  "name ( content )\n",
			types:  []tokenType{tokenWord, tokenLParen, tokenWord, tokenRParen, tokenSemicolon, tokenEOF},
			values: []string{"name", "(", "content", ")", ";", ""},
		},
		{
			name:   "auto-semi after quoted string",
			input:  "description \"hello world\"\n",
			types:  []tokenType{tokenWord, tokenString, tokenSemicolon, tokenEOF},
			values: []string{"description", "hello world", ";", ""},
		},
		{
			name:   "comment ends line like newline",
			input:  "local-as 65000 # comment\npeer-as 1\n",
			types:  []tokenType{tokenWord, tokenWord, tokenSemicolon, tokenWord, tokenWord, tokenSemicolon, tokenEOF},
			values: []string{"local-as", "65000", ";", "peer-as", "1", ";", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := newTokenizer(tt.input)
			tokens := tok.all()

			require.Equal(t, len(tt.types), len(tokens), "token count")
			for i, token := range tokens {
				require.Equal(t, tt.types[i], token.kind, "token %d type", i)
				require.Equal(t, tt.values[i], token.value, "token %d value", i)
			}
		})
	}
}
