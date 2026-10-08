package yang

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rfc7950GrammarText extracts the "yang.abnf" code component of RFC 7950
// Section 14 from rfc/full/rfc7950.txt, dropping the form feeds and the page
// header and footer lines the text format interleaves.
func rfc7950GrammarText(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("../../../../rfc/full/rfc7950.txt")
	require.NoError(t, err)
	text := string(raw)
	begin := strings.Index(text, `<CODE BEGINS> file "yang.abnf"`)
	require.GreaterOrEqual(t, begin, 0)
	begin += strings.IndexByte(text[begin:], '\n') + 1
	end := strings.Index(text[begin:], "<CODE ENDS>")
	require.Greater(t, end, 0)
	end = strings.LastIndexByte(text[:begin+end], '\n') + 1
	var lines []string
	for line := range strings.SplitSeq(strings.ReplaceAll(text[begin:end], "\f", ""), "\n") {
		if strings.HasPrefix(line, "Bjorklund") || strings.HasPrefix(line, "RFC 7950") {
			continue
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// TestEmbeddedGrammarIsTheRFC7950Grammar proves that rfc7950ABNF, the grammar
// extensionSubstatementError reads, is the Section 14 code component of
// rfc/full/rfc7950.txt, byte for byte once the page furniture is gone.
//
// VALIDATES: the declaration statementGrammars is parsed from.
func TestEmbeddedGrammarIsTheRFC7950Grammar(t *testing.T) {
	assert.Equal(t, rfc7950GrammarText(t), rfc7950ABNF)
}

// TestStatementGrammarsMatchTheRFC7950Grammar proves parseStatementGrammar
// reads the grammar right. Its keyword set and its argument-less statements
// are compared with what a second reading of the RFC text finds, a
// "<name>-stmt = [optsep] <name>-keyword" rule and the "optsep" that follows
// a statement taking no argument. Its blocks are compared with rules quoted
// from Section 14: "description-stmt = description-keyword sep string
// stmtend" admits no statement, "config-stmt = config-keyword sep
// config-arg-str stmtend" takes config-arg-str, leaf-stmt lists type-stmt,
// and type-stmt reaches length-stmt through type-body-stmts and
// string-restrictions.
//
// VALIDATES: statementGrammars, the grammar checkStructure applies under an
// extension statement.
func TestStatementGrammarsMatchTheRFC7950Grammar(t *testing.T) {
	text := rfc7950GrammarText(t)
	spelled := map[string]string{}
	for _, m := range regexp.MustCompile(`(?m)^\s+([a-z-]+)-keyword\s+=\s+%s"([a-z-]+)"`).FindAllStringSubmatch(text, -1) {
		spelled[m[1]] = m[2]
	}
	var want, wantArgumentless []string
	rule := regexp.MustCompile(`(?m)^\s+[a-z-]+-stmt\s+=\s+(?:optsep\s+)?([a-z-]+)-keyword(\s+optsep)?`)
	for _, m := range rule.FindAllStringSubmatch(text, -1) {
		keyword, known := spelled[m[1]]
		require.True(t, known, "statement rule names %s-keyword, which the grammar never spells", m[1])
		if !slices.Contains(want, keyword) {
			want = append(want, keyword)
		}
		if m[2] != "" {
			wantArgumentless = append(wantArgumentless, keyword)
		}
	}
	var got, gotArgumentless []string
	for keyword, grammar := range statementGrammars() {
		got = append(got, keyword)
		if len(grammar.arguments) == 0 {
			gotArgumentless = append(gotArgumentless, keyword)
		}
	}
	slices.Sort(want)
	slices.Sort(got)
	slices.Sort(wantArgumentless)
	slices.Sort(gotArgumentless)
	assert.Equal(t, want, got, "statement keywords")
	assert.Equal(t, wantArgumentless, gotArgumentless, "statements without an argument")

	grammars := statementGrammars()
	assert.Empty(t, grammars["description"].children, "description-stmt admits no statement")
	assert.Equal(t, []string{"config-arg-str"}, grammars["config"].arguments)
	assert.Contains(t, grammars["leaf"].children, "type")
	assert.NotContains(t, grammars["leaf"].children, "leaf")
	assert.Contains(t, grammars["type"].children, "length")
	assert.Equal(t, []string{"add-keyword-str", "delete-keyword-str", "not-supported-keyword-str", "replace-keyword-str"},
		grammars["deviate"].arguments)
}

// TestEveryArgumentRuleHasAChecker proves that each argument rule the grammar
// names is either checked by argumentCheckers or listed, by name, in
// uncheckedArgumentRules, so no rule is accepted unchecked by omission.
//
// VALIDATES: argumentCheckers and uncheckedArgumentRules cover the grammar.
func TestEveryArgumentRuleHasAChecker(t *testing.T) {
	for keyword, rule := range statementGrammars() {
		for _, argument := range rule.arguments {
			_, checked := argumentCheckers()[argument]
			unchecked := slices.Contains(uncheckedArgumentRules, argument)
			assert.True(t, checked != unchecked, "%s: argument rule %s must be checked or listed unchecked, not both or neither", keyword, argument)
		}
	}
}
