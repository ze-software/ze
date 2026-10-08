package yang

import (
	"os"
	"regexp"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rfc7950StatementGrammar reads the RFC 7950 Section 14 grammar and answers
// every statement keyword it defines, each mapped to whether its statement
// rule takes no argument ("<keyword>-keyword optsep").
func rfc7950StatementGrammar(t *testing.T) map[string]bool {
	t.Helper()
	text, err := os.ReadFile("../../../../rfc/full/rfc7950.txt")
	require.NoError(t, err)
	spelled := map[string]string{}
	for _, m := range regexp.MustCompile(`(?m)^\s+([a-z-]+)-keyword\s+=\s+%s"([a-z-]+)"`).FindAllStringSubmatch(string(text), -1) {
		spelled[m[1]] = m[2]
	}
	statements := map[string]bool{}
	rule := regexp.MustCompile(`(?m)^\s+[a-z-]+-stmt\s+=\s+(?:optsep\s+)?([a-z-]+)-keyword(\s+optsep)?`)
	for _, m := range rule.FindAllStringSubmatch(string(text), -1) {
		keyword, known := spelled[m[1]]
		require.True(t, known, "statement rule names %s-keyword, which the grammar never spells", m[1])
		statements[keyword] = m[2] != ""
	}
	require.NotEmpty(t, statements)
	return statements
}

// TestYANGKeywordsMatchTheRFC7950Grammar proves that yangKeywords, derived
// from goyang's AST tags, and argumentlessKeywords, written from the Section 14
// grammar, are the statement keywords and the argument-less statements the
// grammar in rfc/full/rfc7950.txt defines. A drift in either copy turns this
// red rather than refusing a legal extension substatement.
//
// VALIDATES: both sets checkStructure reads under an extension statement.
func TestYANGKeywordsMatchTheRFC7950Grammar(t *testing.T) {
	grammar := rfc7950StatementGrammar(t)

	var want, wantArgumentless []string
	for keyword, argumentless := range grammar {
		want = append(want, keyword)
		if argumentless {
			wantArgumentless = append(wantArgumentless, keyword)
		}
	}
	var got, gotArgumentless []string
	for keyword := range yangKeywords() {
		got = append(got, keyword)
	}
	for keyword := range argumentlessKeywords {
		gotArgumentless = append(gotArgumentless, keyword)
	}
	slices.Sort(want)
	slices.Sort(got)
	slices.Sort(wantArgumentless)
	slices.Sort(gotArgumentless)
	assert.Equal(t, want, got, "statement keywords")
	assert.Equal(t, wantArgumentless, gotArgumentless, "statements without an argument")
}
