package yang

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestArgumentCheckersFollowTheRFC7950Rules drives each Section 14 argument
// checker with arguments on both sides of its rule: the forms the ABNF admits
// and the nearest forms it does not (a leading zero, a wrong case, a missing
// part, a separator where none may stand). The rule text each case follows is
// quoted at the checker in loader_grammar.go.
//
// VALIDATES: argumentCheckers, which extensionSubstatementError applies.
func TestArgumentCheckersFollowTheRFC7950Rules(t *testing.T) {
	cases := []struct {
		rule     string
		accepted []string
		refused  []string
	}{
		{"identifier-arg-str", []string{"a", "_x", "a-b.c_9"}, []string{"", "9a", "-a", "a b", "p:a", "é"}},
		{"identifier-ref-arg-str", []string{"a", "p:a"}, []string{"", "p:", ":a", "p:a:b", "a b"}},
		{"revision-date", []string{"2016-08-01"}, []string{"2016-8-01", "2016/08/01", "2016-08-011", "abcd-ef-gh"}},
		{"config-arg-str", []string{"true", "false"}, []string{"maybe", "True", "", " true"}},
		{"yang-version-arg-str", []string{"1.1"}, []string{"1", "1.0"}},
		{"status-arg-str", []string{"current", "obsolete", "deprecated"}, []string{"Current", "old"}},
		{"ordered-by-arg-str", []string{"user", "system"}, []string{"any"}},
		{"modifier-arg-str", []string{"invert-match"}, []string{"invert"}},
		{"min-value-arg-str", []string{"0", "7", "10"}, []string{"", "-1", "07", "1.0"}},
		{"max-value-arg-str", []string{"unbounded", "1", "20"}, []string{"0", "01", "Unbounded"}},
		{"integer-value-str", []string{"0", "-5", "42"}, []string{"-", "+5", "05", "-05"}},
		{"fraction-digits-arg-str", []string{"1", "9", "10", "18"}, []string{"0", "19", "01", "100"}},
		{"length-arg-str", []string{"1", "min..max", "1..5 | 10..20", "0 |\n5"}, []string{"", " 1", "1..", "-1..5", "1..x", "1|", "1.5"}},
		{"range-arg-str", []string{"min..5 | 10..max", "-1.5..2.25", "-3"}, []string{"", "1.", "a..b", "1 ..", "5 "}},
		{"key-arg-str", []string{"a", "a b", "p:a\tq:b", "a\r\nb"}, []string{"", "a ", " a", "a\rb", "a/b"}},
		{"unique-arg-str", []string{"a/b c", "p:a/p:b"}, []string{"/a", "a//b", "a "}},
		{"augment-arg-str", []string{"/a", "/p:a/b"}, []string{"a", "/", "/a/", "//a"}},
		{"refine-arg-str", []string{"a", "a/b"}, []string{"/a", "a/"}},
		{"if-feature-expr-str", []string{"a", "p:a", "not a", "a and b or c", "(a or b) and not c", "( a )", "or", "orange or b"},
			[]string{"", "not(a)", "a or", "a and", "(a", "a)", "a  or(b)", "a oR b"}},
		{"uri-str", []string{"urn:m", "urn:ietf:params:xml:ns:yang:ietf-interfaces", "http://example.com/a/b?q=1#f",
			"http://u:p@[2001:db8::1]:830/", "http://[v1.x]/", "file:///etc/ze", "a+b-c.d:", "mailto:x@y", "urn:a%20b"},
			[]string{"", "m", "1urn:x", ":x", "urn:a b", "urn:%zz", "http://[::1", "http://[fe80::1%25eth0]/", "http://a:x/", "urn:a\"b", "urn:<x>"}},
		{"path-arg-str", []string{"/a", "/p:a/p:b", "../a", "../../a/b", "../a/b[k = current()/../k]",
			"/a[k=current()/../../k]/b", "/a[ k = current ( ) / .. / x / y ]", "../a[k=current()/../k]/b"},
			[]string{"", "a", "/", "/a/", "../", "..", "../a[k=current()/../k]", "/a[k]", "/a[k=../k]",
				"/a[k=current()/k]", "/a[k=current()/../]", "./a", "/a[k=current()/../k", "/a /b"}},
	}
	for _, tc := range cases {
		check, known := argumentCheckers()[tc.rule]
		if !assert.True(t, known, "argument rule %s has no checker", tc.rule) {
			continue
		}
		for _, argument := range tc.accepted {
			assert.NoError(t, check(argument), "%s must accept %q", tc.rule, argument)
		}
		for _, argument := range tc.refused {
			assert.Error(t, check(argument), "%s must refuse %q", tc.rule, argument)
		}
	}
}

// TestDeviateArgumentChoosesItsProduction proves that the four deviate
// productions of Section 14 each take the one argument their rule names, so
// "add", "delete", "replace" and "not-supported" each match exactly one of
// them and "remove" matches none.
//
// VALIDATES: checkArgument over the deviate productions, which
// resolveProduction chooses among.
func TestDeviateArgumentChoosesItsProduction(t *testing.T) {
	productions := rfc7950Grammar().keywords["deviate"]
	for _, argument := range []string{"add", "delete", "replace", "not-supported"} {
		matching := 0
		for _, production := range productions {
			if checkArgument(production.argument, argument) == nil {
				matching++
			}
		}
		assert.Equal(t, 1, matching, "deviate %q", argument)
	}
	for _, production := range productions {
		assert.Error(t, checkArgument(production.argument, "remove"), production.rule)
	}
}

// TestLengthSpanRefusesADescendingSpan proves newLengthSpan, the only
// constructor of lengthSpan, refuses a lower bound above the upper one and
// accepts a single value and an ascending span.
//
// VALIDATES: the lengthSpan construction invariant.
func TestLengthSpanRefusesADescendingSpan(t *testing.T) {
	_, err := newLengthSpan(5, 1)
	assert.Error(t, err)
	span, err := newLengthSpan(3, 3)
	assert.NoError(t, err)
	assert.Equal(t, lengthSpan{lower: 3, upper: 3}, span)
	_, err = newLengthSpan(0, ^uint64(0))
	assert.NoError(t, err)
}
