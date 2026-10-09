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
	grammar := rfc7950Grammar()
	var got, gotArgumentless []string
	for keyword, productions := range grammar.keywords {
		got = append(got, keyword)
		for _, production := range productions {
			if production.argument == "" {
				gotArgumentless = append(gotArgumentless, keyword)
				break
			}
		}
	}
	slices.Sort(want)
	slices.Sort(got)
	slices.Sort(wantArgumentless)
	wantArgumentless = slices.Compact(wantArgumentless)
	slices.Sort(gotArgumentless)
	assert.Equal(t, want, got, "statement keywords")
	assert.Equal(t, wantArgumentless, gotArgumentless, "statements without an argument")

	production := func(rule string) *statementProduction {
		t.Helper()
		found, defined := grammar.productions[rule]
		require.True(t, defined, "production %s", rule)
		return found
	}
	assert.Empty(t, production("description-stmt").block.admitted, "description-stmt admits no statement")
	assert.Equal(t, "config-arg-str", production("config-stmt").argument)
	assert.Contains(t, production("leaf-stmt").block.admitted, "type")
	assert.NotContains(t, production("leaf-stmt").block.admitted, "leaf")
	assert.Contains(t, production("type-stmt").block.admitted, "length")
	var deviate []string
	for _, p := range grammar.keywords["deviate"] {
		deviate = append(deviate, p.argument)
	}
	assert.Equal(t, []string{"add-keyword-str", "delete-keyword-str", "not-supported-keyword-str", "replace-keyword-str"}, deviate)
	augment := grammar.keywords["augment"]
	require.Len(t, augment, 2)
	assert.Equal(t, "augment-arg-str", augment[0].argument)
	assert.Equal(t, "uses-augment-arg-str", augment[1].argument)
	assert.Equal(t, []*statementProduction{augment[0]}, production("module-stmt").block.admitted["augment"], "body-stmts holds augment-stmt")
	assert.Equal(t, []*statementProduction{augment[1]}, production("uses-stmt").block.admitted["augment"], "uses-stmt holds uses-augment-stmt")
	assert.Len(t, production("deviation-stmt").block.admitted["deviate"], 4)
	assert.Len(t, grammar.extension.admitted["augment"], 2, "an extension usage holds any yang-stmt")
}

// TestEveryArgumentRuleHasAChecker proves that each argument rule the grammar
// names is checked by argumentCheckers, so no rule is accepted unchecked.
//
// VALIDATES: argumentCheckers covers the grammar.
func TestEveryArgumentRuleHasAChecker(t *testing.T) {
	for rule, production := range rfc7950Grammar().productions {
		if production.argument == "" {
			continue
		}
		_, checked := argumentCheckers()[production.argument]
		assert.True(t, checked, "%s: argument rule %s has no checker", rule, production.argument)
	}
}

// TestSameKeywordProductionsHaveDisjointArguments proves that where one
// keyword has several productions (augment two, deviate four), no argument
// drawn from the RFC's own forms matches two of them, so resolveProduction
// never meets an argument it cannot attribute.
//
// VALIDATES: the disjointness resolveProduction's BUG branch relies on.
func TestSameKeywordProductionsHaveDisjointArguments(t *testing.T) {
	samples := []string{"add", "delete", "replace", "not-supported", "/a", "/p:a/b", "a", "a/b", "p:a"}
	for keyword, productions := range rfc7950Grammar().keywords {
		if len(productions) < 2 {
			continue
		}
		for _, sample := range samples {
			matching := 0
			for _, production := range productions {
				if checkArgument(production.argument, sample) == nil {
					matching++
				}
			}
			assert.LessOrEqual(t, matching, 1, "%s %q matches %d productions", keyword, sample, matching)
		}
	}
}

// TestRequiredBlocksThatMayBeEmpty names the productions whose rule opens
// its block with a bare "{" although every statement the block names is
// optional: the only ones where "x;" and "x {}" differ, so the only ones
// statementHasBlock is asked about. Read from RFC 7950 Section 14, that is
// refine-stmt alone.
//
// VALIDATES: blockForm and emptyAccepted as parseYANGGrammar derives them.
func TestRequiredBlocksThatMayBeEmpty(t *testing.T) {
	var rules []string
	for rule, production := range rfc7950Grammar().productions {
		if production.form == blockRequired && production.block.emptyAccepted {
			rules = append(rules, rule)
		}
	}
	slices.Sort(rules)
	assert.Equal(t, []string{"refine-stmt"}, rules)
	grammar := rfc7950Grammar()
	assert.Equal(t, blockOptional, grammar.productions["config-stmt"].form, "stmtend")
	assert.Equal(t, blockOptional, grammar.productions["container-stmt"].form)
	assert.Equal(t, blockRequired, grammar.productions["leaf-stmt"].form)
	assert.False(t, grammar.productions["leaf-stmt"].block.emptyAccepted, "leaf-stmt requires type-stmt")
}

// TestOccurrencesFollowTheRepetition proves the bounds occurrences reads from
// the ABNF repetition: leaf-stmt's bare "type-stmt" is exactly one,
// "[description-stmt]" zero or one, "*must-stmt" any number, and
// enum-specification's "1*enum-stmt", through the alternatives of
// type-body-stmts, at most unbounded and at least zero.
//
// VALIDATES: yangGrammar.occurrences.
func TestOccurrencesFollowTheRepetition(t *testing.T) {
	grammar := rfc7950Grammar()
	leaf := grammar.productions["leaf-stmt"].block
	cases := []struct {
		block     *statementBlock
		rule      string
		low, high int
	}{
		{leaf, "type-stmt", 1, 1},
		{leaf, "description-stmt", 0, 1},
		{leaf, "must-stmt", 0, repeatUnbounded},
		{grammar.productions["type-stmt"].block, "enum-stmt", 0, repeatUnbounded},
		{grammar.productions["type-stmt"].block, "length-stmt", 0, 1},
		{grammar.productions["module-stmt"].block, "namespace-stmt", 1, 1},
	}
	for _, tc := range cases {
		low, high := grammar.occurrences(tc.block.content, grammar.productions[tc.rule])
		assert.Equal(t, tc.low, low, "%s in %s: fewest", tc.rule, tc.block.owner)
		assert.Equal(t, tc.high, high, "%s in %s: most", tc.rule, tc.block.owner)
	}
}
